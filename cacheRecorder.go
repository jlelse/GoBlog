package main

import (
	"fmt"
	"net/http"
)

// cacheRecorder is an implementation of http.ResponseWriter
type cacheRecorder struct {
	item *cacheItem
}

func newCacheRecorder() *cacheRecorder {
	return &cacheRecorder{
		item: &cacheItem{
			code:   http.StatusOK,
			header: make(http.Header),
		},
	}
}

func (c *cacheRecorder) finish() (ci *cacheItem) {
	ci = c.item
	c.item = nil
	return
}

// Header implements http.ResponseWriter.
func (rw *cacheRecorder) Header() http.Header {
	if rw.item == nil {
		return nil
	}
	return rw.item.header
}

// Write implements http.ResponseWriter.
func (rw *cacheRecorder) Write(buf []byte) (int, error) {
	if rw.item == nil {
		return 0, nil
	}
	rw.item.body = append(rw.item.body, buf...)
	return len(buf), nil
}

// WriteString implements io.StringWriter.
func (rw *cacheRecorder) WriteString(str string) (int, error) {
	return rw.Write([]byte(str))
}

// WriteHeader implements http.ResponseWriter.
func (rw *cacheRecorder) WriteHeader(code int) {
	if rw.item == nil {
		return
	}
	if code < 100 || code > 999 {
		panic(fmt.Sprintf("invalid WriteHeader code %v", code))
	}
	rw.item.code = code
}

// Flush implements http.Flusher.
func (rw *cacheRecorder) Flush() {
	// Do nothing
}
