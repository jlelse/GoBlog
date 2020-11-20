package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	cacheLru, err = lru.New(200)
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
			cacheTimeString := time.Unix(cache.creationTime, 0).Format(time.RFC1123)
			expiresTimeString := ""
			if cache.expiration != 0 {
				expiresTimeString = time.Unix(cache.creationTime+int64(cache.expiration), 0).Format(time.RFC1123)
			}
			// check conditional request
			if ifModifiedSinceHeader := r.Header.Get("If-Modified-Since"); ifModifiedSinceHeader != "" {
				if t, _ := dateparse.ParseIn(ifModifiedSinceHeader, time.Local); t.Unix() == cache.creationTime {
					// send 304
					setCacheHeaders(w, cacheTimeString, expiresTimeString)
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
			// copy cached headers
			for k, v := range cache.header {
				w.Header()[k] = v
			}
			setCacheHeaders(w, cacheTimeString, expiresTimeString)
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

func setCacheHeaders(w http.ResponseWriter, cacheTimeString string, expiresTimeString string) {
	w.Header().Del(cacheInternalExpirationHeader)
	w.Header().Set("Last-Modified", cacheTimeString)
	if expiresTimeString != "" {
		// Set expires time
		w.Header().Set("Cache-Control", "public")
		w.Header().Set("Expires", expiresTimeString)
	} else {
		w.Header().Set("Cache-Control", fmt.Sprintf("public,max-age=%d", appConfig.Cache.Expiration))
	}
}

type cacheItem struct {
	creationTime int64
	expiration   int
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
		exp, _ := strconv.Atoi(result.Header.Get(cacheInternalExpirationHeader))
		item = &cacheItem{
			creationTime: time.Now().Unix(),
			expiration:   exp,
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
