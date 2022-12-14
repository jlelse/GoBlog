package main

import (
	"net/http"
	"time"

	"github.com/klauspost/compress/gzhttp"
)

func newHttpClient() *http.Client {
	return &http.Client{
		Timeout: time.Minute,
		Transport: newAddUserAgentTransport(
			gzhttp.Transport(
				&http.Transport{
					DisableKeepAlives: true,
				},
			),
		),
	}
}

type addUserAgentTransport struct {
	t http.RoundTripper
}

func (t *addUserAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set(userAgent, appUserAgent)
	return t.t.RoundTrip(r)
}

func newAddUserAgentTransport(t http.RoundTripper) *addUserAgentTransport {
	return &addUserAgentTransport{t}
}
