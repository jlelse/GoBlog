package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/justinas/alice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
)

func Test_authMiddleware(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.User = &configUser{
		Nick:     "test",
		Password: "pass",
		AppPasswords: []*configAppPassword{
			{
				Username: "app1",
				Password: "pass1",
			},
		},
	}

	_ = app.initConfig(false)
	app.initMarkdown()
	app.initSessions()
	_ = app.initTemplateStrings()

	app.d = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_, _ = rw.Write([]byte("ABC Test"))
		if app.isLoggedIn(r) {
			_, _ = rw.Write([]byte("Logged in"))
		}
	})

	h := alice.New(app.checkIsLogin, app.authMiddleware).Then(app.d)

	t.Run("Login required", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/abc", nil)

		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		// Login form
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, res.Header.Get("Content-Type"), contenttype.HTML)
		assert.Contains(t, resString, "name=loginmethod value=POST")
	})

	t.Run("Login with app password", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/abc", nil)

		req.SetBasicAuth("app1", "pass1")

		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, resString, "ABC Test")
		assert.Contains(t, resString, "Logged in")
	})

	t.Run("Login with wrong app password", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/abc", nil)

		req.SetBasicAuth("app1", "wrong")

		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		// Login form
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, res.Header.Get("Content-Type"), contenttype.HTML)
		assert.Contains(t, resString, "name=loginmethod value=POST")
	})

	t.Run("Login with form", func(t *testing.T) {
		// First request

		data := url.Values{}
		data.Add("loginaction", "login")
		data.Add("loginmethod", "GET")
		data.Add("username", "test")
		data.Add("password", "pass")

		req := httptest.NewRequest(http.MethodPost, "/abc", strings.NewReader(data.Encode()))
		req.Header.Add(contentType, contenttype.WWWForm)

		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, resString, "ABC Test")
		assert.Contains(t, resString, "Logged in")

		// Second request using cookie

		require.Len(t, res.Cookies(), 1)

		req = httptest.NewRequest(http.MethodPost, "/abc", nil)
		req.AddCookie(res.Cookies()[0])

		rec = httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		res = rec.Result()
		resBody, _ = io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString = string(resBody)

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, resString, "ABC Test")
		assert.Contains(t, resString, "Logged in")
	})

	t.Run("Login with wrong credentials", func(t *testing.T) {
		data := url.Values{}
		data.Add("loginaction", "login")
		data.Add("loginmethod", "GET")
		data.Add("username", "test")
		data.Add("password", "wrong")

		req := httptest.NewRequest(http.MethodPost, "/abc", strings.NewReader(data.Encode()))
		req.Header.Add("Content-Type", contenttype.WWWForm)

		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
		assert.Contains(t, res.Header.Get("Content-Type"), contenttype.HTML)
		assert.Contains(t, resString, "Incorrect credentials")
	})
}

func Test_setLoggedIn(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/abc", nil)
	setLoggedIn(req, true)
	loggedIn, ok := req.Context().Value(loggedInKey).(bool)
	assert.True(t, ok)
	assert.True(t, loggedIn)

	req = httptest.NewRequest(http.MethodGet, "/abc", nil)
	setLoggedIn(req, false)
	loggedIn, ok = req.Context().Value(loggedInKey).(bool)
	assert.True(t, ok)
	assert.False(t, loggedIn)
}
