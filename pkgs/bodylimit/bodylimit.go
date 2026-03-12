// Package bodylimit provides HTTP request body size limiting middleware.
package bodylimit

import "net/http"

const (
	// KB is one kilobyte (decimal).
	KB int64 = 1000
	// MB is one megabyte (decimal).
	MB = 1000 * KB
	// GB is one gigabyte (decimal).
	GB = 1000 * MB
	// TB is one terabyte (decimal).
	TB = 1000 * GB
	// PB is one petabyte (decimal).
	PB = 1000 * TB

	// KiB is one kibibyte (binary).
	KiB int64 = 1024
	// MiB is one mebibyte (binary).
	MiB = 1024 * KiB
	// GiB is one gibibyte (binary).
	GiB = 1024 * MiB
	// TiB is one tebibyte (binary).
	TiB = 1024 * GiB
	// PiB is one pebibyte (binary).
	PiB = 1024 * TiB
)

// BodyLimit returns HTTP middleware that limits request body size.
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
