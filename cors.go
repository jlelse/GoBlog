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
func (a *goBlog) CORSHandler(next http.Handler) http.Handler {
	return publicCORS(next)
}
