package main

import (
	"embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"go.goblog.app/app/pkgs/contenttype"
)

const defaultGeoMapPath = "/map"

func (a *goBlog) serveGeoMap(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	bc := a.cfg.Blogs[blog]

	allPostsWithLocation, err := a.db.getPosts(&postsRequestConfig{
		blog:               blog,
		status:             statusPublished,
		parameter:          geoParam,
		withOnlyParameters: []string{geoParam},
	})
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(allPostsWithLocation) == 0 {
		a.render(w, r, templateGeoMap, &renderData{
			BlogString: blog,
			Data: map[string]interface{}{
				"nolocations": true,
			},
		})
		return
	}

	type templateLocation struct {
		Lat  float64
		Lon  float64
		Post string
	}

	var locations []*templateLocation
	for _, p := range allPostsWithLocation {
		if g := p.GeoURI(); g != nil {
			locations = append(locations, &templateLocation{
				Lat:  g.Latitude,
				Lon:  g.Longitude,
				Post: p.Path,
			})
		}
	}

	jb, err := json.Marshal(locations)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	mapPath := bc.getRelativePath(defaultIfEmpty(bc.Map.Path, defaultGeoMapPath))
	a.render(w, r, templateGeoMap, &renderData{
		BlogString: blog,
		Canonical:  a.getFullAddress(mapPath),
		Data: map[string]interface{}{
			"mappath":   mapPath,
			"locations": string(jb),
		},
	})
}

//go:embed leaflet/*
var leafletFiles embed.FS

func (a *goBlog) serveLeaflet(basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fileName := strings.TrimPrefix(r.URL.Path, basePath)
		log.Println(basePath, fileName)
		fb, err := leafletFiles.ReadFile(fileName)
		if err != nil {
			a.serve404(w, r)
			return
		}
		switch path.Ext(fileName) {
		case ".js":
			w.Header().Set(contentType, contenttype.JS)
			a.min.Write(w, contenttype.JSUTF8, fb)
		case ".css":
			w.Header().Set(contentType, contenttype.CSS)
			a.min.Write(w, contenttype.CSSUTF8, fb)
		default:
			w.Header().Set(contentType, http.DetectContentType(fb))
			w.Write(fb)
		}
	}
}

func (a *goBlog) proxyTiles(basePath string) http.HandlerFunc {
	osmUrl, _ := url.Parse("https://tile.openstreetmap.org/")
	tileProxy := http.StripPrefix(basePath, httputil.NewSingleHostReverseProxy(osmUrl))
	return func(w http.ResponseWriter, r *http.Request) {
		proxyTarget := "https://tile.openstreetmap.org" + r.URL.Path
		proxyRequest, _ := http.NewRequest(http.MethodGet, proxyTarget, nil)
		// Copy request headers
		for _, k := range []string{
			"Accept-Encoding",
			"Accept-Language",
			"Accept",
			"Cache-Control",
			"If-Modified-Since",
			"If-None-Match",
			"User-Agent",
		} {
			proxyRequest.Header.Set(k, r.Header.Get(k))
		}
		rec := httptest.NewRecorder()
		tileProxy.ServeHTTP(rec, proxyRequest)
		res := rec.Result()
		// Copy result headers
		for _, k := range []string{
			"Accept-Ranges",
			"Access-Control-Allow-Origin",
			"Age",
			"Cache-Control",
			"Content-Length",
			"Content-Type",
			"Etag",
			"Expires",
		} {
			w.Header().Set(k, res.Header.Get(k))
		}
		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body)
		_ = res.Body.Close()
	}
}
