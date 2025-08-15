package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_openSearchDescription(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	require.NoError(t, app.initConfig(false))

	// Enable search for the default blog
	bc := app.cfg.Blogs[app.cfg.DefaultBlog]
	bc.Search = &configSearch{Enabled: true, Path: "/search", Title: "Site Search"}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search/opensearch.xml", nil)
	app.serveOpenSearch(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Basic sanity checks
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/opensearchdescription+xml")
	// ShortName/Description are derived from the blog title by the implementation
	assert.Contains(t, body, "My Blog")
	assert.Contains(t, body, "?q={searchTerms}")
	// Self link to opensearch.xml should be present
	assert.Contains(t, body, "opensearch.xml")
	// SearchForm should point to the blog search path
	assert.Contains(t, body, "http://localhost:8080/search")
}

func Test_openSearchUrl(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	require.NoError(t, app.initConfig(false))

	// Nil search -> empty
	bc := app.cfg.Blogs[app.cfg.DefaultBlog]
	bc.Search = nil
	assert.Equal(t, "", openSearchUrl(bc))

	// Disabled search -> empty
	bc.Search = &configSearch{Enabled: false, Path: "/search"}
	assert.Equal(t, "", openSearchUrl(bc))

	// Enabled search -> path should include opensearch.xml
	bc.Search = &configSearch{Enabled: true, Path: "/search"}
	u := openSearchUrl(bc)
	require.NotEmpty(t, u)
	assert.Contains(t, u, "opensearch.xml")
}
