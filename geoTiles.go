package main

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (a *goBlog) proxyTiles() http.HandlerFunc {
	tileSource := "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
	if c := a.cfg.MapTiles; c != nil && c.Source != "" {
		tileSource = c.Source
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a new request to proxy to the tile server
		targetUrl := strings.NewReplacer(
			"{s}", chi.URLParam(r, "s"),
			"{z}", chi.URLParam(r, "z"),
			"{x}", chi.URLParam(r, "x"),
			"{y}", chi.URLParam(r, "y"),
		).Replace(tileSource)
		proxyRequest, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, targetUrl, nil)
		// Copy request headers
		for _, k := range []string{
			"Accept-Encoding",
			"Accept-Language",
			"Accept",
			cacheControl,
			"If-Modified-Since",
			"If-None-Match",
		} {
			proxyRequest.Header.Set(k, r.Header.Get(k))
		}
		// Do the request
		res, err := a.httpClient.Do(proxyRequest)
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		// Copy result headers
		for _, k := range []string{
			"Accept-Ranges",
			"Access-Control-Allow-Origin",
			"Age",
			cacheControl,
			"Content-Length",
			"Content-Type",
			"Etag",
			"Expires",
		} {
			w.Header().Set(k, res.Header.Get(k))
		}
		// Copy result
		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
		_ = res.Body.Close()
	}
}

func (a *goBlog) getMinZoom() int {
	if c := a.cfg.MapTiles; c != nil {
		return c.MinZoom
	}
	return 0
}

func (a *goBlog) getMaxZoom() int {
	if c := a.cfg.MapTiles; c != nil && c.MaxZoom > 0 {
		return c.MaxZoom
	}
	return 20
}

func (a *goBlog) getMapAttribution() string {
	if c := a.cfg.MapTiles; c != nil {
		return c.Attribution
	}
	return `&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors`
}
