package main

import (
	"net/http"

	"github.com/justinas/alice"
)

func (a *goBlog) isPrivate() bool {
	return a.cfg.PrivateMode != nil && a.cfg.PrivateMode.Enabled
}

func (a *goBlog) privateModeHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.isPrivate() {
			alice.New(a.authMiddleware).Then(next).ServeHTTP(w, r)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
