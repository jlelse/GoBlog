package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_noIndexHeader(t *testing.T) {
	h := noIndexHeader(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// Do nothing
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.org", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	_ = res.Body.Close()
	assert.Equal(t, "noindex", res.Header.Get("X-Robots-Tag"))
}

func Test_fixHTTPHandler(t *testing.T) {

	var got *http.Request

	h := fixHTTPHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		got = r
	}))

	rec := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "http://example.org/übung", nil)
	h.ServeHTTP(rec, req)

	assert.Equal(t, "/übung", got.URL.Path)
	assert.Equal(t, "", got.URL.RawPath)

	req = httptest.NewRequest(http.MethodGet, "http://example.org/%C3%BCbung", nil)
	h.ServeHTTP(rec, req)

	assert.Equal(t, "/übung", got.URL.Path)
	assert.Equal(t, "", got.URL.RawPath)

}

func Test_keepSelectedQueryParams(t *testing.T) {
	var got *http.Request

	h := keepSelectedQueryParams("size")(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		got = r
	}))

	rec := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "http://example.org/test?def=1234&size=123&abc=def", nil)
	h.ServeHTTP(rec, req)

	assert.Equal(t, "/test?size=123", got.URL.RequestURI())
}

// newCSPBenchmarkApp creates a goBlog with the given number of fake registered
// assets, backed by a default test configuration.
func newCSPBenchmarkApp(b *testing.B, numAssets int) *goBlog {
	b.Helper()
	app := &goBlog{
		cfg: createDefaultTestConfig(b),
	}
	require.NoError(b, app.initConfig(false))
	app.assetFileNames = make(map[string]string, numAssets)
	app.assetFiles = make(map[string]*assetFile, numAssets)
	for i := range numAssets {
		ext := ".css"
		if i%2 == 0 {
			ext = ".js"
		}
		name := fmt.Sprintf("asset%d%s", i, ext)
		compiledName := fmt.Sprintf("compiled%d%s", i, ext)
		app.assetFileNames[name] = compiledName
		app.assetFiles[compiledName] = &assetFile{contentType: "text/plain", sha256base64: "ZGVhZGJlZWY="}
	}
	return app
}

// cspBenchmarkWriter is a minimal ResponseWriter that only collects headers, so the
// benchmarks measure the middleware rather than recorder bookkeeping.
type cspBenchmarkWriter struct {
	header http.Header
}

func (w *cspBenchmarkWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (*cspBenchmarkWriter) Write(p []byte) (int, error) { return len(p), nil }

func (*cspBenchmarkWriter) WriteHeader(int) {}

// Benchmark_securityHeaders measures a request through the securityHeaders middleware
// with the CSP cache warm.
func Benchmark_securityHeaders(b *testing.B) {
	app := newCSPBenchmarkApp(b, 50)
	h := app.securityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "http://example.org/", nil)
	w := &cspBenchmarkWriter{}

	b.ReportAllocs()
	for b.Loop() {
		h.ServeHTTP(w, req)
	}
}

// Benchmark_securityHeaders_parallel measures concurrent requests through the
// securityHeaders middleware with the CSP cache warm.
func Benchmark_securityHeaders_parallel(b *testing.B) {
	app := newCSPBenchmarkApp(b, 50)
	h := app.securityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "http://example.org/", nil)

	b.ReportAllocs()
	// RunParallel doesn't reset the timer like B.Loop does, so exclude the setup.
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := &cspBenchmarkWriter{}
		for pb.Next() {
			h.ServeHTTP(w, req)
		}
	})
}

// Benchmark_securityHeaders_rebuild measures requests through the securityHeaders
// middleware when assets change on every request, forcing a CSP rebuild.
func Benchmark_securityHeaders_rebuild(b *testing.B) {
	app := newCSPBenchmarkApp(b, 50)
	h := app.securityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "http://example.org/", nil)
	w := &cspBenchmarkWriter{}

	b.ReportAllocs()
	for b.Loop() {
		app.assetVersion.Add(1)
		h.ServeHTTP(w, req)
	}
}
