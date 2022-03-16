package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/araddon/dateparse"
	"github.com/dgraph-io/ristretto"
	"go.goblog.app/app/pkgs/bufferpool"
	"golang.org/x/sync/singleflight"
)

const (
	cacheLoggedInKey   contextKey = "cacheLoggedIn"
	cacheExpirationKey contextKey = "cacheExpiration"

	lastModified = "Last-Modified"
	cacheControl = "Cache-Control"
)

type cache struct {
	g singleflight.Group
	c *ristretto.Cache
}

func (a *goBlog) initCache() (err error) {
	a.cache = &cache{}
	if a.cfg.Cache != nil && !a.cfg.Cache.Enable {
		// Cache disabled
		return nil
	}
	a.cache.c, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 40 * 1000,        // 4000 items when full with 5 KB items -> x10 = 40.000
		MaxCost:     20 * 1000 * 1000, // 20 MB
		BufferItems: 64,               // recommended
		Metrics:     true,
	})
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		for range ticker.C {
			met := a.cache.c.Metrics
			log.Println("\nCache:", met.String())
		}
	}()
	return
}

func cacheLoggedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), cacheLoggedInKey, true)))
	})
}

func (a *goBlog) cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do checks
		if a.cache.c == nil || !cacheable(r) {
			next.ServeHTTP(w, r)
			return
		}
		// Check login
		if cli, ok := r.Context().Value(cacheLoggedInKey).(bool); ok && cli {
			// Continue caching, but remove login
			setLoggedIn(r, false)
		} else if a.isLoggedIn(r) {
			// Don't cache logged in requests
			next.ServeHTTP(w, r)
			return
		}
		// Search and serve cache
		key := cacheKey(r)
		// Get cache or render it
		cacheInterface, _, _ := a.cache.g.Do(key, func() (any, error) {
			return a.cache.getCache(key, next, r), nil
		})
		ci := cacheInterface.(*cacheItem)
		// copy and set headers
		a.setCacheHeaders(w, ci)
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

func cacheable(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.URL.Query().Get("cache") == "0" || r.URL.Query().Get("cache") == "false" {
		return false
	}
	return true
}

func cacheKey(r *http.Request) (key string) {
	buf := bufferpool.Get()
	// Special cases
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		_, _ = buf.WriteString("as-")
	}
	if torUsed, ok := r.Context().Value(torUsedKey).(bool); ok && torUsed {
		_, _ = buf.WriteString("tor-")
	}
	// Add cache URL
	_, _ = buf.WriteString(r.URL.EscapedPath())
	if query := r.URL.Query(); len(query) > 0 {
		_ = buf.WriteByte('?')
		keys := make([]string, 0, len(query))
		for k := range query {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			keyEscaped := url.QueryEscape(k)
			for j, val := range query[k] {
				if i > 0 || j > 0 {
					buf.WriteByte('&')
				}
				buf.WriteString(keyEscaped)
				buf.WriteByte('=')
				buf.WriteString(url.QueryEscape(val))
			}
		}
	}
	// Get key as string
	key = buf.String()
	// Return buffer to pool
	bufferpool.Put(buf)
	return
}

func (a *goBlog) setCacheHeaders(w http.ResponseWriter, cache *cacheItem) {
	// Copy headers
	for k, v := range cache.header.Clone() {
		w.Header()[k] = v
	}
	// Set cache headers
	w.Header().Set("ETag", cache.eTag)
	w.Header().Set(lastModified, cache.creationTime.UTC().Format(http.TimeFormat))
	if w.Header().Get(cacheControl) == "" {
		if cache.expiration != 0 {
			w.Header().Set(cacheControl, fmt.Sprintf("public,max-age=%d,stale-while-revalidate=%d", cache.expiration, cache.expiration))
		} else {
			exp := a.cfg.Cache.Expiration
			w.Header().Set(cacheControl, fmt.Sprintf("public,max-age=%d,s-max-age=%d,stale-while-revalidate=%d", exp, exp/3, exp))
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

// Calculate byte size of cache item using size of header, body and etag
func (ci *cacheItem) cost() int {
	headerBuf := bufferpool.Get()
	_ = ci.header.Write(headerBuf)
	headerSize := len(headerBuf.Bytes())
	bufferpool.Put(headerBuf)
	return headerSize + len(ci.body) + len(ci.eTag)
}

func (c *cache) getCache(key string, next http.Handler, r *http.Request) *cacheItem {
	if rItem, ok := c.c.Get(key); ok {
		return rItem.(*cacheItem)
	}
	// No cache available
	// Make and use copy of r
	cr := r.Clone(valueOnlyContext{r.Context()})
	// Remove problematic headers
	cr.Header.Del("If-Modified-Since")
	cr.Header.Del("If-Unmodified-Since")
	cr.Header.Del("If-None-Match")
	cr.Header.Del("If-Match")
	cr.Header.Del("If-Range")
	cr.Header.Del("Range")
	// Record request
	rec := newCacheRecorder()
	next.ServeHTTP(rec, cr)
	item := rec.finish()
	// Set eTag
	item.eTag = item.header.Get("ETag")
	if item.eTag == "" {
		item.eTag = fmt.Sprintf("%x", sha256.Sum256(item.body))
	}
	// Set creation time
	item.creationTime = time.Now()
	if lm := item.header.Get(lastModified); lm != "" {
		if parsedTime, te := dateparse.ParseLocal(lm); te == nil {
			item.creationTime = parsedTime
		}
	}
	// Set expiration
	item.expiration, _ = cr.Context().Value(cacheExpirationKey).(int)
	// Remove problematic headers
	item.header.Del("Accept-Ranges")
	item.header.Del("ETag")
	item.header.Del(lastModified)
	// Save cache
	if cch := item.header.Get(cacheControl); !containsStrings(cch, "no-store", "private", "no-cache") {
		cost := int64(item.cost())
		if item.expiration == 0 {
			c.c.Set(key, item, cost)
		} else {
			c.c.SetWithTTL(key, item, cost, time.Duration(item.expiration)*time.Second)
		}
	}
	return item
}

func (c *cache) purge() {
	c.c.Clear()
}

func (a *goBlog) defaultCacheExpiration() int {
	if a.cfg.Cache != nil {
		return a.cfg.Cache.Expiration
	}
	return 0
}
