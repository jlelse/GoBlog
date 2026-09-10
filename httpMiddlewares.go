package main

import (
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/samber/go-singleflightx"
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

// securityHeaders sets static security headers and the Content-Security-Policy.
func (a *goBlog) securityHeaders(next http.Handler) http.Handler {
	csp := newCSPBuilder(a)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000;")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", csp.value())
		next.ServeHTTP(w, r)
	})
}

// cspBuilder builds the Content-Security-Policy header value. The static parts are
// gathered once, while the parts that depend on assets are rebuilt and cached only
// when assets change (e.g. plugins registering assets lazily via CompileAsset).
type cspBuilder struct {
	app *goBlog

	// Source lists that only depend on configuration, precomputed once.
	defaultSrcList []src.SourceVal
	imgSrcList     []src.SourceVal
	frameAncestors *constraint.FrameAncestorsConstraint

	// cached holds the last built CSP keyed by asset version, while group collapses
	// concurrent rebuilds of the same version.
	cached atomic.Pointer[cspCache]
	group  singleflightx.Group[uint64, string]
}

// cspCache is an immutable snapshot of a built CSP for a specific asset version.
type cspCache struct {
	version uint64
	value   string
}

// newCSPBuilder precomputes the CSP parts that only depend on configuration.
func newCSPBuilder(a *goBlog) *cspBuilder {
	allowedDomains := a.cspAllowedDomains()
	defaultSrcList := make([]src.SourceVal, 0, 2+len(allowedDomains))
	defaultSrcList = append(defaultSrcList, src.Self(), src.Scheme("blob:"))
	imgSrcList := make([]src.SourceVal, 0, 2+len(allowedDomains))
	imgSrcList = append(imgSrcList, src.Self(), src.Scheme("data:"))
	for _, domain := range allowedDomains {
		defaultSrcList = append(defaultSrcList, src.Host(domain))
		imgSrcList = append(imgSrcList, src.Host(domain))
	}
	frameAncestors := &constraint.FrameAncestorsConstraint{}
	frameAncestors.Sources(src.None())
	return &cspBuilder{
		app:            a,
		defaultSrcList: defaultSrcList,
		imgSrcList:     imgSrcList,
		frameAncestors: frameAncestors,
	}
}

// cspAllowedDomains returns the deduplicated, sorted hosts allowed as CSP sources.
func (a *goBlog) cspAllowedDomains() []string {
	domains := map[string]struct{}{}
	addDomains := func(candidates ...string) {
		for _, domain := range candidates {
			if domain != "" {
				domains[domain] = struct{}{}
			}
		}
	}
	addDomains(a.cfg.Server.publicHost, a.cfg.Server.shortPublicHost, a.cfg.Server.mediaHost)
	addDomains(a.cfg.Server.altHosts...)
	addDomains(a.cfg.Server.CSPDomains...)
	if mp := a.cfg.Micropub.MediaStorage; mp != nil && mp.MediaURL != "" {
		if u, err := url.Parse(mp.MediaURL); err == nil {
			addDomains(u.Hostname())
		}
	}
	return slices.Sorted(maps.Keys(domains))
}

// value returns the CSP header value for the current assets, rebuilding and caching
// it only when the asset version changed.
func (b *cspBuilder) value() string {
	version := b.app.assetVersion.Load()
	if cached := b.cached.Load(); cached != nil && cached.version == version {
		return cached.value
	}
	// Assets changed (or not built yet): rebuild the CSP, deduplicating concurrent
	// rebuilds for the same version. The result is a deterministic function of the
	// version, so a concurrent rebuild racing us here is harmless.
	value, _, _ := b.group.Do(version, func() (string, error) {
		if cached := b.cached.Load(); cached != nil && cached.version == version {
			return cached.value, nil
		}
		value := b.build()
		b.cached.Store(&cspCache{version: version, value: value})
		return value, nil
	})
	return value
}

// build assembles the full CSP from the precomputed source lists and the hashes of
// all currently registered assets.
func (b *cspBuilder) build() string {
	a := b.app
	styleSrcList := make([]src.SourceVal, 0, 1+len(a.assetFileNames)+len(a.cfg.Server.CSPDomains))
	styleSrcList = append(styleSrcList, src.Self())
	scriptSrcList := make([]src.SourceVal, 0, len(a.assetFileNames)+len(bundleHashes)+len(a.cfg.Server.CSPDomains))
	for name, compiledName := range a.assetFileNames {
		af, ok := a.assetFiles[compiledName]
		if !ok || af == nil {
			continue
		}
		switch {
		case strings.HasSuffix(name, ".css"):
			styleSrcList = append(styleSrcList, src.HashAlgBase64(hashalg.Sha256(), af.sha256base64))
		case strings.HasSuffix(name, ".js"):
			scriptSrcList = append(scriptSrcList, src.HashAlgBase64(hashalg.Sha256(), af.sha256base64))
		}
	}
	for path, hash := range bundleHashes {
		if strings.HasSuffix(path, ".js") {
			scriptSrcList = append(scriptSrcList, src.HashAlgBase64(hashalg.Sha256(), hash))
		}
	}
	for _, domain := range a.cfg.Server.CSPDomains {
		styleSrcList = append(styleSrcList, src.Host(domain))
		scriptSrcList = append(scriptSrcList, src.Host(domain))
	}
	return cspolicy.Build(
		directives.DefaultSrc(b.defaultSrcList...),
		directives.ImgSrc(b.imgSrcList...),
		directives.StyleSrc(styleSrcList...),
		directives.ScriptSrc(scriptSrcList...),
		directives.FrameAncestors(b.frameAncestors),
	)
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
