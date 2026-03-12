// Package bufferpool provides a sync.Pool for bytes.Buffer instances.
package bufferpool

import (
	"bytes"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// Get returns a bytes.Buffer from the pool.
func Get() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

// Put returns a bytes.Buffer to the pool.
func Put(bufs ...*bytes.Buffer) {
	for _, buf := range bufs {
		buf.Reset()
		bufferPool.Put(buf)
	}
}
