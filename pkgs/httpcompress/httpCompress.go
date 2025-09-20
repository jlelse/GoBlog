package httpcompress

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"go.goblog.app/app/pkgs/contenttype"
)

var (
	zstdWriterPool = sync.Pool{
		New: func() any {
			w, _ := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
			return w
		},
	}
	gzipWriterPool = sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
			return w
		},
	}
	// Global list of compressible content types
	compressibleTypes = []string{
		contenttype.AS,
		contenttype.ATOM,
		contenttype.CSS,
		contenttype.HTML,
		contenttype.JS,
		contenttype.JSON,
		contenttype.JSONFeed,
		contenttype.LDJSON,
		contenttype.RSS,
		contenttype.Text,
		contenttype.XML,
		"application/opensearchdescription+xml",
		"application/jrd+json",
		"application/xrd+xml",
	}
)

func CompressMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")
		supportsZstd := strings.Contains(acceptEncoding, "zstd")

		if !supportsGzip && !supportsZstd {
			next.ServeHTTP(w, r)
			return
		}

		cw := &compressWriter{
			ResponseWriter: w,
			supportsGzip:   supportsGzip,
			supportsZstd:   supportsZstd,
		}
		defer cw.Close()
		next.ServeHTTP(cw, r)
	})
}

type compressWriter struct {
	http.ResponseWriter
	supportsGzip, supportsZstd    bool
	writer                        io.Writer
	contentType                   string
	headerWritten, compressionSet bool
	statusCode                    int
}

func (cw *compressWriter) WriteHeader(statusCode int) {
	if cw.headerWritten {
		return
	}
	cw.statusCode = statusCode

	cw.contentType = cw.Header().Get("Content-Type")
	cw.setupCompression()

	cw.ResponseWriter.WriteHeader(statusCode)
	cw.headerWritten = true
}

func (cw *compressWriter) Write(p []byte) (int, error) {
	if !cw.headerWritten {
		cw.WriteHeader(http.StatusOK)
	}
	return cw.writer.Write(p)
}

func (cw *compressWriter) setupCompression() {
	if cw.compressionSet {
		return
	}
	cw.compressionSet = true

	shouldCompress := false
	for _, t := range compressibleTypes {
		if strings.HasPrefix(cw.contentType, t) {
			shouldCompress = true
			break
		}
	}

	if shouldCompress {
		if cw.supportsZstd {
			zw := zstdWriterPool.Get().(*zstd.Encoder)
			zw.Reset(cw.ResponseWriter)
			cw.writer = zw
			cw.Header().Set("Content-Encoding", "zstd")
		} else if cw.supportsGzip {
			gw := gzipWriterPool.Get().(*gzip.Writer)
			gw.Reset(cw.ResponseWriter)
			cw.writer = gw
			cw.Header().Set("Content-Encoding", "gzip")
		}
		cw.Header().Add("Vary", "Accept-Encoding")
		cw.Header().Del("Content-Length")
	} else {
		cw.writer = cw.ResponseWriter
	}
}

func (cw *compressWriter) Close() (err error) {
	if cw.writer != nil {
		if zw, ok := cw.writer.(*zstd.Encoder); ok {
			err = zw.Close()
			zw.Reset(nil)
			zstdWriterPool.Put(zw)
		} else if gw, ok := cw.writer.(*gzip.Writer); ok {
			err = gw.Close()
			gw.Reset(nil)
			gzipWriterPool.Put(gw)
		}
		cw.writer = nil
	}
	return err
}

// Flush implements the http.Flusher interface.
func (cw *compressWriter) Flush() {
	if !cw.headerWritten {
		cw.WriteHeader(cw.statusCode)
	}
	if f, ok := cw.ResponseWriter.(http.Flusher); ok {
		if cw.writer != nil {
			if gw, ok := cw.writer.(*gzip.Writer); ok {
				gw.Flush()
			}
			// Note: zstd.Encoder doesn't have a Flush method
		}
		f.Flush()
	}
}

// Hijack implements the http.Hijacker interface.
func (cw *compressWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := cw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push implements the http.Pusher interface.
func (cw *compressWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := cw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}
