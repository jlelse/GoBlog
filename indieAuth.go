package main

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		if !isAllowedHost(httptest.NewRequest(http.MethodGet, tokenData.Me, nil), appConfig.Server.Domain) {
			http.Error(w, "Forbidden", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), "scope", strings.Join(tokenData.Scopes, " "))
		next.ServeHTTP(w, r.WithContext(ctx))
		return
	})
}
