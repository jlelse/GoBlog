package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dchest/captcha"
	"go.goblog.app/app/pkgs/bodylimit"
	"go.goblog.app/app/pkgs/bufferpool"
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
		if captchaVal, ok := ses.Values["captcha"]; ok && captchaVal == true {
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
		headerBuffer, bodyBuffer := bufferpool.Get(), bufferpool.Get()
		defer bufferpool.Put(headerBuffer, bodyBuffer)
		// Encode headers
		headerEncoder := base64.NewEncoder(base64.StdEncoding, headerBuffer)
		_ = json.NewEncoder(headerEncoder).Encode(r.Header)
		_ = headerEncoder.Close()
		// Encode body
		bodyEncoder := base64.NewEncoder(base64.StdEncoding, bodyBuffer)
		limit := 3 * bodylimit.MB
		written, _ := io.Copy(bodyEncoder, io.LimitReader(r.Body, limit))
		if written == 0 {
			// Maybe it's a form
			_ = r.ParseForm()
			// Encode form
			written = int64(noError(io.WriteString(bodyEncoder, r.Form.Encode())))
		}
		_ = bodyEncoder.Close()
		if written >= limit {
			a.serveError(w, r, "Request body too large, first login", http.StatusRequestEntityTooLarge)
			return
		}
		// Render captcha
		_ = ses.Save(r, w)
		w.Header().Set(cacheControl, "no-store,max-age=0")
		a.renderWithStatusCode(w, r, http.StatusUnauthorized, a.renderCaptcha, &renderData{
			Data: &captchaRenderData{
				captchaMethod:  r.Method,
				captchaHeaders: headerBuffer.String(),
				captchaBody:    bodyBuffer.String(),
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
	// Prepare original request
	bodyDecoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(r.FormValue("captchabody")))
	origReq, _ := http.NewRequestWithContext(r.Context(), r.FormValue("captchamethod"), r.URL.RequestURI(), bodyDecoder)
	headerDecoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(r.FormValue("captchaheaders")))
	_ = json.NewDecoder(headerDecoder).Decode(&origReq.Header)
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
