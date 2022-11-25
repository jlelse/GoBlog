package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	ap "github.com/go-ap/activitypub"
	"github.com/go-chi/chi/v5"
	"github.com/go-fed/httpsig"
	"github.com/google/uuid"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) initActivityPub() error {
	if !a.apEnabled() {
		// ActivityPub disabled
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
	a.pUndeleteHooks = append(a.pUndeleteHooks, func(p *post) {
		if p.isPublishedSectionPost() {
			a.apUndelete(p)
		}
	})
	// Prepare webfinger
	a.prepareWebfinger()
	// Read key and prepare signing
	err := a.loadActivityPubPrivateKey()
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
	// Send profile updates
	go func() {
		// First wait a bit
		time.Sleep(time.Second * 10)
		// Then send profile update
		a.apSendProfileUpdates()
	}()
	return nil
}

func (a *goBlog) apEnabled() bool {
	if a.isPrivate() {
		// Private mode, no AP
		return false
	}
	if apc := a.cfg.ActivityPub; apc == nil || !apc.Enabled {
		// Disabled
		return false
	}
	return true
}

func (a *goBlog) prepareWebfinger() {
	a.webfingerResources = map[string]*configBlog{}
	a.webfingerAccts = map[string]string{}
	for name, blog := range a.cfg.Blogs {
		acct := "acct:" + name + "@" + a.cfg.Server.publicHostname
		a.webfingerResources[acct] = blog
		a.webfingerResources[a.apIri(blog)] = blog
		a.webfingerAccts[a.apIri(blog)] = acct
	}
}

func (a *goBlog) apHandleWebfinger(w http.ResponseWriter, r *http.Request) {
	blog, ok := a.webfingerResources[r.URL.Query().Get("resource")]
	if !ok {
		a.serveError(w, r, "Resource not found", http.StatusNotFound)
		return
	}
	apIri := a.apIri(blog)
	// Encode
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	if err := json.NewEncoder(buf).Encode(map[string]any{
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
	}); err != nil {
		a.serveError(w, r, "Encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, "application/jrd+json"+contenttype.CharsetUtf8Suffix)
	_ = a.min.Get().Minify(contenttype.JSON, w, buf)
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
	requestActor, requestKey, requestActorStatus, err := a.apVerifySignature(r, blogIri)
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
	limit := int64(10 * 1000 * 1000) // 10 MB
	body, err := io.ReadAll(io.LimitReader(r.Body, limit))
	if err != nil {
		a.serveError(w, r, "Failed to read body", http.StatusBadRequest)
		return
	}
	apItem, err := ap.UnmarshalJSON(body)
	if err != nil {
		a.serveError(w, r, "Failed to decode body", http.StatusBadRequest)
		return
	}
	// Check if it's an activity
	activity, err := ap.ToActivity(apItem)
	if err != nil {
		a.serveError(w, r, "No activity", http.StatusBadRequest)
		return
	}
	// Check actor
	activityActor := activity.Actor.GetID()
	if activity.Actor == nil || (!activity.Actor.IsLink() && !activity.Actor.IsObject()) {
		a.serveError(w, r, "Activity has no actor", http.StatusBadRequest)
		return
	}
	if activityActor != requestActor.GetID() {
		a.serveError(w, r, "Request actor isn't activity actor", http.StatusForbidden)
		return
	}
	// Handle activity
	switch activity.GetType() {
	case ap.FollowType:
		a.apAccept(blogName, blogIri, blog, activity)
	case ap.UndoType:
		if activity.Object.IsObject() {
			objectActivity, err := ap.ToActivity(activity.Object)
			if err == nil && objectActivity.GetType() == ap.FollowType && objectActivity.Actor.GetID() == activityActor {
				_ = a.db.apRemoveFollower(blogName, activityActor.String())
			}
		}
	case ap.CreateType, ap.UpdateType:
		if activity.Object.IsObject() {
			a.apOnCreateUpdate(blog, requestActor, activity)
		}
	case ap.DeleteType, ap.BlockType:
		if activity.Object.GetID() == activityActor {
			_ = a.db.apRemoveFollower(blogName, activityActor.String())
		} else {
			// Check if comment exists
			exists, commentId, err := a.db.commentIdByOriginal(activity.Object.GetID().String())
			if err == nil && exists {
				_ = a.db.deleteComment(commentId)
				_ = a.db.deleteWebmentionUUrl(activity.Object.GetID().String())
			}
		}
	case ap.AnnounceType:
		a.sendNotification(fmt.Sprintf("%s announced %s", activityActor, activity.Object.GetID()))
	case ap.LikeType:
		a.sendNotification(fmt.Sprintf("%s liked %s", activityActor, activity.Object.GetID()))
	}
	// Return 200
	w.WriteHeader(http.StatusOK)
}

func (a *goBlog) apOnCreateUpdate(blog *configBlog, requestActor *ap.Actor, activity *ap.Activity) {
	object, err := ap.ToObject(activity.Object)
	if err != nil {
		return
	}
	if object.GetType() != ap.NoteType && object.GetType() != ap.ArticleType {
		// ignore other objects for now
		return
	}
	visible := true
	if !object.To.Contains(ap.PublicNS) && !object.CC.Contains(ap.PublicNS) {
		visible = false
	}
	if replyTarget := object.InReplyTo.GetID().String(); visible && replyTarget != "" && strings.HasPrefix(replyTarget, a.cfg.Server.PublicAddress) {
		// It's a reply
		original := object.GetID().String()
		name := requestActor.Name.First().Value.String()
		if username := requestActor.PreferredUsername.First().String(); name == "" && username != "" {
			name = username
		}
		website := requestActor.GetLink().String()
		if actorUrl := requestActor.URL.GetLink(); actorUrl != "" {
			website = actorUrl.String()
		}
		content := object.Content.First().Value.String()
		_, _, _ = a.createComment(blog, replyTarget, content, name, website, original)
		return
	}
	// Might be a private reply or mention etc.
	// TODO: handle them
}

func (a *goBlog) apVerifySignature(r *http.Request, blogIri string) (*ap.Actor, string, int, error) {
	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		// Error with signature header etc.
		return nil, "", 0, err
	}
	keyID := verifier.KeyId()
	actor, statusCode, err := a.apGetRemoteActor(keyID, blogIri)
	if err != nil || actor == nil || statusCode != 0 {
		// Actor not found or something else bad
		return nil, keyID, statusCode, err
	}
	if actor.PublicKey.PublicKeyPem == "" {
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
	_, _ = io.WriteString(w, xml.Header)
	_, _ = io.WriteString(w, `<XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0"><Link rel="lrdd" type="application/xrd+xml" template="https://`+r.Host+`/.well-known/webfinger?resource={uri}"/></XRD>`)
}

func (a *goBlog) apGetFollowersCollectionId(blogName string, blog *configBlog) ap.IRI {
	return ap.IRI(a.apIri(blog) + "/activitypub/followers/" + blogName)
}

func (a *goBlog) apShowFollowers(w http.ResponseWriter, r *http.Request) {
	blogName := chi.URLParam(r, "blog")
	blog, ok := a.cfg.Blogs[blogName]
	if !ok || blog == nil {
		a.serveError(w, r, "Blog not found", http.StatusNotFound)
		return
	}
	followers, err := a.db.apGetAllFollowers(blogName)
	if err != nil {
		a.serveError(w, r, "Failed to get followers", http.StatusInternalServerError)
		return
	}
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		followersCollection := ap.CollectionNew(a.apGetFollowersCollectionId(blogName, blog))
		for _, follower := range followers {
			followersCollection.Items.Append(ap.IRI(follower.follower))
		}
		followersCollection.TotalItems = uint(len(followers))
		a.serveAPItem(followersCollection, w, r)
		return
	}
	a.render(w, r, a.renderActivityPubFollowers, &renderData{
		BlogString: blogName,
		Data: &activityPubFollowersRenderData{
			apUser:    fmt.Sprintf("@%s@%s", blogName, a.cfg.Server.publicHostname),
			followers: followers,
		},
	})
}

func (a *goBlog) apGetRemoteActor(iri, ownBlogIri string) (*ap.Actor, int, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, iri, strings.NewReader(""))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", contenttype.AS)
	req.Header.Set(userAgent, appUserAgent)
	// Sign request
	req.Header.Set("Date", time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05")+" GMT")
	req.Header.Set("Host", req.URL.Host)
	a.apPostSignMutex.Lock()
	err = a.apPostSigner.SignRequest(a.apPrivateKey, ownBlogIri+"#main-key", req, []byte(""))
	a.apPostSignMutex.Unlock()
	if err != nil {
		return nil, 0, err
	}
	// Do request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if !apRequestIsSuccess(resp.StatusCode) {
		return nil, resp.StatusCode, nil
	}
	// Parse response
	limit := int64(10 * 1000 * 1000) // 10 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, 0, err
	}
	apObject, err := ap.UnmarshalJSON(body)
	if err != nil {
		return nil, 0, err
	}
	actor, err := ap.ToActor(apObject)
	if err != nil {
		return nil, 0, err
	}
	return actor, 0, nil
}

func (db *database) apGetAllInboxes(blog string) (inboxes []string, err error) {
	rows, err := db.Query("select distinct inbox from activitypub_followers where blog = @blog", sql.Named("blog", blog))
	if err != nil {
		return nil, err
	}
	var inbox string
	for rows.Next() {
		err = rows.Scan(&inbox)
		if err != nil {
			return nil, err
		}
		inboxes = append(inboxes, inbox)
	}
	return inboxes, nil
}

type apFollower struct {
	follower, inbox string
}

func (db *database) apGetAllFollowers(blog string) (followers []*apFollower, err error) {
	rows, err := db.Query("select follower, inbox from activitypub_followers where blog = @blog", sql.Named("blog", blog))
	if err != nil {
		return nil, err
	}
	var follower, inbox string
	for rows.Next() {
		err = rows.Scan(&follower, &inbox)
		if err != nil {
			return nil, err
		}
		followers = append(followers, &apFollower{follower: follower, inbox: inbox})
	}
	return followers, nil
}

func (db *database) apAddFollower(blog, follower, inbox string) error {
	_, err := db.Exec("insert or replace into activitypub_followers (blog, follower, inbox) values (@blog, @follower, @inbox)", sql.Named("blog", blog), sql.Named("follower", follower), sql.Named("inbox", inbox))
	return err
}

func (db *database) apRemoveFollower(blog, follower string) error {
	_, err := db.Exec("delete from activitypub_followers where blog = @blog and follower = @follower", sql.Named("blog", blog), sql.Named("follower", follower))
	return err
}

func (db *database) apRemoveInbox(inbox string) error {
	_, err := db.Exec("delete from activitypub_followers where inbox = @inbox", sql.Named("inbox", inbox))
	return err
}

func (a *goBlog) apPost(p *post) {
	blogConfig := a.cfg.Blogs[p.Blog]
	note := a.toAPNote(p)
	create := ap.CreateNew(a.activityPubId(p), note)
	create.Actor = a.apAPIri(blogConfig)
	create.Published = time.Now()
	a.apSendToAllFollowers(p.Blog, create)
}

func (a *goBlog) apUpdate(p *post) {
	blogConfig := a.cfg.Blogs[p.Blog]
	note := a.toAPNote(p)
	update := ap.UpdateNew(a.activityPubId(p), note)
	update.Actor = a.apAPIri(blogConfig)
	update.Published = time.Now()
	a.apSendToAllFollowers(p.Blog, update)
}

func (a *goBlog) apDelete(p *post) {
	blogConfig := a.cfg.Blogs[p.Blog]
	delete := ap.DeleteNew(a.apNewID(blogConfig), a.activityPubId(p))
	delete.Actor = a.apAPIri(blogConfig)
	a.apSendToAllFollowers(p.Blog, delete)
}

func (a *goBlog) apUndelete(p *post) {
	// The optimal way to do this would be to send a "Undo Delete" activity,
	// but that doesn't work with Mastodon yet.
	// see:
	// https://socialhub.activitypub.rocks/t/soft-deletes-and-restoring-deleted-posts/2318
	// https://github.com/mastodon/mastodon/issues/17553

	// Update "activityPubVersion" parameter to current timestamp in nanoseconds
	p.Parameters[activityPubVersionParam] = []string{fmt.Sprintf("%d", utcNowNanos())}
	_ = a.db.replacePostParam(p.Path, activityPubVersionParam, p.Parameters[activityPubVersionParam])
	// Post as new post
	a.apPost(p)
}

func (a *goBlog) apAccept(blogName, blogIri string, blog *configBlog, follow *ap.Activity) {
	newFollower := follow.Actor.GetID()
	log.Println("New follow request:", newFollower.String())
	// Get remote actor
	follower, status, err := a.apGetRemoteActor(newFollower.String(), blogIri)
	if err != nil || status != 0 {
		// Couldn't retrieve remote actor info
		log.Println("Failed to retrieve remote actor info:", newFollower)
		return
	}
	// Add or update follower
	inbox := follower.Inbox.GetID()
	if endpoints := follower.Endpoints; endpoints != nil && endpoints.SharedInbox != nil && endpoints.SharedInbox.GetID() != "" {
		inbox = endpoints.SharedInbox.GetID()
	}
	if inbox == "" {
		return
	}
	if err = a.db.apAddFollower(blogName, follower.GetID().String(), inbox.String()); err != nil {
		return
	}
	// Send accept response to the new follower
	accept := ap.AcceptNew(a.apNewID(blog), follow)
	accept.To = append(accept.To, newFollower)
	accept.Actor = a.apAPIri(blog)
	_ = a.apQueueSendSigned(a.apIri(blog), inbox.String(), accept)
}

func (a *goBlog) apSendProfileUpdates() {
	for blog, config := range a.cfg.Blogs {
		person := a.toApPerson(blog)
		update := ap.UpdateNew(a.apNewID(config), person)
		update.Actor = a.apAPIri(config)
		update.Published = time.Now()
		a.apSendToAllFollowers(blog, update)
	}
}

func (a *goBlog) apSendToAllFollowers(blog string, activity *ap.Activity) {
	inboxes, err := a.db.apGetAllInboxes(blog)
	if err != nil {
		log.Println("Failed to retrieve inboxes:", err.Error())
		return
	}
	a.apSendTo(a.apIri(a.cfg.Blogs[blog]), activity, inboxes)
}

func (a *goBlog) apSendTo(blogIri string, activity *ap.Activity, inboxes []string) {
	for _, i := range inboxes {
		go func(inbox string) {
			_ = a.apQueueSendSigned(blogIri, inbox, activity)
		}(i)
	}
}

func (a *goBlog) apNewID(blog *configBlog) ap.ID {
	return ap.ID(a.apIri(blog) + "#" + uuid.NewString())
}

func (a *goBlog) apIri(b *configBlog) string {
	return a.getFullAddress(b.getRelativePath(""))
}

func (a *goBlog) apAPIri(b *configBlog) ap.IRI {
	return ap.IRI(a.apIri(b))
}

func apRequestIsSuccess(code int) bool {
	return code == http.StatusOK || code == http.StatusCreated || code == http.StatusAccepted || code == http.StatusNoContent
}

// Load or generate key for ActivityPub communication
func (a *goBlog) loadActivityPubPrivateKey() error {
	// Check if already loaded
	if a.apPrivateKey != nil {
		return nil
	}
	// Check if already generated
	if keyData, err := a.db.retrievePersistentCache("activitypub_key"); err == nil && keyData != nil {
		privateKeyDecoded, _ := pem.Decode(keyData)
		if privateKeyDecoded == nil {
			log.Println("failed to decode cached private key")
			// continue
		} else {
			key, err := x509.ParsePKCS1PrivateKey(privateKeyDecoded.Bytes)
			if err != nil {
				return err
			}
			pubKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
			if err != nil {
				return err
			}
			a.apPrivateKey = key
			a.apPubKeyBytes = pubKeyBytes
			return nil
		}
	}
	// Generate and cache key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	a.apPrivateKey = key
	a.apPubKeyBytes = pubKeyBytes
	return a.db.cachePersistently(
		"activitypub_key",
		pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(a.apPrivateKey),
		}),
	)
}
