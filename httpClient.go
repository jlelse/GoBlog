package main

import (
	"net/http"
	"time"
)

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func getHTTPClient() httpClient {
	return &http.Client{
		Timeout: 5 * time.Minute,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
}
