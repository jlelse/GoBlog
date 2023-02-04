package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/pquerna/otp/totp"
	"go.goblog.app/app/pkgs/bodylimit"
	"go.goblog.app/app/pkgs/bufferpool"
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
			sw, _ := io.WriteString(bodyEncoder, r.Form.Encode())
			written = int64(sw)
		}
		bodyEncoder.Close()
		if written >= limit {
			a.serveError(w, r, "Request body too large, first login", http.StatusRequestEntityTooLarge)
			return
		}
		// Render login form
		w.Header().Set(cacheControl, "no-store,max-age=0")
		w.Header().Set("X-Robots-Tag", "noindex")
		a.render(w, r, a.renderLogin, &renderData{
			Data: &loginRenderData{
				loginMethod:  r.Method,
				loginHeaders: headerBuffer.String(),
				loginBody:    bodyBuffer.String(),
				totp:         a.cfg.User.TOTP != "",
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
	bodyDecoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(r.FormValue("loginbody")))
	origReq, _ := http.NewRequestWithContext(r.Context(), r.FormValue("loginmethod"), r.URL.RequestURI(), bodyDecoder)
	headerDecoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(r.FormValue("loginheaders")))
	_ = json.NewDecoder(headerDecoder).Decode(&origReq.Header)
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
	setLoggedIn(origReq, true)
	a.d.ServeHTTP(w, origReq)
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

func (a *goBlog) getDefaultPostStates(r *http.Request) (status []postStatus, visibility []postVisibility) {
	if a.isLoggedIn(r) {
		status = []postStatus{statusPublished}
		visibility = []postVisibility{visibilityPublic, visibilityUnlisted, visibilityPrivate}
	} else {
		status = []postStatus{statusPublished}
		visibility = []postVisibility{visibilityPublic}
	}
	return
}
