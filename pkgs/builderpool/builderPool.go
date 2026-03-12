// Package builderpool provides a sync.Pool for strings.Builder instances.
package builderpool

import (
	"strings"
	"sync"
)

var builderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	},
}

// Get returns a strings.Builder from the pool.
func Get() *strings.Builder {
	return builderPool.Get().(*strings.Builder)
}

// Put returns a strings.Builder to the pool.
func Put(bufs ...*strings.Builder) {
	for _, buf := range bufs {
		buf.Reset()
		builderPool.Put(buf)
	}
}
