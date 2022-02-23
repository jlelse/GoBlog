package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/thoas/go-funk"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
	"willnorris.com/go/microformats"
)

func (a *goBlog) initWebmentionQueue() {
	a.listenOnQueue("wm", time.Minute, func(qi *queueItem, dequeue func(), reschedule func(time.Duration)) {
		var m mention
		if err := gob.NewDecoder(bytes.NewReader(qi.content)).Decode(&m); err != nil {
			log.Println("webmention queue:", err.Error())
			dequeue()
			return
		}
		if err := a.verifyMention(&m); err != nil {
			log.Println(fmt.Sprintf("Failed to verify webmention from %s to %s: %s", m.Source, m.Target, err.Error()))
		}
		dequeue()
	})
}

func (a *goBlog) queueMention(m *mention) error {
	if wm := a.cfg.Webmention; wm != nil && wm.DisableReceiving {
		return errors.New("webmention receiving disabled")
	}
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	if err := gob.NewEncoder(buf).Encode(m); err != nil {
		return err
	}
	return a.db.enqueue("wm", buf.Bytes(), time.Now())
}

func (a *goBlog) verifyMention(m *mention) error {
	// Request target
	targetReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, m.Target, nil)
	if err != nil {
		return err
	}
	targetReq.Header.Set("Accept", contenttype.HTMLUTF8)
	setLoggedIn(targetReq, true)
	targetResp, err := doHandlerRequest(targetReq, a.getAppRouter())
	if err != nil {
		return err
	}
	_ = targetResp.Body.Close()
	// Check if target has a valid status code
	if targetResp.StatusCode != http.StatusOK {
		if a.cfg.Debug {
			a.debug(fmt.Sprintf("Webmention for unknown path: %s", m.Target))
		}
		return a.db.deleteWebmention(m)
	}
	// Check if target has a redirect
	if respReq := targetResp.Request; respReq != nil {
		if ru := respReq.URL; m.Target != ru.String() {
			m.NewTarget = ru.String()
		}
	}
	// Request source
	sourceReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, m.Source, nil)
	if err != nil {
		return err
	}
	sourceReq.Header.Set("Accept", contenttype.HTMLUTF8)
	var sourceResp *http.Response
	if strings.HasPrefix(m.Source, a.cfg.Server.PublicAddress) ||
		(a.cfg.Server.ShortPublicAddress != "" && strings.HasPrefix(m.Source, a.cfg.Server.ShortPublicAddress)) {
		setLoggedIn(sourceReq, true)
		sourceResp, err = doHandlerRequest(sourceReq, a.getAppRouter())
		if err != nil {
			return err
		}
		defer sourceResp.Body.Close()
	} else {
		sourceReq.Header.Set(userAgent, appUserAgent)
		sourceResp, err = a.httpClient.Do(sourceReq)
		if err != nil {
			return err
		}
		defer sourceResp.Body.Close()
	}
	// Check if source has a valid status code
	if sourceResp.StatusCode != http.StatusOK {
		if a.cfg.Debug {
			a.debug(fmt.Sprintf("Delete webmention because source doesn't have valid status code: %s", m.Source))
		}
		return a.db.deleteWebmention(m)
	}
	// Check if source has a redirect
	if respReq := sourceResp.Request; respReq != nil {
		if ru := respReq.URL; m.Source != ru.String() {
			m.NewSource = ru.String()
		}
	}
	// Parse response body
	err = a.verifyReader(m, sourceResp.Body)
	if err != nil {
		if a.cfg.Debug {
			a.debug(fmt.Sprintf("Delete webmention because verifying %s threw error: %s", m.Source, err.Error()))
		}
		return a.db.deleteWebmention(m)
	}
	if cr := []rune(m.Content); len(cr) > 500 {
		m.Content = string(cr[0:497]) + "…"
	}
	if tr := []rune(m.Title); len(tr) > 60 {
		m.Title = string(tr[0:57]) + "…"
	}
	newStatus := webmentionStatusVerified
	// Update or insert webmention
	if a.db.webmentionExists(m) {
		if a.cfg.Debug {
			a.debug(fmt.Sprintf("Update webmention: %s => %s", m.Source, m.Target))
		}
		// Update webmention
		err = a.db.updateWebmention(m, newStatus)
		if err != nil {
			return err
		}
	} else {
		if m.NewSource != "" {
			m.Source = m.NewSource
		}
		if m.NewTarget != "" {
			m.Target = m.NewTarget
		}
		err = a.db.insertWebmention(m, newStatus)
		if err != nil {
			return err
		}
		a.sendNotification(fmt.Sprintf("New webmention from %s to %s", defaultIfEmpty(m.NewSource, m.Source), defaultIfEmpty(m.NewTarget, m.Target)))
	}
	return err
}

func (a *goBlog) verifyReader(m *mention, body io.Reader) error {
	linksBuffer, gqBuffer, mfBuffer := bufferpool.Get(), bufferpool.Get(), bufferpool.Get()
	defer bufferpool.Put(linksBuffer, gqBuffer, mfBuffer)
	if _, err := io.Copy(io.MultiWriter(linksBuffer, gqBuffer, mfBuffer), body); err != nil {
		return err
	}
	// Check if source mentions target
	links, err := allLinksFromHTML(linksBuffer, defaultIfEmpty(m.NewSource, m.Source))
	if err != nil {
		return err
	}
	if _, hasLink := funk.FindString(links, func(s string) bool {
		// Check if link belongs to installation
		hasShortPrefix := a.cfg.Server.ShortPublicAddress != "" && strings.HasPrefix(s, a.cfg.Server.ShortPublicAddress)
		hasLongPrefix := strings.HasPrefix(s, a.cfg.Server.PublicAddress)
		if !hasShortPrefix && !hasLongPrefix {
			return false
		}
		// Check if link is or redirects to target
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, m.Target, nil)
		if err != nil {
			return false
		}
		req.Header.Set("Accept", contenttype.HTMLUTF8)
		setLoggedIn(req, true)
		resp, err := doHandlerRequest(req, a.getAppRouter())
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK && unescapedPath(resp.Request.URL.String()) == unescapedPath(defaultIfEmpty(m.NewTarget, m.Target)) {
			return true
		}
		return false
	}); !hasLink {
		return errors.New("target not found in source")
	}
	// Fill mention attributes
	sourceURL, err := url.Parse(defaultIfEmpty(m.NewSource, m.Source))
	if err != nil {
		return err
	}
	m.Title = ""
	m.Content = ""
	m.Author = ""
	m.Url = ""
	m.hasUrl = false
	m.fillFromData(microformats.Parse(mfBuffer, sourceURL))
	if m.Url == "" {
		m.Url = m.Source
	}
	// Set title when content is empty as well
	if m.Title == "" && m.Content == "" {
		doc, err := goquery.NewDocumentFromReader(gqBuffer)
		if err != nil {
			return err
		}
		if title := doc.Find("title"); title != nil {
			m.Title = title.Text()
		}
	}
	return nil
}

func (m *mention) fillFromData(mf *microformats.Data) {
	// Fill data
	for _, i := range mf.Items {
		if m.fill(i) {
			break
		}
	}
}

func (m *mention) fill(mf *microformats.Microformat) bool {
	if mfHasType(mf, "h-entry") {
		// Check URL
		if url, ok := mf.Properties["url"]; ok && len(url) > 0 {
			if url0, ok := url[0].(string); ok {
				if strings.EqualFold(url0, defaultIfEmpty(m.NewSource, m.Source)) {
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

func (m *mention) fillTitle(mf *microformats.Microformat) {
	if m.Title != "" {
		return
	}
	if name, ok := mf.Properties["name"]; ok && len(name) > 0 {
		if title, ok := name[0].(string); ok {
			m.Title = strings.TrimSpace(title)
		}
	}
}

func (m *mention) fillContent(mf *microformats.Microformat) {
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

func (m *mention) fillAuthor(mf *microformats.Microformat) {
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
