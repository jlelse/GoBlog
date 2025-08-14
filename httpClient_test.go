package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/carlmjohnson/requests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHttpClient struct {
	mu      sync.Mutex
	handler http.Handler
	*http.Client
	req *http.Request
	res *http.Response
}

func newFakeHttpClient() *fakeHttpClient {
	fc := &fakeHttpClient{}
	fc.Client = newHandlerClient(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		fc.req = r
		if fc.handler != nil {
			rec := httptest.NewRecorder()
			fc.handler.ServeHTTP(rec, r)
			res := rec.Result()
			fc.res = res
			// Copy the headers from the response recorder
			for k, v := range rec.Header() {
				rw.Header()[k] = v
			}
			// Copy result status code and body
			rw.WriteHeader(rec.Code)
			_, _ = io.Copy(rw, rec.Body)
			// Close response body
			_ = res.Body.Close()
		}
	}))
	return fc
}

func (c *fakeHttpClient) clean() {
	c.mu.Lock()
	c.req = nil
	c.res = nil
	c.handler = nil
	c.mu.Unlock()
}

func (c *fakeHttpClient) setHandler(handler http.Handler) {
	c.clean()
	c.mu.Lock()
	c.handler = handler
	c.mu.Unlock()
}

func (c *fakeHttpClient) setFakeResponse(statusCode int, body string) {
	c.setHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(statusCode)
		_, _ = rw.Write([]byte(body))
	}))
}

func Test_fakeHttpClient(t *testing.T) {
	fc := newFakeHttpClient()
	fc.setFakeResponse(http.StatusNotFound, "Not found")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8080/", nil)
	resp, err := fc.Client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	_ = resp.Body.Close()
}

func Test_addUserAgent(t *testing.T) {
	ua := "ABC"

	client := &http.Client{
		Transport: newAddUserAgentTransport(&handlerRoundTripper{
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ua = r.Header.Get(userAgent)
			}),
		}),
	}

	err := requests.URL("http://example.com").UserAgent("WRONG").Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Equal(t, appUserAgent, ua)
}
