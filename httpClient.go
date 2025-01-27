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

func cloneHttpClient(original *http.Client) *http.Client {
	return &http.Client{
		Timeout:   original.Timeout,
		Transport: original.Transport,
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

type apSignRequestTransport struct {
	parent  http.RoundTripper
	blogIri string
	app     *goBlog
}

func (t *apSignRequestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := t.app.signRequest(r, t.blogIri); err != nil {
		return nil, err
	}
	return t.parent.RoundTrip(r)
}

func (a *goBlog) newactivityPubSignRequestTransport(parent http.RoundTripper, blogIri string) *apSignRequestTransport {
	return &apSignRequestTransport{parent: parent, blogIri: blogIri, app: a}
}
