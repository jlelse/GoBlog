package main

import (
	"net/http"
	"time"
)

func newHttpClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Minute,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
}
