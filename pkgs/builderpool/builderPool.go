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

func Get() *strings.Builder {
	return builderPool.Get().(*strings.Builder)
}

func Put(bufs ...*strings.Builder) {
	for _, buf := range bufs {
		buf.Reset()
		builderPool.Put(buf)
	}
}
