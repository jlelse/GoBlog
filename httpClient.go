package main

import (
	"net/http"
	"time"

	"github.com/klauspost/compress/gzhttp"
)

func newHttpClient() *http.Client {
	return &http.Client{
		Timeout: time.Minute,
		Transport: gzhttp.Transport(&http.Transport{
			DisableKeepAlives: true,
		}),
	}
}
