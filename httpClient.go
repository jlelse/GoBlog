package main

import (
	"net/http"
	"time"
)

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type appHttpClient struct {
	hc *http.Client
}

var _ httpClient = &appHttpClient{}

func (c *appHttpClient) Do(req *http.Request) (*http.Response, error) {
	if c.hc == nil {
		c.hc = &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		}
	}
	return c.hc.Do(req)
}
