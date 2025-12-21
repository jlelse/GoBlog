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
	"go.goblog.app/app/pkgs/activitypub"
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

func (a *goBlog) toAPNote(p *post) *activitypub.Note {
	// Create a Note object
	note := activitypub.ObjectNew(activitypub.NoteType)
	note.ID = a.activityPubId(p)
	note.URL = activitypub.IRI(a.fullPostURL(p))
	note.AttributedTo = a.apAPIri(a.getBlogFromPost(p))
	// Audience
	switch p.Visibility {
	case visibilityPublic:
		note.To.Append(activitypub.PublicNS, a.apGetFollowersCollectionId(p.Blog, a.getBlogFromPost(p)))
	case visibilityUnlisted:
		note.To.Append(a.apGetFollowersCollectionId(p.Blog, a.getBlogFromPost(p)))
		note.CC.Append(activitypub.PublicNS)
	}
	for _, m := range p.Parameters[activityPubMentionsParameter] {
		note.CC.Append(activitypub.IRI(m))
	}
	// Name and Type
	if title := p.RenderedTitle; title != "" {
		note.Type = activitypub.ArticleType
		note.Name = activitypub.DefaultNaturalLanguage(title)
	}
	// Content
	note.MediaType = activitypub.MimeType(contenttype.HTML)
	note.Content = activitypub.DefaultNaturalLanguage(a.postHtml(&postHtmlOptions{p: p, absolute: true, activityPub: true}))
	// Attachments
	if images := p.Parameters[a.cfg.Micropub.PhotoParam]; len(images) > 0 {
		var attachments activitypub.ItemCollection
		for _, image := range images {
			apImage := activitypub.ObjectNew(activitypub.ImageType)
			apImage.URL = activitypub.IRI(image)
			attachments.Append(apImage)
		}
		note.Attachment = attachments
	}
	// Tags
	for _, tagTax := range a.cfg.ActivityPub.TagsTaxonomies {
		for _, tag := range p.Parameters[tagTax] {
			apTag := &activitypub.Object{Type: "Hashtag"}
			apTag.Name = activitypub.DefaultNaturalLanguage(tag)
			apTag.URL = activitypub.IRI(a.getFullAddress(a.getRelativePath(p.Blog, fmt.Sprintf("/%s/%s", tagTax, urlize(tag)))))
			note.Tag.Append(apTag)
		}
	}
	// Mentions
	for _, mention := range p.Parameters[activityPubMentionsParameter] {
		apMention := activitypub.MentionNew(activitypub.IRI(mention))
		apMention.URL = activitypub.IRI(mention)
		note.Tag.Append(apMention)
	}
	if replyLinkActor := p.firstParameter(activityPubReplyActorParameter); replyLinkActor != "" {
		apMention := activitypub.MentionNew(activitypub.IRI(replyLinkActor))
		apMention.URL = activitypub.IRI(replyLinkActor)
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
		note.InReplyTo = activitypub.IRI(replyLink)
	}
	return note
}

const activityPubVersionParam = "activitypubversion"

func (a *goBlog) activityPubId(p *post) activitypub.IRI {
	fu := a.fullPostURL(p)
	if version := p.firstParameter(activityPubVersionParam); version != "" {
		return activitypub.IRI(fu + "?activitypubversion=" + version)
	}
	return activitypub.IRI(fu)
}

func (a *goBlog) toApPerson(blog string) *activitypub.Person {
	b := a.cfg.Blogs[blog]

	apIri := a.apAPIri(b)

	apBlog := activitypub.PersonNew(apIri)
	apBlog.URL = apIri

	apBlog.Name.Set(activitypub.DefaultLang, string(activitypub.Content(a.renderMdTitle(b.Title))))
	apBlog.Summary.Set(activitypub.DefaultLang, string(activitypub.Content(b.Description)))
	apBlog.PreferredUsername.Set(activitypub.DefaultLang, string(activitypub.Content(blog)))

	apBlog.Inbox = activitypub.IRI(a.getFullAddress("/activitypub/inbox/" + blog))
	apBlog.Followers = activitypub.IRI(a.getFullAddress("/activitypub/followers/" + blog))

	apBlog.PublicKey.Owner = apIri
	apBlog.PublicKey.ID = activitypub.IRI(a.apIri(b) + "#main-key")
	apBlog.PublicKey.PublicKeyPem = string(pem.EncodeToMemory(&pem.Block{
		Type:    "PUBLIC KEY",
		Headers: nil,
		Bytes:   a.apPubKeyBytes,
	}))

	if a.hasProfileImage() {
		icon := &activitypub.Image{}
		icon.Type = activitypub.ImageType
		icon.MediaType = activitypub.MimeType(contenttype.JPEG)
		icon.URL = activitypub.IRI(a.getFullAddress(a.profileImagePath(profileImageFormatJPEG, 0, 0)))
		apBlog.Icon = icon
	}

	var attributionDomains activitypub.ItemCollection
	for _, ad := range a.cfg.ActivityPub.AttributionDomains {
		attributionDomains = append(attributionDomains, activitypub.IRI(ad))
	}
	apBlog.AttributionDomains = attributionDomains

	var alsoKnownAs activitypub.ItemCollection
	for _, aka := range a.cfg.ActivityPub.AlsoKnownAs {
		alsoKnownAs = append(alsoKnownAs, activitypub.IRI(aka))
	}
	apBlog.AlsoKnownAs = alsoKnownAs

	return apBlog
}

func (a *goBlog) serveActivityStreams(w http.ResponseWriter, r *http.Request, status int, blog string) {
	a.serveAPItem(w, r, status, a.toApPerson(blog))
}

func (a *goBlog) serveAPItem(w http.ResponseWriter, r *http.Request, status int, item any) {
	// Encode
	binary, err := jsonld.WithContext(jsonld.IRI(activitypub.ActivityBaseURI), jsonld.IRI(activitypub.SecurityContextURI)).Marshal(item)
	if err != nil {
		a.serveError(w, r, "Encoding failed", http.StatusInternalServerError)
		return
	}
	// Send response
	w.Header().Set(contentType, contenttype.ASUTF8)
	w.WriteHeader(status)
	_ = a.min.Get().Minify(contenttype.AS, w, bytes.NewReader(binary))
}

func apUsername(person *activitypub.Person) string {
	preferredUsername := person.PreferredUsername.First().String()
	u, err := url.Parse(person.GetLink().String())
	if err != nil || u == nil || u.Host == "" || preferredUsername == "" {
		return person.GetLink().String()
	}
	return fmt.Sprintf("@%s@%s", preferredUsername, u.Host)
}
