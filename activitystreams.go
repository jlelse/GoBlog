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

// TODO: Serve index

func servePostActivityStreams(w http.ResponseWriter, r *http.Request) {
	// Remove ".as" from path again
	r.URL.Path = strings.TrimSuffix(r.URL.Path, ".as")
	// Fetch post from db
	post, err := getPost(r.Context(), slashTrimmedPath(r))
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
		ID:           appConfig.Server.PublicAddress + post.Path,
		URL:          appConfig.Server.PublicAddress + post.Path,
		AttributedTo: appConfig.Server.PublicAddress,
	}
	// Name and Type
	if title := post.title(); title != "" {
		as.Name = title
		as.Type = "Article"
	} else {
		as.Type = "Note"
	}
	// Content
	if rendered, err := renderMarkdown(post.Content); err == nil {
		as.Content = string(rendered)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Attachments
	if images := post.Parameters[appConfig.Blogs[post.Blog].ActivityStreams.ImagesParameter]; len(images) > 0 {
		for _, image := range images {
			as.Attachment = append(as.Attachment, &asAttachment{
				Type: "Image",
				URL:  image,
			})
		}
	}
	// Dates
	dateFormat := "2006-01-02T15:04:05-07:00"
	if post.Published != "" {
		if t, err := dateparse.ParseIn(post.Published, time.Local); err == nil {
			as.Published = t.Format(dateFormat)
		}
	}
	if post.Updated != "" {
		if t, err := dateparse.ParseIn(post.Updated, time.Local); err == nil {
			as.Published = t.Format(dateFormat)
		}
	}
	// Reply
	if replyLink := post.firstParameter(appConfig.Blogs[post.Blog].ActivityStreams.ReplyParameter); replyLink != "" {
		as.InReplyTo = replyLink
	}
	// Send JSON
	w.Header().Add(contentType, contentTypeJSONUTF8)
	_ = json.NewEncoder(w).Encode(as)
}
