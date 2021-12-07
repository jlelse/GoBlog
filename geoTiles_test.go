package main

import (
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

	hc := &fakeHttpClient{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("Hello, World!"))
		}),
	}
	app.httpClient = hc

	// Default tile source

	m := chi.NewMux()
	m.Get("/x/tiles/{s}/{z}/{x}/{y}.png", app.proxyTiles("/x/tiles"))

	req, err := http.NewRequest(http.MethodGet, "https://example.org/x/tiles/c/8/134/84.png", nil)
	require.NoError(t, err)
	resp, err := doHandlerRequest(req, m)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "https://c.tile.openstreetmap.org/8/134/84.png", hc.req.URL.String())

	// Custom tile source

	app.cfg.MapTiles = &configMapTiles{
		Source: "https://{s}.tile.opentopomap.org/{z}/{x}/{y}.png",
	}

	m = chi.NewMux()
	m.Get("/x/tiles/{s}/{z}/{x}/{y}.png", app.proxyTiles("/x/tiles"))

	req, err = http.NewRequest(http.MethodGet, "https://example.org/x/tiles/c/8/134/84.png", nil)
	require.NoError(t, err)
	resp, err = doHandlerRequest(req, m)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "https://c.tile.opentopomap.org/8/134/84.png", hc.req.URL.String())
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

	assert.Equal(t, 20, app.getMaxZoom())

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
		Attribution: "attribution",
	}

	assert.Equal(t, "attribution", app.getMapAttribution())
}
