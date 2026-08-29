package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klauspost/compress/gzhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_checkSSRFHost(t *testing.T) {
	tests := []struct {
		host    string
		blocked bool
	}{
		// Private, loopback and link-local IPv4
		{"127.0.0.1", true},
		{"10.0.0.5", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},
		{"100.64.0.1", true},
		// IPv6
		{"::1", true},
		{"fc00::1", true},
		{"::ffff:127.0.0.1", true},
		// Hostname resolving to loopback
		{"localhost", true},
		// Public IPs
		{"8.8.8.8", false},
		{"93.184.216.34", false},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			err := checkSSRFHost(tt.host)
			if tt.blocked {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_webmentionHTTPClientBlocksPrivateIP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	client := newWebmentionHTTPClient()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.Error(t, err)
}

func Test_verifyMentionBlocksPrivateSource(t *testing.T) {
	app := &goBlog{
		httpClient:   newFakeHttpClient().Client,
		wmHTTPClient: newWebmentionHTTPClient(),
		cfg:          createDefaultTestConfig(t),
		d: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// No-op
		}),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	_ = app.initConfig(false)

	m := &mention{
		Source: "http://127.0.0.1:8080/page",
		Target: "https://example.org/any-post",
	}
	err := app.verifyMention(m)
	require.Error(t, err)
}

func Test_isBlockedNetwork(t *testing.T) {
	assert.True(t, isBlockedNetwork(netip.MustParseAddr("127.0.0.1")))
	assert.True(t, isBlockedNetwork(netip.MustParseAddr("::ffff:10.0.0.1")))
	assert.False(t, isBlockedNetwork(netip.MustParseAddr("8.8.8.8")))
	assert.False(t, isBlockedNetwork(netip.MustParseAddr("2606:4700::1111")))
}

func Test_webmentionSSRFProtection(t *testing.T) {
	// Internal "victim" service on the loopback interface that would receive the SSRF request
	var victimHits atomic.Int32
	victim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		victimHits.Add(1)
		_, _ = w.Write([]byte(`<html><body><a href="http://goblog.example/test-post">Link</a></body></html>`))
	}))
	defer victim.Close()

	// Legitimate external source server that actually serves a page linking to the target
	var externalHits atomic.Int32
	externalSource := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		externalHits.Add(1)
		_, _ = w.Write([]byte(`<html><body><a href="http://goblog.example/test-post">Link</a></body></html>`))
	}))
	defer externalSource.Close()

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "http://goblog.example"

	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateStrings())

	// Disable automatic inter-GoBlog mentions so the post hooks don't enqueue extra items
	app.cfg.Webmention.DisableInterGoblogMentions = true
	// Speed up the async webmention queue for testing
	webmentionQueueInterval = 100 * time.Millisecond
	// Set up the real webmention pipeline: SSRF-guarded client + async queue worker
	app.initWebmention()

	// Use a guarded client with the real checkSSRFHost decision, but route allowed
	// connections to the local externalSource server so the test stays hermetic.
	base := newHTTPTransportBase()
	base.DialContext = newSSRFGuardDialContext(func(ctx context.Context, network, _ string) (net.Conn, error) {
		return newNetDialer().DialContext(ctx, network, externalSource.Listener.Addr().String())
	})
	app.wmHTTPClient = &http.Client{
		Timeout:   time.Minute,
		Transport: newAddUserAgentTransport(gzhttp.Transport(base)),
	}

	app.d = app.buildRouter()
	server := httptest.NewServer(app.d)
	defer server.Close()
	t.Cleanup(func() {
		webmentionQueueInterval = 30 * time.Second
		app.shutdown.ShutdownAndWait()
	})

	// Create a target post
	require.NoError(t, app.createPost(&post{Path: "/test-post"}))

	postWebmention := func(source, target string) {
		form := url.Values{}
		form.Set("source", source)
		form.Set("target", target)
		resp, err := http.PostForm(server.URL+webmentionPath, form)
		require.NoError(t, err)
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
		_ = resp.Body.Close()
	}
	waitForQueue := func() {
		require.Eventually(t, func() bool {
			qi, err := app.peekQueue(context.Background(), "wm")
			return err == nil && qi == nil
		}, 10*time.Second, 50*time.Millisecond, "webmention queue should be drained")
	}

	// SSRF attempt: source points to an internal loopback service
	postWebmention(victim.URL+"/page", "http://goblog.example/test-post")
	waitForQueue()
	require.Zero(t, victimHits.Load(), "internal service must not be reached via the webmention source")
	mentions, err := app.getWebmentions(&webmentionsRequestConfig{target: "http://goblog.example/test-post"})
	require.NoError(t, err)
	require.Empty(t, mentions, "webmention from a blocked source must not be stored")

	// Legitimate external source: the page is actually fetched and the webmention verified
	postWebmention("http://8.8.8.8/source-page", "http://goblog.example/test-post")
	waitForQueue()
	require.Equal(t, int32(1), externalHits.Load(), "legitimate external source should be fetched")
	require.Zero(t, victimHits.Load(), "internal service must still not be reached")
	mentions, err = app.getWebmentions(&webmentionsRequestConfig{target: "http://goblog.example/test-post"})
	require.NoError(t, err)
	require.Len(t, mentions, 1, "webmention from a legitimate external source should be stored")
	require.Equal(t, "http://8.8.8.8/source-page", mentions[0].Source)
	require.Equal(t, webmentionStatusVerified, mentions[0].Status)
}
