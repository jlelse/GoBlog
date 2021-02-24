package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
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
		// Do checks
		if !appConfig.Cache.Enable {
			next.ServeHTTP(w, r)
			return
		}
		if !(r.Method == http.MethodGet || r.Method == http.MethodHead) {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Query().Get("cache") == "0" || r.URL.Query().Get("cache") == "false" {
			next.ServeHTTP(w, r)
			return
		}
		if loggedIn, ok := r.Context().Value(loggedInKey).(bool); ok && loggedIn {
			next.ServeHTTP(w, r)
			return
		}
		// Search and serve cache
		key := cacheKey(r)
		// Get cache or render it
		cacheInterface, _, _ := cacheGroup.Do(key, func() (interface{}, error) {
			return getCache(key, next, r), nil
		})
		cache := cacheInterface.(*cacheItem)
		// copy cached headers
		for k, v := range cache.header {
			w.Header()[k] = v
		}
		setCacheHeaders(w, cache)
		// check conditional request
		if ifNoneMatchHeader := r.Header.Get("If-None-Match"); ifNoneMatchHeader != "" && ifNoneMatchHeader == cache.eTag {
			// send 304
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if ifModifiedSinceHeader := r.Header.Get("If-Modified-Since"); ifModifiedSinceHeader != "" {
			if t, err := dateparse.ParseAny(ifModifiedSinceHeader); err == nil && t.After(cache.creationTime) {
				// send 304
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		// set status code
		w.WriteHeader(cache.code)
		// write cached body
		_, _ = w.Write(cache.body)
	})
}

func cacheKey(r *http.Request) string {
	def := cacheURLString(r.URL)
	// Special cases
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		return "as-" + def
	}
	// Default
	return def
}

func cacheURLString(u *url.URL) string {
	var buf strings.Builder
	_, _ = buf.WriteString(u.EscapedPath())
	if u.RawQuery != "" {
		_ = buf.WriteByte('?')
		_, _ = buf.WriteString(u.RawQuery)
	}
	return buf.String()
}

func setCacheHeaders(w http.ResponseWriter, cache *cacheItem) {
	w.Header().Set("ETag", cache.eTag)
	w.Header().Set("Last-Modified", cache.creationTime.UTC().Format(http.TimeFormat))
	if w.Header().Get("Cache-Control") == "" {
		if cache.expiration != 0 {
			w.Header().Set("Cache-Control", fmt.Sprintf("public,max-age=%d,stale-while-revalidate=%d", cache.expiration, cache.expiration))
		} else {
			w.Header().Set("Cache-Control", fmt.Sprintf("public,max-age=%d,s-max-age=%d,stale-while-revalidate=%d", appConfig.Cache.Expiration, appConfig.Cache.Expiration/3, appConfig.Cache.Expiration))
		}
	}
}

type cacheItem struct {
	expiration   int
	creationTime time.Time
	eTag         string
	code         int
	header       http.Header
	body         []byte
}

func (c *cacheItem) expired() bool {
	if c.expiration != 0 {
		return time.Now().After(c.creationTime.Add(time.Duration(c.expiration) * time.Second))
	}
	return false
}

func getCache(key string, next http.Handler, r *http.Request) (item *cacheItem) {
	if lruItem, ok := cacheLru.Get(key); ok {
		item = lruItem.(*cacheItem)
	}
	if item == nil || item.expired() {
		// No cache available
		// Remove problematic headers
		r.Header.Del("If-Modified-Since")
		r.Header.Del("If-Unmodified-Since")
		r.Header.Del("If-None-Match")
		r.Header.Del("If-Match")
		r.Header.Del("If-Range")
		r.Header.Del("Range")
		// Record request
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, r)
		// Cache values from recorder
		result := recorder.Result()
		body, _ := io.ReadAll(result.Body)
		_ = result.Body.Close()
		eTag := result.Header.Get("ETag")
		if eTag == "" {
			h := sha256.New()
			_, _ = io.Copy(h, bytes.NewReader(body))
			eTag = fmt.Sprintf("%x", h.Sum(nil))
		}
		lastMod := time.Now()
		if lm := result.Header.Get("Last-Modified"); lm != "" {
			if parsedTime, te := dateparse.ParseLocal(lm); te == nil {
				lastMod = parsedTime
			}
		}
		exp, _ := strconv.Atoi(result.Header.Get(cacheInternalExpirationHeader))
		// Remove problematic headers
		result.Header.Del(cacheInternalExpirationHeader)
		result.Header.Del("Accept-Ranges")
		result.Header.Del("ETag")
		result.Header.Del("Last-Modified")
		// Create cache item
		item = &cacheItem{
			expiration:   exp,
			creationTime: lastMod,
			eTag:         eTag,
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
