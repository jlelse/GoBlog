package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"github.com/pquerna/otp/totp"
	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) checkCredentials(username, password, totpPasscode string) bool {
	return username == a.cfg.User.Nick &&
		password == a.cfg.User.Password &&
		(a.cfg.User.TOTP == "" || totp.Validate(totpPasscode, a.cfg.User.TOTP))
}

func (a *goBlog) checkAppPasswords(username, password string) bool {
	for _, apw := range a.cfg.User.AppPasswords {
		if apw.Username == username && apw.Password == password {
			return true
		}
	}
	return false
}

func (a *goBlog) jwtKey() []byte {
	return []byte(a.cfg.Server.JWTSecret)
}

func (a *goBlog) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check if already logged in
		if loggedIn, ok := r.Context().Value(loggedInKey).(bool); ok && loggedIn {
			next.ServeHTTP(w, r)
			return
		}
		// 2. Check BasicAuth (just for app passwords)
		if username, password, ok := r.BasicAuth(); ok && a.checkAppPasswords(username, password) {
			next.ServeHTTP(w, r)
			return
		}
		// 3. Check login cookie
		if a.checkLoginCookie(r) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), loggedInKey, true)))
			return
		}
		// 4. Show login form
		w.Header().Set("Cache-Control", "no-store,max-age=0")
		h, _ := json.Marshal(r.Header.Clone())
		b, _ := io.ReadAll(io.LimitReader(r.Body, 2000000)) // Only allow 20 Megabyte
		_ = r.Body.Close()
		if len(b) == 0 {
			// Maybe it's a form
			_ = r.ParseForm()
			b = []byte(r.PostForm.Encode())
		}
		a.render(w, r, templateLogin, &renderData{
			Data: map[string]interface{}{
				"loginmethod":  r.Method,
				"loginheaders": base64.StdEncoding.EncodeToString(h),
				"loginbody":    base64.StdEncoding.EncodeToString(b),
				"totp":         a.cfg.User.TOTP != "",
			},
		})
	})
}

const loggedInKey contextKey = "loggedIn"

func (a *goBlog) checkLoggedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if a.checkLoginCookie(r) {
			next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), loggedInKey, true)))
			return
		}
		next.ServeHTTP(rw, r)
	})
}

func (a *goBlog) checkLoginCookie(r *http.Request) bool {
	ses, err := a.loginSessions.Get(r, "l")
	if err == nil && ses != nil {
		if login, ok := ses.Values["login"]; ok && login.(bool) {
			return true
		}
	}
	return false
}

func (a *goBlog) checkIsLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !a.checkLogin(rw, r) {
			next.ServeHTTP(rw, r)
		}
	})
}

func (a *goBlog) checkLogin(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	if r.Header.Get(contentType) != contenttype.WWWForm {
		return false
	}
	if r.FormValue("loginaction") != "login" {
		return false
	}
	// Check credential
	if !a.checkCredentials(r.FormValue("username"), r.FormValue("password"), r.FormValue("token")) {
		a.serveError(w, r, "Incorrect credentials", http.StatusUnauthorized)
		return true
	}
	// Prepare original request
	loginbody, _ := base64.StdEncoding.DecodeString(r.FormValue("loginbody"))
	req, _ := http.NewRequest(r.FormValue("loginmethod"), r.RequestURI, bytes.NewReader(loginbody))
	// Copy original headers
	loginheaders, _ := base64.StdEncoding.DecodeString(r.FormValue("loginheaders"))
	var headers http.Header
	_ = json.Unmarshal(loginheaders, &headers)
	for k, v := range headers {
		req.Header[k] = v
	}
	// Cookie
	ses, err := a.loginSessions.Get(r, "l")
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return true
	}
	ses.Values["login"] = true
	cookie, err := a.loginSessions.SaveGetCookie(r, w, ses)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return true
	}
	req.AddCookie(cookie)
	// Serve original request
	a.d.ServeHTTP(w, req)
	return true
}

// Need to set auth middleware!
func serveLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *goBlog) serveLogout(w http.ResponseWriter, r *http.Request) {
	if ses, err := a.loginSessions.Get(r, "l"); err == nil && ses != nil {
		_ = a.loginSessions.Delete(r, w, ses)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
