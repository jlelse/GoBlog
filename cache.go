package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var cacheMap = map[string]*cacheItem{}
var cacheMutex = &sync.RWMutex{}

var requestGroup singleflight.Group

func cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if appConfig.Cache.Enable &&
			// check bypass query
			!(r.URL.Query().Get("cache") == "0") &&
			// check method
			(r.Method == http.MethodGet || r.Method == http.MethodHead) {
			// Fix path
			path := slashTrimmedPath(r)
			// Get cache or render it
			cacheInterface, _, _ := requestGroup.Do(path, func() (interface{}, error) {
				return getCache(path, next, r), nil
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

func getCache(path string, next http.Handler, r *http.Request) *cacheItem {
	cacheMutex.RLock()
	item, ok := cacheMap[path]
	cacheMutex.RUnlock()
	if !ok || item.creationTime < time.Now().Unix()-appConfig.Cache.Expiration {
		item = &cacheItem{}
		// No cache available
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, r)
		// copy values from recorder
		now := time.Now()
		item.creationTime = now.Unix()
		item.code = recorder.Code
		item.header = recorder.Header()
		item.body = recorder.Body.Bytes()
		// Save cache
		cacheMutex.Lock()
		cacheMap[path] = item
		cacheMutex.Unlock()
	}
	return item
}

func purgeCache() {
	cacheMutex.Lock()
	cacheMap = map[string]*cacheItem{}
	cacheMutex.Unlock()
}
