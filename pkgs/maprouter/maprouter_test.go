package maprouter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapRouter(t *testing.T) {
	router := &MapRouter{
		DefaultHandler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Write([]byte("Default"))
		}),
		Handlers: map[string]http.Handler{
			"a.example.org": http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.Write([]byte("a"))
			}),
			"b.example.org": http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.Write([]byte("b"))
			}),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://a.example.org", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	resBody, _ := io.ReadAll(rec.Result().Body)
	assert.Equal(t, "a", string(resBody))

	req = httptest.NewRequest(http.MethodGet, "http://b.example.org", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	resBody, _ = io.ReadAll(rec.Result().Body)
	assert.Equal(t, "b", string(resBody))

	req = httptest.NewRequest(http.MethodGet, "http://c.example.org", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	resBody, _ = io.ReadAll(rec.Result().Body)
	assert.Equal(t, "Default", string(resBody))

	router.KeyFunc = func(r *http.Request) string {
		return "a.example.org"
	}

	req = httptest.NewRequest(http.MethodGet, "http://c.example.org", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	resBody, _ = io.ReadAll(rec.Result().Body)
	assert.Equal(t, "a", string(resBody))
}
