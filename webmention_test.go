package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_webmentions(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
		},
	}

	_ = app.initConfig(false)

	_ = app.db.insertWebmention(&mention{
		Source:  "https://example.net/test",
		Target:  "https://example.com/täst",
		Created: time.Now().Unix(),
		Title:   "Test-Title",
		Content: "Test-Content",
		Author:  "Test-Author",
	}, webmentionStatusVerified)

	mentions, err := app.db.getWebmentions(&webmentionsRequestConfig{
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

	mentions = app.db.getWebmentionsByAddress("https://example.com/täst")
	assert.Len(t, mentions, 0)

	mentions = app.db.getWebmentionsByAddress("")
	assert.Len(t, mentions, 0)

	mentions, err = app.db.getWebmentions(&webmentionsRequestConfig{
		sourcelike: "example.net",
	})
	require.NoError(t, err)
	if assert.Len(t, mentions, 1) {
		_ = app.db.approveWebmentionId(mentions[0].ID)
	}

	mentions = app.db.getWebmentionsByAddress("https://example.com/täst")
	assert.Len(t, mentions, 1)

	mentions = app.db.getWebmentionsByAddress("https://example.com/t%C3%A4st")
	assert.Len(t, mentions, 1)

	err = app.db.deleteWebmention(&mention{
		Source: "https://example.net/test",
		Target: "https://example.com/T%C3%84ST",
	})
	assert.NoError(t, err)

	mentions = app.db.getWebmentionsByAddress("https://example.com/täst")
	assert.Len(t, mentions, 0)

}
