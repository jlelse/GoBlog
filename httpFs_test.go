package main

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/carlmjohnson/requests"
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
