package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_checkLinks_detects_dead_links(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	app := &goBlog{cfg: createDefaultTestConfig(t)}
	must.NoError(app.initConfig(false))
	_ = app.initTemplateStrings()

	var mu sync.Mutex
	calls := map[string]int{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls[r.URL.Path]++
		mu.Unlock()
		if strings.HasPrefix(r.URL.Path, "/ok") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		// broken
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("err"))
	}))
	defer srv.Close()

	// Create posts with links to ok and broken
	p1 := &post{Path: "/p1", Content: "<a href=\"" + srv.URL + "/ok\">ok</a>", Parameters: map[string][]string{}}
	p2 := &post{Path: "/p2", Content: "<a href=\"" + srv.URL + "/broken\">bad</a>", Parameters: map[string][]string{}}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run check
	must.NoError(app.checkLinks(p1, p2))

	// Restore stdout and read output
	_ = w.Close()
	outBuf := new(bytes.Buffer)
	_, _ = io.Copy(outBuf, r)
	os.Stdout = oldStdout

	out := outBuf.String()
	// Should contain the broken link
	is.True(strings.Contains(out, "/broken") || strings.Contains(out, srv.URL+"/broken"))

	// Both endpoints were requested
	mu.Lock()
	defer mu.Unlock()
	is.True(calls["/ok"] >= 1 || calls["/ok/"] >= 1)
	is.True(calls["/broken"] >= 1)
}

// Test_checkLinks_highConcurrency verifies checkLinks handles many links and respects
// the concurrency limit (maxGoroutines == 10).
func Test_checkLinks_highConcurrency(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	app := &goBlog{cfg: createDefaultTestConfig(t)}
	must.NoError(app.initConfig(false))
	_ = app.initTemplateStrings()

	var current, max, total int32

	// slow handler to allow concurrency to build up
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&current, 1)
		// update max
		for {
			m := atomic.LoadInt32(&max)
			if cur > m {
				if atomic.CompareAndSwapInt32(&max, m, cur) {
					break
				}
				continue
			}
			break
		}
		// simulate some work
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&total, 1)
		atomic.AddInt32(&current, -1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Create one post with many links
	const N = 2000
	var b strings.Builder
	for i := range N {
		b.WriteString("<a href=\"")
		b.WriteString(srv.URL)
		b.WriteString("/p")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\">x</a>")
	}
	p := &post{Path: "/big", Content: b.String(), Parameters: map[string][]string{}}

	// Run checkLinks and ensure it completes
	must.NoError(app.checkLinks(p))

	// Validate counts
	is.Equal(int32(N), atomic.LoadInt32(&total), "should have requested all links")
	is.LessOrEqual(atomic.LoadInt32(&max), int32(10), "concurrency should not exceed semaphore capacity")
}

func Test_allLinksToCheck_filters_internal_links(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	app := &goBlog{cfg: createDefaultTestConfig(t)}
	must.NoError(app.initConfig(false))
	_ = app.initTemplateStrings()

	// External server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Create a post with one internal and one external link
	internal := app.cfg.Server.PublicAddress + "/internal"
	p := &post{Path: "/p", Content: "<a href=\"" + internal + "\">i</a> <a href=\"" + srv.URL + "/ext\">e</a>", Parameters: map[string][]string{}}

	links, err := app.allLinksToCheck(p)
	must.NoError(err)
	// Should only contain the external link
	foundExt := false
	for _, sp := range links {
		if strings.Contains(sp.Second, "/ext") {
			foundExt = true
		}
		if strings.Contains(sp.Second, "/internal") {
			t.Fatalf("internal link should have been filtered: %s", sp.Second)
		}
	}
	is.True(foundExt)
}

func Test_checkLinks_edge_cases(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	app := &goBlog{cfg: createDefaultTestConfig(t)}
	must.NoError(app.initConfig(false))
	_ = app.initTemplateStrings()

	t.Run("redirects_followed_and_count_as_success", func(t *testing.T) {
		// server that redirects /r -> /final (200)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/r" {
				http.Redirect(w, r, "/final", http.StatusFound)
				return
			}
			if r.URL.Path == "/final" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		p := &post{Path: "/pr", Content: "<a href=\"" + srv.URL + "/r\">r</a>", Parameters: map[string][]string{}}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		must.NoError(app.checkLinks(p))

		_ = w.Close()
		outBuf := new(bytes.Buffer)
		_, _ = io.Copy(outBuf, r)
		os.Stdout = oldStdout
		out := outBuf.String()
		// Should not report /r or /final as broken
		is.False(strings.Contains(out, "/r") || strings.Contains(out, "/final"))
	})

	t.Run("connection_refused_reports_error", func(t *testing.T) {
		// Use a likely-unbound port to cause connection refused
		bad := "http://127.0.0.1:1"
		p := &post{Path: "/px", Content: "<a href=\"" + bad + "\">bad</a>", Parameters: map[string][]string{}}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// This should return an error (printed)
		_ = app.checkLinks(p)

		_ = w.Close()
		outBuf := new(bytes.Buffer)
		_, _ = io.Copy(outBuf, r)
		os.Stdout = oldStdout
		out := outBuf.String()
		is.True(strings.Contains(out, "127.0.0.1") || strings.Contains(out, "connection refused") || strings.Contains(out, "connect: connection refused"))
	})
}
