package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_booleanSettingHandlers(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	blog := app.cfg.DefaultBlog
	bc := app.cfg.Blogs[blog]

	// Test hideOldContentWarning handler
	handler := app.settingsHideOldContentWarning()
	require.NotNil(t, handler)

	// Initially false
	require.False(t, bc.hideOldContentWarning)

	// Simulate request
	req := httptest.NewRequest("POST", "/settings/oldcontentwarning", strings.NewReader("hideoldcontentwarning=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler(rr, req)

	// Should redirect
	require.Equal(t, 302, rr.Code)
	require.Equal(t, "/settings", rr.Header().Get("Location"))

	// Should have updated the setting
	require.True(t, bc.hideOldContentWarning)
}

func Test_settingsUpdateBlog(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	blog := app.cfg.DefaultBlog
	bc := app.cfg.Blogs[blog]

	// Set initial values
	bc.Title = "Initial Title"
	bc.Description = "Initial Description"
	_ = app.setBlogTitle(blog, bc.Title)
	_ = app.setBlogDescription(blog, bc.Description)

	// Simulate request to update blog settings
	req := httptest.NewRequest("POST", "/settings/blog", strings.NewReader("blogtitle=New+Title&blogdescription=New+Description"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.settingsUpdateBlog(rr, req)

	// Should redirect
	require.Equal(t, 302, rr.Code)
	require.Equal(t, "/settings", rr.Header().Get("Location"))

	// Should have updated the settings
	require.Equal(t, "New Title", bc.Title)
	require.Equal(t, "New Description", bc.Description)

	// Verify database values
	dbTitle, err := app.getBlogTitle(blog)
	require.NoError(t, err)
	require.Equal(t, "New Title", dbTitle)

	dbDescription, err := app.getBlogDescription(blog)
	require.NoError(t, err)
	require.Equal(t, "New Description", dbDescription)
}

func Test_settingsUpdateBlog_EmptyTitle(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	// Simulate request with empty title
	req := httptest.NewRequest("POST", "/settings/blog", strings.NewReader("blogtitle=&blogdescription=Some+Description"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.settingsUpdateBlog(rr, req)

	// Should return error
	require.Equal(t, 400, rr.Code)
}

func Test_settingsUpdateBlog_EmptyDescription(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	blog := app.cfg.DefaultBlog
	bc := app.cfg.Blogs[blog]

	// Set initial values
	bc.Title = "Initial Title"
	bc.Description = "Initial Description"
	_ = app.setBlogTitle(blog, bc.Title)
	_ = app.setBlogDescription(blog, bc.Description)

	// Simulate request with empty description (should be allowed)
	req := httptest.NewRequest("POST", "/settings/blog", strings.NewReader("blogtitle=New+Title&blogdescription="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.settingsUpdateBlog(rr, req)

	// Should redirect (empty description is allowed)
	require.Equal(t, 302, rr.Code)

	// Should have updated the settings
	require.Equal(t, "New Title", bc.Title)
	require.Equal(t, "", bc.Description)
}
