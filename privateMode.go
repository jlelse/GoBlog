package main

import (
	"net/http"

	"github.com/justinas/alice"
)

func (a *goBlog) isPrivate() bool {
	return a.cfg.PrivateMode != nil && a.cfg.PrivateMode.Enabled
}

func (a *goBlog) privateModeHandler(next http.Handler) http.Handler {
	private := alice.New(a.authMiddleware).Then(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.isPrivate() {
			private.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
