package httpcompress

import (
	"bufio"
	"cmp"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
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
// on Accept-Encoding request header.
func Compress(types ...string) func(next http.Handler) http.Handler {
	return NewCompressor(types...).Handler
}

// Compressor represents a set of encoding configurations.
type Compressor struct {
	// The mapping of pooled encoders to pools.
	pooledEncoders map[string]*sync.Pool
	// The set of content types allowed to be compressed.
	allowedTypes map[string]any
	// The list of encoders in order of decreasing precedence.
	encodingPrecedence []string
}

// NewCompressor creates a new Compressor that will handle encoding responses.
//
// The types are the content types that are allowed to be compressed.
func NewCompressor(types ...string) *Compressor {
	// If types are provided, set those as the allowed types. If none are
	// provided, use the default list.
	if len(types) == 0 {
		types = defaultCompressibleContentTypes
	}

	// Build map based on types
	allowedTypes := lo.SliceToMap(types, func(t string) (string, any) { return t, nil })

	c := &Compressor{
		pooledEncoders: map[string]*sync.Pool{},
		allowedTypes:   allowedTypes,
	}

	c.SetEncoder("gzip", encoderGzip)
	c.SetEncoder("zstd", encoderZstd)

	return c
}

// Interface for types that allow resetting io.Writers.
type compressWriter interface {
	io.Writer
	Reset(w io.Writer)
	Flush() error
}

// An EncoderFunc is a function that wraps the provided io.Writer with a
// streaming compression algorithm and returns it.
//
// In case of failure, the function should return nil.
type EncoderFunc func(w io.Writer) compressWriter

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
	c.encodingPrecedence = slices.DeleteFunc(c.encodingPrecedence, func(e string) bool { return e == encoding })

	// Register new encoder
	c.pooledEncoders[encoding] = &sync.Pool{
		New: func() any {
			return fn(io.Discard)
		},
	}

	c.encodingPrecedence = append([]string{encoding}, c.encodingPrecedence...)
}

type compressResponseWriter struct {
	http.ResponseWriter                // The response writer to delegate to.
	encoding            string         // The accepted encoding.
	encoder             compressWriter // The encoder to use.
	cleanup             func()         // Cleanup function to reset and repool encoder.
	compressor          *Compressor    // Holds the compressor configuration.
	wroteHeader         bool           // Whether the header has been written.
}

func (c *Compressor) findAcceptedEncoding(r *http.Request) string {
	accepted := strings.Split(strings.ToLower(strings.ReplaceAll(r.Header.Get("Accept-Encoding"), " ", "")), ",")
	for _, name := range c.encodingPrecedence {
		if slices.Contains(accepted, name) {
			// We found accepted encoding
			if _, ok := c.pooledEncoders[name]; ok {
				// And it also exists a pool for the encoder, we can use it
				return name
			}
		}
	}
	return ""
}

func (cw *compressResponseWriter) doCleanup() {
	if cw.cleanup != nil {
		cw.cleanup()
		cw.cleanup = nil
	}
}

// Handler returns a new middleware that will compress the response based on the
// current Compressor.
func (c *Compressor) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoding := c.findAcceptedEncoding(r)
		if encoding == "" {
			// No encoding accepted, serve directly
			next.ServeHTTP(w, r)
			return
		}
		cw := &compressResponseWriter{
			encoding:       encoding,
			compressor:     c,
			ResponseWriter: w,
		}
		next.ServeHTTP(cw, r)
		_ = cw.Close()
		cw.doCleanup()
	})
}

func (cw *compressResponseWriter) isCompressable() bool {
	_, ok := cw.compressor.allowedTypes[strings.SplitN(cw.Header().Get("Content-Type"), ";", 2)[0]]
	return ok
}

func (cw *compressResponseWriter) enableEncoder() {
	pool := cw.compressor.pooledEncoders[cw.encoding]
	cw.encoder = pool.Get().(compressWriter)
	if cw.encoder == nil {
		return
	}
	cw.cleanup = func() {
		encoder := cw.encoder
		cw.encoder = nil
		encoder.Reset(nil)
		pool.Put(encoder)
	}
	cw.encoder.Reset(cw.ResponseWriter)
}

func (cw *compressResponseWriter) WriteHeader(code int) {
	if cw.wroteHeader {
		return
	}

	defer cw.ResponseWriter.WriteHeader(code)
	cw.wroteHeader = true

	if cw.Header().Get("Content-Encoding") != "" || !cw.isCompressable() {
		// Data has already been compressed or is not compressable.
		return
	}

	// Enable encoding
	cw.enableEncoder()
	if cw.encoder == nil {
		return
	}

	cw.Header().Set("Content-Encoding", cw.encoding)
	cw.Header().Add("Vary", "Accept-Encoding")

	// The content-length after compression is unknown
	cw.Header().Del("Content-Length")
}

func (cw *compressResponseWriter) writer() io.Writer {
	return cmp.Or[io.Writer](cw.encoder, cw.ResponseWriter)
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

func encoderGzip(w io.Writer) compressWriter {
	gw, err := gzip.NewWriterLevel(w, 5)
	if err != nil {
		return nil
	}
	return gw
}

func encoderZstd(w io.Writer) compressWriter {
	dw, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil
	}
	return dw
}
