package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"sync"
	"unsafe"
)

// cacheRecorder is a thread-safe implementation of http.ResponseWriter
type cacheRecorder struct {
	mu   sync.Mutex
	item cacheItem
	done bool
}

type cacheItem struct {
	expiration int
	eTag       string
	code       int
	header     http.Header
	body       []byte
}

func newCacheRecorder() *cacheRecorder {
	return &cacheRecorder{
		item: cacheItem{
			code:   http.StatusOK,
			header: make(http.Header),
		},
	}
}

func (c *cacheRecorder) finish() *cacheItem {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done {
		return &c.item
	}

	c.done = true
	c.item.eTag = c.item.header.Get("ETag")
	if c.item.eTag == "" {
		c.item.eTag = fmt.Sprintf("%x", sha256.Sum256(c.item.body))
	}
	return &c.item
}

// Header implements http.ResponseWriter.
func (c *cacheRecorder) Header() http.Header {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done {
		return nil
	}
	return c.item.header
}

// Write implements http.ResponseWriter.
func (c *cacheRecorder) Write(buf []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done {
		return 0, fmt.Errorf("write after finish")
	}
	c.item.body = append(c.item.body, buf...)
	return len(buf), nil
}

// WriteString implements io.StringWriter.
func (c *cacheRecorder) WriteString(str string) (int, error) {
	return c.Write([]byte(str))
}

// WriteHeader implements http.ResponseWriter.
func (c *cacheRecorder) WriteHeader(code int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.done {
		return
	}
	c.item.code = code
}

func (ci *cacheItem) cost() int64 {
	size := int64(unsafe.Sizeof(*ci)) // Base struct size

	// Add sizes of variable-length fields
	size += int64(len(ci.eTag))
	size += int64(len(ci.body))

	// Calculate header size
	for key, values := range ci.header {
		size += int64(len(key))
		for _, value := range values {
			size += int64(len(value))
		}
	}

	return size
}
