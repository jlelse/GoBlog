package main

import (
	"net/http"

	"github.com/go-chi/cors"
)

var publicCORS = cors.Handler(cors.Options{
	AllowedOrigins: []string{"*"},
	AllowedMethods: []string{http.MethodGet, http.MethodHead, http.MethodOptions},
	AllowedHeaders: []string{"Accept", "If-Modified-Since", "If-None-Match"},
	ExposedHeaders: []string{"ETag"},
	MaxAge:         300,
})

// CORSHandler wraps an HTTP handler with GoBlog's public CORS policy.
// The headers are set independently of the request's Origin header, so cached responses
// are also valid for cross-origin requests.
func (a *goBlog) CORSHandler(next http.Handler) http.Handler {
	handler := publicCORS(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "ETag")
		handler.ServeHTTP(w, r)
	})
}
