package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/PuerkitoBio/goquery"
	"github.com/joncrlsn/dque"
	"willnorris.com/go/microformats"
)

var wmQueue *dque.DQue

func wmMentionBuilder() interface{} {
	return &mention{}
}

func initWebmentionQueue() (err error) {
	queuePath := "queues"
	if _, err := os.Stat(queuePath); os.IsNotExist(err) {
		os.Mkdir(queuePath, 0755)
	}
	wmQueue, err = dque.NewOrOpen("webmention", queuePath, 5, wmMentionBuilder)
	if err != nil {
		return err
	}
	startWebmentionQueue()
	return nil
}

func startWebmentionQueue() {
	go func() {
		for {
			if i, err := wmQueue.PeekBlock(); err == nil {
				if i == nil {
					// Empty request
					_, _ = wmQueue.Dequeue()
					continue
				}
				if m, ok := i.(*mention); ok {
					err = m.verifyMention()
					if err != nil {
						log.Println(fmt.Sprintf("Failed to verify webmention from %s to %s: %s", m.Source, m.Target, err.Error()))
					}
					_, _ = wmQueue.Dequeue()
				} else {
					// Invalid type
					_, _ = wmQueue.Dequeue()
				}
			}
		}
	}()
}

func queueMention(m *mention) error {
	return wmQueue.Enqueue(m)
}

func (m *mention) verifyMention() error {
	req, err := http.NewRequest(http.MethodGet, m.Source, nil)
	if err != nil {
		return err
	}
	req.Header.Set(userAgent, appUserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	err = m.verifyReader(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		_, err := appDbExec("delete from webmentions where source = @source and target = @target", sql.Named("source", m.Source), sql.Named("target", m.Target))
		return err
	}
	if len(m.Content) > 500 {
		m.Content = m.Content[0:497] + "…"
	}
	if len(m.Title) > 60 {
		m.Title = m.Title[0:57] + "…"
	}
	newStatus := webmentionStatusVerified
	if webmentionExists(m.Source, m.Target) {
		_, err = appDbExec("update webmentions set status = @status, title = @title, content = @content, author = @author where source = @source and target = @target",
			sql.Named("status", newStatus), sql.Named("title", m.Title), sql.Named("content", m.Content), sql.Named("author", m.Author), sql.Named("source", m.Source), sql.Named("target", m.Target))
	} else {
		_, err = appDbExec("insert into webmentions (source, target, created, status, title, content, author) values (@source, @target, @created, @status, @title, @content, @author)",
			sql.Named("source", m.Source), sql.Named("target", m.Target), sql.Named("created", m.Created), sql.Named("status", newStatus), sql.Named("title", m.Title), sql.Named("content", m.Content), sql.Named("author", m.Author))
		sendNotification(fmt.Sprintf("New webmention from %s to %s", m.Source, m.Target))
	}
	return err
}

func (m *mention) verifyReader(body io.Reader) error {
	var linksBuffer, gqBuffer, mfBuffer bytes.Buffer
	io.Copy(io.MultiWriter(&linksBuffer, &gqBuffer, &mfBuffer), body)
	// Check if source mentions target
	links, err := allLinksFromHTML(&linksBuffer, m.Source)
	if err != nil {
		return err
	}
	hasLink := false
	for _, link := range links {
		if unescapedPath(link) == unescapedPath(m.Target) {
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
	m.fillFromData(microformats.Parse(&mfBuffer, sourceURL))
	return nil
}

func (m *mention) fillFromData(mf *microformats.Data) {
	for _, i := range mf.Items {
		m.fill(i)
	}
}

func (m *mention) fill(mf *microformats.Microformat) bool {
	if mfHasType(mf, "h-entry") {
		if name, ok := mf.Properties["name"]; ok && len(name) > 0 {
			if title, ok := name[0].(string); ok {
				m.Title = title
			}
		}
		if contents, ok := mf.Properties["content"]; ok && len(contents) > 0 {
			if content, ok := contents[0].(map[string]string); ok {
				if contentValue, ok := content["value"]; ok {
					m.Content = contentValue
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
			if m.fill(mfc) {
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
