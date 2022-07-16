package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_httpLogsConfig(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	assert.Equal(t, false, app.cfg.Server.Logging)
	assert.Equal(t, "data/access.log", app.cfg.Server.LogFile)
}

func initTestHttpLogs(logFile string) (http.Handler, error) {

	app := &goBlog{
		cfg: &config{
			Server: &configServer{
				Logging: true,
				LogFile: logFile,
			},
		},
	}

	err := app.initHTTPLog()
	if err != nil {
		return nil, err
	}

	return app.logMiddleware(testHttpHandler()), nil

}

func testHttpHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_, _ = rw.Write([]byte("Test"))
	})
}

func Test_httpLogs(t *testing.T) {

	// Init

	logFile := filepath.Join(t.TempDir(), "access.log")
	handler, err := initTestHttpLogs(logFile)

	require.NoError(t, err)

	// Do fake request

	req := httptest.NewRequest(http.MethodGet, "/testpath", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check response

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Test")

	// Check log

	logBytes, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logString := string(logBytes)
	assert.Contains(t, logString, "GET /testpath")

}

func Benchmark_httpLogs(b *testing.B) {

	// Init

	logFile := filepath.Join(b.TempDir(), "access.log")
	logHandler, err := initTestHttpLogs(logFile)
	require.NoError(b, err)

	noLogHandler := testHttpHandler()

	// Run benchmarks

	b.Run("With logging", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				logHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/testpath", nil))
			}
		})

	})

	b.Run("Without logging", func(b *testing.B) {
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				noLogHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/testpath", nil))
			}
		})
	})

}
