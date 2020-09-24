package main

import (
	"encoding/json"
	"github.com/araddon/dateparse"
	"net/http"
	"strings"
	"time"
)

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
	Id           string          `json:"id"`
	Url          string          `json:"url"`
	AttributedTo string          `json:"attributedTo"`
}

type asAttachment struct {
	Type string `json:"type"`
	Url  string `json:"url"`
}

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
		Id:           appConfig.Server.PublicAddress + post.Path,
		Url:          appConfig.Server.PublicAddress + post.Path,
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
	if rendered, err := renderMarkdown(post.Content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		as.Content = string(rendered)
	}
	// Attachments
	if images := post.Parameters[appConfig.Blog.ActivityStreams.ImagesParameter]; len(images) > 0 {
		for _, image := range images {
			as.Attachment = append(as.Attachment, &asAttachment{
				Type: "Image",
				Url:  image,
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
	if replyLink := post.firstParameter(appConfig.Blog.ActivityStreams.ReplyParameter); replyLink != "" {
		as.InReplyTo = replyLink
	}
	// Send JSON
	w.Header().Add("Content-Type", contentTypeJSON)
	_ = json.NewEncoder(w).Encode(as)
}
