package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
)

// cacheRecorder is an implementation of http.ResponseWriter
type cacheRecorder struct {
	item cacheItem
	done bool
}

func newCacheRecorder() *cacheRecorder {
	return &cacheRecorder{
		item: cacheItem{
			code:   http.StatusOK,
			header: http.Header{},
		},
	}
}

func (c *cacheRecorder) finish() *cacheItem {
	c.done = true
	c.item.eTag = c.item.header.Get("ETag")
	if c.item.eTag == "" {
		c.item.eTag = fmt.Sprintf("%x", sha256.Sum256(c.item.body))
	}
	return &c.item
}

// Header implements http.ResponseWriter.
func (c *cacheRecorder) Header() http.Header {
	if c.done {
		return nil
	}
	return c.item.header
}

// Write implements http.ResponseWriter.
func (c *cacheRecorder) Write(buf []byte) (int, error) {
	if c.done {
		return 0, nil
	}
	c.item.body = append(c.item.body, buf...)
	return len(buf), nil
}

// WriteString implements io.StringWriter.
func (c *cacheRecorder) WriteString(str string) (int, error) {
	if c.done {
		return 0, nil
	}
	return c.Write([]byte(str))
}

// WriteHeader implements http.ResponseWriter.
func (c *cacheRecorder) WriteHeader(code int) {
	if c.done {
		return
	}
	c.item.code = code
}
