package main

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

func CacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestUrl, _ := url.ParseRequestURI(r.RequestURI)
		if appConfig.cache.enable &&
			// Check bypass query
			!(requestUrl != nil && requestUrl.Query().Get("cache") == "0") {
			mime, t, cache := getCache(SlashTrimmedPath(r), r.Context())
			if cache == nil {
				next.ServeHTTP(w, r)
				return
			} else {
				expiresTime := time.Unix(t+appConfig.cache.expiration, 0).Format(time.RFC1123)
				w.Header().Set("Expires", expiresTime)
				w.Header().Set("Content-Type", mime)
				_, _ = w.Write(cache)
			}
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func getCache(path string, context context.Context) (string, int64, []byte) {
	var mime string
	var t int64
	var cache []byte
	allowedTime := time.Now().Unix() - appConfig.cache.expiration
	row := appDb.QueryRowContext(context, "select COALESCE(mime, ''), COALESCE(time, 0), value from cache where path=? and time>=?", path, allowedTime)
	_ = row.Scan(&mime, &t, &cache)
	return mime, t, cache
}

func saveCache(path string, mime string, value []byte) {
	now := time.Now().Unix()
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		return
	}
	_, _ = tx.Exec("delete from cache where time<?;", now-appConfig.cache.expiration)
	_, _ = tx.Exec("insert or replace into cache (path, time, mime, value) values (?, ?, ?, ?);", path, now, mime, value)
	_ = tx.Commit()
	finishWritingToDb()
}
