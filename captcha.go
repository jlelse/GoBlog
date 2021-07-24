package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/dchest/captcha"
	"go.goblog.app/app/pkgs/contenttype"
)

const captchaSolvedKey contextKey = "captchaSolved"

func (a *goBlog) captchaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if captcha already solved
		if solved, ok := r.Context().Value(captchaSolvedKey).(bool); ok && solved {
			next.ServeHTTP(w, r)
			return
		}
		// Check Cookie
		ses, err := a.captchaSessions.Get(r, "c")
		if err == nil && ses != nil {
			if captcha, ok := ses.Values["captcha"]; ok && captcha.(bool) {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), captchaSolvedKey, true)))
				return
			}
		}
		// Show Captcha
		w.Header().Set("Cache-Control", "no-store,max-age=0")
		h, _ := json.Marshal(r.Header)
		b, _ := io.ReadAll(io.LimitReader(r.Body, 20*1000*1000)) // Only allow 20 MB
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
	if !strings.Contains(r.Header.Get(contentType), contenttype.WWWForm) {
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
		err = a.captchaSessions.Save(r, w, ses)
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return true
		}
		req = req.WithContext(context.WithValue(req.Context(), captchaSolvedKey, true))
	}
	// Serve original request
	a.d.ServeHTTP(w, req)
	return true
}
