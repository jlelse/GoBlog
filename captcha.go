package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"github.com/dchest/captcha"
	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) captchaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check Cookie
		ses, err := a.captchaSessions.Get(r, "c")
		if err == nil && ses != nil {
			if captcha, ok := ses.Values["captcha"]; ok && captcha.(bool) {
				next.ServeHTTP(w, r)
				return
			}
		}
		// 2. Show Captcha
		h, _ := json.Marshal(r.Header.Clone())
		b, _ := io.ReadAll(io.LimitReader(r.Body, 2000000)) // Only allow 20 Megabyte
		_ = r.Body.Close()
		if len(b) == 0 {
			// Maybe it's a form
			_ = r.ParseForm()
			b = []byte(r.PostForm.Encode())
		}
		a.renderWithStatusCode(w, r, http.StatusUnauthorized, templateCaptcha, &renderData{
			Data: map[string]string{
				"captchamethod":  r.Method,
				"captchaheaders": base64.StdEncoding.EncodeToString(h),
				"captchabody":    base64.StdEncoding.EncodeToString(b),
				"captchaid":      captcha.New(),
			},
		})
	})
}

func (a *goBlog) checkIsCaptcha(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !a.checkCaptcha(rw, r) {
			next.ServeHTTP(rw, r)
		}
	})
}

func (a *goBlog) checkCaptcha(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	if r.Header.Get(contentType) != contenttype.WWWForm {
		return false
	}
	if r.FormValue("captchaaction") != "captcha" {
		return false
	}
	// Prepare original request
	captchabody, _ := base64.StdEncoding.DecodeString(r.FormValue("captchabody"))
	req, _ := http.NewRequest(r.FormValue("captchamethod"), r.RequestURI, bytes.NewReader(captchabody))
	// Copy original headers
	captchaheaders, _ := base64.StdEncoding.DecodeString(r.FormValue("captchaheaders"))
	var headers http.Header
	_ = json.Unmarshal(captchaheaders, &headers)
	for k, v := range headers {
		req.Header[k] = v
	}
	// Check captcha and create cookie
	if captcha.VerifyString(r.FormValue("captchaid"), r.FormValue("digits")) {
		ses, err := a.captchaSessions.Get(r, "c")
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return true
		}
		ses.Values["captcha"] = true
		cookie, err := a.captchaSessions.SaveGetCookie(r, w, ses)
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return true
		}
		req.AddCookie(cookie)
	}
	// Serve original request
	a.d.ServeHTTP(w, req)
	return true
}
