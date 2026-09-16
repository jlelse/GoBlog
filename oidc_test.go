package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/justinas/alice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
)

const testOIDCKeyID = "test-key"

type fakeOIDCProvider struct {
	t        *testing.T
	server   *httptest.Server
	key      *rsa.PrivateKey
	clientID string
	subject  string

	mu          sync.Mutex
	codes       map[string]*fakeOIDCCode
	brokenNonce bool
}

type fakeOIDCCode struct {
	nonce       string
	challenge   string
	redirectURI string
}

func newFakeOIDCProvider(t *testing.T, clientID, subject string, opts ...func(*fakeOIDCProvider)) *fakeOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	p := &fakeOIDCProvider{
		t:        t,
		key:      key,
		clientID: clientID,
		subject:  subject,
		codes:    map[string]*fakeOIDCCode{},
	}
	for _, opt := range opts {
		opt(p)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.handleDiscovery)
	mux.HandleFunc("/authorize", p.handleAuthorize)
	mux.HandleFunc("/token", p.handleToken)
	mux.HandleFunc("/keys", p.handleKeys)
	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

// withBrokenNonce makes the fake provider return ID tokens with an invalid nonce.
func withBrokenNonce() func(*fakeOIDCProvider) {
	return func(p *fakeOIDCProvider) { p.brokenNonce = true }
}

func (p *fakeOIDCProvider) issuer() string {
	return p.server.URL
}

func (p *fakeOIDCProvider) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(contentType, contenttype.JSON)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                p.issuer(),
		"authorization_endpoint":                p.issuer() + "/authorize",
		"token_endpoint":                        p.issuer() + "/token",
		"jwks_uri":                              p.issuer() + "/keys",
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
	})
}

func (p *fakeOIDCProvider) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	code := randomString(20)
	p.mu.Lock()
	p.codes[code] = &fakeOIDCCode{
		nonce:       q.Get("nonce"),
		challenge:   q.Get("code_challenge"),
		redirectURI: q.Get("redirect_uri"),
	}
	p.mu.Unlock()
	redirect, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		http.Error(w, "invalid redirect", http.StatusBadRequest)
		return
	}
	rq := redirect.Query()
	rq.Set("code", code)
	rq.Set("state", q.Get("state"))
	redirect.RawQuery = rq.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

func (p *fakeOIDCProvider) handleToken(w http.ResponseWriter, r *http.Request) {
	require.NoError(p.t, r.ParseForm())
	code := r.FormValue("code")
	p.mu.Lock()
	c, ok := p.codes[code]
	delete(p.codes, code)
	p.mu.Unlock()
	if !ok {
		http.Error(w, "invalid code", http.StatusBadRequest)
		return
	}
	// Verify PKCE
	if c.challenge != "" {
		sum := sha256.Sum256([]byte(r.FormValue("code_verifier")))
		got := base64.RawURLEncoding.EncodeToString(sum[:])
		if got != c.challenge {
			http.Error(w, "invalid code_verifier", http.StatusBadRequest)
			return
		}
	}
	nonce := c.nonce
	if p.brokenNonce {
		nonce = "invalid-nonce"
	}
	idToken := p.signIDToken(nonce)
	w.Header().Set(contentType, contenttype.JSON)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": randomString(32),
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     idToken,
	})
}

func (p *fakeOIDCProvider) signIDToken(nonce string) string {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: p.key, KeyID: testOIDCKeyID}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(p.t, err)
	now := time.Now()
	raw, err := jwt.Signed(signer).Claims(map[string]any{
		"iss":   p.issuer(),
		"sub":   p.subject,
		"aud":   p.clientID,
		"exp":   now.Add(time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": nonce,
	}).Serialize()
	require.NoError(p.t, err)
	return raw
}

func (p *fakeOIDCProvider) handleKeys(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(contentType, contenttype.JSON)
	_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       p.key.Public(),
		KeyID:     testOIDCKeyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}})
}

func newOIDCTestApp(t *testing.T, provider *fakeOIDCProvider) *goBlog {
	t.Helper()
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.Server.PublicAddress = "https://blog.example.com"
	app.cfg.OIDC = &configOIDC{
		Enabled:  true,
		Issuer:   provider.issuer(),
		ClientID: provider.clientID,
	}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initOIDC())
	require.NoError(t, app.initTemplateStrings())
	return app
}

// authorize follows the redirect chain from the app to the provider and
// returns the code and state that the provider sends back.
func authorize(t *testing.T, loginRecorder *httptest.ResponseRecorder) (code, state string) {
	t.Helper()
	require.Equal(t, http.StatusFound, loginRecorder.Code)
	authURL := loginRecorder.Header().Get("Location")
	require.NotEmpty(t, authURL)

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(authURL) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	cb, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	return cb.Query().Get("code"), cb.Query().Get("state")
}

func oidcCallbackRequest(t *testing.T, code, state string, cookies ...*http.Cookie) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, oidcCallbackPath+"?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req
}

func createLoginSessionCookie(t *testing.T, app *goBlog) *http.Cookie {
	t.Helper()
	app.initSessionStores()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ses, err := app.loginSessions.New(req, "l")
	require.NoError(t, err)
	ses.Values["login"] = true
	require.NoError(t, app.loginSessions.Save(req, rec, ses))
	return rec.Result().Cookies()[0]
}

func Test_oidcLinkStorage(t *testing.T) {
	provider := newFakeOIDCProvider(t, "goblog", "user-123")
	app := newOIDCTestApp(t, provider)

	require.False(t, app.hasOIDCLink())
	require.NoError(t, app.linkOIDC("user-123"))
	assert.True(t, app.hasOIDCLink())
	subject, err := app.getOIDCSubject()
	require.NoError(t, err)
	assert.Equal(t, "user-123", subject)
	issuer, err := app.getSettingValue(oidcIssuerSettingsKey)
	require.NoError(t, err)
	assert.Equal(t, provider.issuer(), issuer)
	require.NoError(t, app.unlinkOIDC())
	assert.False(t, app.hasOIDCLink())
}

func Test_oidcLoginMethods(t *testing.T) {
	provider := newFakeOIDCProvider(t, "goblog", "user-123")
	app := newOIDCTestApp(t, provider)

	// GET without a session starts the login flow (used by the login popup)
	rec := httptest.NewRecorder()
	app.serveOIDCLogin(rec, httptest.NewRequest(http.MethodGet, oidcLoginPath, nil))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Location"))

	// GET while authenticated is not allowed, linking requires POST
	loginCookie := createLoginSessionCookie(t, app)
	linkRec := httptest.NewRecorder()
	linkReq := httptest.NewRequest(http.MethodGet, oidcLoginPath, nil)
	linkReq.AddCookie(loginCookie)
	app.serveOIDCLogin(linkRec, linkReq)
	assert.Equal(t, http.StatusMethodNotAllowed, linkRec.Code)
	assert.False(t, app.hasOIDCLink())
}

func Test_oidcLoginNotLinked(t *testing.T) {
	provider := newFakeOIDCProvider(t, "goblog", "user-123")
	app := newOIDCTestApp(t, provider)

	loginRec := httptest.NewRecorder()
	app.serveOIDCLogin(loginRec, httptest.NewRequest(http.MethodPost, oidcLoginPath, nil))
	require.Len(t, loginRec.Result().Cookies(), 1)
	sessionCookie := loginRec.Result().Cookies()[0]

	code, state := authorize(t, loginRec)

	cbRec := httptest.NewRecorder()
	app.serveOIDCCallback(cbRec, oidcCallbackRequest(t, code, state, sessionCookie))
	assert.Equal(t, http.StatusForbidden, cbRec.Code)
}

func Test_oidcLinkAndLogin(t *testing.T) {
	provider := newFakeOIDCProvider(t, "goblog", "user-123")
	app := newOIDCTestApp(t, provider)
	loginCookie := createLoginSessionCookie(t, app)

	// Link the account while authenticated
	linkRec := httptest.NewRecorder()
	linkReq := httptest.NewRequest(http.MethodPost, oidcLoginPath, nil)
	linkReq.AddCookie(loginCookie)
	app.serveOIDCLogin(linkRec, linkReq)
	require.Len(t, linkRec.Result().Cookies(), 1)
	linkCookie := linkRec.Result().Cookies()[0]

	code, state := authorize(t, linkRec)
	linkCbRec := httptest.NewRecorder()
	app.serveOIDCCallback(linkCbRec, oidcCallbackRequest(t, code, state, linkCookie, loginCookie))
	require.Equal(t, http.StatusFound, linkCbRec.Code)
	assert.Equal(t, settingsPath, linkCbRec.Header().Get("Location"))
	assert.True(t, app.hasOIDCLink())

	// Now log in with the linked account
	loginRec := httptest.NewRecorder()
	app.serveOIDCLogin(loginRec, httptest.NewRequest(http.MethodPost, oidcLoginPath, nil))
	require.Len(t, loginRec.Result().Cookies(), 1)
	oidcCookie := loginRec.Result().Cookies()[0]

	code, state = authorize(t, loginRec)
	loginCbRec := httptest.NewRecorder()
	app.serveOIDCCallback(loginCbRec, oidcCallbackRequest(t, code, state, oidcCookie))
	// The callback now renders the notify/close page instead of redirecting
	require.Equal(t, http.StatusOK, loginCbRec.Code)
	// The login session cookie must be set
	foundLogin := false
	for _, c := range loginCbRec.Result().Cookies() {
		if c.Name == "l" {
			foundLogin = true
		}
	}
	assert.True(t, foundLogin)
}

func Test_oidcLinkRequiresAuthAtCallback(t *testing.T) {
	provider := newFakeOIDCProvider(t, "goblog", "user-123")
	app := newOIDCTestApp(t, provider)
	loginCookie := createLoginSessionCookie(t, app)

	linkRec := httptest.NewRecorder()
	linkReq := httptest.NewRequest(http.MethodPost, oidcLoginPath, nil)
	linkReq.AddCookie(loginCookie)
	app.serveOIDCLogin(linkRec, linkReq)
	require.Len(t, linkRec.Result().Cookies(), 1)
	linkCookie := linkRec.Result().Cookies()[0]

	code, state := authorize(t, linkRec)
	// The login session is missing at callback time, linking must be refused
	linkCbRec := httptest.NewRecorder()
	app.serveOIDCCallback(linkCbRec, oidcCallbackRequest(t, code, state, linkCookie))
	assert.Equal(t, http.StatusUnauthorized, linkCbRec.Code)
	assert.False(t, app.hasOIDCLink())
}

func Test_oidcCallbackErrors(t *testing.T) {
	t.Run("Provider error response", func(t *testing.T) {
		provider := newFakeOIDCProvider(t, "goblog", "user-123")
		app := newOIDCTestApp(t, provider)

		loginRec := httptest.NewRecorder()
		app.serveOIDCLogin(loginRec, httptest.NewRequest(http.MethodPost, oidcLoginPath, nil))
		sessionCookie := loginRec.Result().Cookies()[0]

		_, state := authorize(t, loginRec)
		cbRec := httptest.NewRecorder()
		req := oidcCallbackRequest(t, "code", state, sessionCookie)
		req.URL.RawQuery += "&error=access_denied"
		app.serveOIDCCallback(cbRec, req)
		assert.Equal(t, http.StatusBadRequest, cbRec.Code)
		assert.Contains(t, cbRec.Body.String(), "access_denied")
	})

	t.Run("Invalid nonce", func(t *testing.T) {
		provider := newFakeOIDCProvider(t, "goblog", "user-123", withBrokenNonce())
		app := newOIDCTestApp(t, provider)
		require.NoError(t, app.linkOIDC("user-123"))

		loginRec := httptest.NewRecorder()
		app.serveOIDCLogin(loginRec, httptest.NewRequest(http.MethodPost, oidcLoginPath, nil))
		sessionCookie := loginRec.Result().Cookies()[0]

		code, state := authorize(t, loginRec)
		cbRec := httptest.NewRecorder()
		app.serveOIDCCallback(cbRec, oidcCallbackRequest(t, code, state, sessionCookie))
		assert.Equal(t, http.StatusBadRequest, cbRec.Code)
	})

	t.Run("Different subject than linked", func(t *testing.T) {
		provider := newFakeOIDCProvider(t, "goblog", "user-123")
		app := newOIDCTestApp(t, provider)
		require.NoError(t, app.linkOIDC("other-user"))

		loginRec := httptest.NewRecorder()
		app.serveOIDCLogin(loginRec, httptest.NewRequest(http.MethodPost, oidcLoginPath, nil))
		sessionCookie := loginRec.Result().Cookies()[0]

		code, state := authorize(t, loginRec)
		cbRec := httptest.NewRecorder()
		app.serveOIDCCallback(cbRec, oidcCallbackRequest(t, code, state, sessionCookie))
		assert.Equal(t, http.StatusForbidden, cbRec.Code)
	})
}

func Test_oidcIssuerMismatch(t *testing.T) {
	provider := newFakeOIDCProvider(t, "goblog", "user-123")
	app := newOIDCTestApp(t, provider)
	require.NoError(t, app.linkOIDC("user-123"))

	loginRec := httptest.NewRecorder()
	app.serveOIDCLogin(loginRec, httptest.NewRequest(http.MethodPost, oidcLoginPath, nil))
	oidcCookie := loginRec.Result().Cookies()[0]

	// Simulate that the issuer was changed after the link was created
	require.NoError(t, app.saveSettingValue(oidcIssuerSettingsKey, "https://other-id.example.com"))

	code, state := authorize(t, loginRec)
	cbRec := httptest.NewRecorder()
	app.serveOIDCCallback(cbRec, oidcCallbackRequest(t, code, state, oidcCookie))
	assert.Equal(t, http.StatusForbidden, cbRec.Code)
}

func Test_oidcInvalidState(t *testing.T) {
	provider := newFakeOIDCProvider(t, "goblog", "user-123")
	app := newOIDCTestApp(t, provider)

	loginRec := httptest.NewRecorder()
	app.serveOIDCLogin(loginRec, httptest.NewRequest(http.MethodPost, oidcLoginPath, nil))
	sessionCookie := loginRec.Result().Cookies()[0]

	cbRec := httptest.NewRecorder()
	app.serveOIDCCallback(cbRec, oidcCallbackRequest(t, "code", "wrong-state", sessionCookie))
	assert.Equal(t, http.StatusBadRequest, cbRec.Code)
}

func Test_oidcLoginPageButton(t *testing.T) {
	provider := newFakeOIDCProvider(t, "goblog", "user-123")
	app := newOIDCTestApp(t, provider)
	require.NoError(t, app.initTemplateStrings())
	require.NoError(t, app.linkOIDC("user-123"))

	h := alice.New(app.checkIsLogin, app.authMiddleware).Then(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/abc", nil))
	assert.Contains(t, rec.Body.String(), "OIDC")
}

func Test_settingsDeleteOIDC(t *testing.T) {
	provider := newFakeOIDCProvider(t, "goblog", "user-123")
	app := newOIDCTestApp(t, provider)
	require.NoError(t, app.setPassword("secret"))
	require.NoError(t, app.linkOIDC("user-123"))

	rec := httptest.NewRecorder()
	app.settingsDeleteOIDC(rec, httptest.NewRequest(http.MethodPost, settingsPath+settingsDeleteOIDCPath, strings.NewReader("")))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.False(t, app.hasOIDCLink())

	// Without another login method, unlinking must be refused
	require.NoError(t, app.setPasswordHash(""))
	require.NoError(t, app.linkOIDC("user-123"))
	rec = httptest.NewRecorder()
	app.settingsDeleteOIDC(rec, httptest.NewRequest(http.MethodPost, settingsPath+settingsDeleteOIDCPath, strings.NewReader("")))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.True(t, app.hasOIDCLink())
}
