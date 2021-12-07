package main

import (
	"io"
	"net/http"
	"net/http/httptest"
)

type fakeHttpClient struct {
	*http.Client
	req     *http.Request
	res     *http.Response
	handler http.Handler
}

func newFakeHttpClient() *fakeHttpClient {
	fc := &fakeHttpClient{}
	fc.Client = &http.Client{
		Transport: &handlerRoundTripper{
			handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
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
					io.Copy(rw, rec.Body)
				}
			}),
		},
	}
	return fc
}

func (c *fakeHttpClient) clean() {
	c.req = nil
	c.res = nil
	c.handler = nil
}

func (c *fakeHttpClient) setHandler(handler http.Handler) {
	c.clean()
	c.handler = handler
}

func (c *fakeHttpClient) setFakeResponse(statusCode int, body string) {
	c.setHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(statusCode)
		_, _ = rw.Write([]byte(body))
	}))
}
