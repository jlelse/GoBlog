package main

import (
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

func Benchmark_cache_getCache(b *testing.B) {
	c := &cache{}
	c.c = cpkg.New[string, *cacheItem](time.Minute, 10*bodylimit.MB)
	req := httptest.NewRequest(http.MethodGet, "/abc?abc=def&hij=klm", nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "abcdefghijklmnopqrstuvwxyz")
		_, _ = w.Write([]byte("abcdefghijklmnopqrstuvwxyz"))
	})
	for i := 0; i < b.N; i++ {
		c.getOrCreateCache(strconv.Itoa(i), handler, req)
	}
}

func Test_cache_purge_nil(t *testing.T) {
	var c *cache = nil
	c.purge()

	c = &cache{c: nil}
	c.purge()
}
