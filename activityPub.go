package main

import (
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"git.jlel.se/jlelse/GoBlog/pkgs/contenttype"
	"github.com/go-chi/chi/v5"
	"github.com/go-fed/httpsig"
	"github.com/spf13/cast"
)

func (a *goBlog) initActivityPub() error {
	if !a.cfg.ActivityPub.Enabled {
		return nil
	}
	// Add hooks
	a.pPostHooks = append(a.pPostHooks, func(p *post) {
		if p.isPublishedSectionPost() {
			a.apPost(p)
		}
	})
	a.pUpdateHooks = append(a.pUpdateHooks, func(p *post) {
		if p.isPublishedSectionPost() {
			a.apUpdate(p)
		}
	})
	a.pDeleteHooks = append(a.pDeleteHooks, func(p *post) {
		a.apDelete(p)
	})
	// Prepare webfinger
	a.webfingerResources = map[string]*configBlog{}
	a.webfingerAccts = map[string]string{}
	for name, blog := range a.cfg.Blogs {
		acct := "acct:" + name + "@" + a.cfg.Server.publicHostname
		a.webfingerResources[acct] = blog
		a.webfingerResources[a.apIri(blog)] = blog
		a.webfingerAccts[a.apIri(blog)] = acct
	}
	// Read key and prepare signing
	pkfile, err := os.ReadFile(a.cfg.ActivityPub.KeyPath)
	if err != nil {
		return err
	}
	privateKeyDecoded, _ := pem.Decode(pkfile)
	if privateKeyDecoded == nil {
		return errors.New("failed to decode private key")
	}
	a.apPrivateKey, err = x509.ParsePKCS1PrivateKey(privateKeyDecoded.Bytes)
	if err != nil {
		return err
	}
	a.apPostSigner, _, err = httpsig.NewSigner(
		[]httpsig.Algorithm{httpsig.RSA_SHA256},
		httpsig.DigestSha256,
		[]string{httpsig.RequestTarget, "date", "host", "digest"},
		httpsig.Signature,
		0,
	)
	if err != nil {
		return err
	}
	// Init send queue
	a.initAPSendQueue()
	return nil
}

func (a *goBlog) apHandleWebfinger(w http.ResponseWriter, r *http.Request) {
	blog, ok := a.webfingerResources[r.URL.Query().Get("resource")]
	if !ok {
		a.serveError(w, r, "Resource not found", http.StatusNotFound)
		return
	}
	apIri := a.apIri(blog)
	b, _ := json.Marshal(map[string]interface{}{
		"subject": a.webfingerAccts[apIri],
		"aliases": []string{
			a.webfingerAccts[apIri],
			apIri,
		},
		"links": []map[string]string{
			{
				"rel":  "self",
				"type": contenttype.AS,
				"href": apIri,
			},
			{
				"rel":  "http://webfinger.net/rel/profile-page",
				"type": "text/html",
				"href": apIri,
			},
		},
	})
	w.Header().Set(contentType, "application/jrd+json"+contenttype.CharsetUtf8Suffix)
	_, _ = a.min.Write(w, contenttype.JSON, b)
}

func (a *goBlog) apHandleInbox(w http.ResponseWriter, r *http.Request) {
	blogName := chi.URLParam(r, "blog")
	blog, ok := a.cfg.Blogs[blogName]
	if !ok || blog == nil {
		a.serveError(w, r, "Inbox not found", http.StatusNotFound)
		return
	}
	blogIri := a.apIri(blog)
	// Verify request
	requestActor, requestKey, requestActorStatus, err := a.apVerifySignature(r)
	if err != nil {
		// Send 401 because signature could not be verified
		a.serveError(w, r, err.Error(), http.StatusUnauthorized)
		return
	}
	if requestActorStatus != 0 {
		if requestActorStatus == http.StatusGone || requestActorStatus == http.StatusNotFound {
			u, err := url.Parse(requestKey)
			if err == nil {
				u.Fragment = ""
				u.RawFragment = ""
				_ = a.db.apRemoveFollower(blogName, u.String())
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		a.serveError(w, r, "Error when trying to get request actor", http.StatusBadRequest)
		return
	}
	// Parse activity
	activity := map[string]interface{}{}
	err = json.NewDecoder(r.Body).Decode(&activity)
	_ = r.Body.Close()
	if err != nil {
		a.serveError(w, r, "Failed to decode body", http.StatusBadRequest)
		return
	}
	// Get and check activity actor
	activityActor, ok := activity["actor"].(string)
	if !ok {
		a.serveError(w, r, "actor in activity is no string", http.StatusBadRequest)
		return
	}
	if activityActor != requestActor.ID {
		a.serveError(w, r, "Request actor isn't activity actor", http.StatusForbidden)
		return
	}
	// Do
	switch activity["type"] {
	case "Follow":
		a.apAccept(blogName, blog, activity)
	case "Undo":
		{
			if object, ok := activity["object"].(map[string]interface{}); ok {
				ot := cast.ToString(object["type"])
				actor := cast.ToString(object["actor"])
				if ot == "Follow" && actor == activityActor {
					_ = a.db.apRemoveFollower(blogName, activityActor)
				}
			}
		}
	case "Create":
		{
			if object, ok := activity["object"].(map[string]interface{}); ok {
				baseUrl := cast.ToString(object["id"])
				if ou := cast.ToString(object["url"]); ou != "" {
					baseUrl = ou
				}
				if r := cast.ToString(object["inReplyTo"]); r != "" && baseUrl != "" && strings.HasPrefix(r, blogIri) {
					// It's an ActivityPub reply; save reply as webmention
					_ = a.createWebmention(baseUrl, r)
				} else if content := cast.ToString(object["content"]); content != "" && baseUrl != "" {
					// May be a mention; find links to blog and save them as webmentions
					if links, err := allLinksFromHTMLString(content, baseUrl); err == nil {
						for _, link := range links {
							if strings.HasPrefix(link, blogIri) {
								_ = a.createWebmention(baseUrl, link)
							}
						}
					}
				}
			}
		}
	case "Delete", "Block":
		{
			if o := cast.ToString(activity["object"]); o == activityActor {
				_ = a.db.apRemoveFollower(blogName, activityActor)
			}
		}
	case "Like":
		{
			if o := cast.ToString(activity["object"]); o != "" && strings.HasPrefix(o, blogIri) {
				a.sendNotification(fmt.Sprintf("%s liked %s", activityActor, o))
			}
		}
	case "Announce":
		{
			if o := cast.ToString(activity["object"]); o != "" && strings.HasPrefix(o, blogIri) {
				a.sendNotification(fmt.Sprintf("%s announced %s", activityActor, o))
			}
		}
	}
	// Return 200
	w.WriteHeader(http.StatusOK)
}

func (a *goBlog) apVerifySignature(r *http.Request) (*asPerson, string, int, error) {
	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		// Error with signature header etc.
		return nil, "", 0, err
	}
	keyID := verifier.KeyId()
	actor, statusCode, err := a.apGetRemoteActor(keyID)
	if err != nil || actor == nil || statusCode != 0 {
		// Actor not found or something else bad
		return nil, keyID, statusCode, err
	}
	if actor.PublicKey == nil || actor.PublicKey.PublicKeyPem == "" {
		return nil, keyID, 0, errors.New("actor has no public key")
	}
	block, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))
	if block == nil {
		return nil, keyID, 0, errors.New("public key invalid")
	}
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Unable to parse public key
		return nil, keyID, 0, err
	}
	return actor, keyID, 0, verifier.Verify(pubKey, httpsig.RSA_SHA256)
}

func handleWellKnownHostMeta(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentType, "application/xrd+xml"+contenttype.CharsetUtf8Suffix)
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0"><Link rel="lrdd" type="application/xrd+xml" template="https://` + r.Host + `/.well-known/webfinger?resource={uri}"/></XRD>`))
}

func (a *goBlog) apGetRemoteActor(iri string) (*asPerson, int, error) {
	req, err := http.NewRequest(http.MethodGet, iri, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", contenttype.AS)
	req.Header.Set(userAgent, appUserAgent)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if !apRequestIsSuccess(resp.StatusCode) {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, resp.StatusCode, nil
	}
	actor := &asPerson{}
	err = json.NewDecoder(resp.Body).Decode(actor)
	if err != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, 0, err
	}
	return actor, 0, nil
}

func (db *database) apGetAllInboxes(blog string) ([]string, error) {
	rows, err := db.query("select distinct inbox from activitypub_followers where blog = @blog", sql.Named("blog", blog))
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

func (db *database) apAddFollower(blog, follower, inbox string) error {
	_, err := db.exec("insert or replace into activitypub_followers (blog, follower, inbox) values (@blog, @follower, @inbox)", sql.Named("blog", blog), sql.Named("follower", follower), sql.Named("inbox", inbox))
	return err
}

func (db *database) apRemoveFollower(blog, follower string) error {
	_, err := db.exec("delete from activitypub_followers where blog = @blog and follower = @follower", sql.Named("blog", blog), sql.Named("follower", follower))
	return err
}

func (db *database) apRemoveInbox(inbox string) error {
	_, err := db.exec("delete from activitypub_followers where inbox = @inbox", sql.Named("inbox", inbox))
	return err
}

func (a *goBlog) apPost(p *post) {
	n := a.toASNote(p)
	a.apSendToAllFollowers(p.Blog, map[string]interface{}{
		"@context":  []string{asContext},
		"actor":     a.apIri(a.cfg.Blogs[p.Blog]),
		"id":        a.fullPostURL(p),
		"published": n.Published,
		"type":      "Create",
		"object":    n,
	})
	if n.InReplyTo != "" {
		// Is reply, so announce it
		time.Sleep(30 * time.Second)
		a.apAnnounce(p)
	}
}

func (a *goBlog) apUpdate(p *post) {
	a.apSendToAllFollowers(p.Blog, map[string]interface{}{
		"@context":  []string{asContext},
		"actor":     a.apIri(a.cfg.Blogs[p.Blog]),
		"id":        a.fullPostURL(p),
		"published": time.Now().Format("2006-01-02T15:04:05-07:00"),
		"type":      "Update",
		"object":    a.toASNote(p),
	})
}

func (a *goBlog) apAnnounce(p *post) {
	a.apSendToAllFollowers(p.Blog, map[string]interface{}{
		"@context":  []string{asContext},
		"actor":     a.apIri(a.cfg.Blogs[p.Blog]),
		"id":        a.fullPostURL(p) + "#announce",
		"published": a.toASNote(p).Published,
		"type":      "Announce",
		"object":    a.fullPostURL(p),
	})
}

func (a *goBlog) apDelete(p *post) {
	a.apSendToAllFollowers(p.Blog, map[string]interface{}{
		"@context": []string{asContext},
		"actor":    a.apIri(a.cfg.Blogs[p.Blog]),
		"id":       a.fullPostURL(p) + "#delete",
		"type":     "Delete",
		"object": map[string]string{
			"id":   a.fullPostURL(p),
			"type": "Tombstone",
		},
	})
}

func (a *goBlog) apAccept(blogName string, blog *configBlog, follow map[string]interface{}) {
	// it's a follow, write it down
	newFollower := follow["actor"].(string)
	log.Println("New follow request:", newFollower)
	// check we aren't following ourselves
	if newFollower == follow["object"] {
		// actor and object are equal
		return
	}
	follower, status, err := a.apGetRemoteActor(newFollower)
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
	if err = a.db.apAddFollower(blogName, follower.ID, inbox); err != nil {
		return
	}
	// remove @context from the inner activity
	delete(follow, "@context")
	accept := map[string]interface{}{
		"@context": []string{asContext},
		"to":       follow["actor"],
		"actor":    a.apIri(blog),
		"object":   follow,
		"type":     "Accept",
	}
	_, accept["id"] = a.apNewID(blog)
	_ = a.db.apQueueSendSigned(a.apIri(blog), follower.Inbox, accept)
}

func (a *goBlog) apSendToAllFollowers(blog string, activity interface{}) {
	inboxes, err := a.db.apGetAllInboxes(blog)
	if err != nil {
		log.Println("Failed to retrieve inboxes:", err.Error())
		return
	}
	a.db.apSendTo(a.apIri(a.cfg.Blogs[blog]), activity, inboxes)
}

func (db *database) apSendTo(blogIri string, activity interface{}, inboxes []string) {
	for _, i := range inboxes {
		go func(inbox string) {
			_ = db.apQueueSendSigned(blogIri, inbox, activity)
		}(i)
	}
}

func (a *goBlog) apNewID(blog *configBlog) (hash string, url string) {
	return hash, a.apIri(blog) + generateRandomString(16)
}

func (a *goBlog) apIri(b *configBlog) string {
	return a.getFullAddress(b.getRelativePath(""))
}

func apRequestIsSuccess(code int) bool {
	return code == http.StatusOK || code == http.StatusCreated || code == http.StatusAccepted || code == http.StatusNoContent
}
