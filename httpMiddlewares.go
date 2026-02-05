package main

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/samber/lo"
	"github.com/tiptophelmet/cspolicy"
	"github.com/tiptophelmet/cspolicy/directives"
	"github.com/tiptophelmet/cspolicy/directives/constraint"
	"github.com/tiptophelmet/cspolicy/src"
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
	allowedDomains := []string{a.cfg.Server.publicHost, a.cfg.Server.shortPublicHost, a.cfg.Server.mediaHost}
	allowedDomains = append(allowedDomains, a.cfg.Server.altHosts...)
	allowedDomains = append(allowedDomains, a.cfg.Server.CSPDomains...)
	if mp := a.cfg.Micropub.MediaStorage; mp != nil && mp.MediaURL != "" {
		if u, err := url.Parse(mp.MediaURL); err == nil {
			allowedDomains = append(allowedDomains, u.Hostname())
		}
	}
	allowedDomains = lo.Uniq(lo.Filter(allowedDomains, func(v string, _ int) bool { return v != "" }))
	defaultSrcList := []src.SourceVal{src.Self(), src.Scheme("blob:")}
	for _, d := range allowedDomains {
		defaultSrcList = append(defaultSrcList, src.Host(d))
	}
	imgSrcList := []src.SourceVal{src.Self(), src.Scheme("data:")}
	for _, d := range allowedDomains {
		imgSrcList = append(imgSrcList, src.Host(d))
	}
	fac := &constraint.FrameAncestorsConstraint{}
	fac.Sources(src.None())
	csp := cspolicy.Build(
		directives.DefaultSrc(defaultSrcList...),
		directives.ImgSrc(imgSrcList...),
		directives.FrameAncestors(fac),
	)
	// Return handler
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000;")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
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
				if !slices.Contains(paramsToKeep, param) {
					query.Del(param)
				}
			}
			r.URL.RawQuery = query.Encode()
			next.ServeHTTP(w, r)
		})
	}
}
