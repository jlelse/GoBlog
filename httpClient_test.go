package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

type fakeHttpClient struct {
	mu      sync.Mutex
	handler http.Handler
	*http.Client
	req *http.Request
	res *http.Response
}

func newFakeHttpClient() *fakeHttpClient {
	fc := &fakeHttpClient{}
	fc.Client = newHandlerClient(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		fc.req = r
		if fc.handler != nil {
			rec := httptest.NewRecorder()
			fc.handler.ServeHTTP(rec, r)
			fc.res = rec.Result()
			// Copy the headers from the response recorder
			for k, v := range rec.Header() {
				rw.Header()[k] = v
			}
			// Copy result status code and body
			rw.WriteHeader(fc.res.StatusCode)
			_, _ = io.Copy(rw, rec.Body)
		}
	}))
	return fc
}

func (c *fakeHttpClient) clean() {
	c.mu.Lock()
	c.req = nil
	c.res = nil
	c.handler = nil
	c.mu.Unlock()
}

func (c *fakeHttpClient) setHandler(handler http.Handler) {
	c.clean()
	c.mu.Lock()
	c.handler = handler
	c.mu.Unlock()
}

func (c *fakeHttpClient) setFakeResponse(statusCode int, body string) {
	c.setHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(statusCode)
		_, _ = rw.Write([]byte(body))
	}))
}
