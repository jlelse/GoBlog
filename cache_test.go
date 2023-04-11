package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/dgraph-io/ristretto"
	"github.com/stretchr/testify/assert"
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
	bodyLen := len(ci.body)
	assert.Equal(t, 39, bodyLen)
	eTagLen := len(ci.eTag)
	assert.Equal(t, 3, eTagLen)
	assert.Greater(t, ci.cost(), bodyLen+eTagLen)
}

func Benchmark_cacheKey(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/abc?abc=def&hij=klm", nil)
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			cacheKey(req)
		}
	})
}

func Benchmark_cache_getCache(b *testing.B) {
	c := &cache{}
	c.c, _ = ristretto.NewCache(&ristretto.Config{
		NumCounters: 40 * 1000,
		MaxCost:     20 * 1000 * 1000,
		BufferItems: 64,
	})
	req := httptest.NewRequest(http.MethodGet, "/abc?abc=def&hij=klm", nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "abcdefghijklmnopqrstuvwxyz")
		_, _ = w.Write([]byte("abcdefghijklmnopqrstuvwxyz"))
	})
	for i := 0; i < b.N; i++ {
		c.getCache(strconv.Itoa(i), handler, req)
	}
}
