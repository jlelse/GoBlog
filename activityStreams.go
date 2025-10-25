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
	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/jsonld"
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
	// Create a Note object
	note := ap.ObjectNew(ap.NoteType)
	note.ID = a.activityPubId(p)
	note.URL = ap.IRI(a.fullPostURL(p))
	note.AttributedTo = a.apAPIri(a.getBlogFromPost(p))
	// Audience
	switch p.Visibility {
	case visibilityPublic:
		note.To.Append(ap.PublicNS, a.apGetFollowersCollectionId(p.Blog, a.getBlogFromPost(p)))
	case visibilityUnlisted:
		note.To.Append(a.apGetFollowersCollectionId(p.Blog, a.getBlogFromPost(p)))
		note.CC.Append(ap.PublicNS)
	}
	for _, m := range p.Parameters[activityPubMentionsParameter] {
		note.CC.Append(ap.IRI(m))
	}
	// Name and Type
	if title := p.RenderedTitle; title != "" {
		note.Type = ap.ArticleType
		note.Name = ap.DefaultNaturalLanguage(title)
	}
	// Content
	note.MediaType = ap.MimeType(contenttype.HTML)
	note.Content = ap.DefaultNaturalLanguage(a.postHtml(&postHtmlOptions{p: p, absolute: true, activityPub: true}))
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
			apTag.Name = ap.DefaultNaturalLanguage(tag)
			apTag.URL = ap.IRI(a.getFullAddress(a.getRelativePath(p.Blog, fmt.Sprintf("/%s/%s", tagTax, urlize(tag)))))
			note.Tag.Append(apTag)
		}
	}
	// Mentions
	for _, mention := range p.Parameters[activityPubMentionsParameter] {
		apMention := ap.MentionNew(ap.IRI(mention))
		apMention.Href = ap.IRI(mention)
		note.Tag.Append(apMention)
	}
	if replyLinkActor := p.firstParameter(activityPubReplyActorParameter); replyLinkActor != "" {
		apMention := ap.MentionNew(ap.IRI(replyLinkActor))
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
		note.InReplyTo = ap.IRI(replyLink)
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

func (a *goBlog) toApPerson(blog string) *goBlogPerson {
	b := a.cfg.Blogs[blog]

	apIri := a.apAPIri(b)

	apBlog := &goBlogPerson{
		Person:      *ap.PersonNew(apIri),
		AlsoKnownAs: nil,
	}
	apBlog.URL = apIri

	apBlog.Name.Set(ap.DefaultLang, ap.Content(a.renderMdTitle(b.Title)))
	apBlog.Summary.Set(ap.DefaultLang, ap.Content(b.Description))
	apBlog.PreferredUsername.Set(ap.DefaultLang, ap.Content(blog))

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

	var attributionDomains ap.ItemCollection
	for _, ad := range a.cfg.ActivityPub.AttributionDomains {
		attributionDomains = append(attributionDomains, ap.IRI(ad))
	}
	apBlog.AttributionDomains = attributionDomains

	var alsoKnownAs ap.ItemCollection
	for _, aka := range a.cfg.ActivityPub.AlsoKnownAs {
		alsoKnownAs = append(alsoKnownAs, ap.IRI(aka))
	}
	apBlog.AlsoKnownAs = alsoKnownAs

	return apBlog
}

func (a *goBlog) serveActivityStreams(w http.ResponseWriter, r *http.Request, status int, blog string) {
	a.serveAPItem(w, r, status, a.toApPerson(blog))
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

func apUsername(person *ap.Person) string {
	preferredUsername := person.PreferredUsername.First().String()
	u, err := url.Parse(person.GetLink().String())
	if err != nil || u == nil || u.Host == "" || preferredUsername == "" {
		return person.GetLink().String()
	}
	return fmt.Sprintf("@%s@%s", preferredUsername, u.Host)
}

// Modified types

type goBlogPerson struct {
	ap.Person
	AlsoKnownAs        ap.ItemCollection `jsonld:"alsoKnownAs,omitempty"`
	AttributionDomains ap.ItemCollection `jsonld:"attributionDomains,omitempty"`
}

func (a goBlogPerson) MarshalJSON() ([]byte, error) {
	// Taken from AP library, Person.MarshalJSON

	b := make([]byte, 0)
	notEmpty := false
	ap.JSONWrite(&b, '{')

	ap.OnObject(a.Person, func(o *ap.Object) error {
		notEmpty = ap.JSONWriteObjectValue(&b, *o)
		return nil
	})
	if a.Inbox != nil {
		notEmpty = ap.JSONWriteItemProp(&b, "inbox", a.Inbox) || notEmpty
	}
	if a.Following != nil {
		notEmpty = ap.JSONWriteItemProp(&b, "following", a.Following) || notEmpty
	}
	if a.Followers != nil {
		notEmpty = ap.JSONWriteItemProp(&b, "followers", a.Followers) || notEmpty
	}
	if a.PreferredUsername != nil {
		notEmpty = ap.JSONWriteNaturalLanguageProp(&b, "preferredUsername", a.PreferredUsername) || notEmpty
	}
	if len(a.PublicKey.PublicKeyPem)+len(a.PublicKey.ID) > 0 {
		if v, err := a.PublicKey.MarshalJSON(); err == nil && len(v) > 0 {
			notEmpty = ap.JSONWriteProp(&b, "publicKey", v) || notEmpty
		}
	}

	// Custom
	if len(a.AlsoKnownAs) > 0 {
		notEmpty = ap.JSONWriteItemCollectionProp(&b, "alsoKnownAs", a.AlsoKnownAs, false)
	}
	if len(a.AttributionDomains) > 0 {
		notEmpty = ap.JSONWriteItemCollectionProp(&b, "attributionDomains", a.AttributionDomains, false)
	}

	if notEmpty {
		ap.JSONWrite(&b, '}')
		return b, nil
	}
	return nil, nil
}
