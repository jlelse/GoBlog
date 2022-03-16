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

func Get() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

func Put(bufs ...*bytes.Buffer) {
	for _, buf := range bufs {
		buf.Reset()
		bufferPool.Put(buf)
	}
}
