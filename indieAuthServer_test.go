package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_oauthServer(t *testing.T) {
	defer os.RemoveAll(t.TempDir())

	app := &goBlog{
		httpClient: newFakeHttpClient().Client,
		cfg:        createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
		},
	}
	app.cfg.User = &configUser{
		Name: "John Doe",
		Nick: "jdoe",
	}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	app.d = app.buildRouter()

	metadata := discoverOAuthMetadata(t, app.d, "https://example.org/")
	require.NotNil(t, metadata)
	assert.Equal(t, "https://example.org/", metadata.Issuer)
	assert.Equal(t, "https://example.org/oauth/authorize", metadata.AuthorizationEndpoint)
	assert.Equal(t, "https://example.org/oauth/token", metadata.TokenEndpoint)
	assert.Equal(t, "https://example.org/oauth/revoke", metadata.RevocationEndpoint)

	// Test 1: IndieAuth-style flow (URL-based client, no secret)
	runOAuthTest(t, app, "https://example.org/", metadata, "https://client.example.com/", "https://client.example.com/redirect", "create", "")

	// Test 2: Fediverse-style flow (UUID-based client, with secret)
	runFediverseOAuthTest(t, app, "https://example.org/", metadata)
}

type oauthMetadata struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RevocationEndpoint    string   `json:"revocation_endpoint"`
	ScopesSupported       []string `json:"scopes_supported"`
}

func discoverOAuthMetadata(t *testing.T, handler http.Handler, origin string) *oauthMetadata {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, origin+"/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var m oauthMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
	return &m
}

func runOAuthTest(t *testing.T, app *goBlog, origin string, metadata *oauthMetadata, clientID, redirectURI, scope, clientSecret string) {
	t.Helper()

	codeVerifier := "test-code-verifier-that-is-long-enough-for-sha256-validation-1234567890"
	hash := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(hash[:])

	// 1. Discover authorization endpoint
	authorizeURL := metadata.AuthorizationEndpoint + "?response_type=code&client_id=" + url.QueryEscape(clientID) +
		"&redirect_uri=" + url.QueryEscape(redirectURI) +
		"&scope=" + url.QueryEscape(scope) +
		"&state=test-state" +
		"&code_challenge=" + url.QueryEscape(codeChallenge) +
		"&code_challenge_method=S256"

	// 2. Fetch the authorization page (logged in)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, authorizeURL, nil)
	setLoggedIn(req, true)
	app.d.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "authorize GET failed: %s", rec.Body.String())

	// 3. Submit the authorization form
	formValues := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {scope},
		"state":                 {"test-state"},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, metadata.AuthorizationEndpoint+"?"+formValues.Encode(), nil)
	setLoggedIn(req, true)
	app.d.ServeHTTP(rec, req)
	require.Equal(t, http.StatusFound, rec.Code, "authorize POST failed: %s", rec.Body.String())

	// 4. Verify redirect URL has code, state, iss
	redirectLocation := rec.Header().Get("Location")
	require.NotEmpty(t, redirectLocation)
	redirectURL, err := url.Parse(redirectLocation)
	require.NoError(t, err)
	code := redirectURL.Query().Get("code")
	require.NotEmpty(t, code)
	assert.Equal(t, "test-state", redirectURL.Query().Get("state"))
	assert.Equal(t, metadata.Issuer, redirectURL.Query().Get("iss"))
	assert.Empty(t, redirectURL.Query().Get("me"))

	// 5. Exchange code for token
	tokenValues := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}
	if clientSecret != "" {
		tokenValues.Set("client_secret", clientSecret)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, metadata.TokenEndpoint, strings.NewReader(tokenValues.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.d.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "token response: %s", rec.Body.String())

	var tokenResp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokenResp))
	accessToken, ok := tokenResp["access_token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, accessToken)
	assert.Equal(t, metadata.Issuer, tokenResp["me"])
	assert.Equal(t, scope, tokenResp["scope"])
	assert.Equal(t, "Bearer", tokenResp["token_type"])

	// 6. Verify token via IndieAuth endpoint
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, metadata.TokenEndpoint, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	app.d.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var verifyResp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &verifyResp))
	assert.Equal(t, true, verifyResp["active"])
	assert.Equal(t, metadata.Issuer, verifyResp["me"])

	// 7. Revoke token
	revokeValues := url.Values{
		"token": {accessToken},
	}
	if clientSecret != "" {
		revokeValues.Set("client_id", clientID)
		revokeValues.Set("client_secret", clientSecret)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, metadata.RevocationEndpoint, strings.NewReader(revokeValues.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.d.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "revoke failed: %s", rec.Body.String())

	// 8. Verify token is now inactive
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, metadata.TokenEndpoint, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	app.d.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &verifyResp))
	assert.Equal(t, false, verifyResp["active"])
}

func runFediverseOAuthTest(t *testing.T, app *goBlog, origin string, metadata *oauthMetadata) {
	t.Helper()

	// 1. Create an app
	createValues := url.Values{
		"client_name":   {"Test Fediverse App"},
		"redirect_uris": {"https://client.example.com/redirect"},
		"scopes":        {"read"},
		"website":       {"https://client.example.com"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, origin+"/api/v1/apps", strings.NewReader(createValues.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.d.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "create app: %s", rec.Body.String())

	var createResp struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createResp))
	require.NotEmpty(t, createResp.ClientID)
	require.NotEmpty(t, createResp.ClientSecret)

	// 2. Use the same flow with the registered app
	runOAuthTest(t, app, origin, metadata, createResp.ClientID, "https://client.example.com/redirect", "read", createResp.ClientSecret)
}
