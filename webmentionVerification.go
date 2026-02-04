package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/bodylimit"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) initWebmentionQueue() {
	a.listenOnQueue("wm", 30*time.Second, func(qi *queueItem, dequeue func(), reschedule func(time.Duration)) {
		var m mention
		if err := gob.NewDecoder(bytes.NewReader(qi.content)).Decode(&m); err != nil {
			a.error("webmention queue error", "err", err)
			dequeue()
			return
		}
		if err := a.verifyMention(&m); err != nil {
			a.error("Failed to verify webmention", "source", m.Source, "target", m.Target, "err", err)
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
		a.debug("Webmention for unknown path", "target", m.Target)
		return a.db.deleteWebmention(m)
	}
	// Check if target has a redirect
	if respReq := targetResp.Request; respReq != nil {
		if ru := respReq.URL; m.Target != ru.String() {
			m.NewTarget = ru.String()
		}
	}
	// Request source
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer timeoutCancel()
	sourceReq, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, m.Source, nil)
	if err != nil {
		return err
	}
	sourceReq.Header.Set("Accept", contenttype.HTMLUTF8)
	var sourceResp *http.Response
	if a.isLocalURL(m.Source) {
		setLoggedIn(sourceReq, true)
		sourceResp, err = doHandlerRequest(sourceReq, a.getAppRouter())
		if err != nil {
			return err
		}
	} else {
		sourceResp, err = a.httpClient.Do(sourceReq)
		if err != nil {
			return err
		}
	}
	defer sourceResp.Body.Close()
	// Check if source has a valid status code
	if sourceResp.StatusCode != http.StatusOK {
		a.debug("Delete webmention because source doesn't have valid status code", "source", m.Source)
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
		a.debug("Delete webmention because verifying source threw error", "source", m.Source, "err", err)
		return a.db.deleteWebmention(m)
	}
	newStatus := webmentionStatusVerified
	// Update or insert webmention
	if a.db.webmentionExists(m) {
		a.debug("Update webmention", "source", m.Source, "target", m.Target)
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
		a.sendNotification(fmt.Sprintf("New webmention from %s to %s", cmp.Or(m.NewSource, m.Source), cmp.Or(m.NewTarget, m.Target)))
	}
	return err
}

func (a *goBlog) verifyReader(m *mention, body io.Reader) error {
	mfBuffer := bufferpool.Get()
	defer bufferpool.Put(mfBuffer)
	pr, pw := io.Pipe()
	go func() {
		_, err := io.Copy(io.MultiWriter(pw, mfBuffer), io.LimitReader(body, 10*bodylimit.MB))
		_ = pw.CloseWithError(err)
	}()
	// Check if source mentions target
	links, err := allLinksFromHTML(pr, cmp.Or(m.NewSource, m.Source))
	_ = pr.CloseWithError(err)
	if err != nil {
		return err
	}
	if _, hasLink := lo.Find(links, func(s string) bool {
		// Check if link belongs to this GoBlog installation
		if !a.isLocalURL(s) {
			return false
		}
		// Check if link is or redirects to target
		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer timeoutCancel()
		req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, m.Target, nil)
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
		if resp.StatusCode == http.StatusOK && lowerUnescapedPath(resp.Request.URL.String()) == lowerUnescapedPath(cmp.Or(m.NewTarget, m.Target)) {
			return true
		}
		return false
	}); !hasLink {
		return errors.New("target not found in source")
	}
	// Fill mention attributes
	mf, err := parseMicroformatsFromReader(cmp.Or(m.NewSource, m.Source), mfBuffer)
	if err != nil {
		return err
	}
	m.Title, m.Content, m.Author, m.Url = mf.Title, mf.Content, mf.Author, cmp.Or(mf.Url, m.Source)
	return nil
}
