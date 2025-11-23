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
