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
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_oauthMetadata(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {Lang: "en"},
	}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.prepareWebfinger()
	app.d = app.buildRouter()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://example.org/.well-known/oauth-authorization-server", nil)
	app.d.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var metadata map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &metadata))

	assert.Equal(t, "https://example.org/", metadata["issuer"])
	assert.Equal(t, "https://example.org/oauth/authorize", metadata["authorization_endpoint"])
	assert.Equal(t, "https://example.org/oauth/token", metadata["token_endpoint"])
	assert.Equal(t, "https://example.org/oauth/revoke", metadata["revocation_endpoint"])
	assert.Equal(t, "https://example.org/api/v1/apps", metadata["app_registration_endpoint"])
	assert.Contains(t, metadata["scopes_supported"], "profile")
	assert.Contains(t, metadata["scopes_supported"], "read")
	assert.Contains(t, metadata["scopes_supported"], "create")
	assert.Contains(t, metadata["response_types_supported"], "code")
	assert.Equal(t, []any{"authorization_code"}, metadata["grant_types_supported"])
	assert.Contains(t, metadata["code_challenge_methods_supported"], "S256")
}

func Test_oauthCreateApp(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {Lang: "en"},
	}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	form := url.Values{
		"client_name":   {"Test App"},
		"redirect_uris": {"https://testapp.example/callback"},
		"scopes":        {"profile"},
		"website":       {"https://testapp.example"},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "https://example.org/api/v1/apps", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.d.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.NotEmpty(t, resp["client_id"])
	assert.NotEmpty(t, resp["client_secret"])
	assert.Equal(t, "Test App", resp["name"])
	assert.Equal(t, "https://testapp.example", resp["website"])
	assert.Equal(t, []any{"https://testapp.example/callback"}, resp["redirect_uris"])
	assert.Equal(t, []any{"profile"}, resp["scopes"])
	assert.Equal(t, float64(0), resp["client_secret_expires_at"])

	appInDB, err := app.db.oauthGetApp(resp["client_id"].(string))
	require.NoError(t, err)
	assert.Equal(t, "Test App", appInDB.Name)
	assert.Equal(t, resp["client_secret"].(string), appInDB.Secret)
	assert.Equal(t, "profile", appInDB.Scopes)
}

func Test_oauthAuthorizationFlow(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {Lang: "en"},
	}
	app.cfg.User = &configUser{
		Name: "John Doe",
		Nick: "jdoe",
	}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.prepareWebfinger()
	app.d = app.buildRouter()

	secret := generateOAuthSecret()
	appID, err := app.db.oauthCreateApp("Test App", secret, "https://testapp.example/callback", "profile", "https://testapp.example")
	require.NoError(t, err)

	t.Run("ShowAuthorizePage", func(t *testing.T) {
		authURL := "https://example.org/oauth/authorize?response_type=code&client_id=" + appID + "&redirect_uri=https://testapp.example/callback&scope=profile&state=mystate"

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, authURL, nil)
		app.d.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Authorization")
		assert.Contains(t, rec.Body.String(), "Test App")
		assert.Contains(t, rec.Body.String(), "profile")

		parsed, err := goquery.NewDocumentFromReader(strings.NewReader(rec.Body.String()))
		require.NoError(t, err)

		form := parsed.Find("form[action='/oauth/authorize']")
		assert.Equal(t, 1, form.Length())
		assert.Equal(t, appID, form.Find("input[name='client_id']").AttrOr("value", ""))
		assert.Equal(t, "https://testapp.example/callback", form.Find("input[name='redirect_uri']").AttrOr("value", ""))
		assert.Equal(t, "profile", form.Find("input[name='scope']").AttrOr("value", ""))
		assert.Equal(t, "mystate", form.Find("input[name='state']").AttrOr("value", ""))
	})

	t.Run("SubmitAuthorization", func(t *testing.T) {
		form := url.Values{
			"client_id":    {appID},
			"redirect_uri": {"https://testapp.example/callback"},
			"scope":        {"profile"},
			"state":        {"mystate"},
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/authorize", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		setLoggedIn(req, true)
		app.d.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusFound, rec.Code)

		location := rec.Header().Get("Location")
		assert.NotEmpty(t, location)
		assert.Contains(t, location, "https://testapp.example/callback")
		assert.Contains(t, location, "code=")
		assert.Contains(t, location, "state=mystate")
		assert.Contains(t, location, "iss=https%3A%2F%2Fexample.org%2F")
	})

	t.Run("TokenExchange", func(t *testing.T) {
		form := url.Values{
			"client_id":    {appID},
			"redirect_uri": {"https://testapp.example/callback"},
			"scope":        {"profile"},
			"state":        {"tokentest"},
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/authorize", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		setLoggedIn(req, true)
		app.d.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusFound, rec.Code)
		location, err := url.Parse(rec.Header().Get("Location"))
		require.NoError(t, err)
		code := location.Query().Get("code")
		assert.NotEmpty(t, code)

		tokenForm := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"client_id":     {appID},
			"client_secret": {secret},
			"redirect_uri":  {"https://testapp.example/callback"},
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(tokenForm.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.d.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var tokenResp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokenResp))

		assert.NotEmpty(t, tokenResp["access_token"])
		assert.Equal(t, "Bearer", tokenResp["token_type"])
		assert.Equal(t, "profile", tokenResp["scope"])
		assert.Equal(t, "https://example.org/", tokenResp["me"])
	})

	t.Run("TokenExchangeInvalidClient", func(t *testing.T) {
		tokenForm := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {"invalid"},
			"client_id":     {"invalid"},
			"client_secret": {"invalid"},
			"redirect_uri":  {"https://testapp.example/callback"},
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(tokenForm.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.d.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func Test_oauthVerifyCredentials(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {Lang: "en"},
	}
	app.cfg.User = &configUser{
		Name: "John Doe",
		Nick: "jdoe",
	}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.prepareWebfinger()
	app.d = app.buildRouter()

	secret := generateOAuthSecret()
	appID, err := app.db.oauthCreateApp("Test App", secret, "https://testapp.example/callback", "profile", "")
	require.NoError(t, err)

	token := tokenFromApp(t, app, appID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://example.org/api/v1/accounts/verify_credentials", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	app.d.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var account map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &account))

	assert.Equal(t, "en", account["username"])
	assert.Equal(t, "en@example.org", account["acct"])
	assert.Equal(t, "John Doe", account["display_name"])
	assert.NotEmpty(t, account["id"])
	assert.NotEmpty(t, account["url"])
	assert.NotEmpty(t, account["avatar"])
}

func Test_oauthVerifyCredentialsGranularScope(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {Lang: "en"},
	}
	app.cfg.User = &configUser{Name: "John Doe", Nick: "jdoe"}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.prepareWebfinger()
	app.d = app.buildRouter()

	secret := generateOAuthSecret()
	appID, err := app.db.oauthCreateApp("Test App", secret, "https://testapp.example/callback", "read:accounts", "")
	require.NoError(t, err)

	cases := []struct {
		name       string
		scope      string
		wantStatus int
	}{
		{"granular read:accounts", "read:accounts", http.StatusOK},
		{"top-level read grants read:accounts", "read", http.StatusOK},
		{"profile scope", "profile", http.StatusOK},
		{"unrelated scope is rejected", "media", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := uuid.NewString()
			_, err := app.db.Exec("insert into indieauthtoken (time, token, client, scope) values (?, ?, ?, ?)", time.Now().UTC().Unix(), token, appID, tc.scope)
			require.NoError(t, err)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "https://example.org/api/v1/accounts/verify_credentials", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			app.d.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}

// Test_oauthTokenClientSecretBasic verifies that client credentials supplied via
// HTTP Basic auth (client_secret_basic, as advertised in the server metadata)
// are accepted at the token endpoint. Many Mastodon-compatible clients send
// client_id/client_secret this way instead of in the POST body.
func Test_oauthTokenClientSecretBasic(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {Lang: "en"},
	}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	secret := generateOAuthSecret()
	appID, err := app.db.oauthCreateApp("Test App", secret, "https://testapp.example/callback", "read", "")
	require.NoError(t, err)

	// Issue an authorization code first.
	authForm := url.Values{
		"client_id":    {appID},
		"redirect_uri": {"https://testapp.example/callback"},
		"scope":        {"read"},
		"state":        {"basic"},
	}
	authRec := httptest.NewRecorder()
	authReq := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/authorize", strings.NewReader(authForm.Encode()))
	authReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setLoggedIn(authReq, true)
	app.d.ServeHTTP(authRec, authReq)
	require.Equal(t, http.StatusFound, authRec.Code)
	loc, err := url.Parse(authRec.Header().Get("Location"))
	require.NoError(t, err)
	code := loc.Query().Get("code")
	require.NotEmpty(t, code)

	tokenForm := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {"https://testapp.example/callback"},
	}

	basic := base64.StdEncoding.EncodeToString([]byte(appID + ":" + secret))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(tokenForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basic)
	app.d.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var tokenResp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokenResp))
	assert.NotEmpty(t, tokenResp["access_token"])
}

func Test_oauthRevoke(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {Lang: "en"},
	}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	secret := generateOAuthSecret()
	appID, err := app.db.oauthCreateApp("Test App", secret, "https://testapp.example/callback", "profile", "")
	require.NoError(t, err)

	token := tokenFromApp(t, app, appID)

	revokeForm := url.Values{
		"token": {token},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/revoke", strings.NewReader(revokeForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.d.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	_, err = app.db.oauthVerifyToken("Bearer " + token)
	assert.Error(t, err)
}

func Test_apHandleWebfingerOAuthIssuer(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"default": {
			Path: "/",
		},
	}
	app.cfg.DefaultBlog = "default"
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()
	app.prepareWebfinger()

	req := httptest.NewRequest(http.MethodGet, "https://example.com/.well-known/webfinger?resource=acct:default@example.com", nil)
	rec := httptest.NewRecorder()

	app.apHandleWebfinger(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"rel":"http://openid.net/specs/connect/1.0/issuer"`)
	assert.Contains(t, rec.Body.String(), `"href":"https://example.com/"`)
}

func tokenFromApp(t *testing.T, app *goBlog, appID string) string {
	t.Helper()
	token := uuid.NewString()
	_, err := app.db.Exec("insert into indieauthtoken (time, token, client, scope) values (?, ?, ?, ?)", time.Now().UTC().Unix(), token, appID, "profile")
	require.NoError(t, err)
	return token
}

func Test_oauthIntrospectionPost(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {Lang: "en"},
	}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.prepareWebfinger()
	app.d = app.buildRouter()

	appID := "https://example.com/"
	token := tokenFromApp(t, app, appID)

	t.Run("active token", func(t *testing.T) {
		form := url.Values{"token": {token}}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.d.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, true, resp["active"])
		assert.Equal(t, "https://example.org/", resp["me"])
		assert.Equal(t, appID, resp["client_id"])
		assert.Equal(t, "profile", resp["scope"])
	})

	t.Run("unknown token", func(t *testing.T) {
		form := url.Values{"token": {"not-a-real-token"}}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.d.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, false, resp["active"])
	})
}

func Test_oauthWithoutActivityPub(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {Lang: "en"},
	}
	app.cfg.User = &configUser{
		Name: "John Doe",
		Nick: "jdoe",
	}
	app.cfg.ActivityPub = &configActivityPub{Enabled: false}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	t.Run("IndieAuth metadata is served", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "https://example.org/.well-known/oauth-authorization-server", nil)
		app.d.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var metadata map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &metadata))
		assert.Equal(t, "https://example.org/", metadata["issuer"])
		assert.Equal(t, "https://example.org/oauth/authorize", metadata["authorization_endpoint"])
		// Mastodon app_registration_endpoint must NOT be advertised without ActivityPub
		assert.NotContains(t, metadata, "app_registration_endpoint")
	})

	t.Run("Mastodon app endpoint is not registered", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/api/v1/apps", strings.NewReader("client_name=test&redirect_uris=https://cb"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.d.ServeHTTP(rec, req)
		// Either 404 (no route) or 405 (route exists but wrong method) is acceptable — neither should be 200.
		assert.True(t, rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed, "got status %d", rec.Code)
	})

	t.Run("Mastodon verify_credentials is not registered", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "https://example.org/api/v1/accounts/verify_credentials", nil)
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("IndieAuth authorize page is served", func(t *testing.T) {
		codeChallenge := "OfYAxt8zU2dAPDWQxTAUIteRzMsoj9QBdMIVEDOErUo"
		authURL := "https://example.org/oauth/authorize?response_type=code&client_id=https://example.com/&redirect_uri=https://example.com/cb&scope=create&state=s&code_challenge=" + codeChallenge + "&code_challenge_method=S256"
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, authURL, nil)
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func Test_oauthRedirectURIsMultiple(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {Lang: "en"},
	}
	app.cfg.User = &configUser{Name: "John Doe", Nick: "jdoe"}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	secret := generateOAuthSecret()
	// Register with two URIs separated by space and newline
	appID, err := app.db.oauthCreateApp("Test App", secret, "https://a.example/cb\nhttps://b.example/cb", "read", "")
	require.NoError(t, err)

	cases := []struct {
		name        string
		redirectURI string
		wantOK      bool
	}{
		{"first URI", "https://a.example/cb", true},
		{"second URI", "https://b.example/cb", true},
		{"unknown URI", "https://c.example/cb", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := app.getOAuthApp(appID, tc.redirectURI)
			if tc.wantOK {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// Test_oauthURLClientRedirectOriginMatch verifies that a URL-based (IndieAuth) client
// can only redirect to a URI on the same scheme/host/port as the client_id. This is
// the minimum protection against the open-redirect risk for URL clients (IndieAuth §5.2).
func Test_oauthURLClientRedirectOriginMatch(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{"en": {Lang: "en"}}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	clientID := "https://app.example.com/"
	cases := []struct {
		name        string
		redirectURI string
		wantOK      bool
	}{
		{"same origin", "https://app.example.com/callback", true},
		{"same host different path", "https://app.example.com/other/cb", true},
		{"same host different port", "https://app.example.com:8443/cb", false},
		{"different host", "https://evil.example.com/cb", false},
		{"different scheme", "http://app.example.com/cb", false},
		{"subdomain", "https://sub.app.example.com/cb", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := app.getOAuthApp(clientID, tc.redirectURI)
			if tc.wantOK {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// Test_oauthURLClientPKCERequired verifies that a URL-based IndieAuth client must
// supply a code_challenge at the authorization endpoint.
func Test_oauthURLClientPKCERequired(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{"en": {Lang: "en"}}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	base := "https://example.org/oauth/authorize?response_type=code&client_id=https%3A%2F%2Fapp.example.com%2F&redirect_uri=https%3A%2F%2Fapp.example.com%2Fcb&scope=create&state=s"

	t.Run("PKCE missing is rejected on GET", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, base, nil)
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("PKCE missing is rejected on POST", func(t *testing.T) {
		form := url.Values{
			"client_id":    {"https://app.example.com/"},
			"redirect_uri": {"https://app.example.com/cb"},
			"scope":        {"create"},
			"state":        {"s"},
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/authorize", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		setLoggedIn(req, true)
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("plain PKCE method is rejected", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, base+"&code_challenge=abc&code_challenge_method=plain", nil)
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// Test_oauthTokenPKCEEnforcement verifies that for URL-based (IndieAuth) clients,
// PKCE is enforced at the token endpoint: the code_verifier must be supplied and
// must hash to the stored code_challenge.
func Test_oauthTokenPKCEEnforcement(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{"en": {Lang: "en"}}
	app.cfg.User = &configUser{Name: "John Doe", Nick: "jdoe"}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	clientID := "https://app.example.com/"
	redirectURI := "https://app.example.com/cb"
	codeVerifier := "test-code-verifier-that-is-long-enough-for-sha256-validation-1234567890"
	hash := sha256Sum(codeVerifier)
	codeChallenge := base64.RawURLEncoding.EncodeToString(hash[:])

	insertCode := func(challenge, method string) string {
		code := uuid.NewString()
		_, err := app.db.Exec(
			"insert into indieauthauth (time, code, client, redirect, scope, challenge, challengemethod) values (?, ?, ?, ?, ?, ?, ?)",
			time.Now().UTC().Unix(), code, clientID, redirectURI, "create", challenge, method,
		)
		require.NoError(t, err)
		return code
	}

	t.Run("URL client with no code_verifier is rejected", func(t *testing.T) {
		code := insertCode(codeChallenge, "S256")
		form := url.Values{
			"grant_type":   {"authorization_code"},
			"code":         {code},
			"client_id":    {clientID},
			"redirect_uri": {redirectURI},
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("URL client with wrong code_verifier is rejected", func(t *testing.T) {
		code := insertCode(codeChallenge, "S256")
		form := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"client_id":     {clientID},
			"redirect_uri":  {redirectURI},
			"code_verifier": {"wrong-verifier-that-is-long-enough-for-sha256-validation-12345"},
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("URL client with correct code_verifier succeeds", func(t *testing.T) {
		code := insertCode(codeChallenge, "S256")
		form := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"client_id":     {clientID},
			"redirect_uri":  {redirectURI},
			"code_verifier": {codeVerifier},
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("URL client with mismatched redirect_uri is rejected", func(t *testing.T) {
		code := insertCode(codeChallenge, "S256")
		form := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"client_id":     {clientID},
			"redirect_uri":  {"https://attacker.example.com/cb"},
			"code_verifier": {codeVerifier},
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// Test_oauthTokenCodeReuse verifies that an authorization code can be redeemed only
// once. The server deletes the code on first successful lookup (IndieAuth §5.2.1).
func Test_oauthTokenCodeReuse(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{"en": {Lang: "en"}}
	app.cfg.User = &configUser{Name: "John Doe", Nick: "jdoe"}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	secret := generateOAuthSecret()
	appID, err := app.db.oauthCreateApp("Test App", secret, "https://testapp.example/callback", "profile", "")
	require.NoError(t, err)

	code := uuid.NewString()
	_, err = app.db.Exec(
		"insert into indieauthauth (time, code, client, redirect, scope, challenge, challengemethod) values (?, ?, ?, ?, ?, ?, ?)",
		time.Now().UTC().Unix(), code, appID, "https://testapp.example/callback", "profile", "", "",
	)
	require.NoError(t, err)

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {appID},
		"client_secret": {secret},
		"redirect_uri":  {"https://testapp.example/callback"},
	}
	req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	app.d.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.d.ServeHTTP(rec, req2)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Test_oauthTokenExpiredCode verifies that authorization codes older than the 10-minute
// lifetime are rejected.
func Test_oauthTokenExpiredCode(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{"en": {Lang: "en"}}
	app.cfg.User = &configUser{Name: "John Doe", Nick: "jdoe"}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	secret := generateOAuthSecret()
	appID, err := app.db.oauthCreateApp("Test App", secret, "https://testapp.example/callback", "profile", "")
	require.NoError(t, err)

	code := uuid.NewString()
	_, err = app.db.Exec(
		"insert into indieauthauth (time, code, client, redirect, scope, challenge, challengemethod) values (?, ?, ?, ?, ?, ?, ?)",
		time.Now().UTC().Add(-11*time.Minute).Unix(), code, appID, "https://testapp.example/callback", "profile", "", "",
	)
	require.NoError(t, err)

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {appID},
		"client_secret": {secret},
		"redirect_uri":  {"https://testapp.example/callback"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.d.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Test_oauthTokenUnsupportedGrantType verifies the token endpoint rejects unknown
// grant types per OAuth 2.0 §5.2.
func Test_oauthTokenUnsupportedGrantType(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{"en": {Lang: "en"}}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	secret := generateOAuthSecret()
	appID, err := app.db.oauthCreateApp("Test App", secret, "https://testapp.example/callback", "read", "")
	require.NoError(t, err)

	form := url.Values{
		"grant_type":    {"password"},
		"client_id":     {appID},
		"client_secret": {secret},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "https://example.org/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.d.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Test_oauthTokenGetIntrospection covers the IndieAuth legacy GET token verification
// path (Authorization: Bearer header) at the /oauth/token endpoint.
func Test_oauthTokenGetIntrospection(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.Server.PublicAddress = "https://example.org"
	app.cfg.Blogs = map[string]*configBlog{"en": {Lang: "en"}}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.cfg.Cache.Enable = false
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	secret := generateOAuthSecret()
	appID, err := app.db.oauthCreateApp("Test App", secret, "https://testapp.example/callback", "profile", "")
	require.NoError(t, err)

	token := tokenFromApp(t, app, appID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://example.org/oauth/token", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	app.d.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["active"])
	assert.Equal(t, appID, resp["client_id"])
	assert.Equal(t, "profile", resp["scope"])
	assert.Equal(t, "https://example.org/", resp["me"])
}

// Test_isOAuthURLClientIDCanonicalization verifies the IndieAuth §3.4 canonicalization
// applied to URL-based client_id values: must have http(s) scheme, non-empty path,
// no fragment, no dot-segments.
func Test_isOAuthURLClientIDCanonicalization(t *testing.T) {
	cases := []struct {
		clientID string
		want     bool
	}{
		{"https://example.com/", true},
		{"http://example.com/", true},
		{"https://example.com/path", true},
		{"https://example.com/path?q=1", true},
		{"https://example.com/path#frag", false},
		{"https://example.com/./path", false},
		{"https://example.com/foo/../bar", false},
		{"not-a-url", false},
		{"", false},
		{"https://example.com", false}, // no path
	}
	for _, tc := range cases {
		t.Run(tc.clientID, func(t *testing.T) {
			assert.Equal(t, tc.want, isOAuthURLClientID(tc.clientID))
		})
	}
}

func sha256Sum(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}
