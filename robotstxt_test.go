package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_robotsTXT(t *testing.T) {

	app := &goBlog{
		cfg: &config{
			Server: &configServer{
				PublicAddress: "https://example.com",
			},
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/robots.txt", nil)
	app.serveRobotsTXT(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "User-agent: *\nAllow: /\n\nSitemap: https://example.com/sitemap.xml\n", rec.Body.String())

	app.cfg.PrivateMode = &configPrivateMode{
		Enabled: true,
	}
	assert.True(t, app.isPrivate())

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/robots.txt", nil)
	app.serveRobotsTXT(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "User-agent: *\nDisallow: /\n", rec.Body.String())

}
