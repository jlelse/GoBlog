package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/araddon/dateparse"
)

func manipulateAsPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if lowerAccept := strings.ToLower(r.Header.Get("Accept")); (strings.Contains(lowerAccept, "application/activity+json") || strings.Contains(lowerAccept, "application/ld+json")) && !strings.Contains(lowerAccept, "text/html") {
			// Is ActivityStream, add ".as" to differentiate cache and also trigger as function
			r.URL.Path += ".as"
		}
		next.ServeHTTP(rw, r)
	})
}

type asPost struct {
	Context      []string        `json:"@context"`
	To           []string        `json:"to"`
	InReplyTo    string          `json:"inReplyTo,omitempty"`
	Name         string          `json:"name,omitempty"`
	Type         string          `json:"type"`
	Content      string          `json:"content"`
	MediaType    string          `json:"mediaType"`
	Attachment   []*asAttachment `json:"attachment,omitempty"`
	Published    string          `json:"published"`
	Updated      string          `json:"updated,omitempty"`
	ID           string          `json:"id"`
	URL          string          `json:"url"`
	AttributedTo string          `json:"attributedTo"`
}

type asAttachment struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

func servePostActivityStreams(w http.ResponseWriter, r *http.Request) {
	// Remove ".as" from path again
	r.URL.Path = strings.TrimSuffix(r.URL.Path, ".as")
	// Fetch post from db
	p, err := getPost(slashTrimmedPath(r))
	if err == errPostNotFound {
		serve404(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Create a Note object
	as := &asPost{
		Context:      []string{"https://www.w3.org/ns/activitystreams"},
		To:           []string{"https://www.w3.org/ns/activitystreams#Public"},
		MediaType:    "text/html",
		ID:           appConfig.Server.PublicAddress + p.Path,
		URL:          appConfig.Server.PublicAddress + p.Path,
		AttributedTo: appConfig.Server.PublicAddress,
	}
	// Name and Type
	if title := p.title(); title != "" {
		as.Name = title
		as.Type = "Article"
	} else {
		as.Type = "Note"
	}
	// Content
	as.Content = string(p.html())
	// Attachments
	if images := p.Parameters[appConfig.Blogs[p.Blog].ActivityStreams.ImagesParameter]; len(images) > 0 {
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
		if t, err := dateparse.ParseIn(p.Published, time.Local); err == nil {
			as.Published = t.Format(dateFormat)
		}
	}
	if p.Updated != "" {
		if t, err := dateparse.ParseIn(p.Updated, time.Local); err == nil {
			as.Published = t.Format(dateFormat)
		}
	}
	// Reply
	if replyLink := p.firstParameter(appConfig.Blogs[p.Blog].ActivityStreams.ReplyParameter); replyLink != "" {
		as.InReplyTo = replyLink
	}
	// Send JSON
	w.Header().Add(contentType, contentTypeJSONUTF8)
	_ = json.NewEncoder(w).Encode(as)
}

type asPerson struct {
	Context    []string `json:"@context"`
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Summary    string   `json:"summary"`
	Attachment []struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"attachment"`
	PreferredUsername string `json:"preferredUsername"`
	Icon              struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"icon"`
	Inbox     string `json:"inbox"`
	PublicKey struct {
		ID           string `json:"id"`
		Owner        string `json:"owner"`
		PublicKeyPem string `json:"publicKeyPem"`
	} `json:"publicKey"`
}
