package bodylimit

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBodyLimit_EnforcesLimit(t *testing.T) {
	// handler tries to read the whole body and returns 413 on read error
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// create a body larger than 10 bytes
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("this body is definitely larger than ten bytes")))
	rr := httptest.NewRecorder()

	// wrap with a small limit
	BodyLimit(10)(h).ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rr.Code)
	}
}

func TestBodyLimit_NoLimitWhenZero(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("unexpected read error: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("this body is larger than zero but should be allowed")))
	rr := httptest.NewRecorder()

	BodyLimit(0)(h).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}
