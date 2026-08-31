package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startMainServer(t *testing.T, app *goBlog) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	app.cfg.Server.Port = ln.Addr().(*net.TCPAddr).Port
	go func() {
		_ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	}()
}

func Test_serveHealth(t *testing.T) {
	t.Run("returns ok when healthy", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		require.NoError(t, app.initConfig(false))
		app.httpClient = newHTTPClient()
		startMainServer(t, app)

		req := httptest.NewRequest(http.MethodGet, healthPath, nil)
		rr := httptest.NewRecorder()
		app.serveHealth(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("returns service unavailable when database is unreachable", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		require.NoError(t, app.initConfig(false))
		app.httpClient = newHTTPClient()
		startMainServer(t, app)
		require.NoError(t, app.db.readDb.Close())

		req := httptest.NewRequest(http.MethodGet, healthPath, nil)
		rr := httptest.NewRecorder()
		app.serveHealth(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	t.Run("returns service unavailable when main server is down", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		require.NoError(t, app.initConfig(false))
		app.httpClient = newHTTPClient()

		req := httptest.NewRequest(http.MethodGet, healthPath, nil)
		rr := httptest.NewRecorder()
		app.serveHealth(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	t.Run("returns service unavailable when imgproxy is unreachable", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		require.NoError(t, app.initConfig(false))
		app.httpClient = newHTTPClient()
		startMainServer(t, app)
		app.cfg.MediaOptimization = &configMediaOptimization{
			Enabled:     true,
			ImgproxyURL: "http://127.0.0.1:1",
		}

		req := httptest.NewRequest(http.MethodGet, healthPath, nil)
		rr := httptest.NewRecorder()
		app.serveHealth(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})
}

func Test_healthcheck(t *testing.T) {
	t.Run("returns nil when healthy via health check address", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, healthPath, r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		app := &goBlog{
			cfg:        createDefaultTestConfig(t),
			httpClient: server.Client(),
		}
		app.cfg.Server.HealthCheckAddress = strings.TrimPrefix(server.URL, "http://")

		require.NoError(t, app.healthcheck())
	})

	t.Run("falls back to ping without health check address", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, pingPath, r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		app := &goBlog{
			cfg:        createDefaultTestConfig(t),
			httpClient: server.Client(),
		}
		app.cfg.Server.PublicAddress = server.URL

		require.NoError(t, app.healthcheck())
	})

	t.Run("returns error on unhealthy status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "database not reachable", http.StatusServiceUnavailable)
		}))
		defer server.Close()

		app := &goBlog{
			cfg:        createDefaultTestConfig(t),
			httpClient: server.Client(),
		}
		app.cfg.Server.HealthCheckAddress = strings.TrimPrefix(server.URL, "http://")

		err := app.healthcheck()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "503")
		assert.Contains(t, err.Error(), "database not reachable")
	})

	t.Run("returns error on request failure", func(t *testing.T) {
		app := &goBlog{
			cfg:        createDefaultTestConfig(t),
			httpClient: newHTTPClient(),
		}
		app.cfg.Server.HealthCheckAddress = "127.0.0.1:1"

		err := app.healthcheck()
		require.Error(t, err)
	})
}