package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/hacdias/indieauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_indieAuthServer(t *testing.T) {
	defer os.RemoveAll(t.TempDir()) // I don't know why this is necessary, but it is.

	var err error

	app := &goBlog{
		httpClient: &fakeHttpClient{},
		cfg: &config{
			Db: &configDb{
				File: filepath.Join(t.TempDir(), "test.db"),
			},
			Server: &configServer{
				PublicAddress: "https://example.org",
			},
			DefaultBlog: "en",
			Blogs: map[string]*configBlog{
				"en": {
					Lang: "en",
				},
			},
			User: &configUser{
				Name: "John Doe",
				Nick: "jdoe",
			},
			Cache: &configCache{
				Enable: false,
			},
		},
	}

	app.d, err = app.buildRouter()
	require.NoError(t, err)

	_ = app.initDatabase(false)
	app.initComponents(false)

	app.ias.Client = &http.Client{
		Transport: &handlerRoundTripper{
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		},
	}

	iac := indieauth.NewClient(
		"https://example.com/",
		"https://example.com/redirect",
		&http.Client{
			Transport: &handlerRoundTripper{
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					app.d.ServeHTTP(w, r)
				}),
			},
		},
	)
	require.NotNil(t, iac)

	endpoints, err := iac.DiscoverEndpoints("https://example.org/")
	require.NoError(t, err)
	if assert.NotNil(t, endpoints) {
		assert.Equal(t, "https://example.org/indieauth", endpoints.Authorization)
		assert.Equal(t, "https://example.org/indieauth/token", endpoints.Token)
	}

	for _, test := range []int{1, 2} {

		authinfo, redirect, err := iac.Authenticate("https://example.org/", "create")
		require.NoError(t, err)
		assert.NotNil(t, authinfo)
		assert.NotEmpty(t, redirect)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, redirect, nil)
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "https://example.com/redirect")

		parsedHtml, err := goquery.NewDocumentFromReader(strings.NewReader(rec.Body.String()))
		require.NoError(t, err)

		indieauthForm := parsedHtml.Find("form[action='/indieauth/accept']")
		assert.Equal(t, 1, indieauthForm.Length())
		indieAuthFormRedirectUri := indieauthForm.Find("input[name='redirect_uri']").AttrOr("value", "")
		assert.Equal(t, "https://example.com/redirect", indieAuthFormRedirectUri)
		indieAuthFormClientId := indieauthForm.Find("input[name='client_id']").AttrOr("value", "")
		assert.Equal(t, "https://example.com/", indieAuthFormClientId)
		indieAuthFormScopes := indieauthForm.Find("input[name='scopes']").AttrOr("value", "")
		assert.Equal(t, "create", indieAuthFormScopes)
		indieAuthFormCodeChallenge := indieauthForm.Find("input[name='code_challenge']").AttrOr("value", "")
		assert.NotEmpty(t, indieAuthFormCodeChallenge)
		indieAuthFormCodeChallengeMethod := indieauthForm.Find("input[name='code_challenge_method']").AttrOr("value", "")
		assert.Equal(t, "S256", indieAuthFormCodeChallengeMethod)
		indieAuthFormState := indieauthForm.Find("input[name='state']").AttrOr("value", "")
		assert.NotEmpty(t, indieAuthFormState)

		rec = httptest.NewRecorder()
		reqBody := url.Values{
			"redirect_uri":          {indieAuthFormRedirectUri},
			"client_id":             {indieAuthFormClientId},
			"scopes":                {indieAuthFormScopes},
			"code_challenge":        {indieAuthFormCodeChallenge},
			"code_challenge_method": {indieAuthFormCodeChallengeMethod},
			"state":                 {indieAuthFormState},
		}
		req = httptest.NewRequest(http.MethodPost, "https://example.org/indieauth/accept?"+reqBody.Encode(), nil)
		setLoggedIn(req, true)
		app.d.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusFound, rec.Code)

		redirectLocation := rec.Header().Get("Location")
		assert.NotEmpty(t, redirectLocation)
		redirectUrl, err := url.Parse(redirectLocation)
		require.NoError(t, err)
		assert.NotEmpty(t, redirectUrl.Query().Get("code"))
		assert.NotEmpty(t, redirectUrl.Query().Get("state"))

		validateReq := httptest.NewRequest(http.MethodGet, redirectLocation, nil)
		code, err := iac.ValidateCallback(authinfo, validateReq)
		require.NoError(t, err)
		assert.NotEmpty(t, code)

		if test == 1 {

			profile, err := iac.FetchProfile(authinfo, code)
			require.NoError(t, err)
			assert.NotNil(t, profile)
			assert.Equal(t, "https://example.org/", profile.Me)

		} else if test == 2 {

			token, _, err := iac.GetToken(authinfo, code)
			require.NoError(t, err)
			assert.NotNil(t, token)
			assert.NotEqual(t, "", token.AccessToken)

			rec = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodGet, "https://example.org/indieauth/token", nil)
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			app.d.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)

			rec = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodPost, "https://example.org/indieauth/token?action=revoke&token="+token.AccessToken, nil)
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			app.d.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)

			rec = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodGet, "https://example.org/indieauth/token", nil)
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			app.d.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)

		}

	}

}
