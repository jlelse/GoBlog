package main

import (
	"net/http"
	"time"
)

var appHttpClient = &http.Client{
	Timeout: 5 * time.Minute,
	Transport: &http.Transport{
		DisableKeepAlives: true,
	},
}
