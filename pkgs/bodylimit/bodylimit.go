// package bodylimit provides a HTTP middleware that limits the maximum body size of requests
package bodylimit

import "net/http"

const (
	// Decimal
	KB int64 = 1000
	MB       = 1000 * KB
	GB       = 1000 * MB
	TB       = 1000 * GB
	PB       = 1000 * TB

	// Binary
	KiB int64 = 1024
	MiB       = 1024 * KiB
	GiB       = 1024 * MiB
	TiB       = 1024 * GiB
	PiB       = 1024 * TiB
)

func BodyLimit(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if n > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}
