package main

import (
	"context"
	"net/http"
	"strings"
)

func checkIndieAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearerToken := r.Header.Get("Authorization")
		if len(bearerToken) == 0 {
			bearerToken = r.URL.Query().Get("access_token")
		}
		tokenData, err := verifyIndieAuthToken(bearerToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "scope", strings.Join(tokenData.Scopes, " "))))
		return
	})
}

func addAllScopes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), "scope", "create update delete media")))
		return
	})
}
