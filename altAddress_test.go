package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sha256ForCode(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func base64URLEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func Test_oauthWithAltAddress(t *testing.T) {
	app := &goBlog{
		httpClient: newFakeHttpClient().Client,
		cfg:        createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://new.example.com"
	app.cfg.Server.AltAddresses = []string{"https://old.example.com"}
	app.cfg.Server.IndieAuthAddress = "https://old.example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
		},
	}
	app.cfg.User.Name = "John Doe"
	app.cfg.User.Nick = "jdoe"
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false

	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()
	app.reloadRouter()

	t.Run("discover metadata from main domain returns alt address endpoints", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://new.example.com/.well-known/oauth-authorization-server", nil)
		req.Host = "new.example.com"
		rec := httptest.NewRecorder()
		app.d.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var metadata map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &metadata))
		assert.Equal(t, "https://old.example.com/", metadata["issuer"])
		assert.Equal(t, "https://old.example.com/oauth/authorize", metadata["authorization_endpoint"])
		assert.Equal(t, "https://old.example.com/oauth/token", metadata["token_endpoint"])
	})

	t.Run("full authentication flow via alt address", func(t *testing.T) {
		clientID := "https://client.example.com/"
		redirectURI := "https://client.example.com/redirect"
		scope := "create"
		state := "test-state"

		codeVerifier := "test-code-verifier-that-is-long-enough-for-sha256-validation-1234567890"
		hash := sha256ForCode(codeVerifier)
		codeChallenge := base64URLEncode(hash)

		authorizeURL := "https://old.example.com/oauth/authorize?response_type=code" +
			"&client_id=" + url.QueryEscape(clientID) +
			"&redirect_uri=" + url.QueryEscape(redirectURI) +
			"&scope=" + url.QueryEscape(scope) +
			"&state=" + url.QueryEscape(state) +
			"&code_challenge=" + url.QueryEscape(codeChallenge) +
			"&code_challenge_method=S256"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, authorizeURL, nil)
		setLoggedIn(req, true)
		app.d.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "authorize page failed: %s", rec.Body.String())

		parsedHtml, err := goquery.NewDocumentFromReader(strings.NewReader(rec.Body.String()))
		require.NoError(t, err)

		indieauthForm := parsedHtml.Find("form[action='/oauth/authorize']")
		require.Equal(t, 1, indieauthForm.Length(), "form not found")
		formRedirectUri := indieauthForm.Find("input[name='redirect_uri']").AttrOr("value", "")
		assert.Equal(t, redirectURI, formRedirectUri)
		formClientId := indieauthForm.Find("input[name='client_id']").AttrOr("value", "")
		assert.Equal(t, clientID, formClientId)
		formCodeChallenge := indieauthForm.Find("input[name='code_challenge']").AttrOr("value", "")
		assert.NotEmpty(t, formCodeChallenge)
		formCodeChallengeMethod := indieauthForm.Find("input[name='code_challenge_method']").AttrOr("value", "")
		assert.Equal(t, "S256", formCodeChallengeMethod)
		formState := indieauthForm.Find("input[name='state']").AttrOr("value", "")
		assert.NotEmpty(t, formState)

		rec = httptest.NewRecorder()
		reqBody := url.Values{
			"redirect_uri":          {formRedirectUri},
			"client_id":             {formClientId},
			"scope":                 {scope},
			"code_challenge":        {formCodeChallenge},
			"code_challenge_method": {formCodeChallengeMethod},
			"state":                 {formState},
		}
		req = httptest.NewRequest(http.MethodPost, "https://old.example.com/oauth/authorize?"+reqBody.Encode(), nil)
		req.Host = "old.example.com"
		setLoggedIn(req, true)
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusFound, rec.Code)

		redirectLocation := rec.Header().Get("Location")
		require.NotEmpty(t, redirectLocation)
		redirectUrl, err := url.Parse(redirectLocation)
		require.NoError(t, err)
		code := redirectUrl.Query().Get("code")
		require.NotEmpty(t, code)
		assert.Equal(t, state, redirectUrl.Query().Get("state"))
		// Verify iss parameter is the alt address; me is returned in the token response, not the redirect
		assert.Equal(t, "https://old.example.com/", redirectUrl.Query().Get("iss"))
		assert.Empty(t, redirectUrl.Query().Get("me"))

		// Exchange code for token via alt address
		tokenValues := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"redirect_uri":  {redirectURI},
			"client_id":     {clientID},
			"code_verifier": {codeVerifier},
		}
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "https://old.example.com/oauth/token", strings.NewReader(tokenValues.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.d.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "token exchange failed: %s", rec.Body.String())

		var tokenResp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokenResp))
		accessToken, ok := tokenResp["access_token"].(string)
		require.True(t, ok)
		require.NotEmpty(t, accessToken)
		assert.Equal(t, "https://old.example.com/", tokenResp["me"])

		// Verify token via alt address
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "https://old.example.com/oauth/token", nil)
		req.Host = "old.example.com"
		req.Header.Set("Authorization", "Bearer "+accessToken)
		app.d.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var verifyResp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &verifyResp))
		assert.Equal(t, true, verifyResp["active"])
		assert.Equal(t, "https://old.example.com/", verifyResp["me"])
	})

	t.Run("html header has indieauth links to alt address", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://new.example.com/", nil)
		req.Host = "new.example.com"
		req.Header.Set("Accept", "text/html")

		rec := httptest.NewRecorder()
		app.d.ServeHTTP(rec, req)

		body := rec.Body.String()
		// Note: HTML may be minified with unquoted attributes
		assert.Contains(t, body, "https://old.example.com/oauth/authorize")
		assert.Contains(t, body, "https://old.example.com/oauth/token")
		assert.Contains(t, body, "https://old.example.com/.well-known/oauth-authorization-server")
	})
}
