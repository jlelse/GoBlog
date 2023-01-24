package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/carlmjohnson/requests"
	"github.com/dgraph-io/ristretto"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/httpcachetransport"
	"willnorris.com/go/microformats"
)

func (a *goBlog) initMicroformatsCache() {
	a.mfInit.Do(func() {
		a.mfCache, _ = ristretto.NewCache(&ristretto.Config{
			NumCounters:        100,
			MaxCost:            10, // Cache http responses for 10 requests
			BufferItems:        64,
			IgnoreInternalCost: true,
		})
	})
}

type microformatsResult struct {
	Title, Content, Author, Url string
	source                      string
	hasUrl                      bool
}

func (a *goBlog) parseMicroformats(u string, cache bool) (*microformatsResult, error) {
	pr, pw := io.Pipe()
	rb := requests.URL(u).
		Method(http.MethodGet).
		Accept(contenttype.HTMLUTF8).
		Client(a.httpClient).
		ToWriter(pw)
	if cache {
		a.initMicroformatsCache()
		rb.Transport(httpcachetransport.NewHttpCacheTransport(a.httpClient.Transport, a.mfCache, 10*time.Minute))
	}
	go func() {
		_ = pw.CloseWithError(rb.Fetch(context.Background()))
	}()
	result, err := parseMicroformatsFromReader(u, pr)
	_ = pr.CloseWithError(err)
	return result, err
}

func parseMicroformatsFromReader(u string, r io.Reader) (*microformatsResult, error) {
	parsedUrl, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	// Temporary buffer
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	// Parse microformats
	m := &microformatsResult{
		source: u,
	}
	mfd := microformats.Parse(io.TeeReader(r, buf), parsedUrl)
	m.fillFromData(mfd)
	// Set URL if not parsed from microformats
	if m.Url == "" {
		m.Url = u
	}
	// Parse title from HTML if needed
	if m.Title == "" && m.Content == "" {
		doc, err := goquery.NewDocumentFromReader(buf)
		if err != nil {
			return nil, err
		}
		if title := doc.Find("title"); title != nil {
			m.Title = title.Text()
		}
	}
	// Reset title if it's just a prefix of the content
	if m.Title != "" && strings.HasPrefix(m.Content, m.Title) {
		m.Title = ""
	}
	// Shorten content and title if too long
	m.Content = truncateStringWithEllipsis(m.Content, 500)
	m.Title = truncateStringWithEllipsis(m.Title, 60)
	return m, nil
}

func (m *microformatsResult) fillFromData(mf *microformats.Data) {
	// Fill data
	for _, i := range mf.Items {
		if m.fill(i) {
			break
		}
	}
}

func (m *microformatsResult) fill(mf *microformats.Microformat) bool {
	if mfHasType(mf, "h-entry") {
		// Check URL
		if url, ok := mf.Properties["url"]; ok && len(url) > 0 {
			if url0, ok := url[0].(string); ok {
				if strings.EqualFold(url0, m.source) {
					// Is searched entry
					m.hasUrl = true
					m.Url = url0
					// Reset attributes to refill
					m.Author = ""
					m.Title = ""
					m.Content = ""
				} else if m.hasUrl {
					// Already found entry
					return false
				} else if m.Url == "" {
					// Is the first entry
					m.Url = url0
				} else {
					// Is not the first entry
					return false
				}
			}
		}
		// Title
		m.fillTitle(mf)
		// Content
		m.fillContent(mf)
		// Author
		m.fillAuthor(mf)
		return m.hasUrl
	}
	for _, mfc := range mf.Children {
		if m.fill(mfc) {
			return true
		}
	}
	return false
}

func (m *microformatsResult) fillTitle(mf *microformats.Microformat) {
	if m.Title != "" {
		return
	}
	if name, ok := mf.Properties["name"]; ok && len(name) > 0 {
		if title, ok := name[0].(string); ok {
			m.Title = strings.TrimSpace(title)
		}
	}
}

func (m *microformatsResult) fillContent(mf *microformats.Microformat) {
	if m.Content != "" {
		return
	}
	if contents, ok := mf.Properties["content"]; ok && len(contents) > 0 {
		if content, ok := contents[0].(map[string]string); ok {
			if contentHTML, ok := content["html"]; ok {
				m.Content = cleanHTMLText(contentHTML)
				// Replace newlines with spaces
				m.Content = strings.ReplaceAll(m.Content, "\n", " ")
				// Collapse double spaces
				m.Content = strings.Join(strings.Fields(m.Content), " ")
				// Trim spaces
				m.Content = strings.TrimSpace(m.Content)
			}
		}
	}
}

func (m *microformatsResult) fillAuthor(mf *microformats.Microformat) {
	if m.Author != "" {
		return
	}
	if authors, ok := mf.Properties["author"]; ok && len(authors) > 0 {
		if author, ok := authors[0].(*microformats.Microformat); ok {
			if names, ok := author.Properties["name"]; ok && len(names) > 0 {
				if name, ok := names[0].(string); ok {
					m.Author = strings.TrimSpace(name)
				}
			}
		}
	}
}

func mfHasType(mf *microformats.Microformat, typ string) bool {
	for _, t := range mf.Type {
		if typ == t {
			return true
		}
	}
	return false
}
