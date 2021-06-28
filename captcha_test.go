package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.goblog.app/app/pkgs/contenttype"
)

func Test_captchaMiddleware(t *testing.T) {
	app := &goBlog{
		cfg: &config{
			Server: &configServer{
				PublicAddress: "https://example.com",
			},
			Blogs: map[string]*configBlog{
				"en": {
					Lang: "en",
				},
			},
			DefaultBlog: "en",
			User:        &configUser{},
		},
	}

	app.setInMemoryDatabase()
	app.initSessions()
	_ = app.initTemplateStrings()
	_ = app.initRendering()

	h := app.captchaMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_, _ = rw.Write([]byte("ABC Test"))
	}))

	t.Run("Default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/abc", nil)

		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
		assert.Contains(t, res.Header.Get("Content-Type"), contenttype.HTML)
		assert.Contains(t, resString, "name=captchamethod value=POST")
	})

	t.Run("Captcha session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/abc", nil)
		rec1 := httptest.NewRecorder()

		session, _ := app.captchaSessions.Get(req, "c")
		session.Values["captcha"] = true
		_ = session.Save(req, rec1)

		for _, cookie := range rec1.Result().Cookies() {
			req.AddCookie(cookie)
		}

		rec2 := httptest.NewRecorder()

		h.ServeHTTP(rec2, req)

		res := rec2.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, resString, "ABC Test")
	})
}
