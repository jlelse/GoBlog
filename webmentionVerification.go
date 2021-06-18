package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/thoas/go-funk"
	"willnorris.com/go/microformats"
)

func (a *goBlog) initWebmentionQueue() {
	go func() {
		for {
			qi, err := a.db.peekQueue("wm")
			if err != nil {
				log.Println(err.Error())
				continue
			} else if qi != nil {
				var m mention
				err = gob.NewDecoder(bytes.NewReader(qi.content)).Decode(&m)
				if err != nil {
					log.Println(err.Error())
					_ = a.db.dequeue(qi)
					continue
				}
				err = a.verifyMention(&m)
				if err != nil {
					log.Println(fmt.Sprintf("Failed to verify webmention from %s to %s: %s", m.Source, m.Target, err.Error()))
				}
				err = a.db.dequeue(qi)
				if err != nil {
					log.Println(err.Error())
				}
			} else {
				// No item in the queue, wait a moment
				time.Sleep(15 * time.Second)
			}
		}
	}()
}

func (a *goBlog) queueMention(m *mention) error {
	if wm := a.cfg.Webmention; wm != nil && wm.DisableReceiving {
		return errors.New("webmention receiving disabled")
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(m); err != nil {
		return err
	}
	return a.db.enqueue("wm", buf.Bytes(), time.Now())
}

func (a *goBlog) verifyMention(m *mention) error {
	req, err := http.NewRequest(http.MethodGet, m.Source, nil)
	if err != nil {
		return err
	}
	var resp *http.Response
	if strings.HasPrefix(m.Source, a.cfg.Server.PublicAddress) {
		rec := httptest.NewRecorder()
		for a.d == nil {
			// Server not yet started
			time.Sleep(1 * time.Second)
		}
		a.d.ServeHTTP(rec, req.WithContext(context.WithValue(req.Context(), loggedInKey, true)))
		resp = rec.Result()
	} else {
		req.Header.Set(userAgent, appUserAgent)
		resp, err = appHttpClient.Do(req)
		if err != nil {
			return err
		}
	}
	err = m.verifyReader(resp.Body)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		_, err := a.db.exec("delete from webmentions where source = @source and target = @target", sql.Named("source", m.Source), sql.Named("target", m.Target))
		return err
	}
	if cr := []rune(m.Content); len(cr) > 500 {
		m.Content = string(cr[0:497]) + "…"
	}
	if tr := []rune(m.Title); len(tr) > 60 {
		m.Title = string(tr[0:57]) + "…"
	}
	newStatus := webmentionStatusVerified
	if a.db.webmentionExists(m.Source, m.Target) {
		_, err = a.db.exec("update webmentions set status = @status, title = @title, content = @content, author = @author where source = @source and target = @target",
			sql.Named("status", newStatus), sql.Named("title", m.Title), sql.Named("content", m.Content), sql.Named("author", m.Author), sql.Named("source", m.Source), sql.Named("target", m.Target))
	} else {
		_, err = a.db.exec("insert into webmentions (source, target, created, status, title, content, author) values (@source, @target, @created, @status, @title, @content, @author)",
			sql.Named("source", m.Source), sql.Named("target", m.Target), sql.Named("created", m.Created), sql.Named("status", newStatus), sql.Named("title", m.Title), sql.Named("content", m.Content), sql.Named("author", m.Author))
		a.sendNotification(fmt.Sprintf("New webmention from %s to %s", m.Source, m.Target))
	}
	return err
}

func (m *mention) verifyReader(body io.Reader) error {
	var linksBuffer, gqBuffer, mfBuffer bytes.Buffer
	if _, err := io.Copy(io.MultiWriter(&linksBuffer, &gqBuffer, &mfBuffer), body); err != nil {
		return err
	}
	// Check if source mentions target
	links, err := allLinksFromHTML(&linksBuffer, m.Source)
	if err != nil {
		return err
	}
	if _, hasLink := funk.FindString(links, func(s string) bool {
		return unescapedPath(s) == unescapedPath(m.Target)
	}); !hasLink {
		return errors.New("target not found in source")
	}
	// Fill mention attributes
	sourceURL, err := url.Parse(m.Source)
	if err != nil {
		return err
	}
	m.Title = ""
	m.Content = ""
	m.Author = ""
	m.fillFromData(microformats.Parse(&mfBuffer, sourceURL))
	// Set title when content is empty as well
	if m.Title == "" && m.Content == "" {
		doc, err := goquery.NewDocumentFromReader(&gqBuffer)
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
	for _, i := range mf.Items {
		m.fill(i)
	}
}

func (m *mention) fill(mf *microformats.Microformat) bool {
	if mfHasType(mf, "h-entry") {
		// Check URL
		if url, ok := mf.Properties["url"]; ok && len(url) > 0 {
			if url0, ok := url[0].(string); ok {
				if url0 != m.Source {
					// Not correct URL
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
		return true
	}
	for _, mfc := range mf.Children {
		if m.fill(mfc) {
			return true
		}
	}
	return false
}

func (m *mention) fillTitle(mf *microformats.Microformat) {
	if name, ok := mf.Properties["name"]; ok && len(name) > 0 {
		if title, ok := name[0].(string); ok {
			m.Title = strings.TrimSpace(title)
		}
	}
}

func (m *mention) fillContent(mf *microformats.Microformat) {
	if contents, ok := mf.Properties["content"]; ok && len(contents) > 0 {
		if content, ok := contents[0].(map[string]string); ok {
			if contentValue, ok := content["value"]; ok {
				m.Content = strings.TrimSpace(contentValue)
			}
		}
	}
}

func (m *mention) fillAuthor(mf *microformats.Microformat) {
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
