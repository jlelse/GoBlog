package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"
)

func cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestUrl, _ := url.ParseRequestURI(r.RequestURI)
		path := slashTrimmedPath(r)
		if appConfig.cache.enable &&
			// check bypass query
			!(requestUrl != nil && requestUrl.Query().Get("cache") == "0") {
			cacheTime, header, body := getCache(r.Context(), path)
			if cacheTime == 0 {
				recorder := httptest.NewRecorder()
				next.ServeHTTP(recorder, r)
				// copy values from recorder
				code := recorder.Code
				// send response
				for k, v := range recorder.Header() {
					w.Header()[k] = v
				}
				now := time.Now()
				setCacheHeaders(w, now.Format(time.RFC1123), time.Unix(now.Unix()+appConfig.cache.expiration, 0).Format(time.RFC1123))
				w.Header().Set("GoBlog-Cache", "MISS")
				w.WriteHeader(code)
				_, _ = w.Write(recorder.Body.Bytes())
				// Save cache
				if code == http.StatusOK {
					saveCache(path, now, recorder.Header(), recorder.Body.Bytes())
				}
				return
			}
			cacheTimeString := time.Unix(cacheTime, 0).Format(time.RFC1123)
			expiresTimeString := time.Unix(cacheTime+appConfig.cache.expiration, 0).Format(time.RFC1123)
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

func getCache(context context.Context, path string) (creationTime int64, header map[string][]string, body []byte) {
	var headerBytes []byte
	allowedTime := time.Now().Unix() - appConfig.cache.expiration
	row := appDb.QueryRowContext(context, "select COALESCE(time, 0), header, body from cache where path=? and time>=?", path, allowedTime)
	_ = row.Scan(&creationTime, &headerBytes, &body)
	header = make(map[string][]string)
	_ = json.Unmarshal(headerBytes, &header)
	return
}

func saveCache(path string, now time.Time, header map[string][]string, body []byte) {
	headerBytes, _ := json.Marshal(header)
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		return
	}
	_, _ = tx.Exec("delete from cache where time<?;", now.Unix()-appConfig.cache.expiration)
	_, _ = tx.Exec("insert or replace into cache (path, time, header, body) values (?, ?, ?, ?);", path, now.Unix(), headerBytes, body)
	_ = tx.Commit()
	finishWritingToDb()
}

func purgeCache(path string) {
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		return
	}
	_, _ = tx.Exec("delete from cache where path=?", path)
	_ = tx.Commit()
	finishWritingToDb()
}
