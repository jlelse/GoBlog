package main

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/klauspost/compress/gzhttp"
)

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   time.Minute,
		Transport: newHTTPTransport(),
	}
}

func newHTTPTransport() http.RoundTripper {
	return newAddUserAgentTransport(
		gzhttp.Transport(newHTTPTransportBase()),
	)
}

func newHTTPTransportBase() *http.Transport {
	return &http.Transport{
		// Default
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext:           newDialContext(),
		// Custom
		DisableKeepAlives: true,
	}
}

func newDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	return newNetDialer().DialContext
}

func newNetDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
}

// newWebmentionHTTPClient returns a client for verifying webmention sources that
// refuses to connect to private or otherwise reserved IP addresses to prevent
// SSRF via user-supplied source URLs.
func newWebmentionHTTPClient() *http.Client {
	base := newHTTPTransportBase()
	base.DialContext = newSSRFGuardDialContext(newNetDialer().DialContext)
	return &http.Client{
		Timeout:   time.Minute,
		Transport: newAddUserAgentTransport(gzhttp.Transport(base)),
	}
}

type addUserAgentTransport struct {
	parent http.RoundTripper
}

func (t *addUserAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Header.Get(userAgent) == "" {
		r.Header.Set(userAgent, appUserAgent)
	}
	return t.parent.RoundTrip(r)
}

func newAddUserAgentTransport(parent http.RoundTripper) *addUserAgentTransport {
	return &addUserAgentTransport{parent}
}
