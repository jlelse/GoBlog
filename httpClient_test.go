package main

import (
	"net/http"
	"net/http/httptest"
)

type fakeHttpClient struct {
	req     *http.Request
	res     *http.Response
	handler http.Handler
}

func (c *fakeHttpClient) Do(req *http.Request) (*http.Response, error) {
	if c.handler == nil {
		return nil, nil
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	c.req = req
	c.res = rec.Result()
	return c.res, nil
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

func getFakeHTTPClient() *fakeHttpClient {
	return &fakeHttpClient{}
}
