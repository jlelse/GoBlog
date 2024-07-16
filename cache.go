package main

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/dgraph-io/ristretto"
	"go.goblog.app/app/pkgs/bodylimit"
	"go.goblog.app/app/pkgs/bufferpool"
	"golang.org/x/sync/singleflight"
)

const (
	cacheLoggedInKey   contextKey = "cacheLoggedIn"
	cacheExpirationKey contextKey = "cacheExpiration"
	cacheControl                  = "Cache-Control"
)

type cache struct {
	g singleflight.Group
	c *ristretto.Cache
}

func (a *goBlog) initCache() error {
	a.cache = &cache{}
	if a.cfg.Cache != nil && !a.cfg.Cache.Enable {
		return nil // Cache disabled
	}

	c, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 40000,
		MaxCost:     20 * bodylimit.MB,
		BufferItems: 64,
		Metrics:     true,
	})
	if err != nil {
		return err
	}

	a.cache.c = c
	go a.logCacheMetrics()
	return nil
}

func (a *goBlog) logCacheMetrics() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		met := a.cache.c.Metrics
		a.info("Cache metrics", "metrics", met.String())
	}
}

func cacheLoggedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), cacheLoggedInKey, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *goBlog) cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.cache.c == nil || !isCacheable(r) || a.shouldSkipLoggedIn(r) {
			next.ServeHTTP(w, r)
			return
		}

		key := generateCacheKey(r)
		cacheInterface, _, _ := a.cache.g.Do(key, func() (interface{}, error) {
			return a.cache.getOrCreateCache(key, next, r), nil
		})

		ci := cacheInterface.(*cacheItem)
		a.serveCachedResponse(w, r, ci)
	})
}

func isCacheable(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return r.URL.Query().Get("cache") != "0" && r.URL.Query().Get("cache") != "false"
}

func (a *goBlog) shouldSkipLoggedIn(r *http.Request) bool {
	if cli, ok := r.Context().Value(cacheLoggedInKey).(bool); ok && cli {
		setLoggedIn(r, false)
		return false
	}
	return a.isLoggedIn(r)
}

func generateCacheKey(r *http.Request) string {
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	// Special cases
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		buf.WriteString("as-")
	}
	if torUsed, ok := r.Context().Value(torUsedKey).(bool); ok && torUsed {
		buf.WriteString("tor-")
	}
	// Add cache URL
	buf.WriteString(r.URL.EscapedPath())
	if query := r.URL.Query(); len(query) > 0 {
		buf.WriteByte('?')
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

	return buf.String()
}

func (a *goBlog) serveCachedResponse(w http.ResponseWriter, r *http.Request, ci *cacheItem) {
	a.setCacheHeaders(w, ci)

	if ifNoneMatchHeader := r.Header.Get("If-None-Match"); ifNoneMatchHeader != "" && ifNoneMatchHeader == ci.eTag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.WriteHeader(ci.code)
	_, _ = w.Write(ci.body)
}

func (a *goBlog) setCacheHeaders(w http.ResponseWriter, cache *cacheItem) {
	// Copy headers
	for k, v := range cache.header.Clone() {
		w.Header()[k] = v
	}
	// Set cache headers
	w.Header().Set("ETag", cache.eTag)
	w.Header().Set(cacheControl, "public,no-cache")
}

func (c *cache) getOrCreateCache(key string, next http.Handler, r *http.Request) *cacheItem {
	if rItem, ok := c.c.Get(key); ok {
		return rItem.(*cacheItem)
	}

	// Remove original timeout, add new one
	withoutCancelCtx := context.WithoutCancel(r.Context())
	newCancelCtx, cancel := context.WithTimeout(withoutCancelCtx, 5*time.Minute)
	defer cancel()

	cr := r.Clone(newCancelCtx)
	removeConditionalHeaders(cr)

	rec := newCacheRecorder()
	next.ServeHTTP(rec, cr)
	item := rec.finish()

	item.expiration, _ = cr.Context().Value(cacheExpirationKey).(int)
	removeProblematicHeaders(item.header)

	if shouldCacheItem(item.header.Get(cacheControl)) {
		c.saveCache(key, item)
	}

	return item
}

func removeConditionalHeaders(r *http.Request) {
	headers := []string{"If-Modified-Since", "If-Unmodified-Since", "If-None-Match", "If-Match", "If-Range", "Range"}
	for _, h := range headers {
		r.Header.Del(h)
	}
}

func removeProblematicHeaders(header http.Header) {
	headers := []string{"Accept-Ranges", "ETag", "Last-Modified"}
	for _, h := range headers {
		header.Del(h)
	}
}

func shouldCacheItem(cacheControlHeader string) bool {
	return !containsStrings(cacheControlHeader, "no-store", "private", "no-cache")
}

func (c *cache) saveCache(key string, item *cacheItem) {
	ttl := 6 * time.Hour
	if item.expiration > 0 {
		ttl = time.Duration(item.expiration) * time.Second
	}
	c.c.SetWithTTL(key, item, item.cost(), ttl)
	c.c.Wait()
}

func (c *cache) purge() {
	if c == nil {
		return
	}
	c.c.Clear()
}

func (a *goBlog) defaultCacheExpiration() int {
	if a.cfg.Cache != nil {
		return a.cfg.Cache.Expiration
	}
	return 0
}
