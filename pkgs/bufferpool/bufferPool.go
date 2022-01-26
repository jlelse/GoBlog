package bufferpool

import (
	"bytes"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

var poolMutex sync.Mutex

func Get() *bytes.Buffer {
	poolMutex.Lock()
	defer poolMutex.Unlock()
	return bufferPool.Get().(*bytes.Buffer)
}

func Put(bufs ...*bytes.Buffer) {
	poolMutex.Lock()
	defer poolMutex.Unlock()
	for _, buf := range bufs {
		buf.Reset()
		bufferPool.Put(buf)
	}
}
