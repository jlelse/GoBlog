package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/builderpool"
)

func noIndexHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex")
		next.ServeHTTP(w, r)
	})
}

func fixHTTPHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.RawPath = ""
		next.ServeHTTP(w, r)
	})
}

func headAsGetHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			// Clone request and change method
			newReq := new(http.Request)
			*newReq = *r
			newReq.Method = http.MethodGet
			// Serve new request
			next.ServeHTTP(w, newReq)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *goBlog) securityHeaders(next http.Handler) http.Handler {
	// Build CSP domains list
	cspBuilder := builderpool.Get()
	if mp := a.cfg.Micropub.MediaStorage; mp != nil && mp.MediaURL != "" {
		if u, err := url.Parse(mp.MediaURL); err == nil {
			cspBuilder.WriteString(" ")
			cspBuilder.WriteString(u.Hostname())
		}
	}
	if len(a.cfg.Server.CSPDomains) > 0 {
		cspBuilder.WriteString(" ")
		cspBuilder.WriteString(strings.Join(a.cfg.Server.CSPDomains, " "))
	}
	cspDomains := cspBuilder.String()
	csp := "default-src 'self' blob:" + cspDomains + "; img-src 'self'" + cspDomains + " data:; frame-ancestors 'none';"
	builderpool.Put(cspBuilder)
	// Return handler
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000;")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Xss-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}

func (a *goBlog) addOnionLocation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.torAddress != "" {
			w.Header().Set("Onion-Location", a.torAddress+r.URL.RequestURI())
		}
		next.ServeHTTP(w, r)
	})
}

func keepSelectedQueryParams(paramsToKeep ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			for param := range query {
				if !lo.Contains(paramsToKeep, param) {
					query.Del(param)
				}
			}
			r.URL.RawQuery = query.Encode()
			next.ServeHTTP(w, r)
		})
	}
}
