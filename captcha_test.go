package main

import (
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/justinas/alice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
)

func Test_captchaMiddleware(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
	app.initMarkdown()
	app.initSessions()
	_ = app.initTemplateStrings()

	app.d = alice.New(app.checkIsCaptcha, app.captchaMiddleware).ThenFunc(func(rw http.ResponseWriter, r *http.Request) {
		_, _ = rw.Write([]byte("ABC Test"))
	})

	t.Run("Show captcha", func(t *testing.T) {
		rec := httptest.NewRecorder()

		app.d.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/abc", nil))

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
		assert.Contains(t, res.Header.Get("Content-Type"), contenttype.HTML)
		assert.Contains(t, resString, "name=captchamethod value=POST")
	})

	t.Run("Show no captcha, when already solved", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/abc", nil)
		rec1 := httptest.NewRecorder()

		session, _ := app.captchaSessions.Get(req, "c")
		session.Values["captcha"] = true
		_ = session.Save(req, rec1)

		res1 := rec1.Result()
		for _, cookie := range res1.Cookies() {
			req.AddCookie(cookie)
		}
		_ = res1.Body.Close()

		rec2 := httptest.NewRecorder()

		app.d.ServeHTTP(rec2, req)

		res := rec2.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, resString, "ABC Test")
	})

	t.Run("Captcha flow", func(t *testing.T) {
		// Do original request
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/abc", strings.NewReader("test"))

		app.d.ServeHTTP(rec, req)

		// Check response
		res := rec.Result()
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)

		// Check cookie
		cookies := res.Cookies()
		require.Len(t, cookies, 1)
		captchaCookie := cookies[0]
		assert.Equal(t, "c", captchaCookie.Name)
		captchaSessionId := captchaCookie.Value
		assert.NotEmpty(t, captchaSessionId)

		// Check session
		sr := httptest.NewRequest(http.MethodPost, "/abc", strings.NewReader("test"))
		sr.AddCookie(captchaCookie)
		session, err := app.captchaSessions.Get(sr, "c")
		require.NoError(t, err)
		assert.Equal(t, captchaSessionId, session.ID)
		captchaId := session.Values["captchaid"].(string)
		assert.NotEmpty(t, captchaId)
		_, captchaSolved := session.Values["captcha"].(bool)
		assert.False(t, captchaSolved)

		log.Println("Captcha ID:", captchaId)

		// Check form values
		doc, err := goquery.NewDocumentFromReader(res.Body)
		_ = res.Body.Close()
		require.NoError(t, err)
		form := doc.Find("form")
		cm := form.Find("input[name=captchamethod]")
		assert.Equal(t, "POST", cm.AttrOr("value", ""))
		ch := form.Find("input[name=captchaheaders]")
		assert.NotEmpty(t, ch.AttrOr("value", ""))
		cb := form.Find("input[name=captchabody]")
		assert.NotEmpty(t, cb.AttrOr("value", ""))
		dcb, _ := base64.StdEncoding.DecodeString(cb.AttrOr("value", ""))
		assert.Equal(t, "test", string(dcb))
		ci := doc.Find("img.captchaimg")
		assert.Contains(t, ci.AttrOr("src", ""), captchaId)

		// Do second request with wrong captcha
		rec = httptest.NewRecorder()

		formValues := &url.Values{}
		formValues.Add("captchaaction", "captcha")
		formValues.Add("captchamethod", cm.AttrOr("value", ""))
		formValues.Add("captchaheaders", ch.AttrOr("value", ""))
		formValues.Add("captchabody", cb.AttrOr("value", ""))
		formValues.Add("digits", "123456") // Wrong captcha

		req = httptest.NewRequest(http.MethodPost, "/abc", strings.NewReader(formValues.Encode()))
		req.Header.Set(contentType, contenttype.WWWForm)
		req.AddCookie(captchaCookie)

		app.d.ServeHTTP(rec, req)

		// Check response
		res = rec.Result()
		assert.Equal(t, http.StatusUnauthorized, res.StatusCode)

		// Check cookie
		require.Len(t, res.Cookies(), 1)
		assert.Equal(t, captchaSessionId, res.Cookies()[0].Value)

		// Check session
		sr = httptest.NewRequest(http.MethodPost, "/abc", strings.NewReader("test"))
		sr.AddCookie(captchaCookie)
		session, err = app.captchaSessions.Get(sr, "c")
		require.NoError(t, err)
		assert.Equal(t, captchaSessionId, session.ID)
		captchaId = session.Values["captchaid"].(string)
		assert.NotEmpty(t, captchaId)
		_, captchaSolved = session.Values["captcha"].(bool)
		assert.False(t, captchaSolved)

		log.Println("Captcha ID:", captchaId)

		// Check form values
		doc, err = goquery.NewDocumentFromReader(res.Body)
		_ = res.Body.Close()
		require.NoError(t, err)
		ci = doc.Find("img.captchaimg")
		assert.Contains(t, ci.AttrOr("src", ""), captchaId)

		// Solve captcha
		digits := captchaStore.Get(captchaId, false)
		digitsString := ""
		for _, digit := range digits {
			digitsString += strconv.Itoa(int(digit))
		}

		// Do third request with solved captcha
		rec = httptest.NewRecorder()

		formValues = &url.Values{}
		formValues.Add("captchaaction", "captcha")
		formValues.Add("captchamethod", cm.AttrOr("value", ""))
		formValues.Add("captchaheaders", ch.AttrOr("value", ""))
		formValues.Add("captchabody", cb.AttrOr("value", ""))
		formValues.Add("digits", digitsString) // Correct captcha

		req = httptest.NewRequest(http.MethodPost, "/abc", strings.NewReader(formValues.Encode()))
		req.Header.Set(contentType, contenttype.WWWForm)
		req.AddCookie(captchaCookie)

		app.d.ServeHTTP(rec, req)

		// Check response
		res = rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, resString, "ABC Test")
	})

}
