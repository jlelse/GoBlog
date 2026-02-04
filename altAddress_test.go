package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	app.cfg.Server.AltAddresses = []string{"https://old.example.com"}
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

	// Test that the main domain actor has the old domain in alsoKnownAs
	mainPerson := app.toApPerson("testblog", "")
	assert.NotNil(t, mainPerson)
	foundAltDomain := false
	for _, aka := range mainPerson.AlsoKnownAs {
		if aka.GetLink().String() == "https://old.example.com" {
			foundAltDomain = true
			break
		}
	}
	assert.True(t, foundAltDomain, "main domain actor should have alt domain in alsoKnownAs")

	// Test that the alt domain actor has movedTo pointing to main domain
	altPerson := app.toApPerson("testblog", "https://old.example.com")
	assert.NotNil(t, altPerson)
	assert.Contains(t, altPerson.MovedTo.GetLink().String(), "new.example.com")

	// Check alsoKnownAs on alt domain actor
	foundMainDomain := false
	for _, aka := range altPerson.AlsoKnownAs {
		if aka.GetLink().String() == "https://new.example.com" {
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
	app.cfg.Server.AltAddresses = []string{"https://old.example.com"}
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
	assert.Contains(t, body, `"id":"https://old.example.com"`)

	// Test non-ActivityStreams request to alt domain - should redirect to main domain
	req = httptest.NewRequest(http.MethodGet, "https://old.example.com/some-path?some-query=1", nil)
	req.Host = "old.example.com"
	req.Header.Set("Accept", "text/html")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusPermanentRedirect, rec.Code)
	assert.Equal(t, "https://new.example.com/some-path?some-query=1", rec.Header().Get("Location"))
}

func Test_altDomainRoutingWithoutActivityPub(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://new.example.com"
	app.cfg.Server.AltAddresses = []string{"https://old.example.com"}

	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	// Build the router
	router := app.buildRouter()

	// Test request to alt domain - should redirect to main domain
	req := httptest.NewRequest(http.MethodGet, "https://old.example.com/some-path?some-query=1", nil)
	req.Host = "old.example.com"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusPermanentRedirect, rec.Code)
	assert.Equal(t, "https://new.example.com/some-path?some-query=1", rec.Header().Get("Location"))

	// Test request with ActivityStreams Accept header - should also redirect
	req = httptest.NewRequest(http.MethodGet, "https://old.example.com/some-path?some-query=1", nil)
	req.Host = "old.example.com"
	req.Header.Set("Accept", contenttype.AS)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusPermanentRedirect, rec.Code)
	assert.Equal(t, "https://new.example.com/some-path?some-query=1", rec.Header().Get("Location"))
}

func Test_isLocalURL(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://new.example.com"
	app.cfg.Server.ShortPublicAddress = "https://short.example.com"
	app.cfg.Server.AltAddresses = []string{"https://old.example.com"}
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

func Test_loginWithAltAddress(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.AltAddresses = []string{"https://old.example.com"}
	_ = app.initConfig(false)
	app.initCache()
	app.initMarkdown()
	_ = app.initTemplateStrings()
	app.initSessionStores()

	router := app.buildRouter()

	t.Run("login page accessible on alt address", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		req.Host = "old.example.com"

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Should not redirect, should serve login page
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Login")
	})

	t.Run("logout accessible on alt address", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logout", nil)
		req.Host = "old.example.com"

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Should not redirect - logout redirects but doesn't permanently redirect to main domain
		assert.NotEqual(t, http.StatusPermanentRedirect, rec.Code)
	})

	t.Run("regular page redirects to main domain", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/some-page", nil)
		req.Host = "old.example.com"

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Should redirect to main domain
		assert.Equal(t, http.StatusPermanentRedirect, rec.Code)
		assert.Contains(t, rec.Header().Get("Location"), "localhost:8080")
	})
}

func Test_indieAuthWithAltAddress(t *testing.T) {
	// This test verifies that IndieAuth works when accessed through an alternative address
	// and that the "me" parameter correctly reflects the alt address that was used
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://new.example.com"
	app.cfg.Server.AltAddresses = []string{"https://old.example.com"}
	app.cfg.Blogs = map[string]*configBlog{
		"default": {
			Title:       "Test Blog",
			Description: "A test blog",
		},
	}
	app.cfg.DefaultBlog = "default"

	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()
	app.initIndieAuth()

	router := app.buildRouter()

	// Test: Authorization via alt address returns correct "me" parameter
	t.Run("authorization via alt address returns correct me", func(t *testing.T) {
		form := url.Values{
			"response_type":         {"code"},
			"client_id":             {"https://client.example.com/"},
			"redirect_uri":          {"https://client.example.com/callback"},
			"state":                 {"test-state"},
			"code_challenge":        {strings.Repeat("a", 43)},
			"code_challenge_method": {"plain"},
		}

		req := httptest.NewRequest(http.MethodPost, "https://old.example.com/indieauth/accept", strings.NewReader(form.Encode()))
		req.Host = "old.example.com"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		setLoggedIn(req, true)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusFound, rec.Code)
		location, err := url.Parse(rec.Header().Get("Location"))
		require.NoError(t, err)
		assert.Equal(t, "https://old.example.com/", location.Query().Get("me"))
	})

	// Test: Authorization via main address returns correct "me" parameter
	t.Run("authorization via main address returns correct me", func(t *testing.T) {
		form := url.Values{
			"response_type":         {"code"},
			"client_id":             {"https://client.example.com/"},
			"redirect_uri":          {"https://client.example.com/callback"},
			"state":                 {"test-state-2"},
			"code_challenge":        {strings.Repeat("b", 43)},
			"code_challenge_method": {"plain"},
		}

		req := httptest.NewRequest(http.MethodPost, "https://new.example.com/indieauth/accept", strings.NewReader(form.Encode()))
		req.Host = "new.example.com"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		setLoggedIn(req, true)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusFound, rec.Code)
		location, err := url.Parse(rec.Header().Get("Location"))
		require.NoError(t, err)
		assert.Equal(t, "https://new.example.com/", location.Query().Get("me"))
	})

	// Test: Token verification returns correct me for alt address
	t.Run("token verification returns correct me for alt address", func(t *testing.T) {
		// Create authorization via alt address
		form := url.Values{
			"response_type":         {"code"},
			"client_id":             {"https://client.example.com/"},
			"redirect_uri":          {"https://client.example.com/callback"},
			"state":                 {"test-state-3"},
			"code_challenge":        {strings.Repeat("c", 43)},
			"code_challenge_method": {"plain"},
		}

		req := httptest.NewRequest(http.MethodPost, "https://old.example.com/indieauth/accept", strings.NewReader(form.Encode()))
		req.Host = "old.example.com"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		setLoggedIn(req, true)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Extract code
		location, err := url.Parse(rec.Header().Get("Location"))
		require.NoError(t, err)
		code := location.Query().Get("code")
		require.NotEmpty(t, code)

		// Exchange code for token
		tokenForm := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"client_id":     {"https://client.example.com/"},
			"redirect_uri":  {"https://client.example.com/callback"},
			"code_verifier": {strings.Repeat("c", 43)},
		}

		tokenReq := httptest.NewRequest(http.MethodPost, "/indieauth/token", strings.NewReader(tokenForm.Encode()))
		tokenReq.Host = "old.example.com"
		tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		tokenRec := httptest.NewRecorder()
		router.ServeHTTP(tokenRec, tokenReq)

		assert.Equal(t, http.StatusOK, tokenRec.Code)
		body := tokenRec.Body.String()
		assert.Contains(t, body, `"me":"https://old.example.com/"`)
		assert.Contains(t, body, `"access_token"`)

		// Parse token
		var tokenResp map[string]interface{}
		err = json.Unmarshal(tokenRec.Body.Bytes(), &tokenResp)
		require.NoError(t, err)
		token := tokenResp["access_token"].(string)

		// Verify token
		verifyReq := httptest.NewRequest(http.MethodGet, "/indieauth/token", nil)
		verifyReq.Host = "old.example.com"
		verifyReq.Header.Set("Authorization", "Bearer "+token)

		verifyRec := httptest.NewRecorder()
		router.ServeHTTP(verifyRec, verifyReq)

		assert.Equal(t, http.StatusOK, verifyRec.Code)
		verifyBody := verifyRec.Body.String()
		assert.Contains(t, verifyBody, `"me":"https://old.example.com/"`)
		assert.Contains(t, verifyBody, `"active":true`)
	})

	// Test: Metadata endpoint works on alt address
	t.Run("metadata endpoint works on alt address", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		req.Host = "old.example.com"

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, `"issuer":"https://old.example.com/"`)
		assert.Contains(t, body, `"authorization_endpoint"`)
		assert.Contains(t, body, `"token_endpoint"`)
	})
}
