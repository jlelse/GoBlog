package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_sendWebmentions_OutgoingBlocklist(t *testing.T) {
	mockClient := newFakeHttpClient()
	var requestCount int
	mockClient.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<link rel="webmention" href="https://blocked.example.com/webmention">`))
	}))

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: mockClient.Client,
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	// Insert outgoing blocklist entry
	_ = app.addWebmentionBlocklistEntry("blocked.example.com", false, true)

	// Save post linking to blocked host
	err := app.db.savePost(&post{
		Path:       "/test/outgoingblock",
		Content:    `<a href="https://blocked.example.com/post">Blocked</a>`,
		Status:     statusPublished,
		Visibility: visibilityPublic,
	}, &postCreationOptions{isNew: true})
	assert.NoError(t, err)

	// Get the post
	posts, err := app.getPosts(&postsRequestConfig{
		path: "/test/outgoingblock",
	})
	assert.NoError(t, err)
	assert.Len(t, posts, 1)

	// Send webmentions should skip the blocked target
	err = app.sendWebmentions(posts[0])
	assert.NoError(t, err)

	// Verify no HTTP requests were made
	assert.Equal(t, 0, requestCount, "No HTTP requests should be made when sending is disabled")
}
func Test_sendWebmentions_SendingDisabled(t *testing.T) {
	mockClient := newFakeHttpClient()
	var requestCount int
	mockClient.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<link rel="webmention" href="https://example.net/webmention">`))
	}))

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: mockClient.Client,
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	// Disable sending
	app.cfg.Webmention.DisableSending = true

	// Save post linking to external site
	err := app.db.savePost(&post{
		Path:       "/test/sendingdisabled",
		Content:    `<a href="https://example.net/post">External</a>`,
		Status:     statusPublished,
		Visibility: visibilityPublic,
	}, &postCreationOptions{isNew: true})
	assert.NoError(t, err)

	// Get the post
	posts, err := app.getPosts(&postsRequestConfig{
		path: "/test/sendingdisabled",
	})
	assert.NoError(t, err)
	assert.Len(t, posts, 1)

	// Send webmentions should return early
	err = app.sendWebmentions(posts[0])
	assert.NoError(t, err)

	// Verify no HTTP requests were made
	assert.Equal(t, 0, requestCount, "No HTTP requests should be made when sending is disabled")
}

func Test_sendWebmentions_ExternalMentionSent(t *testing.T) {
	mockClient := newFakeHttpClient()
	var requestCount int

	mockClient.setHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch r.Method {
		case http.MethodHead, http.MethodGet:
			// Endpoint discovery request
			w.Header().Add("Link", `<https://example.net/webmention>; rel="webmention"`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Test page</body></html>"))
		case http.MethodPost:
			// Webmention sending request
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Webmention accepted"))
		}
	}))

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: mockClient.Client,
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	// Save post with HTML link to external site
	err := app.db.savePost(&post{
		Path:       "/test/external",
		Content:    `<a href="https://example.net/post">Check out this post</a>`,
		Status:     statusPublished,
		Visibility: visibilityPublic,
	}, &postCreationOptions{isNew: true})
	require.NoError(t, err)

	// Get the post
	posts, err := app.getPosts(&postsRequestConfig{
		path: "/test/external",
	})
	require.NoError(t, err)
	require.Len(t, posts, 1)

	// Send webmentions
	err = app.sendWebmentions(posts[0])
	assert.NoError(t, err)

	// Verify HTTP requests were made (endpoint discovery and/or webmention sending)
	assert.Equal(t, 2, requestCount, "HTTP requests should be made to discover endpoint and send webmention")
}

func Test_sendWebmentions_InterGoblogEnabled(t *testing.T) {
	mockClient := newFakeHttpClient()
	mockClient.setFakeResponse(http.StatusOK, "<html><body>Test</body></html>")

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: mockClient.Client,
		d: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simple mock router that returns OK for test paths
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body><a href=\"https://example.com/test/source\">Link back</a></body></html>"))
		}),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	// Ensure inter-GoBlog mentions are enabled
	app.cfg.Webmention.DisableInterGoblogMentions = false
	// Ensure webmention receiving is enabled (this allows queueMention to succeed)
	app.cfg.Webmention.DisableReceiving = false

	// Save target post
	err := app.db.savePost(&post{
		Path:       "/test/target",
		Content:    "Target post",
		Status:     statusPublished,
		Visibility: visibilityPublic,
	}, &postCreationOptions{isNew: true})
	assert.NoError(t, err)

	// Save source post linking to target
	err = app.db.savePost(&post{
		Path:       "/test/source",
		Content:    `<a href="https://example.com/test/target">Link</a>`,
		Status:     statusPublished,
		Visibility: visibilityPublic,
	}, &postCreationOptions{isNew: true})
	assert.NoError(t, err)

	// Get the source post
	posts, err := app.getPosts(&postsRequestConfig{
		path: "/test/source",
	})
	assert.NoError(t, err)
	assert.Len(t, posts, 1)

	targetURL := "https://example.com/test/target"
	sourceURL := app.fullPostURL(posts[0])

	// Call sendWebmentions which should queue the internal mention
	err = app.sendWebmentions(posts[0])
	assert.NoError(t, err, "sendWebmentions should succeed")

	// Manually verify the mention (simulating what the queue processor does)
	// This is necessary because the queue processor runs asynchronously
	m := &mention{
		Source: sourceURL,
		Target: targetURL,
	}
	err = app.verifyMention(m)
	assert.NoError(t, err, "Webmention verification should succeed")

	// Now check that the mention was stored in the database
	mentions, err := app.getWebmentions(&webmentionsRequestConfig{
		target: targetURL,
	})
	assert.NoError(t, err)
	assert.Greater(t, len(mentions), 0, "Internal mention should be created after verification")
	assert.Equal(t, sourceURL, mentions[0].Source, "Mention source should match")
}

func Test_sendWebmentions_InterGoblogDisabled(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
		d: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simple mock router that returns OK for test paths
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body><a href=\"https://example.com/test/source\">Link back</a></body></html>"))
		}),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	// Disable inter-GoBlog mentions
	app.cfg.Webmention.DisableInterGoblogMentions = true
	// Ensure webmention receiving is enabled (to test the receiving check doesn't interfere)
	app.cfg.Webmention.DisableReceiving = false

	// Save target post
	err := app.db.savePost(&post{
		Path:       "/test/target",
		Content:    "Target post",
		Status:     statusPublished,
		Visibility: visibilityPublic,
	}, &postCreationOptions{isNew: true})
	assert.NoError(t, err)

	// Save source post linking to target
	err = app.db.savePost(&post{
		Path:       "/test/source",
		Content:    `<a href="https://example.com/test/target">Link</a>`,
		Status:     statusPublished,
		Visibility: visibilityPublic,
	}, &postCreationOptions{isNew: true})
	assert.NoError(t, err)

	// Get the source post
	posts, err := app.getPosts(&postsRequestConfig{
		path: "/test/source",
	})
	assert.NoError(t, err)
	assert.Len(t, posts, 1)

	// Call sendWebmentions with disabled inter-GoBlog mentions
	err = app.sendWebmentions(posts[0])
	assert.NoError(t, err, "sendWebmentions should succeed")

	// Verify NO webmention was queued or created because inter-GoBlog mentions are disabled
	targetURL := "https://example.com/test/target"
	mentions, err := app.getWebmentions(&webmentionsRequestConfig{
		target: targetURL,
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(mentions), "No internal mention should be persisted when disabled")
}
