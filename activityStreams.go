package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"strings"

	"github.com/araddon/dateparse"
)

var asContext = []string{"https://www.w3.org/ns/activitystreams"}

const asRequestKey requestContextKey = "asRequest"

func manipulateAsPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if ap := appConfig.ActivityPub; ap != nil && ap.Enabled {
			if lowerAccept := strings.ToLower(r.Header.Get("Accept")); (strings.Contains(lowerAccept, contentTypeAS) || strings.Contains(lowerAccept, "application/ld+json")) && !strings.Contains(lowerAccept, contentTypeHTML) {
				next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), asRequestKey, true)))
				return
			}
		}
		next.ServeHTTP(rw, r)
	})
}

type asNote struct {
	Context      interface{}     `json:"@context,omitempty"`
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
}

type asPerson struct {
	Context           interface{}   `json:"@context,omitempty"`
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
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
}

type asPublicKey struct {
	ID           string `json:"id,omitempty"`
	Owner        string `json:"owner,omitempty"`
	PublicKeyPem string `json:"publicKeyPem,omitempty"`
}

type asEndpoints struct {
	SharedInbox string `json:"sharedInbox,omitempty"`
}

func (p *post) serveActivityStreams(w http.ResponseWriter) {
	b, _ := json.Marshal(p.toASNote())
	w.Header().Set(contentType, contentTypeASUTF8)
	_, _ = writeMinified(w, contentTypeAS, b)
}

func (p *post) toASNote() *asNote {
	// Create a Note object
	as := &asNote{
		Context:      asContext,
		To:           []string{"https://www.w3.org/ns/activitystreams#Public"},
		MediaType:    contentTypeHTML,
		ID:           p.fullURL(),
		URL:          p.fullURL(),
		AttributedTo: appConfig.Blogs[p.Blog].apIri(),
	}
	// Name and Type
	if title := p.title(); title != "" {
		as.Name = title
		as.Type = "Article"
	} else {
		as.Type = "Note"
	}
	// Content
	as.Content = string(p.absoluteHTML())
	// Attachments
	if images := p.Parameters[appConfig.Micropub.PhotoParam]; len(images) > 0 {
		for _, image := range images {
			as.Attachment = append(as.Attachment, &asAttachment{
				Type: "Image",
				URL:  image,
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
	if replyLink := p.firstParameter(appConfig.Micropub.ReplyParam); replyLink != "" {
		as.InReplyTo = replyLink
	}
	return as
}

func (b *configBlog) serveActivityStreams(blog string, w http.ResponseWriter, r *http.Request) {
	publicKeyDer, err := x509.MarshalPKIXPublicKey(&apPrivateKey.PublicKey)
	if err != nil {
		serveError(w, r, "Failed to marshal public key", http.StatusInternalServerError)
		return
	}
	asBlog := &asPerson{
		Context:           asContext,
		Type:              "Person",
		ID:                b.apIri(),
		URL:               b.apIri(),
		Name:              b.Title,
		Summary:           b.Description,
		PreferredUsername: blog,
		Inbox:             appConfig.Server.PublicAddress + "/activitypub/inbox/" + blog,
		PublicKey: &asPublicKey{
			Owner: b.apIri(),
			ID:    b.apIri() + "#main-key",
			PublicKeyPem: string(pem.EncodeToMemory(&pem.Block{
				Type:    "PUBLIC KEY",
				Headers: nil,
				Bytes:   publicKeyDer,
			})),
		},
	}
	// Add profile picture
	if appConfig.User.Picture != "" {
		asBlog.Icon = &asAttachment{
			Type: "Image",
			URL:  appConfig.User.Picture,
		}
	}
	jb, _ := json.Marshal(asBlog)
	w.Header().Set(contentType, contentTypeASUTF8)
	_, _ = writeMinified(w, contentTypeAS, jb)
}
