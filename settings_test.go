package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestSettingsConnectedApps(t *testing.T) {
	a := &goBlog{cfg: createDefaultTestConfig(t)}
	require.NoError(t, a.initConfig(false))
	require.NoError(t, a.initTemplateStrings())
	bc := a.cfg.Blogs[a.cfg.DefaultBlog]
	bc.Path = "/blog"
	router := chi.NewRouter()
	router.Route(bc.getRelativePath(settingsPath), a.blogSettingsRouter(bc))
	page := func(loggedIn bool) *goquery.Document {
		r := httptest.NewRequest(http.MethodGet, "/blog/settings/connected-apps", nil)
		setLoggedIn(r, loggedIn)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code)
		require.NotContains(t, w.Body.String(), "secret-")
		doc, err := goquery.NewDocumentFromReader(w.Body)
		require.NoError(t, err)
		return doc
	}
	require.Contains(t, page(true).Find("main").Text(), "No websites or apps")

	id, err := a.db.oauthCreateApp("Reader <One>", "secret-client", "", "read", "")
	require.NoError(t, err)
	const site = "https://example.org/client"
	_, err = a.db.Exec(`insert into indieauthtoken (time, token, client, scope) values
		(9, 'secret-first', ?, 'update create'), (99, 'secret-app', ?, ''), (100, 'secret-latest', ?, 'profile create')`, site, id, site)
	require.NoError(t, err)
	require.NotContains(t, page(false).Text(), site)
	doc := page(true)
	require.Equal(t, "/blog/settings", doc.Find("main a").AttrOr("href", ""))
	clients := doc.Find("main li")
	require.Equal(t, 2, clients.Length())
	require.Contains(t, clients.Eq(0).Text(), site)
	require.Contains(t, clients.Eq(0).Text(), "Scopes: create, profile, update")
	require.Equal(t, "1970-01-01T00:01:40Z", clients.Eq(0).Find("time").AttrOr("datetime", ""))
	require.Contains(t, clients.Eq(1).Text(), "Reader <One>")
	require.Contains(t, clients.Eq(1).Text(), "No explicit scopes")

	form := clients.First().Find("form")
	action := form.AttrOr("action", "")
	require.Equal(t, "/blog/settings/connected-apps/revoke", action)
	button := form.Find("button")
	require.Equal(t, site, button.AttrOr("value", ""))
	values := url.Values{button.AttrOr("name", ""): {site}}
	revoke := func(loggedIn bool, body url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, action, strings.NewReader(body.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		setLoggedIn(r, loggedIn)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	revoke(false, values)
	require.Equal(t, http.StatusBadRequest, revoke(true, nil).Code)
	_, err = a.db.oauthVerifyToken("secret-latest")
	require.NoError(t, err)

	w := revoke(true, values)
	require.Equal(t, http.StatusSeeOther, w.Code)
	require.Equal(t, "/blog/settings/connected-apps", w.Header().Get("Location"))
	for _, token := range []string{"secret-first", "secret-latest"} {
		_, err := a.db.oauthVerifyToken(token)
		require.ErrorIs(t, err, errInvalidToken)
	}
	_, err = a.db.oauthVerifyToken("secret-app")
	require.NoError(t, err)
	require.NotContains(t, page(true).Text(), site)
}

func Test_booleanSettingHandlers(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	blog := app.cfg.DefaultBlog
	bc := app.cfg.Blogs[blog]

	// Test hideOldContentWarning handler
	handler := app.settingsHideOldContentWarning()
	require.NotNil(t, handler)

	// Initially false
	require.False(t, bc.hideOldContentWarning)

	// Simulate request
	req := httptest.NewRequest("POST", "/settings/oldcontentwarning", strings.NewReader("hideoldcontentwarning=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	handler(rr, req)

	// Should redirect
	require.Equal(t, 302, rr.Code)
	require.Equal(t, "/settings", rr.Header().Get("Location"))

	// Should have updated the setting
	require.True(t, bc.hideOldContentWarning)
}

func Test_settingsUpdateBlog(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	blog := app.cfg.DefaultBlog
	bc := app.cfg.Blogs[blog]

	// Set initial values
	bc.Title = "Initial Title"
	bc.Description = "Initial Description"
	_ = app.setBlogTitle(blog, bc.Title)
	_ = app.setBlogDescription(blog, bc.Description)

	// Simulate request to update blog settings
	req := httptest.NewRequest("POST", "/settings/blog", strings.NewReader("blogtitle=New+Title&blogdescription=New+Description"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.settingsUpdateBlog(rr, req)

	// Should redirect
	require.Equal(t, 302, rr.Code)
	require.Equal(t, "/settings", rr.Header().Get("Location"))

	// Should have updated the settings
	require.Equal(t, "New Title", bc.Title)
	require.Equal(t, "New Description", bc.Description)

	// Verify database values
	dbTitle, err := app.getBlogTitle(blog)
	require.NoError(t, err)
	require.Equal(t, "New Title", dbTitle)

	dbDescription, err := app.getBlogDescription(blog)
	require.NoError(t, err)
	require.Equal(t, "New Description", dbDescription)
}

func Test_settingsUpdateBlog_EmptyTitle(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	// Simulate request with empty title
	req := httptest.NewRequest("POST", "/settings/blog", strings.NewReader("blogtitle=&blogdescription=Some+Description"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.settingsUpdateBlog(rr, req)

	// Should return error
	require.Equal(t, 400, rr.Code)
}

func Test_settingsUpdateBlog_EmptyDescription(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	blog := app.cfg.DefaultBlog
	bc := app.cfg.Blogs[blog]

	// Set initial values
	bc.Title = "Initial Title"
	bc.Description = "Initial Description"
	_ = app.setBlogTitle(blog, bc.Title)
	_ = app.setBlogDescription(blog, bc.Description)

	// Simulate request with empty description (should be allowed)
	req := httptest.NewRequest("POST", "/settings/blog", strings.NewReader("blogtitle=New+Title&blogdescription="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	app.settingsUpdateBlog(rr, req)

	// Should redirect (empty description is allowed)
	require.Equal(t, 302, rr.Code)

	// Should have updated the settings
	require.Equal(t, "New Title", bc.Title)
	require.Equal(t, "", bc.Description)
}

func Test_settingsUpdateUser(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)
	// Simulate request to update user settings
	req := httptest.NewRequest("POST", "/settings/user", strings.NewReader("usernick=New+Nick&username=New+Name&useremail=new%40example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.settingsUpdateUser(rr, req)

	// Should redirect
	require.Equal(t, 302, rr.Code)
	require.Equal(t, "/settings", rr.Header().Get("Location"))

	// Should have updated the config
	require.Equal(t, "New Nick", app.cfg.User.Nick)
	require.Equal(t, "New Name", app.cfg.User.Name)
	require.Equal(t, "new@example.com", app.cfg.User.Email)

	// Verify database values
	dbEmail, err := app.getSettingValue(userEmailSetting)
	require.NoError(t, err)
	require.Equal(t, "new@example.com", dbEmail)

	// Simulate request to clear the email (should be allowed)
	req = httptest.NewRequest("POST", "/settings/user", strings.NewReader("usernick=New+Nick&username=New+Name&useremail="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.settingsUpdateUser(rr, req)

	require.Equal(t, 302, rr.Code)
	require.Equal(t, "", app.cfg.User.Email)

	// Simulate request with empty nick (should fail)
	req = httptest.NewRequest("POST", "/settings/user", strings.NewReader("usernick=&username=New+Name&useremail="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	app.settingsUpdateUser(rr, req)

	require.Equal(t, 500, rr.Code)
}
