package main

import (
	"context"
	"log"
	"net/http"

	"github.com/carlmjohnson/requests"
	"github.com/thoas/go-funk"
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
		if !p.isPublishedSectionPost() {
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

func (a *goBlog) serveIndexNow(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(a.indexNowKey()))
}

func (a *goBlog) indexNow(url string) {
	if !a.indexNowEnabled() {
		return
	}
	key := a.indexNowKey()
	if key == "" {
		log.Println("Skipping IndexNow")
		return
	}
	err := requests.URL("https://api.indexnow.org/indexnow").
		Client(a.httpClient).
		UserAgent(appUserAgent).
		Param("url", url).
		Param("key", key).
		Fetch(context.Background())
	if err != nil {
		log.Println("Sending IndexNow request failed:", err.Error())
		return
	} else {
		log.Println("IndexNow request sent for", url)
	}
}

func (a *goBlog) indexNowKey() string {
	res, _, _ := a.inLoad.Do("", func() (interface{}, error) {
		// Check if already loaded
		if a.inKey != "" {
			return a.inKey, nil
		}
		// Try to load key from database
		keyBytes, err := a.db.retrievePersistentCache("indexnowkey")
		if err != nil {
			log.Println("Failed to retrieve cached IndexNow key:", err.Error())
			return "", err
		}
		if keyBytes == nil {
			// Generate 128 character key with hexadecimal characters
			keyBytes = []byte(funk.RandomString(128, []rune("0123456789abcdef")))
			// Store key in database
			err = a.db.cachePersistently("indexnowkey", keyBytes)
			if err != nil {
				log.Println("Failed to cache IndexNow key:", err.Error())
				return "", err
			}
		}
		a.inKey = string(keyBytes)
		return a.inKey, nil
	})
	return res.(string)
}
