package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/pquerna/otp/totp"
	"go.goblog.app/app/pkgs/contenttype"
)

const loggedInKey contextKey = "loggedIn"

// Check if credentials are correct
func (a *goBlog) checkCredentials(username, password, totpPasscode string) bool {
	return username == a.cfg.User.Nick &&
		password == a.cfg.User.Password &&
		(a.cfg.User.TOTP == "" || totp.Validate(totpPasscode, a.cfg.User.TOTP))
}

// Check if app passwords are correct
func (a *goBlog) checkAppPasswords(username, password string) bool {
	for _, apw := range a.cfg.User.AppPasswords {
		if apw.Username == username && apw.Password == password {
			return true
		}
	}
	return false
}

// Check if cookie is known and logged in
func (a *goBlog) checkLoginCookie(r *http.Request) bool {
	ses, err := a.loginSessions.Get(r, "l")
	if err == nil && ses != nil {
		if login, ok := ses.Values["login"]; ok && login.(bool) {
			return true
		}
	}
	return false
}

// Middleware to force login
func (a *goBlog) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if already logged in
		if a.isLoggedIn(r) {
			next.ServeHTTP(w, r)
			return
		}
		// Show login form
		w.Header().Set("Cache-Control", "no-store,max-age=0")
		w.Header().Set("X-Robots-Tag", "noindex")
		h, _ := json.Marshal(r.Header)
		b, _ := io.ReadAll(io.LimitReader(r.Body, 20*1000*1000)) // Only allow 20 MB
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

// Middleware to check if the request is a login request
func (a *goBlog) checkIsLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !a.checkLogin(rw, r) {
			next.ServeHTTP(rw, r)
		}
	})
}

// Checks login and returns true if it already served an error
func (a *goBlog) checkLogin(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	if !strings.Contains(r.Header.Get(contentType), contenttype.WWWForm) {
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
	err = a.loginSessions.Save(r, w, ses)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return true
	}
	// Serve original request
	setLoggedIn(req, true)
	a.d.ServeHTTP(w, req)
	return true
}

func (a *goBlog) isLoggedIn(r *http.Request) bool {
	// Check if context key already set
	if loggedIn, ok := r.Context().Value(loggedInKey).(bool); ok {
		return loggedIn
	}
	// Check app passwords
	if username, password, ok := r.BasicAuth(); ok && a.checkAppPasswords(username, password) {
		setLoggedIn(r, true)
		return true
	}
	// Check session cookie
	if a.checkLoginCookie(r) {
		setLoggedIn(r, true)
		return true
	}
	// Not logged in
	return false
}

// Set request context value
func setLoggedIn(r *http.Request, loggedIn bool) {
	// Overwrite the value of r (r is a pointer)
	(*r) = *(r.WithContext(context.WithValue(r.Context(), loggedInKey, loggedIn)))
}

// HandlerFunc to redirect to home after login
// Need to set auth middleware!
func serveLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

// HandlerFunc to delete login session and cookie
func (a *goBlog) serveLogout(w http.ResponseWriter, r *http.Request) {
	if ses, err := a.loginSessions.Get(r, "l"); err == nil && ses != nil {
		_ = a.loginSessions.Delete(r, w, ses)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
