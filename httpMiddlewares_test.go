package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
