package main

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/araddon/dateparse"
	ct "github.com/elnormous/contenttype"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

const asContext = "https://www.w3.org/ns/activitystreams"

const asRequestKey contextKey = "asRequest"

func (a *goBlog) checkActivityStreamsRequest(next http.Handler) http.Handler {
	if len(a.asCheckMediaTypes) == 0 {
		a.asCheckMediaTypes = []ct.MediaType{
			ct.NewMediaType(contenttype.HTML),
			ct.NewMediaType(contenttype.AS),
			ct.NewMediaType(contenttype.LDJSON),
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

type asNote struct {
	Context      any             `json:"@context,omitempty"`
	To           []string        `json:"to,omitempty"`
	InReplyTo    string          `json:"inReplyTo,omitempty"`
	Name         string          `json:"name,omitempty"`
	Type         string          `json:"type,omitempty"`
	Content      string          `json:"content,omitempty"`
	MediaType    string          `json:"mediaType,omitempty"`
	Attachment   []*asAttachment `json:"attachment,omitempty"`
	Published    string          `json:"published,omitempty"`
	Updated      string          `json:"updated,omitempty"`
	ID           string          `json:"id,omitempty"`
	URL          string          `json:"url,omitempty"`
	AttributedTo string          `json:"attributedTo,omitempty"`
	Tag          []*asTag        `json:"tag,omitempty"`
}

type asPerson struct {
	Context           any           `json:"@context,omitempty"`
	ID                string        `json:"id,omitempty"`
	URL               string        `json:"url,omitempty"`
	Type              string        `json:"type,omitempty"`
	Name              string        `json:"name,omitempty"`
	Summary           string        `json:"summary,omitempty"`
	PreferredUsername string        `json:"preferredUsername,omitempty"`
	Icon              *asAttachment `json:"icon,omitempty"`
	Inbox             string        `json:"inbox,omitempty"`
	PublicKey         *asPublicKey  `json:"publicKey,omitempty"`
	Endpoints         *asEndpoints  `json:"endpoints,omitempty"`
}

type asAttachment struct {
	Type      string `json:"type,omitempty"`
	URL       string `json:"url,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
}

type asTag struct {
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
	Href string `json:"href,omitempty"`
}

type asPublicKey struct {
	ID           string `json:"id,omitempty"`
	Owner        string `json:"owner,omitempty"`
	PublicKeyPem string `json:"publicKeyPem,omitempty"`
}

type asEndpoints struct {
	SharedInbox string `json:"sharedInbox,omitempty"`
}

func (a *goBlog) serveActivityStreamsPost(p *post, w http.ResponseWriter) {
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	if err := json.NewEncoder(buf).Encode(a.toASNote(p)); err != nil {
		http.Error(w, "Encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, contenttype.ASUTF8)
	_ = a.min.Get().Minify(contenttype.AS, w, buf)
}

func (a *goBlog) toASNote(p *post) *asNote {
	// Create a Note object
	as := &asNote{
		Context:      []string{asContext},
		To:           []string{"https://www.w3.org/ns/activitystreams#Public"},
		MediaType:    contenttype.HTML,
		ID:           a.activityPubId(p),
		URL:          a.fullPostURL(p),
		AttributedTo: a.apIri(a.cfg.Blogs[p.Blog]),
	}
	// Name and Type
	if title := p.RenderedTitle; title != "" {
		as.Name = title
		as.Type = "Article"
	} else {
		as.Type = "Note"
	}
	// Content
	as.Content = a.postHtml(p, true)
	// Attachments
	if images := p.Parameters[a.cfg.Micropub.PhotoParam]; len(images) > 0 {
		for _, image := range images {
			as.Attachment = append(as.Attachment, &asAttachment{
				Type: "Image",
				URL:  image,
			})
		}
	}
	// Tags
	for _, tagTax := range a.cfg.ActivityPub.TagsTaxonomies {
		for _, tag := range p.Parameters[tagTax] {
			as.Tag = append(as.Tag, &asTag{
				Type: "Hashtag",
				Name: tag,
				Href: a.getFullAddress(a.getRelativePath(p.Blog, fmt.Sprintf("/%s/%s", tagTax, urlize(tag)))),
			})
		}
	}
	// Dates
	dateFormat := "2006-01-02T15:04:05-07:00"
	if p.Published != "" {
		if t, err := dateparse.ParseLocal(p.Published); err == nil {
			as.Published = t.Format(dateFormat)
		}
	}
	if p.Updated != "" {
		if t, err := dateparse.ParseLocal(p.Updated); err == nil {
			as.Updated = t.Format(dateFormat)
		}
	}
	// Reply
	if replyLink := p.firstParameter(a.cfg.Micropub.ReplyParam); replyLink != "" {
		as.InReplyTo = replyLink
	}
	return as
}

const activityPubVersionParam = "activitypubversion"

func (a *goBlog) activityPubId(p *post) string {
	fu := a.fullPostURL(p)
	if version := p.firstParameter(activityPubVersionParam); version != "" {
		return fu + "?activitypubversion=" + version
	}
	return fu
}

func (a *goBlog) toAsPerson(blog string) *asPerson {
	b := a.cfg.Blogs[blog]
	asBlog := &asPerson{
		Context:           []string{asContext},
		Type:              "Person",
		ID:                a.apIri(b),
		URL:               a.apIri(b),
		Name:              a.renderMdTitle(b.Title),
		Summary:           b.Description,
		PreferredUsername: blog,
		Inbox:             a.getFullAddress("/activitypub/inbox/" + blog),
		PublicKey: &asPublicKey{
			Owner: a.apIri(b),
			ID:    a.apIri(b) + "#main-key",
			PublicKeyPem: string(pem.EncodeToMemory(&pem.Block{
				Type:    "PUBLIC KEY",
				Headers: nil,
				Bytes:   a.apPubKeyBytes,
			})),
		},
	}
	if pic := a.cfg.User.Picture; pic != "" {
		asBlog.Icon = &asAttachment{
			Type:      "Image",
			URL:       pic,
			MediaType: mimeTypeFromUrl(pic),
		}
	}
	return asBlog
}

func (a *goBlog) serveActivityStreams(blog string, w http.ResponseWriter, r *http.Request) {
	person := a.toAsPerson(blog)
	// Encode
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	if err := json.NewEncoder(buf).Encode(person); err != nil {
		a.serveError(w, r, "Encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, contenttype.ASUTF8)
	_ = a.min.Get().Minify(contenttype.AS, w, buf)
}
