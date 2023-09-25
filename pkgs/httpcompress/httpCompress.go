package httpcompress

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/gzip"
	"github.com/samber/lo"

	"go.goblog.app/app/pkgs/contenttype"
)

var defaultCompressibleContentTypes = []string{
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

// Compress is a middleware that compresses response
// body of a given content types to a data format based
// on Accept-Encoding request header. It uses a given
// compression level.
//
// Passing a compression level of 5 is sensible value
func Compress(level int, types ...string) func(next http.Handler) http.Handler {
	return NewCompressor(level, types...).Handler
}

// Compressor represents a set of encoding configurations.
type Compressor struct {
	// The mapping of pooled encoders to pools.
	pooledEncoders map[string]*sync.Pool
	// The set of content types allowed to be compressed.
	allowedTypes map[string]any
	// The list of encoders in order of decreasing precedence.
	encodingPrecedence []string
	// The compression level.
	level int
}

// NewCompressor creates a new Compressor that will handle encoding responses.
//
// The level should be one of the ones defined in the flate package.
// The types are the content types that are allowed to be compressed.
func NewCompressor(level int, types ...string) *Compressor {
	// If types are provided, set those as the allowed types. If none are
	// provided, use the default list.
	allowedTypes := lo.SliceToMap(
		lo.If(len(types) > 0, types).Else(defaultCompressibleContentTypes),
		func(t string) (string, any) { return t, nil },
	)

	c := &Compressor{
		level:          level,
		pooledEncoders: map[string]*sync.Pool{},
		allowedTypes:   allowedTypes,
	}

	c.SetEncoder("deflate", encoderDeflate)
	c.SetEncoder("gzip", encoderGzip)

	return c
}

// SetEncoder can be used to set the implementation of a compression algorithm.
//
// The encoding should be a standardised identifier. See:
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept-Encoding
func (c *Compressor) SetEncoder(encoding string, fn EncoderFunc) {
	encoding = strings.ToLower(encoding)
	if encoding == "" {
		panic("the encoding can not be empty")
	}
	if fn == nil {
		panic("attempted to set a nil encoder function")
	}

	// Deleted already registered encoder
	delete(c.pooledEncoders, encoding)

	c.pooledEncoders[encoding] = &sync.Pool{
		New: func() any {
			return fn(io.Discard, c.level)
		},
	}

	for i, v := range c.encodingPrecedence {
		if v == encoding {
			c.encodingPrecedence = append(c.encodingPrecedence[:i], c.encodingPrecedence[i+1:]...)
			break
		}
	}

	c.encodingPrecedence = append([]string{encoding}, c.encodingPrecedence...)
}

// Handler returns a new middleware that will compress the response based on the
// current Compressor.
func (c *Compressor) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cw := &compressResponseWriter{
			compressor:     c,
			ResponseWriter: w,
			request:        r,
		}
		next.ServeHTTP(cw, r)
		_ = cw.Close()
		cw.doCleanup()
	})
}

// An EncoderFunc is a function that wraps the provided io.Writer with a
// streaming compression algorithm and returns it.
//
// In case of failure, the function should return nil.
type EncoderFunc func(w io.Writer, level int) compressWriter

// Interface for types that allow resetting io.Writers.
type compressWriter interface {
	io.Writer
	Reset(w io.Writer)
	Flush() error
}

type compressResponseWriter struct {
	http.ResponseWriter                // The response writer to delegate to.
	encoder             compressWriter // The encoder to use (if any).
	cleanup             func()         // Cleanup function to reset and repool encoder.
	compressor          *Compressor    // Holds the compressor configuration.
	request             *http.Request  // The request that is being handled.
	wroteHeader         bool           // Whether the header has been written.
}

func (cw *compressResponseWriter) isCompressable() bool {
	// Parse the first part of the Content-Type response header.
	contentType := cw.Header().Get("Content-Type")
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[0:idx]
	}

	// Is the content type compressable?
	_, ok := cw.compressor.allowedTypes[contentType]
	return ok
}

func (cw *compressResponseWriter) writer() io.Writer {
	if cw.encoder != nil {
		return cw.encoder
	}
	return cw.ResponseWriter
}

// selectEncoder returns the encoder, the name of the encoder, and a closer function.
func (cw *compressResponseWriter) selectEncoder() (compressWriter, string, func()) {
	// Parse the names of all accepted algorithms from the header.
	accepted := strings.Split(strings.ToLower(cw.request.Header.Get("Accept-Encoding")), ",")

	// Find supported encoder by accepted list by precedence
	for _, name := range cw.compressor.encodingPrecedence {
		if lo.Contains(accepted, name) {
			if pool, ok := cw.compressor.pooledEncoders[name]; ok {
				encoder := pool.Get().(compressWriter)
				cleanup := func() {
					encoder.Reset(nil)
					pool.Put(encoder)
				}
				encoder.Reset(cw.ResponseWriter)
				return encoder, name, cleanup
			}
		}
	}

	// No encoder found to match the accepted encoding
	return nil, "", nil
}

func (cw *compressResponseWriter) doCleanup() {
	if cw.encoder != nil {
		cw.encoder = nil
		cw.cleanup()
		cw.cleanup = nil
	}
}

func (cw *compressResponseWriter) WriteHeader(code int) {
	defer cw.ResponseWriter.WriteHeader(code)

	if cw.wroteHeader {
		return
	}

	cw.wroteHeader = true

	if cw.Header().Get("Content-Encoding") != "" {
		// Data has already been compressed.
		return
	}

	if !cw.isCompressable() {
		// Data is not compressable.
		return
	}

	var encoding string
	cw.encoder, encoding, cw.cleanup = cw.selectEncoder()
	if encoding != "" {
		cw.Header().Set("Content-Encoding", encoding)
		cw.Header().Add("Vary", "Accept-Encoding")

		// The content-length after compression is unknown
		cw.Header().Del("Content-Length")
	}
}

func (cw *compressResponseWriter) Write(p []byte) (int, error) {
	if !cw.wroteHeader {
		cw.WriteHeader(http.StatusOK)
	}
	return cw.writer().Write(p)
}

func (cw *compressResponseWriter) Flush() {
	if cw.encoder != nil {
		cw.encoder.Flush()
	}
	if f, ok := cw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (cw *compressResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := cw.writer().(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("http.Hijacker is unavailable on the writer")
}

func (cw *compressResponseWriter) Push(target string, opts *http.PushOptions) error {
	if ps, ok := cw.writer().(http.Pusher); ok {
		return ps.Push(target, opts)
	}
	return errors.New("http.Pusher is unavailable on the writer")
}

func (cw *compressResponseWriter) Close() error {
	if c, ok := cw.writer().(io.WriteCloser); ok {
		return c.Close()
	}
	return errors.New("io.WriteCloser is unavailable on the writer")
}

func encoderGzip(w io.Writer, level int) compressWriter {
	gw, err := gzip.NewWriterLevel(w, level)
	if err != nil {
		return nil
	}
	return gw
}

func encoderDeflate(w io.Writer, level int) compressWriter {
	dw, err := flate.NewWriter(w, level)
	if err != nil {
		return nil
	}
	return dw
}
