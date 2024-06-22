package httpcompress

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
)

func TestCompressor(t *testing.T) {
	r := chi.NewRouter()

	compressor := NewCompressor("text/html", "text/css")
	if len(compressor.pooledEncoders) != 2 {
		t.Errorf("gzip and zstd should be pooled")
	}

	r.Use(compressor.Handler)

	r.Get("/gethtml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("textstring"))
	})

	r.Get("/getjpeg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "images/jpeg")
		w.Write([]byte("textstring"))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	tests := []struct {
		name              string
		path              string
		expectedEncoding  string
		acceptedEncodings string
	}{
		{
			name:              "no expected encodings due to no accepted encodings",
			path:              "/gethtml",
			acceptedEncodings: "",
			expectedEncoding:  "",
		},
		{
			name:              "no expected encodings due to content type",
			path:              "/getjpeg",
			acceptedEncodings: "",
			expectedEncoding:  "",
		},
		{
			name:              "gzip is only encoding",
			path:              "/gethtml",
			acceptedEncodings: "gzip",
			expectedEncoding:  "gzip",
		},
		{
			name:              "zstd is only encoding",
			path:              "/gethtml",
			acceptedEncodings: "zstd",
			expectedEncoding:  "zstd",
		},
		{
			name:              "deflate is only encoding",
			path:              "/gethtml",
			acceptedEncodings: "deflate",
			expectedEncoding:  "",
		},
		{
			name:              "multiple encoding seperated with comma and space",
			path:              "/gethtml",
			acceptedEncodings: "zstd, gzip, deflate",
			expectedEncoding:  "zstd",
		},
		{
			name:              "multiple encoding seperated with comma and without space",
			path:              "/gethtml",
			acceptedEncodings: "zstd,gzip,deflate",
			expectedEncoding:  "zstd",
		},
		{
			name:              "multiple encoding",
			path:              "/gethtml",
			acceptedEncodings: "gzip, zstd",
			expectedEncoding:  "zstd",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, respString := testRequestWithAcceptedEncodings(t, ts, "GET", tc.path, tc.acceptedEncodings)
			if respString != "textstring" {
				t.Errorf("response text doesn't match; expected:%q, got:%q", "textstring", respString)
			}
			if got := resp.Header.Get("Content-Encoding"); got != tc.expectedEncoding {
				t.Errorf("expected encoding %q but got %q", tc.expectedEncoding, got)
			}

		})

	}
}

func testRequestWithAcceptedEncodings(t *testing.T, ts *httptest.Server, method, path string, encodings string) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, nil)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}
	if encodings != "" {
		req.Header.Set("Accept-Encoding", encodings)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}

	respBody := decodeResponseBody(t, resp)
	defer resp.Body.Close()

	return resp, respBody
}

func decodeResponseBody(t *testing.T, resp *http.Response) string {
	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		var err error
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
	case "zstd":
		var err error
		reader, err = zstd.NewReader(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
	default:
		reader = resp.Body
	}
	respBody, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
		return ""
	}
	if closer, ok := reader.(io.ReadCloser); ok {
		closer.Close()
	}

	return string(respBody)
}
