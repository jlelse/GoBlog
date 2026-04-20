package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_reactionsLowLevel(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"default": createDefaultBlog(),
	}
	app.cfg.Blogs["default"].reactionsEnabled = true

	_ = app.initConfig(false)

	p := &post{
		Path:    "/testpost",
		Content: "test",
		Status:  statusPublished,
		Blog:    "default",
	}

	err := app.saveReaction("🖕", p)
	assert.ErrorContains(t, err, "not allowed")

	err = app.saveReaction("❤️", p)
	assert.ErrorContains(t, err, "constraint failed")

	// Create a post
	err = app.createPost(p)
	require.NoError(t, err)

	// Create 4 reactions
	for range 4 {
		err = app.saveReaction("❤️", p)
		assert.NoError(t, err)
	}

	// Check if reaction count is 4
	reacts, err := app.getReactionsFromDatabase(p)
	require.NoError(t, err)
	assert.Equal(t, "{\"❤️\":4}", reacts)

	// Change post path
	pNew := &post{
		Path:    "/newpost",
		Content: "test",
		Status:  statusPublished,
		Blog:    "default",
	}
	err = app.replacePost(pNew, "/testpost", statusPublished, visibilityPublic, false)
	require.NoError(t, err)

	// Check if reaction count is 4
	reacts, err = app.getReactionsFromDatabase(pNew)
	require.NoError(t, err)
	assert.Equal(t, "{\"❤️\":4}", reacts)

	// Delete post
	err = app.deletePost("/newpost")
	require.NoError(t, err)
	err = app.deletePost("/newpost")
	require.NoError(t, err)

	// Check if reaction count is 0
	reacts, err = app.getReactionsFromDatabase(pNew)
	require.NoError(t, err)
	assert.Equal(t, "{}", reacts)

	// Create a post with disabled reactions
	p2 := &post{
		Path:    "/testpost2",
		Content: "test",
		Status:  statusPublished,
		Blog:    "default",
		Parameters: map[string][]string{
			"reactions": {"false"},
		},
	}
	err = app.createPost(p2)
	require.NoError(t, err)

	// Create reaction
	err = app.saveReaction("❤️", p2)
	require.NoError(t, err)

	// Check if reaction count is 0
	reacts, err = app.getReactionsFromDatabase(p2)
	require.NoError(t, err)
	assert.Equal(t, "{}", reacts)

}

func Test_reactionsConfigurable(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"default": createDefaultBlog(),
	}

	_ = app.initConfig(false)

	// Test disabling reactions via setting
	err := app.saveBooleanSettingValue(settingNameWithBlog("default", reactionsEnabledSetting), false)
	require.NoError(t, err)
	app.cfg.Blogs["default"].reactionsEnabled = false // Sync config
	assert.False(t, app.reactionsEnabled("default"))

	// Re-enable
	err = app.saveBooleanSettingValue(settingNameWithBlog("default", reactionsEnabledSetting), true)
	require.NoError(t, err)
	app.cfg.Blogs["default"].reactionsEnabled = true // Sync config
	assert.True(t, app.reactionsEnabled("default"))

	// Create a post
	p := &post{
		Path:    "/testpost",
		Content: "test",
		Status:  statusPublished,
		Blog:    "default",
	}
	err = app.createPost(p)
	require.NoError(t, err)

	// Default reactions: ❤️, 👍, 🎉, 😂, 😱
	// Test adding a custom reaction via setting with spaces and empty entries
	err = app.saveSettingValue(settingNameWithBlog("default", reactionsSetting), " 🔥 , 🚀 ,, ")
	require.NoError(t, err)
	app.cfg.Blogs["default"].allowedReactions = []string{"🔥", "🚀"} // Sync config

	// Old default reaction should now fail
	err = app.saveReaction("❤️", p)
	assert.ErrorContains(t, err, "not allowed")

	// New custom reactions should work
	err = app.saveReaction("🔥", p)
	assert.NoError(t, err)
	err = app.saveReaction("🚀", p)
	assert.NoError(t, err)

	// Check if reaction counts are correct
	reacts, err := app.getReactionsFromDatabase(p)
	require.NoError(t, err)
	assert.Contains(t, reacts, "\"🔥\":1")
	assert.Contains(t, reacts, "\"🚀\":1")

	// Test reordering/removing
	err = app.saveSettingValue(settingNameWithBlog("default", reactionsSetting), "🚀")
	require.NoError(t, err)
	app.cfg.Blogs["default"].allowedReactions = []string{"🚀"} // Sync config

	// 🔥 should now be "not allowed" even if it exists in DB (it will be filtered out by getReactionsFromDatabase)
	err = app.saveReaction("🔥", p)
	assert.ErrorContains(t, err, "not allowed")

	// Only 🚀 should be returned
	reacts, err = app.getReactionsFromDatabase(p)
	require.NoError(t, err)
	assert.Equal(t, "{\"🚀\":1}", reacts)
}

func Test_reactionsHighLevel(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"default": createDefaultBlog(),
	}
	app.cfg.Blogs["default"].reactionsEnabled = true

	_ = app.initConfig(false)

	// Send unsuccessful reaction
	form := url.Values{
		"reaction": {"❤️"},
		"path":     {"/testpost"},
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.postReaction(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Create a post
	p := &post{
		Path:    "/testpost",
		Content: "test",
		Blog:    "default",
	}
	err := app.createPost(p)
	require.NoError(t, err)

	// Send successful reaction
	form = url.Values{
		"reaction": {"❤️"},
		"path":     {"/testpost"},
	}
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.postReaction(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Check if reaction count is 1
	req = httptest.NewRequest(http.MethodGet, "/?path=/testpost", nil)
	rec = httptest.NewRecorder()
	app.getReactions(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "{\"❤️\":1}", rec.Body.String())

	// Get reactions for a non-existing post
	req = httptest.NewRequest(http.MethodGet, "/?path=/non-existing-post", nil)
	rec = httptest.NewRecorder()
	app.getReactions(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code) // Should fail because post not found

}
