package main

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/samber/go-singleflightx"
	"go.goblog.app/app/pkgs/bodylimit"
	"go.goblog.app/app/pkgs/bufferpool"
	c "go.goblog.app/app/pkgs/cache"
	"go.goblog.app/app/pkgs/contenttype"
)

const (
	cacheLoggedInKey   contextKey = "cacheLoggedIn"
	cacheExpirationKey contextKey = "cacheExpiration"
	publicPostCacheKey contextKey = "publicPostCache"
	cacheControl                  = "Cache-Control"
	cdnCacheableHeader            = "X-GoBlog-Cacheable"
)

type cache struct {
	g singleflightx.Group[string, *cacheItem]
	c *c.Cache[string, *cacheItem]
}

func (a *goBlog) initCache() {
	a.initCacheOnce.Do(func() {
		if a.cfg.Cache != nil && !a.cfg.Cache.Enable {
			return
		}
		a.cache = &cache{
			c: c.New[string, *cacheItem](time.Minute, 20*bodylimit.MB),
		}
	})
}

func (a *goBlog) purgeCache() {
	a.initCache()
	a.cache.purge()
}

func cacheLoggedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), cacheLoggedInKey, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *goBlog) CacheMiddleware(next http.Handler) http.Handler {
	a.initCache()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.cache == nil || a.cache.c == nil || !isCacheable(r) || a.shouldSkipLoggedIn(r) {
			next.ServeHTTP(w, r)
			return
		}

		key := generateCacheKey(r)
		ci, _, _ := a.cache.g.Do(key, func() (*cacheItem, error) {
			return a.cache.getOrCreateCache(key, next, r), nil
		})

		a.serveCachedResponse(w, r, ci)
	})
}

func isCacheable(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	// Skip parsing the query string if it cannot contain the cache parameter.
	if !rawQueryMayContain(r, "cache") {
		return true
	}
	cacheParam := r.URL.Query().Get("cache")
	return cacheParam != "0" && cacheParam != "false"
}

func (a *goBlog) shouldSkipLoggedIn(r *http.Request) bool {
	if cli, ok := r.Context().Value(cacheLoggedInKey).(bool); ok && cli {
		setLoggedIn(r, false)
		return false
	}
	return a.isLoggedIn(r)
}

func generateCacheKey(r *http.Request) string {
	altAddress, _ := r.Context().Value(altAddressKey).(string)
	asRequest, _ := r.Context().Value(asRequestKey).(bool)
	torUsed, _ := r.Context().Value(torUsedKey).(bool)
	escapedPath := r.URL.EscapedPath()
	if altAddress == "" && !asRequest && !torUsed && r.URL.RawQuery == "" {
		return escapedPath
	}

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	// Special cases
	if altAddress != "" {
		buf.WriteString(altAddress)
		buf.WriteByte('-')
	}
	if asRequest {
		buf.WriteString("as-")
	}
	if torUsed {
		buf.WriteString("tor-")
	}
	// Add cache URL
	buf.WriteString(escapedPath)
	if r.URL.RawQuery != "" {
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
	}

	return buf.String()
}

func (a *goBlog) serveCachedResponse(w http.ResponseWriter, r *http.Request, ci *cacheItem) {
	header := w.Header()
	maps.Copy(header, ci.header)
	header.Set("ETag", ci.eTag)
	header.Del(cdnCacheableHeader)
	if canStoreResponse(header) {
		header.Set(cacheControl, "public,no-cache")
		publicPost, _ := r.Context().Value(publicPostCacheKey).(bool)
		mediaType, _, _ := strings.Cut(header.Get(contentType), ";")
		if publicPost && ci.code == http.StatusOK && mediaType == contenttype.HTML &&
			len(r.Header.Values("Cookie")) == 0 && len(r.Header.Values("Authorization")) == 0 {
			header.Set(cdnCacheableHeader, "public-html")
		}
	}

	if tag := r.Header.Get("If-None-Match"); ci.code == http.StatusOK && tag != "" &&
		strings.TrimPrefix(tag, "W/") == strings.TrimPrefix(ci.eTag, "W/") {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.WriteHeader(ci.code)
	_, _ = w.Write(ci.body)
}

func (c *cache) getOrCreateCache(key string, next http.Handler, r *http.Request) *cacheItem {
	if rItem, ok := c.c.Get(key); ok {
		return rItem
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

	if canStoreResponse(item.header) {
		c.saveCache(key, item)
	}

	return item
}

var (
	conditionalHeaders = [...]string{"If-Modified-Since", "If-Unmodified-Since", "If-None-Match", "If-Match", "If-Range", "Range"}
	problematicHeaders = [...]string{"Accept-Ranges", "ETag", "Last-Modified"}
)

func removeConditionalHeaders(r *http.Request) {
	for _, h := range conditionalHeaders {
		r.Header.Del(h)
	}
}

func removeProblematicHeaders(header http.Header) {
	for _, h := range problematicHeaders {
		header.Del(h)
	}
}

func canStoreResponse(header http.Header) bool {
	if len(header.Values("Set-Cookie")) > 0 {
		return false
	}
	for _, value := range header.Values(cacheControl) {
		if containsStrings(strings.ToLower(value), "no-store", "private", "no-cache") {
			return false
		}
	}
	return true
}

func (c *cache) saveCache(key string, item *cacheItem) {
	ttl := 6 * time.Hour
	if item.expiration > 0 {
		ttl = time.Duration(item.expiration) * time.Second
	}
	c.c.Set(key, item, ttl, item.cost())
}

func (c *cache) purge() {
	if c == nil || c.c == nil {
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
