package main

import (
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

func (a *goBlog) initHTTPLog() (err error) {
	if !a.cfg.Server.Logging || a.cfg.Server.LogFile == "" {
		return nil
	}
	a.logf, err = rotatelogs.New(
		a.cfg.Server.LogFile+".%Y%m%d",
		rotatelogs.WithLinkName(a.cfg.Server.LogFile),
		rotatelogs.WithClock(rotatelogs.UTC),
		rotatelogs.WithMaxAge(30*24*time.Hour),
		rotatelogs.WithRotationTime(24*time.Hour),
	)
	return
}

func (a *goBlog) logMiddleware(next http.Handler) http.Handler {
	h := handlers.CombinedLoggingHandler(a.logf, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Remove remote address for privacy
		r.RemoteAddr = ""
		h.ServeHTTP(w, r)
	})
}
