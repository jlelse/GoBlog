package httpcompress

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
)

func TestCompressMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		acceptEncoding string
		contentType    string
		body           string
		shouldEncode   bool
	}{
		{
			name:           "No compression",
			acceptEncoding: "",
			contentType:    "text/plain",
			body:           "Hello, World!",
			shouldEncode:   false,
		},
		{
			name:           "Gzip compression",
			acceptEncoding: "gzip",
			contentType:    "text/html",
			body:           "<html><body>Hello, World!</body></html>",
			shouldEncode:   true,
		},
		{
			name:           "Zstd compression",
			acceptEncoding: "zstd",
			contentType:    "application/json",
			body:           `{"message": "Hello, World!"}`,
			shouldEncode:   true,
		},
		{
			name:           "Non-compressible content type",
			acceptEncoding: "gzip,zstd",
			contentType:    "image/jpeg",
			body:           "binary data",
			shouldEncode:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.body))
			})

			compressHandler := CompressMiddleware(handler)

			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			rec := httptest.NewRecorder()

			compressHandler.ServeHTTP(rec, req)

			resp := rec.Result()
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
			}

			decompressedBody, err := decompressBody(body, resp.Header.Get("Content-Encoding"))
			if err != nil {
				t.Fatalf("Failed to decompress body: %v", err)
			}

			if string(decompressedBody) != tt.body {
				t.Errorf("Response body doesn't match. Expected %q, got %q", tt.body, string(decompressedBody))
			}

			contentEncoding := resp.Header.Get("Content-Encoding")
			if tt.shouldEncode {
				if contentEncoding == "" {
					t.Errorf("Expected Content-Encoding header to be set for compressible content")
				}
				if resp.Header.Get("Vary") != "Accept-Encoding" {
					t.Errorf("Expected Vary header to be set")
				}
			} else if contentEncoding != "" {
				t.Errorf("Content-Encoding header should not be set for non-compressible content")
			}
		})
	}
}

func decompressBody(body []byte, encoding string) ([]byte, error) {
	switch encoding {
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	case "zstd":
		reader, err := zstd.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	default:
		return body, nil
	}
}
