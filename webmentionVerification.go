package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"willnorris.com/go/microformats"
)

func startWebmentionVerifier() {
	go func() {
		for {
			time.Sleep(30 * time.Second)
			verifyNextWebmention()
		}
	}()
}

func verifyNextWebmention() error {
	m := &mention{}
	oldStatus := ""
	row, err := appDbQueryRow("select id, source, target, status from webmentions where (status = ? or status = ?) limit 1", webmentionStatusNew, webmentionStatusRenew)
	if err != nil {
		return err
	}
	if err := row.Scan(&m.ID, &m.Source, &m.Target, &oldStatus); err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return err
	}
	if err := wmVerify(m); err != nil {
		// Invalid
		return deleteWebmention(m.ID)
	}
	if len(m.Content) > 500 {
		m.Content = m.Content[0:497] + "…"
	}
	newStatus := webmentionStatusVerified
	if strings.HasPrefix(m.Source, appConfig.Server.PublicAddress) {
		// Approve if it's server-intern
		newStatus = webmentionStatusApproved
	}
	_, err = appDbExec("update webmentions set status = ?, title = ?, content = ?, author = ? where id = ?", newStatus, m.Title, m.Content, m.Author, m.ID)
	if oldStatus == string(webmentionStatusNew) {
		sendNotification(fmt.Sprintf("New webmention from %s to %s", m.Source, m.Target))
	}
	return err
}

func wmVerify(m *mention) error {
	req, err := http.NewRequest(http.MethodGet, m.Source, nil)
	if err != nil {
		return err
	}
	req.Header.Set(userAgent, appUserAgent)
	resp, err := http.DefaultClient.Do(req)
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
