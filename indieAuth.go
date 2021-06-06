package main

import (
	"context"
	"net/http"
	"strings"
)

const indieAuthScope requestContextKey = "scope"

func (a *goBlog) checkIndieAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearerToken := r.Header.Get("Authorization")
		if len(bearerToken) == 0 {
			bearerToken = r.URL.Query().Get("access_token")
		}
		tokenData, err := a.db.verifyIndieAuthToken(bearerToken)
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), indieAuthScope, strings.Join(tokenData.Scopes, " "))))
	})
}

func addAllScopes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), indieAuthScope, "create update delete media")))
	})
}
