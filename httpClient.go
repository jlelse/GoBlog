package main

import (
	"net"
	"net/http"
	"time"

	"github.com/klauspost/compress/gzhttp"
)

func newHttpClient() *http.Client {
	return &http.Client{
		Timeout:   time.Minute,
		Transport: newHttpTransport(),
	}
}

func newHttpTransport() http.RoundTripper {
	return newAddUserAgentTransport(
		gzhttp.Transport(
			&http.Transport{
				// Default
				Proxy:                 http.ProxyFromEnvironment,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				// Custom
				DisableKeepAlives: true,
			},
		),
	)
}

type addUserAgentTransport struct {
	parent http.RoundTripper
}

func (t *addUserAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set(userAgent, appUserAgent)
	return t.parent.RoundTrip(r)
}

func newAddUserAgentTransport(parent http.RoundTripper) *addUserAgentTransport {
	return &addUserAgentTransport{parent}
}
