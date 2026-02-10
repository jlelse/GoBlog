package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
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
	"slices"
	"time"

	"code.superseriousbusiness.org/httpsig"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/samber/lo"
	ap "go.goblog.app/app/pkgs/activitypub"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

// ActivityPub path constants
const (
	activityPubBasePath     = "/activitypub"
	apInboxPathTemplate     = activityPubBasePath + "/inbox/"     // + blog name
	apFollowersPathTemplate = activityPubBasePath + "/followers/" // + blog name
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
	a.apSignerNoDigest, _, err = httpsig.NewSigner(
		[]httpsig.Algorithm{httpsig.RSA_SHA256},
		httpsig.DigestSha256,
		[]string{httpsig.RequestTarget, "date", "host"},
		httpsig.Signature,
		0,
	)
	return err
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
		a.apUserHandle[name] = "@" + name + "@" + a.cfg.Server.publicHost
		acct := "acct:" + name + "@" + a.cfg.Server.publicHost
		a.webfingerResources[acct] = blog
		iri := a.apIri(blog)
		a.webfingerResources[iri] = blog
		a.webfingerAccts[iri] = acct
	}
	// Also prepare webfinger for alternative domains
	for _, altAddress := range a.cfg.Server.AltAddresses {
		for name, blog := range a.cfg.Blogs {
			acct := "acct:" + name + "@" + getHost(altAddress)
			a.webfingerResources[acct] = blog
			iri := a.apIriForAddress(blog, altAddress)
			a.webfingerResources[iri] = blog
			a.webfingerAccts[iri] = acct
		}
	}
}

func (a *goBlog) apHandleWebfinger(w http.ResponseWriter, r *http.Request) {
	// Get blog
	blog, ok := a.webfingerResources[r.URL.Query().Get("resource")]
	if !ok {
		a.serveError(w, r, "Resource not found", http.StatusNotFound)
		return
	}
	apIri := a.apIri(blog)
	// Check if it is an alternative host
	altHostname, ok := r.Context().Value(altAddressKey).(string)
	if ok && altHostname != "" {
		// Alternative domain webfinger
		apIri = a.apIriForAddress(blog, altHostname)
	}
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
		if err != nil || act == nil || !ap.IsActorType(act.Type) {
			continue
		}
		mentions = append(mentions, cmp.Or(string(act.GetLink()), link))
	}
	if p.Parameters == nil {
		p.Parameters = map[string][]string{}
	}
	p.Parameters[activityPubMentionsParameter] = mentions
	_ = a.db.replacePostParam(p.Path, activityPubMentionsParameter, mentions)
}

const (
	activityPubReplyActorParameter  = "activitypubreplyactor"
	activityPubReplyObjectParameter = "activitypubreplyobject"
)

func (a *goBlog) apCheckActivityPubReply(p *post) {
	replyLink := a.replyLink(p)
	if replyLink == "" {
		return
	}
	item, err := a.apLoadRemoteIRI(p.Blog, ap.IRI(replyLink))
	if err != nil || item == nil || !item.IsObject() {
		return
	}
	obj, err := ap.ToObject(item)
	if err != nil || obj == nil || obj.GetLink() == "" {
		return
	}
	if p.Parameters == nil {
		p.Parameters = map[string][]string{}
	}
	replyLinkObject := []string{obj.GetLink().String()}
	p.Parameters[activityPubReplyObjectParameter] = replyLinkObject
	_ = a.db.replacePostParam(p.Path, activityPubReplyObjectParameter, replyLinkObject)
	if obj.AttributedTo != nil && obj.AttributedTo.GetLink() != "" {
		replyLinkActor := []string{obj.AttributedTo.GetLink().String()}
		p.Parameters[activityPubReplyActorParameter] = replyLinkActor
		_ = a.db.replacePostParam(p.Path, activityPubReplyActorParameter, replyLinkActor)
	}
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
			if err == nil &&
				objectActivity.GetType() == ap.FollowType &&
				objectActivity.Actor.GetLink() == activityActor &&
				objectActivity.Object.GetLink() == a.apAPIri(blog) {
				a.info("Follower unfollowed", "blog", blogName, "actor", activityActor.String())
				_ = a.db.apRemoveFollower(blogName, activityActor.String())
			}
		}
	case ap.CreateType, ap.UpdateType:
		if activity.Object.IsObject() {
			a.apOnCreateUpdate(blog, requestActor, activity)
		}
	case ap.DeleteType, ap.BlockType:
		if activity.Object.GetLink() == activityActor {
			a.info("Follower got deleted or blocked", "blog", blogName, "actor", activityActor.String(), "activity_type", activity.GetType())
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
		if announceTarget := activity.Object.GetLink().String(); announceTarget != "" && a.isLocalURL(announceTarget) {
			a.sendNotification(fmt.Sprintf("%s announced %s", activityActor, announceTarget))
		}
	case ap.LikeType:
		if likeTarget := activity.Object.GetLink().String(); likeTarget != "" && a.isLocalURL(likeTarget) {
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
		if replyTarget := inReplyTo.GetLink().String(); replyTarget != "" && a.isLocalURL(replyTarget) {
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
	var pubKey any
	switch block.Type {
	case "RSA PUBLIC KEY":
		pubKey, err = x509.ParsePKCS1PublicKey(block.Bytes)
	default:
		pubKey, err = x509.ParsePKIXPublicKey(block.Bytes)
	}
	if err != nil {
		// Unable to parse public key
		return nil, err
	}
	var algo httpsig.Algorithm
	switch pubKey.(type) {
	case *rsa.PublicKey:
		algo = httpsig.RSA_SHA256
	case *ecdsa.PublicKey:
		algo = httpsig.ECDSA_SHA256
	case ed25519.PublicKey:
		algo = httpsig.ED25519
	default:
		return nil, errors.New("unsupported public key type")
	}
	return actor, verifier.Verify(pubKey, algo)
}

func handleWellKnownHostMeta(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentType, "application/xrd+xml"+contenttype.CharsetUtf8Suffix)
	_, _ = io.WriteString(w, xml.Header)
	_, _ = io.WriteString(w, `<XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0"><Link rel="lrdd" type="application/xrd+xml" template="https://`+r.Host+`/.well-known/webfinger?resource={uri}"/></XRD>`)
}

func (a *goBlog) apGetFollowersCollectionId(blogName string) ap.IRI {
	return a.apGetFollowersCollectionIdForAddress(blogName, "")
}

func (a *goBlog) apGetFollowersCollectionIdForAddress(blogName string, address string) ap.IRI {
	path := apFollowersPathTemplate + blogName
	if address == "" {
		return ap.IRI(a.getFullAddress(path))
	} else {
		return ap.IRI(getFullAddressStatic(address, path))
	}
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
		followersCollection := ap.CollectionNew(a.apGetFollowersCollectionId(blogName))
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
			apUser:    fmt.Sprintf("@%s@%s", blogName, a.cfg.Server.publicHost),
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
	c := ap.ActivityNew(ap.CreateType, a.apNewID(blogConfig), a.toAPNote(p))
	c.Actor = a.apAPIri(blogConfig)
	c.Published = time.Now()
	a.apSendToAllFollowers(p.Blog, c, append(p.Parameters[activityPubMentionsParameter], p.firstParameter(activityPubReplyActorParameter))...)
}

func (a *goBlog) apUpdate(p *post) {
	blogConfig := a.getBlogFromPost(p)
	u := ap.ActivityNew(ap.UpdateType, a.apNewID(blogConfig), a.toAPNote(p))
	u.Actor = a.apAPIri(blogConfig)
	u.Published = time.Now()
	a.apSendToAllFollowers(p.Blog, u, append(p.Parameters[activityPubMentionsParameter], p.firstParameter(activityPubReplyActorParameter))...)
}

func (a *goBlog) apDelete(p *post) {
	blogConfig := a.getBlogFromPost(p)
	d := ap.ActivityNew(ap.DeleteType, a.apNewID(blogConfig), a.activityPubId(p))
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
	a.info("ActivityPub: New follow request from follower", "id", newFollower.String())
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
	accept := ap.ActivityNew(ap.AcceptType, a.apNewID(blog), follow)
	accept.To.Append(newFollower)
	accept.Actor = a.apAPIri(blog)
	_ = a.apQueueSendSigned(a.apIri(blog), inbox.String(), accept)
	// Notification
	followerLink := follower.GetLink().String()
	if follower.URL != nil && follower.URL.GetLink() != "" {
		followerLink = follower.URL.GetLink().String()
	}
	a.sendNotification(fmt.Sprintf("%s (%s) started following %s", username, followerLink, a.apIri(blog)))
}

func (a *goBlog) apSendProfileUpdates() {
	for blog, config := range a.cfg.Blogs {
		person := a.toApPerson(blog, "")
		update := ap.ActivityNew(ap.UpdateType, a.apNewID(config), person)
		update.Actor = a.apAPIri(config)
		update.Published = time.Now()
		update.To.Append(ap.PublicNS, a.apGetFollowersCollectionId(blog))
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

func (a *goBlog) apNewID(blog *configBlog) ap.IRI {
	return ap.IRI(a.apIri(blog) + "#" + uuid.NewString())
}

func (a *goBlog) apIri(b *configBlog) string {
	return a.getFullAddress(b.getRelativePath(""))
}

func (a *goBlog) apIriForAddress(b *configBlog, address string) string {
	if address == "" {
		return a.apIri(b)
	}
	return getFullAddressStatic(address, b.getRelativePath(""))
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
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		a.apSignMutex.Lock()
		defer a.apSignMutex.Unlock()
		return a.apSignerNoDigest.SignRequest(a.apPrivateKey, blogIri+"#main-key", r, nil)
	}
	bodyBuf := bufferpool.Get()
	defer bufferpool.Put(bodyBuf)
	if r.Body != nil {
		_, _ = io.Copy(bodyBuf, r.Body)
		r.Body = io.NopCloser(bytes.NewReader(bodyBuf.Bytes()))
	}
	a.apSignMutex.Lock()
	defer a.apSignMutex.Unlock()
	return a.apSigner.SignRequest(a.apPrivateKey, blogIri+"#main-key", r, bodyBuf.Bytes())
}

func (a *goBlog) apMoveFollowers(blogName string, targetAccount string) error {
	// Check if blog exists
	blog, ok := a.cfg.Blogs[blogName]
	if !ok || blog == nil {
		return fmt.Errorf("blog not found: %s", blogName)
	}

	// Fetch and validate the target account
	targetActor, err := a.apGetRemoteActor(blogName, ap.IRI(targetAccount))
	if err != nil || targetActor == nil {
		return fmt.Errorf("failed to fetch target account %s: %w", targetAccount, err)
	}

	// Verify that the target account has the GoBlog account in alsoKnownAs
	blogIri := a.apIri(blog)
	hasAlias := false
	for _, aka := range targetActor.AlsoKnownAs {
		if aka.GetLink().String() == blogIri {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		return fmt.Errorf("target account %s does not have %s in alsoKnownAs - add it before moving followers", targetAccount, blogIri)
	}

	// Get all followers
	followers, err := a.db.apGetAllFollowers(blogName)
	if err != nil {
		return fmt.Errorf("failed to get followers: %w", err)
	}

	if len(followers) == 0 {
		a.info("No followers to move")
		return nil
	}

	// Get all follower inboxes
	inboxes, err := a.db.apGetAllInboxes(blogName)
	if err != nil {
		return fmt.Errorf("failed to get follower inboxes: %w", err)
	}

	a.info("Moving followers to new account", "count", len(followers), "target", targetAccount)

	// Save the movedTo setting in the database so that the actor profile reflects the move
	if err := a.setApMovedTo(blogName, targetAccount); err != nil {
		return fmt.Errorf("failed to save movedTo setting: %w", err)
	}

	// Purge cache to ensure the actor profile with movedTo is served immediately
	a.purgeCache()

	// Create Move activity per ActivityPub spec:
	// - actor: the account performing the move (this blog)
	// - object: the account being moved (also this blog - it's moving itself)
	// - target: the new account to move to
	// Actor and Object are the same because the blog is announcing it's moving itself.
	// The Move is addressed only to followers (not public) per ActivityPub conventions.
	blogApiIri := a.apAPIri(blog)
	move := ap.ActivityNew(ap.MoveType, a.apNewID(blog), blogApiIri)
	move.Actor = blogApiIri
	move.Target = ap.IRI(targetAccount)
	move.To.Append(a.apGetFollowersCollectionId(blogName))

	// Send Move activity to all follower inboxes using the same pattern as other activities
	uniqueInboxes := lo.Uniq(inboxes)
	a.apSendTo(blogIri, move, uniqueInboxes...)

	a.info("Move activities queued for all followers", "count", len(uniqueInboxes), "target", targetAccount)
	return nil
}

func (a *goBlog) apDomainMove(oldAddress, newAddress string) error {
	// Validate old domain is in altAddresses
	if !slices.Contains(a.cfg.Server.AltAddresses, oldAddress) {
		return fmt.Errorf("old domain %s is not in altAddresses configuration - add it first and restart GoBlog", oldAddress)
	}

	// Validate new domain is the public address
	if newAddress != a.cfg.Server.PublicAddress {
		return fmt.Errorf("new domain %s does not match public address %s", newAddress, a.cfg.Server.PublicAddress)
	}

	a.info("Starting domain move", "oldAddress", oldAddress, "newAddress", newAddress)

	// For each blog, send Move activity from old domain actor to followers
	for blogName, blog := range a.cfg.Blogs {
		if err := a.apSendDomainMoveForBlog(blogName, blog, oldAddress, newAddress); err != nil {
			return fmt.Errorf("failed to send domain move for blog %s: %w", blogName, err)
		}
	}

	return nil
}

func (a *goBlog) apSendDomainMoveForBlog(blogName string, blog *configBlog, oldAddress, newAddress string) error {
	// Get all followers
	followers, err := a.db.apGetAllFollowers(blogName)
	if err != nil {
		return fmt.Errorf("failed to get followers: %w", err)
	}

	if len(followers) == 0 {
		a.info("No followers for blog", "blog", blogName)
		return nil
	}

	// Get all follower inboxes
	inboxes, err := a.db.apGetAllInboxes(blogName)
	if err != nil {
		return fmt.Errorf("failed to get follower inboxes: %w", err)
	}

	a.info("Sending domain move for blog", "blog", blogName, "followers", len(followers))

	// Old actor IRI (on the old domain)
	oldActorIri := a.apIriForAddress(blog, oldAddress)
	// New actor IRI (on the new domain)
	newActorIri := a.apAPIri(blog)

	// Create Move activity
	// actor: the old domain actor (the one moving)
	// object: also the old domain actor (it's moving itself)
	// target: the new domain actor (where it's moving to)
	move := ap.ActivityNew(ap.MoveType, ap.IRI(oldActorIri+"#move-"+blogName+"-"+time.Now().Format("20060102150405")), ap.IRI(oldActorIri))
	move.Actor = ap.IRI(oldActorIri)
	move.Target = newActorIri
	// Followers collection on the old domain
	move.To.Append(a.apGetFollowersCollectionIdForAddress(blogName, oldAddress))
	move.Published = time.Now()

	// Send Move activity to all follower inboxes
	// We sign with the main key since it's the same instance
	uniqueInboxes := lo.Uniq(inboxes)
	for _, inbox := range uniqueInboxes {
		if err := a.apQueueSendSigned(oldActorIri, inbox, move); err != nil {
			a.error("Failed to queue Move activity", "inbox", inbox, "err", err)
		}
	}

	a.info("Domain move activities queued for blog", "blog", blogName, "count", len(uniqueInboxes))
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
	return ap.ToActor(item)
}

// Inspired by go-ap/client's LoadIRI
func (a *goBlog) apLoadRemoteIRI(blog string, id ap.IRI) (ap.Item, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("invalid IRI, nil value: %s", id)
	}
	if _, err := id.URL(); err != nil {
		return nil, fmt.Errorf("trying to load an invalid IRI: %s, Error: %v", id, err)
	}
	bc, ok := a.cfg.Blogs[blog]
	if !ok {
		return nil, fmt.Errorf("blog not found: %s", blog)
	}

	var req *http.Request
	var resp *http.Response
	var err error

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

	if err = a.signRequest(req, a.apIri(bc)); err != nil {
		return nil, err
	}
	if resp, err = a.httpClient.Do(req); err != nil {
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
