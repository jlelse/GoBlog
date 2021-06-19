package main

import (
	"io"
	"net/http"
	"strings"
)

type fakeHttpClient struct {
	req *http.Request
	res *http.Response
	err error
}

func (c *fakeHttpClient) Do(req *http.Request) (*http.Response, error) {
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

func getFakeHTTPClient() *fakeHttpClient {
	return &fakeHttpClient{}
}
