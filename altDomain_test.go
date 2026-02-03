package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
)

func Test_toApPersonForAltDomain(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://new.example.com"
	app.cfg.Server.AltDomains = []string{"https://old.example.com"}
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Title:       "Test Blog",
			Description: "A test blog",
		},
	}
	app.cfg.ActivityPub = &configActivityPub{
		Enabled: true,
	}
	app.apPubKeyBytes = []byte("test-key")
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	// Verify altHostnames was set
	require.Len(t, app.cfg.Server.altHostnames, 1)
	assert.Equal(t, "old.example.com", app.cfg.Server.altHostnames[0])

	// Test that the main domain actor has the old domain in alsoKnownAs
	mainPerson := app.toApPerson("testblog")
	assert.NotNil(t, mainPerson)
	foundAltDomain := false
	for _, aka := range mainPerson.AlsoKnownAs {
		if aka.GetLink().String() == "https://old.example.com/" {
			foundAltDomain = true
			break
		}
	}
	assert.True(t, foundAltDomain, "main domain actor should have alt domain in alsoKnownAs")

	// Test that the alt domain actor has movedTo pointing to main domain
	altPerson := app.toApPersonForAltDomain("testblog", "old.example.com")
	assert.NotNil(t, altPerson)
	// Note: the IRI doesn't have trailing slash because getRelativePath("") returns "" for non-default blogs
	assert.Contains(t, altPerson.MovedTo.GetLink().String(), "new.example.com")

	// Check alsoKnownAs on alt domain actor
	foundMainDomain := false
	for _, aka := range altPerson.AlsoKnownAs {
		if aka.GetLink().String() == "https://new.example.com" || aka.GetLink().String() == "https://new.example.com/" {
			foundMainDomain = true
			break
		}
	}
	assert.True(t, foundMainDomain, "alt domain actor should have main domain in alsoKnownAs")

	// Verify the actor IDs are correct
	assert.Contains(t, altPerson.ID.GetLink().String(), "old.example.com")
	assert.Contains(t, mainPerson.ID.GetLink().String(), "new.example.com")
}

func Test_altDomainRouting(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://new.example.com"
	app.cfg.Server.AltDomains = []string{"https://old.example.com"}
	app.cfg.Blogs = map[string]*configBlog{
		"default": {
			Title:       "Test Blog",
			Description: "A test blog",
		},
	}
	app.cfg.DefaultBlog = "default"
	app.cfg.ActivityPub = &configActivityPub{
		Enabled: true,
	}
	app.apPubKeyBytes = []byte("test-key")
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()
	require.NoError(t, app.initActivityPub())

	// Build the router
	router := app.buildRouter()

	// Test ActivityStreams request to alt domain - should return actor with movedTo
	req := httptest.NewRequest(http.MethodGet, "https://old.example.com/", nil)
	req.Host = "old.example.com"
	req.Header.Set("Accept", contenttype.AS)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"movedTo"`)
	assert.Contains(t, body, `new.example.com`)
	assert.Contains(t, body, `"id":"https://old.example.com/"`)

	// Test non-ActivityStreams request to alt domain - should redirect to main domain
	req2 := httptest.NewRequest(http.MethodGet, "https://old.example.com/some-path", nil)
	req2.Host = "old.example.com"
	req2.Header.Set("Accept", "text/html")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusMovedPermanently, rec2.Code)
	assert.Contains(t, rec2.Header().Get("Location"), "new.example.com")
}

func Test_isLocalURL(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://new.example.com"
	app.cfg.Server.ShortPublicAddress = "https://short.example.com"
	app.cfg.Server.AltDomains = []string{"https://old.example.com"}
	err := app.initConfig(false)
	require.NoError(t, err)

	// Test main public address
	assert.True(t, app.isLocalURL("https://new.example.com/some/path"))

	// Test short public address
	assert.True(t, app.isLocalURL("https://short.example.com/s/abc123"))

	// Test alt domain
	assert.True(t, app.isLocalURL("https://old.example.com/test"))

	// Test external domain
	assert.False(t, app.isLocalURL("https://external.example.com/test"))
}

func Test_normalizeLocalURL(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://new.example.com"
	app.cfg.Server.AltDomains = []string{"https://old.example.com"}
	err := app.initConfig(false)
	require.NoError(t, err)

	// Alt domain URL should be normalized to main domain
	normalized := app.normalizeLocalURL("https://old.example.com/some/path")
	assert.Equal(t, "https://new.example.com/some/path", normalized)

	// Main domain URL should stay the same
	normalized2 := app.normalizeLocalURL("https://new.example.com/other/path")
	assert.Equal(t, "https://new.example.com/other/path", normalized2)

	// External URL should stay the same
	normalized3 := app.normalizeLocalURL("https://external.example.com/path")
	assert.Equal(t, "https://external.example.com/path", normalized3)
}
