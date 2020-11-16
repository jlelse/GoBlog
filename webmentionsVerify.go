package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/PuerkitoBio/goquery"
	"willnorris.com/go/microformats"
)

func wmVerify(m *mention) error {
	client := &http.Client{}
	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) > 15 {
			return errors.New("too many redirects")
		}
		return nil
	}
	req, err := http.NewRequest(http.MethodGet, m.Source, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "GoBlog")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return wmVerifyReader(resp.Body, m)
}

func wmVerifyReader(body io.Reader, m *mention) error {
	var linksBuffer, gqBuffer, mfBuffer bytes.Buffer
	io.Copy(io.MultiWriter(&linksBuffer, &gqBuffer, &mfBuffer), body)
	// Check if source mentions target
	links, err := allLinksFromHTML(&linksBuffer, m.Source)
	if err != nil {
		return err
	}
	hasLink := false
	for _, link := range links {
		if link == m.Target {
			hasLink = true
			break
		}
	}
	if !hasLink {
		return errors.New("target not found in source")
	}
	// Set title
	doc, err := goquery.NewDocumentFromReader(&gqBuffer)
	if err != nil {
		return err
	}
	if title := doc.Find("title"); title != nil {
		m.Title = title.Text()
	}
	// Fill mention attributes
	sourceURL, err := url.Parse(m.Source)
	if err != nil {
		return err
	}
	mfFillMentionFromData(m, microformats.Parse(&mfBuffer, sourceURL))
	return nil
}

func mfFillMentionFromData(m *mention, mf *microformats.Data) {
	for _, i := range mf.Items {
		mfFillMention(m, i)
	}
}

func mfFillMention(m *mention, mf *microformats.Microformat) bool {
	if mfHasType(mf, "h-entry") {
		if name, ok := mf.Properties["name"]; ok && len(name) > 0 {
			if title, ok := name[0].(string); ok {
				m.Title = title
			}
		}
		if reply, ok := mf.Properties["in-reply-to"]; ok && len(reply) > 0 {
			if replyLink, ok := reply[0].(string); ok && replyLink == m.Target {
				m.Type = "comment"
			}
		}
		if like, ok := mf.Properties["like-of"]; ok && len(like) > 0 {
			if likeLink, ok := like[0].(string); ok && likeLink == m.Target {
				m.Type = "like"
			}
		}
		if contents, ok := mf.Properties["content"]; ok && len(contents) > 0 {
			if content, ok := contents[0].(map[string]interface{}); ok {
				if rawContentValue, ok := content["value"]; ok {
					if contentValue, ok := rawContentValue.(string); ok {
						m.Content = contentValue
					}
				}
			}
		}
		if authors, ok := mf.Properties["author"]; ok && len(authors) > 0 {
			if author, ok := authors[0].(*microformats.Microformat); ok {
				if names, ok := author.Properties["name"]; ok && len(names) > 0 {
					if name, ok := names[0].(string); ok {
						m.Author = name
					}
				}
			}
		}
		return true
	} else if len(mf.Children) > 0 {
		for _, mfc := range mf.Children {
			if mfFillMention(m, mfc) {
				return true
			}
		}
	}
	return false
}

func mfHasType(mf *microformats.Microformat, typ string) bool {
	for _, t := range mf.Type {
		if typ == t {
			return true
		}
	}
	return false
}
