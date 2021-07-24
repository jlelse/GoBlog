package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/dgraph-io/ristretto"
	"golang.org/x/sync/singleflight"
)

const cacheInternalExpirationHeader = "Goblog-Expire"

type cache struct {
	g   singleflight.Group
	c   *ristretto.Cache
	cfg *configCache
}

func (a *goBlog) initCache() (err error) {
	a.cache = &cache{
		cfg: a.cfg.Cache,
	}
	if a.cache.cfg != nil && !a.cache.cfg.Enable {
		return nil
	}
	a.cache.c, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 40 * 1000,        // 4000 items when full with 5 KB items -> x10 = 40.000
		MaxCost:     20 * 1000 * 1000, // 20 MB
		BufferItems: 64,               // recommended
	})
	return
}

func (a *goBlog) cacheMiddleware(next http.Handler) http.Handler {
	c := a.cache
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.c == nil {
			// No cache configured
			next.ServeHTTP(w, r)
			return
		}
		// Do checks
		if !(r.Method == http.MethodGet || r.Method == http.MethodHead) {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Query().Get("cache") == "0" || r.URL.Query().Get("cache") == "false" {
			next.ServeHTTP(w, r)
			return
		}
		if a.isLoggedIn(r) {
			next.ServeHTTP(w, r)
			return
		}
		// Search and serve cache
		key := cacheKey(r)
		// Get cache or render it
		cacheInterface, _, _ := c.g.Do(key, func() (interface{}, error) {
			return c.getCache(key, next, r), nil
		})
		ci := cacheInterface.(*cacheItem)
		// copy cached headers
		for k, v := range ci.header {
			w.Header()[k] = v
		}
		c.setCacheHeaders(w, ci)
		// check conditional request
		if ifNoneMatchHeader := r.Header.Get("If-None-Match"); ifNoneMatchHeader != "" && ifNoneMatchHeader == ci.eTag {
			// send 304
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if ifModifiedSinceHeader := r.Header.Get("If-Modified-Since"); ifModifiedSinceHeader != "" {
			if t, err := dateparse.ParseAny(ifModifiedSinceHeader); err == nil && t.After(ci.creationTime) {
				// send 304
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		// set status code
		w.WriteHeader(ci.code)
		// write cached body
		_, _ = w.Write(ci.body)
	})
}

func cacheKey(r *http.Request) string {
	var buf strings.Builder
	// Special cases
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		buf.WriteString("as-")
	}
	if torUsed, ok := r.Context().Value(torUsedKey).(bool); ok && torUsed {
		buf.WriteString("tor-")
	}
	// Add cache URL
	_, _ = buf.WriteString(r.URL.EscapedPath())
	if q := r.URL.Query(); len(q) > 0 {
		_ = buf.WriteByte('?')
		_, _ = buf.WriteString(q.Encode())
	}
	// Return string
	return buf.String()
}

func (c *cache) setCacheHeaders(w http.ResponseWriter, cache *cacheItem) {
	w.Header().Set("ETag", cache.eTag)
	w.Header().Set("Last-Modified", cache.creationTime.UTC().Format(http.TimeFormat))
	if w.Header().Get("Cache-Control") == "" {
		if cache.expiration != 0 {
			w.Header().Set("Cache-Control", fmt.Sprintf("public,max-age=%d,stale-while-revalidate=%d", cache.expiration, cache.expiration))
		} else {
			w.Header().Set("Cache-Control", fmt.Sprintf("public,max-age=%d,s-max-age=%d,stale-while-revalidate=%d", c.cfg.Expiration, c.cfg.Expiration/3, c.cfg.Expiration))
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

// Calculate byte size of cache item using size of body and header
func (ci *cacheItem) cost() int64 {
	var headerBuf strings.Builder
	_ = ci.header.Write(&headerBuf)
	headerSize := int64(binary.Size(headerBuf.String()))
	bodySize := int64(binary.Size(ci.body))
	return headerSize + bodySize
}

func (c *cache) getCache(key string, next http.Handler, r *http.Request) (item *cacheItem) {
	if rItem, ok := c.c.Get(key); ok {
		item = rItem.(*cacheItem)
	}
	if item == nil {
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
		if cch := item.header.Get("Cache-Control"); !containsStrings(cch, "no-store", "private", "no-cache") {
			if exp == 0 {
				c.c.Set(key, item, item.cost())
			} else {
				c.c.SetWithTTL(key, item, item.cost(), time.Duration(exp)*time.Second)
			}
		}
	}
	return item
}

func (c *cache) purge() {
	c.c.Clear()
}

func (a *goBlog) setInternalCacheExpirationHeader(w http.ResponseWriter, r *http.Request, expiration int) {
	if a.isLoggedIn(r) {
		return
	}
	w.Header().Set(cacheInternalExpirationHeader, strconv.Itoa(expiration))
}
