package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"
)

func CacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestUrl, _ := url.ParseRequestURI(r.RequestURI)
		path := SlashTrimmedPath(r)
		if appConfig.cache.enable &&
			// Check bypass query
			!(requestUrl != nil && requestUrl.Query().Get("cache") == "0") {
			cacheTime, header, body := getCache(path, r.Context())
			if cacheTime == 0 {
				recorder := httptest.NewRecorder()
				next.ServeHTTP(recorder, r)
				// Copy values from recorder
				code := recorder.Code
				// Send response
				for k, v := range recorder.Header() {
					w.Header()[k] = v
				}
				w.Header().Set("GoBlog-Cache", "MISS")
				w.WriteHeader(code)
				_, _ = w.Write(recorder.Body.Bytes())
				// Save cache
				if code == http.StatusOK {
					saveCache(path, recorder.Header(), recorder.Body.Bytes())
				}
				return
			} else {
				expiresTime := time.Unix(cacheTime+appConfig.cache.expiration, 0).Format(time.RFC1123)
				for k, v := range header {
					w.Header()[k] = v
				}
				w.Header().Set("Expires", expiresTime)
				w.Header().Set("GoBlog-Cache", "HIT")
				_, _ = w.Write(body)
			}
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func getCache(path string, context context.Context) (creationTime int64, header map[string][]string, body []byte) {
	var headerBytes []byte
	allowedTime := time.Now().Unix() - appConfig.cache.expiration
	row := appDb.QueryRowContext(context, "select COALESCE(time, 0), header, body from cache where path=? and time>=?", path, allowedTime)
	_ = row.Scan(&creationTime, &headerBytes, &body)
	header = make(map[string][]string)
	_ = json.Unmarshal(headerBytes, &header)
	return
}

func saveCache(path string, header map[string][]string, body []byte) {
	now := time.Now().Unix()
	headerBytes, _ := json.Marshal(header)
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		return
	}
	_, _ = tx.Exec("delete from cache where time<?;", now-appConfig.cache.expiration)
	_, _ = tx.Exec("insert or replace into cache (path, time, header, body) values (?, ?, ?, ?);", path, now, headerBytes, body)
	_ = tx.Commit()
	finishWritingToDb()
}
