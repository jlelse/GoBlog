package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/sync/singleflight"
)

const (
	cacheInternalExpirationHeader = "GoBlog-Expire"
)

var (
	cacheGroup singleflight.Group
	cacheLru   *lru.Cache
)

func initCache() (err error) {
	cacheLru, err = lru.New(500)
	return
}

func cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if appConfig.Cache.Enable &&
			// check method
			(r.Method == http.MethodGet || r.Method == http.MethodHead) &&
			// check bypass query
			!(r.URL.Query().Get("cache") == "0" || r.URL.Query().Get("cache") == "false") {
			key := cacheKey(r)
			// Get cache or render it
			cacheInterface, _, _ := cacheGroup.Do(key, func() (interface{}, error) {
				return getCache(key, next, r), nil
			})
			cache := cacheInterface.(*cacheItem)
			var expiresIn int64 = 0
			if cache.expiration != 0 {
				expiresIn = cache.creationTime + int64(cache.expiration) - time.Now().Unix()
			}
			// copy cached headers
			for k, v := range cache.header {
				w.Header()[k] = v
			}
			setCacheHeaders(w, cache.hash, expiresIn)
			// check conditional request
			if ifNoneMatchHeader := r.Header.Get("If-None-Match"); ifNoneMatchHeader != "" && ifNoneMatchHeader == cache.hash {
				// send 304
				w.WriteHeader(http.StatusNotModified)
				return
			}
			// set status code
			w.WriteHeader(cache.code)
			// write cached body
			_, _ = w.Write(cache.body)
			return
		}
		next.ServeHTTP(w, r)
		return
	})
}

func cacheKey(r *http.Request) string {
	return r.URL.String()
}

func setCacheHeaders(w http.ResponseWriter, hash string, expiresIn int64) {
	w.Header().Del(cacheInternalExpirationHeader)
	w.Header().Set("ETag", hash)
	if expiresIn != 0 {
		// Set expires time
		w.Header().Set("Cache-Control", fmt.Sprintf("public,max-age=%d", expiresIn))
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public,max-age=%d,s-max-age=%d", appConfig.Cache.Expiration, appConfig.Cache.Expiration/3))
	}
}

type cacheItem struct {
	expiration   int
	creationTime int64
	hash         string
	code         int
	header       http.Header
	body         []byte
}

func (c *cacheItem) expired() bool {
	if c.expiration != 0 {
		return c.creationTime < time.Now().Unix()-int64(c.expiration)
	}
	return false
}

func getCache(key string, next http.Handler, r *http.Request) (item *cacheItem) {
	if lruItem, ok := cacheLru.Get(key); ok {
		item = lruItem.(*cacheItem)
	}
	if item == nil || item.expired() {
		// No cache available
		// Record request
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, r)
		// Cache values from recorder
		result := recorder.Result()
		body, _ := ioutil.ReadAll(result.Body)
		_ = result.Body.Close()
		h := sha256.New()
		_, _ = io.Copy(h, bytes.NewReader(body))
		hash := fmt.Sprintf("%x", h.Sum(nil))
		exp, _ := strconv.Atoi(result.Header.Get(cacheInternalExpirationHeader))
		item = &cacheItem{
			expiration:   exp,
			creationTime: time.Now().Unix(),
			hash:         hash,
			code:         result.StatusCode,
			header:       result.Header,
			body:         body,
		}
		// Save cache
		cacheLru.Add(key, item)
	}
	return item
}

func purgeCache() {
	cacheLru.Purge()
}

func setInternalCacheExpirationHeader(w http.ResponseWriter, expiration int) {
	w.Header().Set(cacheInternalExpirationHeader, strconv.Itoa(expiration))
}
