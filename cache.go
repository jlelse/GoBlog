package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"time"
)

var cacheMutexMapMutex *sync.Mutex
var cacheMutexes map[string]*sync.Mutex
var cacheDb *sql.DB
var cacheDbWriteMutex = &sync.Mutex{}

func initCache() (err error) {
	cacheMutexMapMutex = &sync.Mutex{}
	cacheMutexes = map[string]*sync.Mutex{}
	cacheDb, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		return err
	}
	tx, err := cacheDb.Begin()
	if err != nil {
		return
	}
	_, err = tx.Exec("CREATE TABLE cache (path text not null primary key, time integer, header blob, body blob);")
	if err != nil {
		return
	}
	err = tx.Commit()
	if err != nil {
		return
	}
	return
}

func startWritingToCacheDb() {
	cacheDbWriteMutex.Lock()
}

func finishWritingToCacheDb() {
	cacheDbWriteMutex.Unlock()
}

func cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL, _ := url.ParseRequestURI(r.RequestURI)
		path := slashTrimmedPath(r)
		if appConfig.Cache.Enable &&
			// check bypass query
			!(requestURL != nil && requestURL.Query().Get("cache") == "0") {
			// Check cache mutex
			cacheMutexMapMutex.Lock()
			if cacheMutexes[path] == nil {
				cacheMutexes[path] = &sync.Mutex{}
			}
			cacheMutexMapMutex.Unlock()
			// Get cache
			cm := cacheMutexes[path]
			cm.Lock()
			cacheTime, header, body := getCache(path)
			cm.Unlock()
			if cacheTime == 0 {
				cm.Lock()
				// Render cache
				renderCache(path, next, w, r)
				cm.Unlock()
				return
			}
			cacheTimeString := time.Unix(cacheTime, 0).Format(time.RFC1123)
			expiresTimeString := time.Unix(cacheTime+appConfig.Cache.Expiration, 0).Format(time.RFC1123)
			// check conditional request
			ifModifiedSinceHeader := r.Header.Get("If-Modified-Since")
			if ifModifiedSinceHeader != "" && ifModifiedSinceHeader == cacheTimeString {
				setCacheHeaders(w, cacheTimeString, expiresTimeString)
				// send 304
				w.WriteHeader(http.StatusNotModified)
				return
			}
			// copy cached headers
			for k, v := range header {
				w.Header()[k] = v
			}
			setCacheHeaders(w, cacheTimeString, expiresTimeString)
			w.Header().Set("GoBlog-Cache", "HIT")
			// write cached body
			_, _ = w.Write(body)
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

func renderCache(path string, next http.Handler, w http.ResponseWriter, r *http.Request) {
	// No cache available
	recorder := httptest.NewRecorder()
	next.ServeHTTP(recorder, r)
	// copy values from recorder
	code := recorder.Code
	// send response
	for k, v := range recorder.Header() {
		w.Header()[k] = v
	}
	now := time.Now()
	setCacheHeaders(w, now.Format(time.RFC1123), time.Unix(now.Unix()+appConfig.Cache.Expiration, 0).Format(time.RFC1123))
	w.Header().Set("GoBlog-Cache", "MISS")
	w.WriteHeader(code)
	_, _ = w.Write(recorder.Body.Bytes())
	// Save cache
	if code == http.StatusOK {
		saveCache(path, now, recorder.Header(), recorder.Body.Bytes())
	}
}

func getCache(path string) (creationTime int64, header map[string][]string, body []byte) {
	var headerBytes []byte
	allowedTime := time.Now().Unix() - appConfig.Cache.Expiration
	row := cacheDb.QueryRow("select COALESCE(time, 0), header, body from cache where path=? and time>=?", path, allowedTime)
	_ = row.Scan(&creationTime, &headerBytes, &body)
	header = make(map[string][]string)
	_ = json.Unmarshal(headerBytes, &header)
	return
}

func saveCache(path string, now time.Time, header map[string][]string, body []byte) {
	headerBytes, _ := json.Marshal(header)
	startWritingToCacheDb()
	defer finishWritingToCacheDb()
	_, _ = cacheDb.Exec("insert or replace into cache (path, time, header, body) values (?, ?, ?, ?);", path, now.Unix(), headerBytes, body)
}

func purgeCache() {
	startWritingToCacheDb()
	defer finishWritingToCacheDb()
	_, _ = cacheDb.Exec("delete from cache; vacuum;")
}
