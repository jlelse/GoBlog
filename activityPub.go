package main

import (
	"bytes"
	"cmp"
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
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-fed/httpsig"
	"github.com/google/uuid"
	"github.com/samber/lo"
	ap "go.goblog.app/app/pkgs/activitypub"
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
		if p.isPublishedSectionPost() && (p.Visibility == visibilityPublic || p.Visibility == visibilityUnlisted) {
			a.apCheckMentions(p)
			a.apCheckActivityPubReply(p)
			a.apPost(p)
		}
	})
	a.pUpdateHooks = append(a.pUpdateHooks, func(p *post) {
		if p.isPublishedSectionPost() && (p.Visibility == visibilityPublic || p.Visibility == visibilityUnlisted) {
			a.apCheckMentions(p)
			a.apCheckActivityPubReply(p)
			a.apUpdate(p)
		}
	})
	a.pDeleteHooks = append(a.pDeleteHooks, func(p *post) {
		a.apDelete(p)
	})
	a.pUndeleteHooks = append(a.pUndeleteHooks, func(p *post) {
		if p.isPublishedSectionPost() && (p.Visibility == visibilityPublic || p.Visibility == visibilityUnlisted) {
			a.apUndelete(p)
		}
	})
	// Prepare webfinger
	a.prepareWebfinger()
	// Init base
	if err := a.initActivityPubBase(); err != nil {
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

func (a *goBlog) initActivityPubBase() error {
	// Read key and prepare signing
	err := a.loadActivityPubPrivateKey()
	if err != nil {
		return err
	}
	a.apSigner, _, err = httpsig.NewSigner(
		[]httpsig.Algorithm{httpsig.RSA_SHA256},
		httpsig.DigestSha256,
		[]string{httpsig.RequestTarget, "date", "host", "digest"},
		httpsig.Signature,
		0,
	)
	if err != nil {
		return err
	}
	// Init http client
	a.apHttpClients = map[string]*http.Client{}
	for blog, bc := range a.cfg.Blogs {
		httpClient := cloneHttpClient(a.httpClient)
		httpClient.Transport = a.newactivityPubSignRequestTransport(httpClient.Transport, string(a.apAPIri(bc)))
		a.apHttpClients[blog] = httpClient
	}
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
	a.apUserHandle = map[string]string{}
	a.webfingerResources = map[string]*configBlog{}
	a.webfingerAccts = map[string]string{}
	for name, blog := range a.cfg.Blogs {
		a.apUserHandle[name] = "@" + name + "@" + a.cfg.Server.publicHostname
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
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(json.NewEncoder(pw).Encode(map[string]any{
			"subject": a.webfingerAccts[apIri],
			"aliases": []string{a.webfingerAccts[apIri], apIri},
			"links": []map[string]string{
				{
					"rel": "self", "type": contenttype.AS, "href": apIri,
				},
				{
					"rel":  "http://webfinger.net/rel/profile-page",
					"type": "text/html", "href": apIri,
				},
			},
		}))
	}()
	w.Header().Set(contentType, "application/jrd+json"+contenttype.CharsetUtf8Suffix)
	_ = pr.CloseWithError(a.min.Get().Minify(contenttype.JSON, w, pr))
}

const activityPubMentionsParameter = "activitypubmentions"

func (a *goBlog) apCheckMentions(p *post) {
	pr, pw := io.Pipe()
	go func() {
		a.postHtmlToWriter(pw, &postHtmlOptions{p: p})
		_ = pw.Close()
	}()
	links, err := allLinksFromHTML(pr, a.fullPostURL(p))
	_ = pr.CloseWithError(err)
	if err != nil {
		a.error("ActivityPub: Failed to extract links from post", err)
		return
	}
	mentions := []string{}
	for _, link := range links {
		act, err := a.apGetRemoteActor(p.Blog, ap.IRI(link))
		if err != nil || act == nil || act.Type != ap.PersonType {
			continue
		}
		mentions = append(mentions, link)
	}
	if p.Parameters == nil {
		p.Parameters = map[string][]string{}
	}
	p.Parameters[activityPubMentionsParameter] = mentions
	_ = a.db.replacePostParam(p.Path, activityPubMentionsParameter, mentions)
}

const activityPubReplyActorParameter = "activitypubreplyactor"

func (a *goBlog) apCheckActivityPubReply(p *post) {
	replyLink := a.replyLink(p)
	if replyLink == "" {
		return
	}
	item, err := a.apLoadRemoteIRI(p.Blog, ap.IRI(replyLink))
	if err != nil || item == nil || !ap.IsObject(item) {
		return
	}
	obj, err := ap.ToObject(item)
	if err != nil || obj == nil || obj.GetLink() == "" || obj.AttributedTo == nil || obj.AttributedTo.GetLink() == "" {
		return
	}
	replyLinkActor := []string{obj.AttributedTo.GetLink().String()}
	if p.Parameters == nil {
		p.Parameters = map[string][]string{}
	}
	p.Parameters[activityPubReplyActorParameter] = replyLinkActor
	_ = a.db.replacePostParam(p.Path, activityPubReplyActorParameter, replyLinkActor)
}

func (a *goBlog) apHandleInbox(w http.ResponseWriter, r *http.Request) {
	// Get blog
	blogName := chi.URLParam(r, "blog")
	blog, ok := a.cfg.Blogs[blogName]
	if !ok || blog == nil {
		a.serveError(w, r, "Inbox not found", http.StatusNotFound)
		return
	}
	// Verify request
	requestActor, err := a.apVerifySignature(r, blogName)
	if err != nil {
		// Send 401 because signature could not be verified
		a.serveError(w, r, err.Error(), http.StatusUnauthorized)
		return
	}
	// Parse activity
	body, err := io.ReadAll(r.Body)
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
	activityActor := activity.Actor.GetLink()
	if activity.Actor == nil || (!activity.Actor.IsLink() && !activity.Actor.IsObject()) {
		a.serveError(w, r, "Activity has no actor", http.StatusBadRequest)
		return
	}
	if activityActor != requestActor.GetLink() {
		a.serveError(w, r, "Request actor isn't activity actor", http.StatusForbidden)
		return
	}
	// Handle activity
	switch activity.GetType() {
	case ap.FollowType:
		a.apAccept(blogName, blog, activity)
	case ap.UndoType:
		if activity.Object.IsObject() {
			objectActivity, err := ap.ToActivity(activity.Object)
			if err == nil && objectActivity.GetType() == ap.FollowType && objectActivity.Actor.GetLink() == activityActor {
				_ = a.db.apRemoveFollower(blogName, activityActor.String())
			}
		}
	case ap.CreateType, ap.UpdateType:
		if activity.Object.IsObject() {
			a.apOnCreateUpdate(blog, requestActor, activity)
		}
	case ap.DeleteType, ap.BlockType:
		if activity.Object.GetLink() == activityActor {
			_ = a.db.apRemoveFollower(blogName, activityActor.String())
		} else {
			// Check if comment exists
			exists, commentId, err := a.db.commentIdByOriginal(activity.Object.GetLink().String())
			if err == nil && exists {
				_ = a.db.deleteComment(commentId)
				_ = a.db.deleteWebmentionUUrl(activity.Object.GetLink().String())
			}
		}
	case ap.AnnounceType:
		if announceTarget := activity.Object.GetLink().String(); announceTarget != "" && strings.HasPrefix(announceTarget, a.cfg.Server.PublicAddress) {
			a.sendNotification(fmt.Sprintf("%s announced %s", activityActor, announceTarget))
		}
	case ap.LikeType:
		if likeTarget := activity.Object.GetLink().String(); likeTarget != "" && strings.HasPrefix(likeTarget, a.cfg.Server.PublicAddress) {
			a.sendNotification(fmt.Sprintf("%s liked %s", activityActor, likeTarget))
		}
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
	// Get information from the note
	noteUri := object.GetLink().String()
	actorName := cmp.Or(requestActor.Name.First().String(), apUsername(requestActor))
	actorLink := requestActor.GetLink().String()
	if requestActor.URL != nil {
		actorLink = requestActor.URL.GetLink().String()
	}
	content := object.Content.First().String()
	// Handle reply
	if inReplyTo := object.InReplyTo; inReplyTo != nil {
		if replyTarget := inReplyTo.GetLink().String(); replyTarget != "" && strings.HasPrefix(replyTarget, a.cfg.Server.PublicAddress) {
			if object.To.Contains(ap.PublicNS) || object.CC.Contains(ap.PublicNS) {
				// Public reply - comment
				_, _, _ = a.createComment(blog, replyTarget, content, actorName, actorLink, noteUri)
				return
			} else {
				// Private reply - notification
				buf := bufferpool.Get()
				defer bufferpool.Put(buf)
				fmt.Fprintf(buf, "New private ActivityPub reply to %s from %s\n", cleanHTMLText(replyTarget), cleanHTMLText(noteUri))
				fmt.Fprintf(buf, "Author: %s (%s)", cleanHTMLText(actorName), cleanHTMLText(actorLink))
				buf.WriteString("\n\n")
				buf.WriteString(cleanHTMLText(content))
				a.sendNotification(buf.String())
				return
			}
		}
	}
	// Handle mention
	if blogIri := ap.IRI(a.apIri(blog)); object.To.Contains(blogIri) || object.CC.Contains(blogIri) {
		// Notification
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		fmt.Fprintf(buf, "New ActivityPub mention on %s\n", cleanHTMLText(noteUri))
		fmt.Fprintf(buf, "Author: %s (%s)", cleanHTMLText(actorName), cleanHTMLText(actorLink))
		buf.WriteString("\n\n")
		buf.WriteString(cleanHTMLText(content))
		a.sendNotification(buf.String())
		return
	}
	// Ignore other cases, maybe it's just spam
}

func (a *goBlog) apVerifySignature(r *http.Request, blog string) (*ap.Actor, error) {
	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		// Error with signature header etc.
		return nil, err
	}
	actor, err := a.apGetRemoteActor(blog, ap.IRI(verifier.KeyId()))
	if err != nil || actor == nil {
		// Actor not found or something else bad
		return nil, errors.New("failed to get actor")
	}
	if actor.PublicKey.PublicKeyPem == "" {
		return nil, errors.New("actor has no public key")
	}
	block, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))
	if block == nil {
		return nil, errors.New("public key invalid")
	}
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Unable to parse public key
		return nil, err
	}
	return actor, verifier.Verify(pubKey, httpsig.RSA_SHA256)
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
		a.serveAPItem(w, r, http.StatusOK, followersCollection)
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

func (db *database) apGetAllInboxes(blog string) (inboxes []string, err error) {
	rows, err := db.Query("select distinct inbox from activitypub_followers where blog = @blog", sql.Named("blog", blog))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
	follower, inbox, username string
}

func (db *database) apGetAllFollowers(blog string) (followers []*apFollower, err error) {
	rows, err := db.Query("select follower, inbox, username from activitypub_followers where blog = @blog", sql.Named("blog", blog))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var follower, inbox, username string
	for rows.Next() {
		err = rows.Scan(&follower, &inbox, &username)
		if err != nil {
			return nil, err
		}
		followers = append(followers, &apFollower{follower: follower, inbox: inbox, username: username})
	}
	return followers, nil
}

func (db *database) apAddFollower(blog, follower, inbox, username string) error {
	_, err := db.Exec(
		"insert or replace into activitypub_followers (blog, follower, inbox, username) values (@blog, @follower, @inbox, @username)",
		sql.Named("blog", blog), sql.Named("follower", follower), sql.Named("inbox", inbox), sql.Named("username", username),
	)
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
	blogConfig := a.getBlogFromPost(p)
	c := ap.CreateNew(a.apNewID(blogConfig), a.toAPNote(p))
	c.Actor = a.apAPIri(blogConfig)
	c.Published = time.Now()
	a.apSendToAllFollowers(p.Blog, c, append(p.Parameters[activityPubMentionsParameter], p.firstParameter(activityPubReplyActorParameter))...)
}

func (a *goBlog) apUpdate(p *post) {
	blogConfig := a.getBlogFromPost(p)
	u := ap.UpdateNew(a.apNewID(blogConfig), a.toAPNote(p))
	u.Actor = a.apAPIri(blogConfig)
	u.Published = time.Now()
	a.apSendToAllFollowers(p.Blog, u, append(p.Parameters[activityPubMentionsParameter], p.firstParameter(activityPubReplyActorParameter))...)
}

func (a *goBlog) apDelete(p *post) {
	blogConfig := a.getBlogFromPost(p)
	d := ap.DeleteNew(a.apNewID(blogConfig), a.activityPubId(p))
	d.Actor = a.apAPIri(blogConfig)
	d.Published = time.Now()
	a.apSendToAllFollowers(p.Blog, d, append(p.Parameters[activityPubMentionsParameter], p.firstParameter(activityPubReplyActorParameter))...)
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

func (a *goBlog) apAccept(blogName string, blog *configBlog, follow *ap.Activity) {
	newFollower := follow.Actor.GetLink()
	a.info("AcitivyPub: New follow request from follower", "id", newFollower.String())
	// Get remote actor
	follower, err := a.apGetRemoteActor(blogName, newFollower)
	if err != nil || follower == nil {
		// Couldn't retrieve remote actor info
		a.error("ActivityPub: Failed to retrieve remote actor info", "actor", newFollower, "err", err)
		return
	}
	// Add or update follower
	inbox := follower.Inbox.GetLink()
	if endpoints := follower.Endpoints; endpoints != nil && endpoints.SharedInbox != nil && endpoints.SharedInbox.GetLink() != "" {
		inbox = endpoints.SharedInbox.GetLink()
	}
	if inbox == "" {
		return
	}
	username := apUsername(follower)
	if err = a.db.apAddFollower(blogName, follower.GetLink().String(), inbox.String(), username); err != nil {
		return
	}
	// Send accept response to the new follower
	accept := ap.AcceptNew(a.apNewID(blog), follow)
	accept.To.Append(newFollower)
	accept.Actor = a.apAPIri(blog)
	_ = a.apQueueSendSigned(a.apIri(blog), inbox.String(), accept)
	// Notification
	a.sendNotification(fmt.Sprintf("%s (%s) started following %s", username, follower.GetLink().String(), a.apIri(blog)))
}

func (a *goBlog) apSendProfileUpdates() {
	for blog, config := range a.cfg.Blogs {
		person := a.toApPerson(blog)
		update := ap.UpdateNew(a.apNewID(config), person)
		update.Actor = a.apAPIri(config)
		update.Published = time.Now()
		update.To.Append(ap.PublicNS, a.apGetFollowersCollectionId(blog, config))
		a.apSendToAllFollowers(blog, update)
	}
}

func (a *goBlog) apSendToAllFollowers(blog string, activity *ap.Activity, mentions ...string) {
	inboxes, err := a.db.apGetAllInboxes(blog)
	if err != nil {
		a.error("ActivityPub: Failed to retrieve follower inboxes", "err", err)
		return
	}
	for _, m := range mentions {
		go func(m string) {
			if m == "" {
				return
			}
			actor, err := a.apGetRemoteActor(blog, ap.IRI(m))
			if err != nil || actor == nil || actor.Inbox == "" || actor.Inbox.GetLink() == "" {
				return
			}
			inbox := actor.Inbox.GetLink().String()
			a.apSendTo(a.apIri(a.cfg.Blogs[blog]), activity, inbox)
		}(m)
	}
	a.apSendTo(a.apIri(a.cfg.Blogs[blog]), activity, inboxes...)
}

func (a *goBlog) apSendTo(blogIri string, activity *ap.Activity, inboxes ...string) {
	for _, i := range lo.Uniq(inboxes) {
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
			a.error("ActivityPub: failed to decode cached private key")
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

func (a *goBlog) signRequest(r *http.Request, blogIri string) error {
	if date := r.Header.Get("Date"); date == "" {
		r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	}
	if host := r.Header.Get("Host"); host == "" {
		r.Header.Set("Host", r.URL.Host)
	}
	bodyBuf := bytes.NewBufferString("")
	if r.Body != nil {
		if _, err := io.Copy(bodyBuf, r.Body); err == nil {
			r.Body = io.NopCloser(bodyBuf)
		}
	}
	a.apSignMutex.Lock()
	defer a.apSignMutex.Unlock()
	return a.apSigner.SignRequest(a.apPrivateKey, blogIri+"#main-key", r, bodyBuf.Bytes())
}

func (a *goBlog) apRefetchFollowers(blogName string) error {
	followers, err := a.db.apGetAllFollowers(blogName)
	if err != nil {
		return err
	}
	for _, fol := range followers {
		actor, err := a.apGetRemoteActor(blogName, ap.IRI(fol.follower))
		if err != nil || actor == nil {
			a.error("ActivityPub: Failed to retrieve remote actor info", "actor", fol.follower, "err", err)
			continue
		}
		inbox := actor.Inbox.GetLink()
		if endpoints := actor.Endpoints; endpoints != nil && endpoints.SharedInbox != nil && endpoints.SharedInbox.GetLink() != "" {
			inbox = endpoints.SharedInbox.GetLink()
		}
		if inbox == "" {
			a.error("ActivityPub: Failed to get inbox for actor", "actor", fol.follower)
			continue
		}
		username := apUsername(actor)
		if err = a.db.apAddFollower(blogName, actor.GetLink().String(), inbox.String(), username); err != nil {
			a.error("ActivityPub: Failed to update follower info", "err", err)
			return err
		}
	}
	return nil
}

func (a *goBlog) apGetRemoteActor(blog string, iri ap.IRI) (*ap.Actor, error) {
	item, err := a.apLoadRemoteIRI(blog, iri)
	if err != nil {
		return nil, fmt.Errorf("failed to load remote actor: %w", err)
	}
	if item == nil {
		return nil, fmt.Errorf("failed to load remote actor, item is nil: %s", iri)
	}
	var actor *ap.Actor
	err = ap.OnActor(item, func(act *ap.Actor) error {
		actor = act
		return nil
	})
	return actor, err
}

// Inspired by go-ap/client's LoadIRI
func (a *goBlog) apLoadRemoteIRI(blog string, id ap.IRI) (ap.Item, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("invalid IRI, nil value: %s", id)
	}
	if _, err := id.URL(); err != nil {
		return nil, fmt.Errorf("trying to load an invalid IRI: %s, Error: %v", id, err)
	}

	var req *http.Request
	var resp *http.Response
	var err error

	if a.apHttpClients[blog] == nil {
		return nil, fmt.Errorf("no ActivityPub HTTP client for blog: %s", blog)
	}

	if req, err = http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		id.String(),
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to create request for IRI: %s, Error: %v", id, err)
	}

	req.Header.Add("Accept", contenttype.LDJSON)
	req.Header.Add("Accept", contenttype.AS)
	req.Header.Add("Accept", contenttype.JSON)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Host", req.URL.Host)

	if resp, err = a.apHttpClients[blog].Do(req); err != nil {
		return nil, err
	}
	if resp == nil {
		err := fmt.Errorf("unable to load from the AP endpoint: nil response, IRI: %s", id)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unable to load from the AP endpoint: invalid status %d, IRI: %s", resp.StatusCode, id)
		return nil, err
	}

	var body []byte
	if body, err = io.ReadAll(resp.Body); err != nil {
		err := fmt.Errorf("failed to read response body, IRI: %s, Err: %v", id, err)
		return nil, err
	}

	it, err := ap.UnmarshalJSON(body)
	if err != nil {
		return nil, err
	}

	return it, nil
}
