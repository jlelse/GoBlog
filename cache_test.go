package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.goblog.app/app/pkgs/bodylimit"
	cpkg "go.goblog.app/app/pkgs/cache"
)

func Benchmark_cacheItem_cost(b *testing.B) {
	ci := &cacheItem{
		eTag: "abc",
		code: 200,
		header: http.Header{
			"Content-Type": []string{"text/html"},
		},
		body: []byte("<html>abcdefghijklmnopqrstuvwxyz</html>"),
	}
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			ci.cost()
		}
	})
}

func Test_cacheItem_cost(t *testing.T) {
	ci := &cacheItem{
		header: http.Header{
			"Content-Type": []string{"text/html"},
		},
		body: []byte("<html>abcdefghijklmnopqrstuvwxyz</html>"),
		eTag: "abc",
	}
	bodyLen := int64(len(ci.body))
	assert.Equal(t, int64(39), bodyLen)
	eTagLen := int64(len(ci.eTag))
	assert.Equal(t, int64(3), eTagLen)
	assert.Greater(t, ci.cost(), bodyLen+eTagLen)
}

func Benchmark_cacheKey(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/abc?abc=def&hij=klm", nil)
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			generateCacheKey(req)
		}
	})
}

func Benchmark_cacheKey_noQuery(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/posts/hello-world", nil)
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			generateCacheKey(req)
		}
	})
}

func Test_isCacheable(t *testing.T) {
	assert.True(t, isCacheable(httptest.NewRequest(http.MethodGet, "/post", nil)))
	assert.True(t, isCacheable(httptest.NewRequest(http.MethodHead, "/post?foo=bar", nil)))
	assert.False(t, isCacheable(httptest.NewRequest(http.MethodPost, "/post", nil)))
	assert.False(t, isCacheable(httptest.NewRequest(http.MethodGet, "/post?cache=0", nil)))
	assert.False(t, isCacheable(httptest.NewRequest(http.MethodGet, "/post?cache=false", nil)))
	assert.False(t, isCacheable(httptest.NewRequest(http.MethodGet, "/post?%63ache=0", nil)))
}

func Test_isCacheable_queryPreCheck(t *testing.T) {
	// isCacheable skips parsing the query string when it cannot contain the
	// cache parameter. Ensure the fast path always agrees with parsing.
	queries := []string{
		"", "foo=bar", "cache", "cache=", "cache=0", "cache=false", "cache=1",
		"%63ache=0", "c%61che=false", "ca%63he=0", "%63%61%63%68%65=0",
		"cache=%30", "cache=%66alse", "x=1&cache=0", "cache=0&x=1",
		"x=%63&cache=0", "cache=0;x=1", "cache=%zz", "CACHE=0",
		"ncache=0", "cachex=0", "x=cache=0",
	}
	for _, q := range queries {
		r := httptest.NewRequest(http.MethodGet, "/post?"+q, nil)
		cacheParam := r.URL.Query().Get("cache")
		want := cacheParam != "0" && cacheParam != "false"
		assert.Equal(t, want, isCacheable(r), "query %q", q)
	}
}

func Benchmark_cache_getCache(b *testing.B) {
	c := &cache{}
	c.c = cpkg.New[string, *cacheItem](time.Minute, 10*bodylimit.MB)
	req := httptest.NewRequest(http.MethodGet, "/abc?abc=def&hij=klm", nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "abcdefghijklmnopqrstuvwxyz")
		_, _ = w.Write([]byte("abcdefghijklmnopqrstuvwxyz"))
	})
	for i := 0; b.Loop(); i++ {
		c.getOrCreateCache(strconv.Itoa(i), handler, req)
	}
}

func Test_cache_purge_nil(t *testing.T) {
	var c *cache = nil
	c.purge()

	c = &cache{c: nil}
	c.purge()
}

func Test_etagMatches(t *testing.T) {
	const eTag = `"abc123"`
	for _, tc := range []struct {
		name        string
		ifNoneMatch []string
		want        bool
	}{
		{name: "none", ifNoneMatch: nil, want: false},
		{name: "empty", ifNoneMatch: []string{""}, want: false},
		{name: "single", ifNoneMatch: []string{eTag}, want: true},
		{name: "weak", ifNoneMatch: []string{"W/" + eTag}, want: true},
		{name: "different", ifNoneMatch: []string{`"xyz789"`}, want: false},
		{name: "list", ifNoneMatch: []string{`"xyz789", "abc123"`}, want: true},
		{name: "multiple headers", ifNoneMatch: []string{`"xyz789"`, eTag}, want: true},
		{name: "star", ifNoneMatch: []string{"*"}, want: true},
		{name: "whitespace", ifNoneMatch: []string{" " + eTag + " "}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, etagMatches(tc.ifNoneMatch, eTag))
		})
	}
}

func TestCachedResponsePreservesCacheRestrictions(t *testing.T) {
	app := &goBlog{}
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	req = req.WithContext(context.WithValue(req.Context(), publicCacheKey, true))
	for _, tc := range []struct {
		name   string
		header http.Header
	}{
		{name: "private", header: http.Header{cacheControl: {"private, max-age=0"}}},
		{name: "no-store", header: http.Header{cacheControl: {"max-age=0", "No-Store"}}},
		{name: "cookie", header: http.Header{"Set-Cookie": {"session=probe"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.header.Set(contentType, "text/html")
			assert.False(t, canStoreResponse(tc.header))
			res := httptest.NewRecorder()
			app.serveCachedResponse(res, req, &cacheItem{code: http.StatusOK, header: tc.header})
			assert.Equal(t, tc.header.Values(cacheControl), res.Header().Values(cacheControl))
			assert.Empty(t, res.Header().Get(cdnCacheableHeader))
			assert.Equal(t, tc.header.Values("Set-Cookie"), res.Header().Values("Set-Cookie"))
		})
	}
}
