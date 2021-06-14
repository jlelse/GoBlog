package main

import (
	"io"
	"net/http"
	"strings"
	"sync"
)

type fakeHttpClient struct {
	req     *http.Request
	res     *http.Response
	err     error
	enabled bool
	// internal
	alt httpClient
	mx  sync.Mutex
}

var fakeAppHttpClient *fakeHttpClient

func init() {
	fakeAppHttpClient = &fakeHttpClient{
		alt: appHttpClient,
	}
	appHttpClient = fakeAppHttpClient
}

func (c *fakeHttpClient) Do(req *http.Request) (*http.Response, error) {
	if !c.enabled {
		return c.alt.Do(req)
	}
	c.req = req
	return c.res, c.err
}

func (c *fakeHttpClient) clean() {
	c.req = nil
	c.err = nil
	c.res = nil
}

func (c *fakeHttpClient) setFakeResponse(statusCode int, body string, err error) {
	c.clean()
	c.err = err
	c.res = &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func (c *fakeHttpClient) lock(enabled bool) {
	c.mx.Lock()
	c.clean()
	c.enabled = enabled
}

func (c *fakeHttpClient) unlock() {
	c.enabled = false
	c.clean()
	c.mx.Unlock()
}
