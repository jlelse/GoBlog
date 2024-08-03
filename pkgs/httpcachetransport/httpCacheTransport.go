package httpcachetransport

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"time"

	cpkg "go.goblog.app/app/pkgs/cache"
)

type httpCacheTransport struct {
	parent  http.RoundTripper
	cache   *cpkg.Cache[string, []byte]
	ttl     time.Duration
	body    bool
	maxSize int64
}

func (t *httpCacheTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	requestUrl := r.URL.String()
	if t.cache != nil {
		if cached, hasCached := t.cache.Get(requestUrl); hasCached {
			return http.ReadResponse(bufio.NewReader(bytes.NewReader(cached)), r)
		}
	}

	resp, err := t.parent.RoundTrip(r)
	if err == nil && t.cache != nil {
		// Limit the response size
		limitedResp := &http.Response{
			Status:        resp.Status,
			StatusCode:    resp.StatusCode,
			Proto:         resp.Proto,
			ProtoMajor:    resp.ProtoMajor,
			ProtoMinor:    resp.ProtoMinor,
			Header:        resp.Header,
			Body:          io.NopCloser(io.LimitReader(resp.Body, t.maxSize)),
			ContentLength: -1,
		}

		respBytes, err := httputil.DumpResponse(limitedResp, t.body)
		if err != nil {
			return resp, err
		}
		t.cache.Set(requestUrl, respBytes, t.ttl, 1)
		return http.ReadResponse(bufio.NewReader(bytes.NewReader(respBytes)), r)
	}
	return resp, err
}

// Creates a new http.RoundTripper that caches all
// request responses (by the request URL) in ristretto.
func NewHttpCacheTransport(parent http.RoundTripper, c *cpkg.Cache[string, []byte], ttl time.Duration, maxSize int64) http.RoundTripper {
	return &httpCacheTransport{parent, c, ttl, true, maxSize}
}

// Like NewHttpCacheTransport but doesn't cache body
func NewHttpCacheTransportNoBody(parent http.RoundTripper, c *cpkg.Cache[string, []byte], ttl time.Duration, maxSize int64) http.RoundTripper {
	return &httpCacheTransport{parent, c, ttl, false, maxSize}
}
