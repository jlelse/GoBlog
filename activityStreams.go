package main

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"

	"github.com/araddon/dateparse"
	ct "github.com/elnormous/contenttype"
	ap "go.goblog.app/app/pkgs/activitypub"
	"go.goblog.app/app/pkgs/activitypub/jsonld"
	"go.goblog.app/app/pkgs/contenttype"
)

const asRequestKey contextKey = "asRequest"

func (a *goBlog) checkActivityStreamsRequest(next http.Handler) http.Handler {
	if len(a.asCheckMediaTypes) == 0 {
		a.asCheckMediaTypes = []ct.MediaType{
			ct.NewMediaType(contenttype.HTML),
			ct.NewMediaType(contenttype.AS),
			ct.NewMediaType(contenttype.LDJSON),
			ct.NewMediaType(contenttype.LDJSON + "; profile=\"https://www.w3.org/ns/activitystreams\""),
		}
	}
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if ap := a.cfg.ActivityPub; ap != nil && ap.Enabled && !a.isPrivate() {
			// Check if accepted media type is not HTML
			if mt, _, err := ct.GetAcceptableMediaType(r, a.asCheckMediaTypes); err == nil && mt.String() != a.asCheckMediaTypes[0].String() {
				next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), asRequestKey, true)))
				return
			}
		}
		next.ServeHTTP(rw, r)
	})
}

func (a *goBlog) serveActivityStreamsPost(w http.ResponseWriter, r *http.Request, status int, p *post) {
	a.serveAPItem(w, r, status, a.toAPNote(p))
}

func (a *goBlog) toAPNote(p *post) *ap.Note {
	bc := a.getBlogFromPost(p)
	// Create a Note object
	note := ap.ObjectNew(ap.NoteType)
	note.ID = a.activityPubId(p)
	note.URL = ap.IRI(a.fullPostURL(p))
	note.AttributedTo = a.apAPIri(bc)
	// Audience
	switch p.Visibility {
	case visibilityPublic:
		note.To.Append(ap.PublicNS, a.apGetFollowersCollectionId(p.Blog, bc))
	case visibilityUnlisted:
		note.To.Append(a.apGetFollowersCollectionId(p.Blog, bc))
		note.CC.Append(ap.PublicNS)
	}
	for _, m := range p.Parameters[activityPubMentionsParameter] {
		note.CC.Append(ap.IRI(m))
	}
	// Name and Type
	if title := p.RenderedTitle; title != "" {
		note.Type = ap.ArticleType
		note.Name = ap.NaturalLanguageValues{{Lang: bc.Lang, Value: title}}
	}
	// Content
	note.MediaType = ap.MimeType(contenttype.HTML)
	note.Content = ap.NaturalLanguageValues{{Lang: bc.Lang, Value: a.postHtml(&postHtmlOptions{p: p, absolute: true, activityPub: true})}}
	// Attachments
	if images := p.Parameters[a.cfg.Micropub.PhotoParam]; len(images) > 0 {
		var attachments ap.ItemCollection
		for _, image := range images {
			apImage := ap.ObjectNew(ap.ImageType)
			apImage.URL = ap.IRI(image)
			attachments.Append(apImage)
		}
		note.Attachment = attachments
	}
	// Tags
	for _, tagTax := range a.cfg.ActivityPub.TagsTaxonomies {
		for _, tag := range p.Parameters[tagTax] {
			apTag := &ap.Object{Type: "Hashtag"}
			apTag.Name = ap.NaturalLanguageValues{{Lang: bc.Lang, Value: tag}}
			apTag.URL = ap.IRI(a.getFullAddress(a.getRelativePath(p.Blog, fmt.Sprintf("/%s/%s", tagTax, urlize(tag)))))
			note.Tag.Append(apTag)
		}
	}
	// Mentions
	for _, mention := range p.Parameters[activityPubMentionsParameter] {
		apMention := ap.ObjectNew(ap.MentionType)
		apMention.ID = ap.IRI(mention)
		apMention.Href = ap.IRI(mention)
		note.Tag.Append(apMention)
	}
	if replyLinkActor := p.firstParameter(activityPubReplyActorParameter); replyLinkActor != "" {
		apMention := ap.ObjectNew(ap.MentionType)
		apMention.ID = ap.IRI(replyLinkActor)
		apMention.Href = ap.IRI(replyLinkActor)
		note.Tag.Append(apMention)
	}
	// Dates
	if p.Published != "" {
		if t, err := dateparse.ParseLocal(p.Published); err == nil {
			note.Published = t
		}
	}
	if p.Updated != "" {
		if t, err := dateparse.ParseLocal(p.Updated); err == nil {
			note.Updated = t
		}
	}
	// Reply
	if replyLink := p.firstParameter(a.cfg.Micropub.ReplyParam); replyLink != "" {
		if replyObject := p.firstParameter(activityPubReplyObjectParameter); replyObject != "" {
			note.InReplyTo = ap.IRI(replyObject)
		} else {
			// Fallback to reply link if reply object is not available
			note.InReplyTo = ap.IRI(replyLink)
		}
	}
	return note
}

const activityPubVersionParam = "activitypubversion"

func (a *goBlog) activityPubId(p *post) ap.IRI {
	fu := a.fullPostURL(p)
	if version := p.firstParameter(activityPubVersionParam); version != "" {
		return ap.IRI(fu + "?activitypubversion=" + version)
	}
	return ap.IRI(fu)
}

func (a *goBlog) toApPerson(blog string) *ap.Actor {
	b := a.cfg.Blogs[blog]

	apIri := a.apAPIri(b)

	apBlog := ap.PersonNew(apIri)
	apBlog.URL = apIri

	apBlog.Name = ap.NaturalLanguageValues{{Lang: b.Lang, Value: a.renderMdTitle(b.Title)}}
	apBlog.Summary = ap.NaturalLanguageValues{{Lang: b.Lang, Value: b.Description}}
	apBlog.PreferredUsername = ap.NaturalLanguageValues{{Lang: b.Lang, Value: blog}}

	apBlog.Inbox = ap.IRI(a.getFullAddress("/activitypub/inbox/" + blog))
	apBlog.Followers = ap.IRI(a.getFullAddress("/activitypub/followers/" + blog))

	apBlog.PublicKey.Owner = apIri
	apBlog.PublicKey.ID = ap.IRI(a.apIri(b) + "#main-key")
	apBlog.PublicKey.PublicKeyPem = string(pem.EncodeToMemory(&pem.Block{
		Type:    "PUBLIC KEY",
		Headers: nil,
		Bytes:   a.apPubKeyBytes,
	}))

	if a.hasProfileImage() {
		icon := &ap.Image{}
		icon.Type = ap.ImageType
		icon.MediaType = ap.MimeType(contenttype.JPEG)
		icon.URL = ap.IRI(a.getFullAddress(a.profileImagePath(profileImageFormatJPEG, 0, 0)))
		apBlog.Icon = icon
	}

	for _, ad := range a.cfg.ActivityPub.AttributionDomains {
		apBlog.AttributionDomains = append(apBlog.AttributionDomains, ap.IRI(ad))
	}

	// Add alternative domains as alsoKnownAs
	for _, altHostname := range a.cfg.Server.altHostnames {
		altIri := a.apIriForHostname(b, altHostname)
		apBlog.AlsoKnownAs = append(apBlog.AlsoKnownAs, ap.IRI(altIri))
	}

	for _, aka := range a.cfg.ActivityPub.AlsoKnownAs {
		apBlog.AlsoKnownAs = append(apBlog.AlsoKnownAs, ap.IRI(aka))
	}

	// Check if this blog has a movedTo target set (account migration)
	if movedTo, err := a.getApMovedTo(blog); err == nil && movedTo != "" {
		apBlog.MovedTo = ap.IRI(movedTo)
	}

	return apBlog
}

func (a *goBlog) serveActivityStreams(w http.ResponseWriter, r *http.Request, status int, blog string) {
	a.serveAPItem(w, r, status, a.toApPerson(blog))
}

// serveActivityStreamsAltDomain serves the ActivityPub actor for an alternative domain
// The actor has movedTo pointing to the main domain
func (a *goBlog) serveActivityStreamsAltDomain(w http.ResponseWriter, r *http.Request) {
	// Check if this is an ActivityStreams request
	asRequest, ok := r.Context().Value(asRequestKey).(bool)
	if !ok || !asRequest {
		// Not an AS request, redirect to main domain
		http.Redirect(w, r, a.cfg.Server.PublicAddress+r.URL.RequestURI(), http.StatusMovedPermanently)
		return
	}

	// Get blog from context
	blogName, ok := r.Context().Value(blogKey).(string)
	if !ok || blogName == "" {
		a.serveError(w, r, "Blog not found", http.StatusNotFound)
		return
	}

	// Get the alternative domain hostname
	altHostname, _ := r.Context().Value(altDomainKey).(string)
	if altHostname == "" {
		a.serveError(w, r, "Alternative domain not set", http.StatusInternalServerError)
		return
	}

	// Create the actor for this alternative domain
	person := a.toApPersonForAltDomain(blogName, altHostname)
	a.serveAPItem(w, r, http.StatusOK, person)
}

// toApPersonForAltDomain creates an ActivityPub person for an alternative domain
// It has movedTo pointing to the main domain
func (a *goBlog) toApPersonForAltDomain(blog string, altHostname string) *ap.Actor {
	b := a.cfg.Blogs[blog]

	altAddress := a.getAltDomainAddress(altHostname)
	apIri := ap.IRI(altAddress + b.getRelativePath(""))

	apBlog := ap.PersonNew(apIri)
	apBlog.URL = apIri

	apBlog.Name = ap.NaturalLanguageValues{{Lang: b.Lang, Value: a.renderMdTitle(b.Title)}}
	apBlog.Summary = ap.NaturalLanguageValues{{Lang: b.Lang, Value: b.Description}}
	apBlog.PreferredUsername = ap.NaturalLanguageValues{{Lang: b.Lang, Value: blog}}

	// Inbox and followers still on the alternative domain
	apBlog.Inbox = ap.IRI(altAddress + "/activitypub/inbox/" + blog)
	apBlog.Followers = ap.IRI(altAddress + "/activitypub/followers/" + blog)

	// Public key uses same key for both domains
	apBlog.PublicKey.Owner = apIri
	apBlog.PublicKey.ID = ap.IRI(altAddress + b.getRelativePath("") + "#main-key")
	apBlog.PublicKey.PublicKeyPem = string(pem.EncodeToMemory(&pem.Block{
		Type:    "PUBLIC KEY",
		Headers: nil,
		Bytes:   a.apPubKeyBytes,
	}))

	if a.hasProfileImage() {
		icon := &ap.Image{}
		icon.Type = ap.ImageType
		icon.MediaType = ap.MimeType(contenttype.JPEG)
		icon.URL = ap.IRI(altAddress + a.profileImagePath(profileImageFormatJPEG, 0, 0))
		apBlog.Icon = icon
	}

	// Add main domain as alsoKnownAs
	apBlog.AlsoKnownAs = append(apBlog.AlsoKnownAs, a.apAPIri(b))
	// Also add any configured alsoKnownAs
	for _, aka := range a.cfg.ActivityPub.AlsoKnownAs {
		apBlog.AlsoKnownAs = append(apBlog.AlsoKnownAs, ap.IRI(aka))
	}

	// Set movedTo to point to the main domain
	apBlog.MovedTo = a.apAPIri(b)

	return apBlog
}

func (a *goBlog) serveAPItem(w http.ResponseWriter, r *http.Request, status int, item any) {
	// Encode
	binary, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(item)
	if err != nil {
		a.serveError(w, r, "Encoding failed", http.StatusInternalServerError)
		return
	}
	// Send response
	w.Header().Set(contentType, contenttype.ASUTF8)
	w.WriteHeader(status)
	_ = a.min.Get().Minify(contenttype.AS, w, bytes.NewReader(binary))
}

func apUsername(actor *ap.Actor) string {
	preferredUsername := actor.PreferredUsername.First().String()
	u, err := url.Parse(actor.GetLink().String())
	if err != nil || u == nil || u.Host == "" || preferredUsername == "" {
		return actor.GetLink().String()
	}
	return fmt.Sprintf("@%s@%s", preferredUsername, u.Host)
}
