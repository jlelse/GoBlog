package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/carlmjohnson/requests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_httpFs(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	t.Run("Leaflet", func(t *testing.T) {
		t.Parallel()
		testFs(t, app, leafletFiles, "/-/", []string{
			"/-/leaflet/leaflet.js",
			"/-/leaflet/leaflet.css",
			"/-/leaflet/markercluster.js",
			"/-/leaflet/markercluster.css",
			"/-/leaflet/markercluster.default.css",
		})
	})

	t.Run("Hls.js", func(t *testing.T) {
		t.Parallel()
		testFs(t, app, hlsjsFiles, "/-/", []string{
			"/-/hlsjs/hls.js",
		})
	})

	t.Run("MapLibre", func(t *testing.T) {
		t.Parallel()
		testFs(t, app, mapLibreFiles, "/-/", []string{
			"/-/maplibre/maplibre-gl.js",
			"/-/maplibre/maplibre-gl-worker.js",
			"/-/maplibre/maplibre-gl.css",
			"/-/maplibre/goblog.css",
		})
	})

	t.Run("BundleHashesAndCSP", func(t *testing.T) {
		t.Parallel()
		filesystems := map[string]fs.FS{
			"leaflet/leaflet.js":       leafletFiles,
			"leaflet/markercluster.js": leafletFiles,
			"hlsjs/hls.js":             hlsjsFiles,
			"maplibre/maplibre-gl.js":  mapLibreFiles,
		}
		geomapJS, err := os.ReadFile("templates/assets/js/geomap.js")
		require.NoError(t, err)
		videoJS, err := os.ReadFile("templates/assets/js/video.js")
		require.NoError(t, err)

		for path, expectedHash := range bundleHashes {
			fsys, ok := filesystems[path]
			require.True(t, ok, "untested bundle path %s", path)
			data, err := fs.ReadFile(fsys, path)
			require.NoError(t, err)
			sum := sha256.Sum256(data)
			actualHash := base64.StdEncoding.EncodeToString(sum[:])
			assert.Equal(t, expectedHash, actualHash, "bundleHashes mismatch for %s", path)
		}

		assert.Contains(t, string(geomapJS), "sha256-"+bundleHashes["leaflet/leaflet.js"])
		assert.Contains(t, string(geomapJS), "sha256-"+bundleHashes["leaflet/markercluster.js"])
		assert.Contains(t, string(geomapJS), "sha256-"+bundleHashes["maplibre/maplibre-gl.js"])
		assert.Contains(t, string(videoJS), "sha256-"+bundleHashes["hlsjs/hls.js"])

		maplibreJS, err := fs.ReadFile(mapLibreFiles, "maplibre/maplibre-gl.js")
		require.NoError(t, err)
		assert.NotContains(t, string(maplibreJS), "this._canvas.style.width")
	})

}

func testFs(t *testing.T, app *goBlog, files fs.FS, prefix string, paths []string) {
	handler := app.serveFs(files, prefix)

	for _, fp := range paths {
		t.Run(fp, func(t *testing.T) {
			fp := fp

			t.Parallel()

			w := httptest.NewRecorder()
			r, _ := requests.URL(fp).Method(http.MethodGet).Request(context.Background())

			handler.ServeHTTP(w, r)

			result := w.Result()
			bodyContent, _ := io.ReadAll(result.Body)
			result.Body.Close()

			require.NotEmpty(t, bodyContent)
		})
	}
}
