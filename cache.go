package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var (
	cacheGroup singleflight.Group
	cacheMap   = map[string]*cacheItem{}
	cacheMutex = &sync.RWMutex{}
)

func initCache() {
	go func() {
		for {
			// GC the entries every 60 seconds
			time.Sleep(60 * time.Second)
			cacheMutex.Lock()
			for key, item := range cacheMap {
				if item.expired() {
					delete(cacheMap, key)
				}
			}
			cacheMutex.Unlock()
		}
	}()
}

func cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if appConfig.Cache.Enable &&
			// check method
			(r.Method == http.MethodGet || r.Method == http.MethodHead) &&
			// check bypass query
			!(r.URL.Query().Get("cache") == "0") {
			key := cacheKey(r)
			// Get cache or render it
			cacheInterface, _, _ := cacheGroup.Do(key, func() (interface{}, error) {
				return getCache(key, next, r), nil
			})
			cache := cacheInterface.(*cacheItem)
			// log.Println(string(cache.body))
			cacheTimeString := time.Unix(cache.creationTime, 0).Format(time.RFC1123)
			expiresTimeString := time.Unix(cache.creationTime+appConfig.Cache.Expiration, 0).Format(time.RFC1123)
			// check conditional request
			ifModifiedSinceHeader := r.Header.Get("If-Modified-Since")
			if ifModifiedSinceHeader != "" && ifModifiedSinceHeader == cacheTimeString {
				setCacheHeaders(w, cacheTimeString, expiresTimeString)
				// send 304
				w.WriteHeader(http.StatusNotModified)
				return
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
	w.Header().Set("Cache-Control", "public")
	w.Header().Set("Last-Modified", cacheTimeString)
	w.Header().Set("Expires", expiresTimeString)
}

type cacheItem struct {
	creationTime int64
	code         int
	header       http.Header
	body         []byte
}

func (c *cacheItem) expired() bool {
	return c.creationTime < time.Now().Unix()-appConfig.Cache.Expiration
}

func getCache(key string, next http.Handler, r *http.Request) *cacheItem {
	cacheMutex.RLock()
	item, ok := cacheMap[key]
	cacheMutex.RUnlock()
	if !ok || item.expired() {
		// No cache available
		// Record request
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, r)
		// Cache values from recorder
		item = &cacheItem{
			creationTime: time.Now().Unix(),
			code:         recorder.Code,
			header:       recorder.Header(),
			body:         recorder.Body.Bytes(),
		}
		// Save cache
		cacheMutex.Lock()
		cacheMap[key] = item
		cacheMutex.Unlock()
	}
	return item
}

func purgeCache() {
	cacheMutex.Lock()
	cacheMap = map[string]*cacheItem{}
	cacheMutex.Unlock()
}
