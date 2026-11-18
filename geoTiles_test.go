package main

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_proxyTiles(t *testing.T) {
	app := &goBlog{
		cfg: &config{},
	}

	hc := newFakeHttpClient()
	hc.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hello, World!"))
	}))
	app.httpClient = hc.Client

	// Default tile source

	m := chi.NewMux()
	m.Get("/-/tiles/{s}/{z}/{x}/{y}.png", app.proxyTiles())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.org/-/tiles/c/8/134/84.png", nil)
	require.NoError(t, err)
	resp, err := doHandlerRequest(req, m)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "https://c.tile.openstreetmap.org/8/134/84.png", hc.req.URL.String())

	// Custom tile source

	app.cfg.MapTiles = &configMapTiles{
		Source: "https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png",
	}

	m = chi.NewMux()
	m.Get("/-/tiles/{s}/{z}/{x}/{y}.png", app.proxyTiles())

	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.org/-/tiles/c/8/134/84.png", nil)
	require.NoError(t, err)
	resp, err = doHandlerRequest(req, m)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "https://c.tile.opentopomap.org/8/134/84.png", hc.req.URL.String())
}

func Test_proxyVectorTiles(t *testing.T) {
	t.Run("Rewrites JSON URLs and strips Etag and private request data", func(t *testing.T) {
		app := &goBlog{cfg: &config{}}
		hc := newFakeHttpClient()
		hc.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.Header().Set("Expires", "Wed, 21 Oct 2026 07:28:00 GMT")
			w.Header().Set("Last-Modified", "Wed, 21 Oct 2026 06:28:00 GMT")
			w.Header().Set("Etag", "upstream-etag")
			_, _ = w.Write([]byte(`{"tiles":["https://tiles.openfreemap.org/planet/{z}/{x}/{y}.pbf"],"sprite":"http://tiles.openfreemap.org/sprites/ofm"}`))
		}))
		app.httpClient = hc.Client

		m := chi.NewMux()
		m.Get("/-/maptiles/*", app.proxyVectorTiles())
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.org/-/maptiles/planet?visitor=private", nil)
		req.Header.Set("Accept", "application/private")
		require.NoError(t, err)
		resp, err := doHandlerRequest(req, m)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "https://tiles.openfreemap.org/planet", hc.req.URL.String())
		assert.Empty(t, hc.req.Header.Get("Accept"))
		assert.NotContains(t, string(body), "tiles.openfreemap.org")
		assert.Contains(t, string(body), "/-/maptiles/planet/{z}/{x}/{y}.pbf")
		assert.Contains(t, string(body), "/-/maptiles/sprites/ofm")
		assert.Equal(t, "public, max-age=3600", resp.Header.Get("Cache-Control"))
		assert.Equal(t, "Wed, 21 Oct 2026 07:28:00 GMT", resp.Header.Get("Expires"))
		assert.Equal(t, "Wed, 21 Oct 2026 06:28:00 GMT", resp.Header.Get("Last-Modified"))
		assert.Empty(t, resp.Header.Get("Etag"))
	})

	t.Run("Streams binary tiles and preserves Etag", func(t *testing.T) {
		app := &goBlog{cfg: &config{}}
		hc := newFakeHttpClient()
		hc.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.Header().Set("Content-Length", "11")
			w.Header().Set("Etag", `"tile-etag"`)
			_, _ = w.Write([]byte("pbf-content"))
		}))
		app.httpClient = hc.Client

		m := chi.NewMux()
		m.Get("/-/maptiles/*", app.proxyVectorTiles())
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.org/-/maptiles/fonts/Noto%20Sans%20Regular/0-255.pbf", nil)
		require.NoError(t, err)
		resp, err := doHandlerRequest(req, m)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "https://tiles.openfreemap.org/fonts/Noto%20Sans%20Regular/0-255.pbf", hc.req.URL.String())
		assert.Equal(t, "pbf-content", string(body))
		assert.Equal(t, `"tile-etag"`, resp.Header.Get("Etag"))
		assert.Equal(t, "11", resp.Header.Get("Content-Length"))
	})

	t.Run("Rejects invalid and traversal paths", func(t *testing.T) {
		app := &goBlog{cfg: &config{}}
		hc := newFakeHttpClient()
		app.httpClient = hc.Client

		handler := app.proxyVectorTiles()
		for _, invalidPath := range []string{
			"/-/maptiles/",
			"/-/maptiles/.",
			"/-/maptiles/..",
			"/-/maptiles/../secret",
			"/-/maptiles/planet/../secret",
			"/-/maptiles//evil.com/foo",
			"/-/maptiles/planet/",
			"/-/maptiles/planet//1.pbf",
		} {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.org", nil)
			require.NoError(t, err)
			req.URL.Path = invalidPath
			resp, err := doHandlerRequest(req, handler)
			require.NoError(t, err)
			_ = resp.Body.Close()
			assert.Equal(t, http.StatusNotFound, resp.StatusCode, "expected 404 for %s", invalidPath)
			assert.Nil(t, hc.req, "expected no upstream request for %s", invalidPath)
		}
	})

	t.Run("Escapes special characters in resource path", func(t *testing.T) {
		app := &goBlog{cfg: &config{}}
		hc := newFakeHttpClient()
		hc.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		app.httpClient = hc.Client

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.org", nil)
		require.NoError(t, err)
		req.URL.Path = "/-/maptiles/planet?injected=1#frag"
		resp, err := doHandlerRequest(req, app.proxyVectorTiles())
		require.NoError(t, err)
		_ = resp.Body.Close()

		require.NotNil(t, hc.req)
		assert.Equal(t, "tiles.openfreemap.org", hc.req.URL.Host)
		assert.Empty(t, hc.req.URL.RawQuery)
		assert.Empty(t, hc.req.URL.Fragment)
		assert.Equal(t, "/planet?injected=1#frag", hc.req.URL.Path)
	})
}

func Test_getMinZoom(t *testing.T) {
	app := &goBlog{
		cfg: &config{},
	}

	assert.Equal(t, 0, app.getMinZoom())

	app.cfg.MapTiles = &configMapTiles{
		MinZoom: 1,
	}

	assert.Equal(t, 1, app.getMinZoom())
}

func Test_getMaxZoom(t *testing.T) {
	app := &goBlog{
		cfg: &config{},
	}

	assert.Equal(t, 19, app.getMaxZoom())

	app.cfg.MapTiles = &configMapTiles{
		MaxZoom: 10,
	}

	assert.Equal(t, 10, app.getMaxZoom())
}

func Test_getMapAttribution(t *testing.T) {
	app := &goBlog{
		cfg: &config{},
	}

	assert.Equal(t, `&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors`, app.getMapAttribution())

	app.cfg.MapTiles = &configMapTiles{
		MinZoom: 2,
	}

	assert.Equal(t, `&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors`, app.getMapAttribution())

	app.cfg.MapTiles.Attribution = "attribution"

	assert.Equal(t, "attribution", app.getMapAttribution())

	app.cfg.MapTiles.Vector = true
	assert.Contains(t, app.getMapAttribution(), "OpenFreeMap")
}
