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
	"strings"
	"time"

	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) initWebmentionQueue() {
	a.listenOnQueue("wm", 30*time.Second, func(qi *queueItem, dequeue func(), reschedule func(time.Duration)) {
		var m mention
		if err := gob.NewDecoder(bytes.NewReader(qi.content)).Decode(&m); err != nil {
			log.Println("webmention queue:", err.Error())
			dequeue()
			return
		}
		if err := a.verifyMention(&m); err != nil {
			log.Printf("Failed to verify webmention from %s to %s: %s", m.Source, m.Target, err.Error())
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
	return a.enqueue("wm", buf.Bytes(), time.Now())
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
	mfBuffer := bufferpool.Get()
	defer bufferpool.Put(mfBuffer)
	pr, pw := io.Pipe()
	go func() {
		_, err := io.Copy(io.MultiWriter(pw, mfBuffer), body)
		_ = pw.CloseWithError(err)
	}()
	// Check if source mentions target
	links, err := allLinksFromHTML(pr, defaultIfEmpty(m.NewSource, m.Source))
	_ = pr.CloseWithError(err)
	if err != nil {
		return err
	}
	if _, hasLink := lo.Find(links, func(s string) bool {
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
		if resp.StatusCode == http.StatusOK && lowerUnescapedPath(resp.Request.URL.String()) == lowerUnescapedPath(defaultIfEmpty(m.NewTarget, m.Target)) {
			return true
		}
		return false
	}); !hasLink {
		return errors.New("target not found in source")
	}
	// Fill mention attributes
	mf, err := parseMicroformatsFromReader(defaultIfEmpty(m.NewSource, m.Source), mfBuffer)
	if err != nil {
		return err
	}
	m.Title, m.Content, m.Author, m.Url = mf.Title, mf.Content, mf.Author, defaultIfEmpty(mf.Url, m.Source)
	return nil
}
