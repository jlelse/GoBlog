package main

import (
	"context"
	"net/http"
	"strings"

	"go.hacdias.com/indielib/indieauth"
)

const indieAuthScope contextKey = "scope"

func (a *goBlog) initIndieAuth() {
	a.ias = indieauth.NewServer(
		false,
		a.httpClient,
	)
}

func (a *goBlog) checkIndieAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearerToken := defaultIfEmpty(r.Header.Get("Authorization"), r.URL.Query().Get("access_token"))
		data, err := a.db.indieAuthVerifyToken(bearerToken)
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), indieAuthScope, strings.Join(data.Scopes, " "))))
	})
}

func addAllScopes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), indieAuthScope, "create update delete undelete media")))
	})
}
