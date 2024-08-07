package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testConfigParam interface {
	*testing.T | *testing.B
	TempDir() string
}

func createDefaultTestConfig[V testConfigParam](t V) *config {
	c := createDefaultConfig()
	dir := t.TempDir()
	c.Db.File = filepath.Join(dir, "blog.db")
	c.User.ProfileImageFile = filepath.Join(dir, "profileImage")
	return c
}

func Test_configPort(t *testing.T) {

	t.Run("Default", func(t *testing.T) {
		c := createDefaultTestConfig(t)
		app := &goBlog{
			cfg: c,
		}
		_ = app.initConfig(false)
		assert.Equal(t, 8080, app.cfg.Server.Port)
	})

	t.Run("HTTPS", func(t *testing.T) {
		c := createDefaultTestConfig(t)
		c.Server.PublicAddress = "https://example.com"
		app := &goBlog{
			cfg: c,
		}
		_ = app.initConfig(false)
		assert.Equal(t, 443, app.cfg.Server.Port)
	})

	t.Run("HTTPS with custom port in address", func(t *testing.T) {
		c := createDefaultTestConfig(t)
		c.Server.PublicAddress = "https://example.com:1234/"
		app := &goBlog{
			cfg: c,
		}
		_ = app.initConfig(false)
		assert.Equal(t, 1234, app.cfg.Server.Port)
	})

	t.Run("HTTP with custom port in address", func(t *testing.T) {
		c := createDefaultTestConfig(t)
		c.Server.PublicAddress = "http://example.com:1234/"
		app := &goBlog{
			cfg: c,
		}
		_ = app.initConfig(false)
		assert.Equal(t, 1234, app.cfg.Server.Port)
	})

	t.Run("Custom port set", func(t *testing.T) {
		c := createDefaultTestConfig(t)
		c.Server.Port = 3456
		c.Server.PublicAddress = "https://example.com/"
		app := &goBlog{
			cfg: c,
		}
		_ = app.initConfig(false)
		assert.Equal(t, 3456, app.cfg.Server.Port)
	})

	t.Run("Custom port set with public https", func(t *testing.T) {
		c := createDefaultTestConfig(t)
		c.Server.Port = 3456
		c.Server.PublicHTTPS = true
		c.Server.PublicAddress = "https://example.com/"
		app := &goBlog{
			cfg: c,
		}
		_ = app.initConfig(false)
		assert.Equal(t, 3456, app.cfg.Server.Port)
	})

}

func Test_configHttps(t *testing.T) {

	t.Run("Default", func(t *testing.T) {
		c := createDefaultTestConfig(t)
		app := &goBlog{
			cfg: c,
		}
		_ = app.initConfig(false)
		assert.False(t, app.cfg.Server.PublicHTTPS)
		assert.False(t, app.cfg.Server.manualHttps)
		assert.False(t, app.cfg.Server.SecurityHeaders)
		assert.False(t, app.cfg.Server.HttpsRedirect)
		assert.False(t, app.useSecureCookies())
	})

	t.Run("Public HTTPS", func(t *testing.T) {
		c := createDefaultTestConfig(t)
		c.Server.PublicHTTPS = true
		app := &goBlog{
			cfg: c,
		}
		_ = app.initConfig(false)
		assert.True(t, app.cfg.Server.PublicHTTPS)
		assert.False(t, app.cfg.Server.manualHttps)
		assert.True(t, app.cfg.Server.SecurityHeaders)
		assert.True(t, app.cfg.Server.HttpsRedirect)
		assert.True(t, app.useSecureCookies())
	})

	t.Run("Manual HTTPS", func(t *testing.T) {
		c := createDefaultTestConfig(t)
		c.Server.HttpsCert = "/tmp/https.cert"
		c.Server.HttpsKey = "/tmp/https.key"
		app := &goBlog{
			cfg: c,
		}
		_ = app.initConfig(false)
		assert.False(t, app.cfg.Server.PublicHTTPS)
		assert.True(t, app.cfg.Server.manualHttps)
		assert.True(t, app.cfg.Server.SecurityHeaders)
		assert.False(t, app.cfg.Server.HttpsRedirect)
		assert.True(t, app.useSecureCookies())
	})

	t.Run("HTTPS only in address", func(t *testing.T) {
		c := createDefaultTestConfig(t)
		c.Server.PublicAddress = "https://example.com"
		app := &goBlog{
			cfg: c,
		}
		_ = app.initConfig(false)
		assert.False(t, app.cfg.Server.PublicHTTPS)
		assert.False(t, app.cfg.Server.manualHttps)
		assert.False(t, app.cfg.Server.SecurityHeaders)
		assert.False(t, app.cfg.Server.HttpsRedirect)
		assert.True(t, app.useSecureCookies())
	})

}

func Test_configDefaults(t *testing.T) {
	t.Run("Pagination", func(t *testing.T) {
		app := &goBlog{
			cfg: createDefaultTestConfig(t),
		}
		err := app.initConfig(false)
		require.NoError(t, err)
		if assert.Len(t, app.cfg.Blogs, 1) {
			for _, bc := range app.cfg.Blogs {
				assert.Equal(t, 10, bc.Pagination)
			}
		}
	})
}
