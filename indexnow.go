package main

import (
	"context"
	"log"
	"net/http"

	"github.com/carlmjohnson/requests"
)

// Implement support for the IndexNow protocol
// https://www.indexnow.org/documentation

func (a *goBlog) initIndexNow() {
	if !a.indexNowEnabled() {
		return
	}
	// Add hooks
	hook := func(p *post) {
		// Check if post is published
		if !p.isPublicPublishedSectionPost() {
			return
		}
		// Send IndexNow request
		a.indexNow(a.fullPostURL(p))
	}
	a.pPostHooks = append(a.pPostHooks, hook)
	a.pUpdateHooks = append(a.pUpdateHooks, hook)
}

func (a *goBlog) indexNowEnabled() bool {
	// Check if private mode is enabled
	if a.isPrivate() {
		return false
	}
	// Check if IndexNow is disabled
	if inc := a.cfg.IndexNow; inc == nil || !inc.Enabled {
		return false
	}
	return true
}

func (a *goBlog) serveIndexNow(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write(a.indexNowKey())
}

func (a *goBlog) indexNow(url string) {
	if !a.indexNowEnabled() {
		return
	}
	key := a.indexNowKey()
	if len(key) == 0 {
		log.Println("Skipping IndexNow")
		return
	}
	err := requests.URL("https://api.indexnow.org/indexnow").
		Client(a.httpClient).
		Param("url", url).
		Param("key", string(key)).
		Fetch(context.Background())
	if err != nil {
		log.Println("Sending IndexNow request failed:", err.Error())
		return
	} else {
		log.Println("IndexNow request sent for", url)
	}
}

func (a *goBlog) indexNowKey() []byte {
	a.inLoad.Do(func() {
		// Try to load key from database
		keyBytes, err := a.db.retrievePersistentCache("indexnowkey")
		if err != nil {
			log.Println("Failed to retrieve cached IndexNow key:", err.Error())
			return
		}
		if keyBytes == nil {
			// Generate 128 character key with hexadecimal characters
			keyBytes = []byte(randomString(128, []rune("0123456789abcdef")...))
			// Store key in database
			err = a.db.cachePersistently("indexnowkey", keyBytes)
			if err != nil {
				log.Println("Failed to cache IndexNow key:", err.Error())
				return
			}
		}
		a.inKey = keyBytes
	})
	return a.inKey
}
