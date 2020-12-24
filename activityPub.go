package main

import (
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-fed/httpsig"
)

var (
	apPrivateKey    *rsa.PrivateKey
	apPostSigner    httpsig.Signer
	apPostSignMutex *sync.Mutex = &sync.Mutex{}
)

func initActivityPub() error {
	if !appConfig.ActivityPub.Enabled {
		return nil
	}
	// Add hooks
	postHooks[postPostHook] = append(postHooks[postPostHook], func(p *post) {
		if p.isPublishedSectionPost() {
			p.apPost()
		}
	})
	postHooks[postUpdateHook] = append(postHooks[postUpdateHook], func(p *post) {
		if p.isPublishedSectionPost() {
			p.apUpdate()
		}
	})
	postHooks[postDeleteHook] = append(postHooks[postDeleteHook], func(p *post) {
		p.apDelete()
	})
	// Read key and prepare signing
	pkfile, err := ioutil.ReadFile(appConfig.ActivityPub.KeyPath)
	if err != nil {
		return err
	}
	privateKeyDecoded, _ := pem.Decode(pkfile)
	if privateKeyDecoded == nil {
		return errors.New("failed to decode private key")
	}
	apPrivateKey, err = x509.ParsePKCS1PrivateKey(privateKeyDecoded.Bytes)
	if err != nil {
		return err
	}
	prefs := []httpsig.Algorithm{httpsig.RSA_SHA256}
	digestAlgorithm := httpsig.DigestSha256
	headersToSign := []string{httpsig.RequestTarget, "date", "host", "digest"}
	apPostSigner, _, err = httpsig.NewSigner(prefs, digestAlgorithm, headersToSign, httpsig.Signature, 0)
	if err != nil {
		return err
	}
	// Init send queue
	if err = initAPSendQueue(); err != nil {
		return err
	}
	return nil
}

func apHandleWebfinger(w http.ResponseWriter, r *http.Request) {
	re, err := regexp.Compile(`^acct:(.*)@` + regexp.QuoteMeta(appConfig.Server.publicHostname) + `$`)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	name := re.ReplaceAllString(r.URL.Query().Get("resource"), "$1")
	blog := appConfig.Blogs[name]
	if blog == nil {
		serveError(w, r, "Blog not found", http.StatusNotFound)
		return
	}
	w.Header().Set(contentType, "application/jrd+json"+charsetUtf8Suffix)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"subject": "acct:" + name + "@" + appConfig.Server.publicHostname,
		"links": []map[string]string{
			{
				"rel":  "self",
				"type": contentTypeAS,
				"href": blog.apIri(),
			},
		},
	})
}

func apHandleInbox(w http.ResponseWriter, r *http.Request) {
	blogName := chi.URLParam(r, "blog")
	blog := appConfig.Blogs[blogName]
	if blog == nil {
		serveError(w, r, "Inbox not found", http.StatusNotFound)
		return
	}
	blogIri := blog.apIri()
	// Verify request
	requestActor, requestKey, requestActorStatus, err := apVerifySignature(r)
	if err != nil {
		// Send 401 because signature could not be verified
		serveError(w, r, err.Error(), http.StatusUnauthorized)
		return
	}
	if requestActorStatus != 0 {
		if requestActorStatus == http.StatusGone || requestActorStatus == http.StatusNotFound {
			u, err := url.Parse(requestKey)
			if err == nil {
				u.Fragment = ""
				u.RawFragment = ""
				apRemoveFollower(blogName, u.String())
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		serveError(w, r, "Error when trying to get request actor", http.StatusBadRequest)
		return
	}
	// Parse activity
	activity := map[string]interface{}{}
	err = json.NewDecoder(r.Body).Decode(&activity)
	_ = r.Body.Close()
	if err != nil {
		serveError(w, r, "Failed to decode body", http.StatusBadRequest)
		return
	}
	// Get and check activity actor
	activityActor, ok := activity["actor"].(string)
	if !ok {
		serveError(w, r, "actor in activity is no string", http.StatusBadRequest)
		return
	}
	if activityActor != requestActor.ID {
		serveError(w, r, "Request actor isn't activity actor", http.StatusForbidden)
		return
	}
	// Do
	switch activity["type"] {
	case "Follow":
		apAccept(blogName, blog, activity)
	case "Undo":
		{
			if object, ok := activity["object"].(map[string]interface{}); ok {
				if objectType, ok := object["type"].(string); ok && objectType == "Follow" {
					if iri, ok := object["actor"].(string); ok && iri == activityActor {
						_ = apRemoveFollower(blogName, activityActor)
					}
				}
			}
		}
	case "Create":
		{
			if object, ok := activity["object"].(map[string]interface{}); ok {
				inReplyTo, hasReplyToString := object["inReplyTo"].(string)
				id, hasID := object["id"].(string)
				if hasReplyToString && hasID && len(inReplyTo) > 0 && len(id) > 0 && strings.Contains(inReplyTo, blogIri) {
					// It's an ActivityPub reply; save reply as webmention
					createWebmention(id, inReplyTo)
				} else if content, hasContent := object["content"].(string); hasContent && hasID && len(id) > 0 {
					// May be a mention; find links to blog and save them as webmentions
					if links, err := allLinksFromHTML(strings.NewReader(content), id); err == nil {
						for _, link := range links {
							if strings.Contains(link, blogIri) {
								createWebmention(id, link)
							}
						}
					}
				}
			}
		}
	case "Delete":
	case "Block":
		{
			if object, ok := activity["object"].(string); ok && len(object) > 0 && object == activityActor {
				_ = apRemoveFollower(blogName, activityActor)
			}
		}
	case "Like":
		{
			likeObject, likeObjectOk := activity["object"].(string)
			if likeObjectOk && len(likeObject) > 0 && strings.Contains(likeObject, blogIri) {
				sendNotification(fmt.Sprintf("%s liked %s", activityActor, likeObject))
			}
		}
	case "Announce":
		{
			announceObject, announceObjectOk := activity["object"].(string)
			if announceObjectOk && len(announceObject) > 0 && strings.Contains(announceObject, blogIri) {
				sendNotification(fmt.Sprintf("%s announced %s", activityActor, announceObject))
			}
		}
	}
	// Return 200
	w.WriteHeader(http.StatusOK)
}

func apVerifySignature(r *http.Request) (*asPerson, string, int, error) {
	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		// Error with signature header etc.
		return nil, "", 0, err
	}
	keyID := verifier.KeyId()
	actor, statusCode, err := apGetRemoteActor(keyID)
	if err != nil || actor == nil || statusCode != 0 {
		// Actor not found or something else bad
		return nil, keyID, statusCode, err
	}
	if actor.PublicKey == nil || actor.PublicKey.PublicKeyPem == "" {
		return nil, keyID, 0, errors.New("Actor has no public key")
	}
	block, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))
	if block == nil {
		return nil, keyID, 0, errors.New("Public key invalid")
	}
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Unable to parse public key
		return nil, keyID, 0, err
	}
	return actor, keyID, 0, verifier.Verify(pubKey, httpsig.RSA_SHA256)
}

func handleWellKnownHostMeta(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentType, "application/xrd+xml"+charsetUtf8Suffix)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0"><Link rel="lrdd" type="application/xrd+xml" template="https://` + r.Host + `/.well-known/webfinger?resource={uri}"/></XRD>`))
}

func apGetRemoteActor(iri string) (*asPerson, int, error) {
	req, err := http.NewRequest(http.MethodGet, iri, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", contentTypeAS)
	req.Header.Set(userAgent, appUserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	if !apRequestIsSuccess(resp.StatusCode) {
		return nil, resp.StatusCode, nil
	}
	actor := &asPerson{}
	err = json.NewDecoder(resp.Body).Decode(actor)
	defer resp.Body.Close()
	if err != nil {
		return nil, 0, err
	}
	return actor, 0, nil
}

func apGetAllInboxes(blog string) ([]string, error) {
	rows, err := appDbQuery("select distinct inbox from activitypub_followers where blog = @blog", sql.Named("blog", blog))
	if err != nil {
		return nil, err
	}
	inboxes := []string{}
	for rows.Next() {
		var inbox string
		err = rows.Scan(&inbox)
		if err != nil {
			return nil, err
		}
		inboxes = append(inboxes, inbox)
	}
	return inboxes, nil
}

func apAddFollower(blog, follower, inbox string) error {
	_, err := appDbExec("insert or replace into activitypub_followers (blog, follower, inbox) values (@blog, @follower, @inbox)", sql.Named("blog", blog), sql.Named("follower", follower), sql.Named("inbox", inbox))
	return err
}

func apRemoveFollower(blog, follower string) error {
	_, err := appDbExec("delete from activitypub_followers where blog = @blog and follower = @follower", sql.Named("blog", blog), sql.Named("follower", follower))
	return err
}

func apRemoveInbox(inbox string) error {
	_, err := appDbExec("delete from activitypub_followers where inbox = @inbox", sql.Named("inbox", inbox))
	return err
}

func (p *post) apPost() {
	n := p.toASNote()
	apSendToAllFollowers(p.Blog, map[string]interface{}{
		"@context":  asContext,
		"actor":     appConfig.Blogs[p.Blog].apIri(),
		"id":        p.fullURL(),
		"published": n.Published,
		"type":      "Create",
		"object":    n,
	})
	if n.InReplyTo != "" {
		// Is reply, so announce it
		time.Sleep(30 * time.Second)
		p.apAnnounce()
	}
}

func (p *post) apUpdate() {
	apSendToAllFollowers(p.Blog, map[string]interface{}{
		"@context":  asContext,
		"actor":     appConfig.Blogs[p.Blog].apIri(),
		"id":        p.fullURL(),
		"published": time.Now().Format("2006-01-02T15:04:05-07:00"),
		"type":      "Update",
		"object":    p.toASNote(),
	})
}

func (p *post) apAnnounce() {
	apSendToAllFollowers(p.Blog, map[string]interface{}{
		"@context":  asContext,
		"actor":     appConfig.Blogs[p.Blog].apIri(),
		"id":        p.fullURL() + "#announce",
		"published": p.toASNote().Published,
		"type":      "Announce",
		"object":    p.fullURL(),
	})
}

func (p *post) apDelete() {
	apSendToAllFollowers(p.Blog, map[string]interface{}{
		"@context": asContext,
		"actor":    appConfig.Blogs[p.Blog].apIri(),
		"id":       p.fullURL() + "#delete",
		"type":     "Delete",
		"object": map[string]string{
			"id":   p.fullURL(),
			"type": "Tombstone",
		},
	})
}

func apAccept(blogName string, blog *configBlog, follow map[string]interface{}) {
	// it's a follow, write it down
	newFollower := follow["actor"].(string)
	log.Println("New follow request:", newFollower)
	// check we aren't following ourselves
	if newFollower == follow["object"] {
		// actor and object are equal
		return
	}
	follower, status, err := apGetRemoteActor(newFollower)
	if err != nil || status != 0 {
		// Couldn't retrieve remote actor info
		log.Println("Failed to retrieve remote actor info:", newFollower)
		return
	}
	// Add or update follower
	inbox := follower.Inbox
	if endpoints := follower.Endpoints; endpoints != nil && endpoints.SharedInbox != "" {
		inbox = endpoints.SharedInbox
	}
	apAddFollower(blogName, follower.ID, inbox)
	// remove @context from the inner activity
	delete(follow, "@context")
	accept := map[string]interface{}{
		"@context": asContext,
		"to":       follow["actor"],
		"actor":    blog.apIri(),
		"object":   follow,
		"type":     "Accept",
	}
	_, accept["id"] = apNewID(blog)
	apQueueSendSigned(blog.apIri(), follower.Inbox, accept)
}

func apSendToAllFollowers(blog string, activity interface{}) {
	inboxes, err := apGetAllInboxes(blog)
	if err != nil {
		log.Println("Failed to retrieve inboxes:", err.Error())
		return
	}
	apSendTo(appConfig.Blogs[blog].apIri(), activity, inboxes)
}

func apSendTo(blogIri string, activity interface{}, inboxes []string) {
	for _, i := range inboxes {
		go func(inbox string) {
			apQueueSendSigned(blogIri, inbox, activity)
		}(i)
	}
}

func apNewID(blog *configBlog) (hash string, url string) {
	return hash, blog.apIri() + generateRandomString(16)
}

func (b *configBlog) apIri() string {
	return appConfig.Server.PublicAddress + b.Path
}

func apRequestIsSuccess(code int) bool {
	return code == http.StatusOK || code == http.StatusCreated || code == http.StatusAccepted || code == http.StatusNoContent
}
