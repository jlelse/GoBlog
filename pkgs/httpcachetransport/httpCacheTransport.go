package httpcachetransport

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/dgraph-io/ristretto"
)

type httpCacheTransport struct {
	parent         http.RoundTripper
	ristrettoCache *ristretto.Cache
	ttl            time.Duration
	body           bool
	maxSize        int64
}

func (t *httpCacheTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	requestUrl := r.URL.String()
	if t.ristrettoCache != nil {
		if cached, hasCached := t.ristrettoCache.Get(requestUrl); hasCached {
			if cachedResp, ok := cached.([]byte); ok {
				return http.ReadResponse(bufio.NewReader(bytes.NewReader(cachedResp)), r)
			}
		}
	}

	resp, err := t.parent.RoundTrip(r)
	if err == nil && t.ristrettoCache != nil {
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
		t.ristrettoCache.SetWithTTL(requestUrl, respBytes, 1, t.ttl)
		t.ristrettoCache.Wait()
		return http.ReadResponse(bufio.NewReader(bytes.NewReader(respBytes)), r)
	}
	return resp, err
}

// Creates a new http.RoundTripper that caches all
// request responses (by the request URL) in ristretto.
func NewHttpCacheTransport(parent http.RoundTripper, ristrettoCache *ristretto.Cache, ttl time.Duration, maxSize int64) http.RoundTripper {
	return &httpCacheTransport{parent, ristrettoCache, ttl, true, maxSize}
}

// Like NewHttpCacheTransport but doesn't cache body
func NewHttpCacheTransportNoBody(parent http.RoundTripper, ristrettoCache *ristretto.Cache, ttl time.Duration, maxSize int64) http.RoundTripper {
	return &httpCacheTransport{parent, ristrettoCache, ttl, false, maxSize}
}
