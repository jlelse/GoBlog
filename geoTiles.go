package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

func copyHeaders(dst, src http.Header, headers ...string) {
	for _, header := range headers {
		if value := src.Get(header); value != "" {
			dst.Set(header, value)
		}
	}
}

func (a *goBlog) proxyTiles() http.HandlerFunc {
	tileSource := "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
	if c := a.cfg.MapTiles; c != nil && c.Source != "" {
		tileSource = c.Source
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a new request to proxy to the tile server
		targetURL := strings.NewReplacer(
			"{s}", chi.URLParam(r, "s"),
			"{z}", chi.URLParam(r, "z"),
			"{x}", chi.URLParam(r, "x"),
			"{y}", chi.URLParam(r, "y"),
		).Replace(tileSource)
		proxyRequest, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL, nil) //nolint:gosec
		if err != nil {
			a.serveError(w, r, "", http.StatusInternalServerError)
			return
		}
		// Copy request headers
		copyHeaders(
			proxyRequest.Header, r.Header,
			"Accept-Encoding",
			"Accept-Language",
			"Accept",
			cacheControl,
			"If-Modified-Since",
			"If-None-Match",
		)
		// Do the request
		res, err := a.httpClient.Do(proxyRequest) //nolint:gosec
		if err != nil {
			a.serveError(w, r, "", http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		// Copy result headers
		copyHeaders(
			w.Header(), res.Header,
			"Accept-Ranges",
			"Access-Control-Allow-Origin",
			"Age",
			cacheControl,
			"Content-Length",
			contentType,
			"Etag",
			"Expires",
		)
		// Copy result
		w.WriteHeader(res.StatusCode)
		_, _ = io.Copy(w, res.Body)
	}
}

func (a *goBlog) vectorMapEnabled() bool {
	return a.cfg.MapTiles != nil && a.cfg.MapTiles.Vector
}

func (a *goBlog) proxyVectorTiles() http.HandlerFunc {
	const tileHost = "tiles.openfreemap.org"
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		resource := strings.TrimPrefix(r.URL.Path, "/-/maptiles/")
		if resource == "." || !fs.ValidPath(resource) {
			http.NotFound(w, r)
			return
		}
		targetURL := (&url.URL{
			Scheme: "https",
			Host:   tileHost,
			Path:   "/" + resource,
		}).String()
		request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL, nil)
		if err != nil {
			a.serveError(w, r, "", http.StatusInternalServerError)
			return
		}
		client := *a.httpClient
		checkRedirect := client.CheckRedirect
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			u := req.URL
			if u.Scheme != "https" || !strings.EqualFold(u.Hostname(), tileHost) ||
				(u.Port() != "" && u.Port() != "443") || u.User != nil {
				return errors.New("unexpected vector tile redirect destination")
			}
			if len(via) >= 10 {
				return errors.New("too many vector tile redirects")
			}
			if checkRedirect != nil {
				return checkRedirect(req, via)
			}
			return nil
		}
		response, err := client.Do(request) //nolint:gosec
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		if status := response.StatusCode; status < 200 || status >= 300 {
			if status < 400 || status > 599 {
				status = http.StatusBadGateway
			}
			http.Error(w, http.StatusText(status), status)
			return
		}
		mediaType, _, err := mime.ParseMediaType(response.Header.Get(contentType))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}
		switch mediaType {
		case contenttype.JSON, contenttype.PNG, "application/x-protobuf", "application/vnd.mapbox-vector-tile":
		default:
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}
		if mediaType == contenttype.JSON {
			buf := bufferpool.Get()
			defer bufferpool.Put(buf)
			if _, err := io.Copy(buf, response.Body); err != nil {
				http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
				return
			}
			body := bytes.ReplaceAll(buf.Bytes(), []byte("https://"+tileHost+"/"), []byte("/-/maptiles/"))
			body = bytes.ReplaceAll(body, []byte("http://"+tileHost+"/"), []byte("/-/maptiles/"))
			copyHeaders(w.Header(), response.Header, cacheControl, "Expires", "Last-Modified")
			w.Header().Set(contentType, contenttype.JSONUTF8)
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.WriteHeader(response.StatusCode)
			_, _ = w.Write(body)
			return
		}
		copyHeaders(w.Header(), response.Header, cacheControl, "Expires", "Last-Modified", "Content-Length", "Etag")
		w.Header().Set(contentType, mediaType)
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
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
	return 19
}

func (a *goBlog) getMapAttribution() string {
	if a.vectorMapEnabled() {
		return `<a href="https://openfreemap.org">OpenFreeMap</a> <a href="https://www.openmaptiles.org/">&copy; OpenMapTiles</a> data from <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>`
	}
	if c := a.cfg.MapTiles; c != nil && c.Attribution != "" {
		return c.Attribution
	}
	return `&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors`
}
