package main

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/tiptophelmet/cspolicy"
	"github.com/tiptophelmet/cspolicy/directives"
	"github.com/tiptophelmet/cspolicy/directives/constraint"
	"github.com/tiptophelmet/cspolicy/src"
	"github.com/tiptophelmet/cspolicy/src/hashalg"
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
	// Set things that don't change per request or after startup anymore
	allowedDomains := map[string]struct{}{}
	addAllowedDomain := func(domains ...string) {
		for _, domain := range domains {
			if domain != "" {
				if _, ok := allowedDomains[domain]; !ok {
					allowedDomains[domain] = struct{}{}
				}
			}
		}
	}
	addAllowedDomain(a.cfg.Server.publicHost, a.cfg.Server.shortPublicHost, a.cfg.Server.mediaHost)
	addAllowedDomain(a.cfg.Server.altHosts...)
	addAllowedDomain(a.cfg.Server.CSPDomains...)
	if mp := a.cfg.Micropub.MediaStorage; mp != nil && mp.MediaURL != "" {
		if u, err := url.Parse(mp.MediaURL); err == nil {
			addAllowedDomain(u.Hostname())
		}
	}
	defaultSrcList := make([]src.SourceVal, 0, 2+len(allowedDomains))
	defaultSrcList = append(defaultSrcList, src.Self(), src.Scheme("blob:"))
	for d := range allowedDomains {
		defaultSrcList = append(defaultSrcList, src.Host(d))
	}
	imgSrcList := make([]src.SourceVal, 0, 2+len(allowedDomains))
	imgSrcList = append(imgSrcList, src.Self(), src.Scheme("data:"))
	for d := range allowedDomains {
		imgSrcList = append(imgSrcList, src.Host(d))
	}
	fac := &constraint.FrameAncestorsConstraint{}
	fac.Sources(src.None())
	// Provide function to build CSP header value that also includes hashes etc. for plugins
	buildCSP := func() string {
		styleSrcList := make([]src.SourceVal, 0, 1+len(a.assetFileNames)+len(a.cfg.Server.CSPDomains))
		styleSrcList = append(styleSrcList, src.Self())
		scriptSrcList := make([]src.SourceVal, 0, len(a.assetFileNames)+len(bundleHashes)+len(a.cfg.Server.CSPDomains))
		for name, compiledName := range a.assetFileNames {
			if strings.HasSuffix(name, ".css") {
				if af, ok := a.assetFiles[compiledName]; ok && af != nil {
					styleSrcList = append(styleSrcList, src.HashAlgBase64(hashalg.Sha256(), af.sha256base64))
				}
			} else if strings.HasSuffix(name, ".js") {
				if af, ok := a.assetFiles[compiledName]; ok && af != nil {
					scriptSrcList = append(scriptSrcList, src.HashAlgBase64(hashalg.Sha256(), af.sha256base64))
				}
			}
		}
		for path, hash := range bundleHashes {
			if strings.HasSuffix(path, ".js") {
				scriptSrcList = append(scriptSrcList, src.HashAlgBase64(hashalg.Sha256(), hash))
			}
		}
		for _, d := range a.cfg.Server.CSPDomains {
			styleSrcList = append(styleSrcList, src.Host(d))
			scriptSrcList = append(scriptSrcList, src.Host(d))
		}
		return cspolicy.Build(
			directives.DefaultSrc(defaultSrcList...),
			directives.ImgSrc(imgSrcList...),
			directives.StyleSrc(styleSrcList...),
			directives.ScriptSrc(scriptSrcList...),
			directives.FrameAncestors(fac),
		)
	}
	// Return handler
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000;")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", buildCSP())
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
