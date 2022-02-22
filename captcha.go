package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dchest/captcha"
	"go.goblog.app/app/pkgs/contenttype"
)

const captchaSolvedKey contextKey = "captchaSolved"

var captchaStore = captcha.NewMemoryStore(100, 10*time.Minute)

func init() {
	captcha.SetCustomStore(captchaStore)
}

func (a *goBlog) captchaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if captcha already solved
		if solved, ok := r.Context().Value(captchaSolvedKey).(bool); ok && solved {
			next.ServeHTTP(w, r)
			return
		}
		// Check session
		ses, err := a.captchaSessions.Get(r, "c")
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		if captcha, ok := ses.Values["captcha"]; ok && captcha == true {
			// Captcha already solved
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), captchaSolvedKey, true)))
			return
		}
		// Get captcha ID
		captchaId := ""
		if sesCaptchaId, ok := ses.Values["captchaid"]; ok {
			// Already has a captcha ID
			ci := sesCaptchaId.(string)
			if captcha.Reload(ci) {
				captchaId = ci
			}
		}
		if captchaId == "" {
			captchaId = captcha.New()
			ses.Values["captchaid"] = captchaId
		}
		// Encode original request
		h, _ := json.Marshal(r.Header)
		b, _ := io.ReadAll(io.LimitReader(r.Body, 20000000)) // Only allow 20 MB
		_ = r.Body.Close()
		if len(b) == 0 {
			// Maybe it's a form
			_ = r.ParseForm()
			b = []byte(r.PostForm.Encode())
		}
		// Render captcha
		_ = ses.Save(r, w)
		w.Header().Set(cacheControl, "no-store,max-age=0")
		a.renderWithStatusCode(w, r, http.StatusUnauthorized, a.renderCaptcha, &renderData{
			Data: &captchaRenderData{
				captchaMethod:  r.Method,
				captchaHeaders: base64.StdEncoding.EncodeToString(h),
				captchaBody:    base64.StdEncoding.EncodeToString(b),
				captchaId:      captchaId,
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
	// Decode and prepare original request
	captchabody, _ := base64.StdEncoding.DecodeString(r.FormValue("captchabody"))
	origReq, _ := http.NewRequestWithContext(r.Context(), r.FormValue("captchamethod"), r.RequestURI, bytes.NewReader(captchabody))
	// Copy original headers
	captchaheaders, _ := base64.StdEncoding.DecodeString(r.FormValue("captchaheaders"))
	var headers http.Header
	_ = json.Unmarshal(captchaheaders, &headers)
	for k, v := range headers {
		origReq.Header[k] = v
	}
	// Get session
	ses, err := a.captchaSessions.Get(r, "c")
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return true
	}
	// Check if session contains a captchaId and if captcha is solved
	if sesCaptchaId, ok := ses.Values["captchaid"]; ok && captcha.VerifyString(sesCaptchaId.(string), r.FormValue("digits")) {
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
		origReq = origReq.WithContext(context.WithValue(origReq.Context(), captchaSolvedKey, true))
	}
	// Copy captcha cookie to original request
	if captchaCookie, err := r.Cookie("c"); err == nil {
		origReq.AddCookie(captchaCookie)
	}
	// Serve original request
	a.d.ServeHTTP(w, origReq)
	return true
}
