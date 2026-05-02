package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
)

func Test_webmentionBlocklistIncoming(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)

	// Insert blocklist entry
	_ = app.addWebmentionBlocklistEntry("blocked.example.com", true, false)

	// Create request with blocked source
	r := httptest.NewRequest(http.MethodPost, "/webmention", strings.NewReader(url.Values{
		"source": {"https://blocked.example.com/post"},
		"target": {"https://example.com/test"},
	}.Encode()))
	r.Header.Set("Content-Type", contenttype.WWWForm)
	w := httptest.NewRecorder()

	app.handleWebmention(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Create request with non-blocked source
	r2 := httptest.NewRequest(http.MethodPost, "/webmention", strings.NewReader(url.Values{
		"source": {"https://allowed.example.com/post"},
		"target": {"https://example.com/test"},
	}.Encode()))
	r2.Header.Set("Content-Type", contenttype.WWWForm)
	w2 := httptest.NewRecorder()

	app.handleWebmention(w2, r2)
	assert.Equal(t, http.StatusAccepted, w2.Code)
}

func Test_webmentionReceivingDisabled(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)

	// Enable disable receiving
	app.cfg.Webmention.DisableReceiving = true

	// Create request
	r := httptest.NewRequest(http.MethodPost, "/webmention", strings.NewReader(url.Values{
		"source": {"https://example.net/post"},
		"target": {"https://example.com/test"},
	}.Encode()))
	r.Header.Set("Content-Type", contenttype.WWWForm)
	w := httptest.NewRecorder()

	app.handleWebmention(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func Test_webmentions(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)

	_ = app.db.insertWebmention(&mention{
		Source:  "https://example.net/test",
		Target:  "https://example.com/täst",
		Created: time.Now().Unix(),
		Title:   "Test-Title",
		Content: "Test-Content",
		Author:  "Test-Author",
	}, webmentionStatusVerified)

	mentions, err := app.getWebmentions(&webmentionsRequestConfig{
		sourcelike: "example.xyz",
	})
	require.NoError(t, err)
	assert.Len(t, mentions, 0)

	count, err := app.db.countWebmentions(&webmentionsRequestConfig{
		sourcelike: "example.net",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	exists := app.db.webmentionExists(&mention{Source: "Https://Example.net/test", Target: "Https://Example.com/TÄst"})
	assert.True(t, exists)

	mentions = app.getWebmentionsByAddress("https://example.com/täst")
	assert.Len(t, mentions, 0)

	mentions = app.getWebmentionsByAddress("")
	assert.Len(t, mentions, 0)

	mentions, err = app.getWebmentions(&webmentionsRequestConfig{
		sourcelike: "example.net",
	})
	require.NoError(t, err)
	if assert.Len(t, mentions, 1) {
		_ = app.db.approveWebmentionID(mentions[0].ID)
	}

	mentions = app.getWebmentionsByAddress("https://example.com/täst")
	assert.Len(t, mentions, 1)

	mentions = app.getWebmentionsByAddress("https://example.com/t%C3%A4st")
	assert.Len(t, mentions, 1)

	err = app.db.deleteWebmention(&mention{
		Source: "https://example.net/test",
		Target: "https://example.com/T%C3%84ST",
	})
	assert.NoError(t, err)

	mentions = app.getWebmentionsByAddress("https://example.com/täst")
	assert.Len(t, mentions, 0)

}
