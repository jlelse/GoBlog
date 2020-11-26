package main

import (
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

var logf *rotatelogs.RotateLogs

func initHTTPLog() (err error) {
	if !appConfig.Server.Logging {
		return nil
	}
	logf, err = rotatelogs.New(
		appConfig.Server.LogFile+".%Y%m%d",
		rotatelogs.WithLinkName(appConfig.Server.LogFile),
		rotatelogs.WithClock(rotatelogs.UTC),
		rotatelogs.WithMaxAge(30*24*time.Hour),
		rotatelogs.WithRotationTime(24*time.Hour),
	)
	return
}

func logMiddleware(next http.Handler) http.Handler {
	h := handlers.CombinedLoggingHandler(logf, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Remove remote address for privacy
		r.RemoteAddr = ""
		h.ServeHTTP(w, r)
	})
}
