package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pquerna/otp/totp"
)

func checkCredentials(username, password, totpPasscode string) bool {
	return username == appConfig.User.Nick &&
		password == appConfig.User.Password &&
		(appConfig.User.TOTP == "" || totp.Validate(totpPasscode, appConfig.User.TOTP))
}

func checkUsernameTOTP(username string, totp bool) bool {
	return username == appConfig.User.Nick && totp == (appConfig.User.TOTP != "")
}

func checkAppPasswords(username, password string) bool {
	for _, apw := range appConfig.User.AppPasswords {
		if apw.Username == username && apw.Password == password {
			return true
		}
	}
	return false
}

func jwtKey() []byte {
	return []byte(appConfig.Server.JWTSecret)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if already logged in
		if loggedIn, ok := r.Context().Value(loggedInKey).(bool); ok && loggedIn {
			next.ServeHTTP(w, r)
			return
		}
		// 1. Check BasicAuth (just for app passwords)
		if username, password, ok := r.BasicAuth(); ok && checkAppPasswords(username, password) {
			next.ServeHTTP(w, r)
			return
		}
		// 2. Check JWT
		if checkAuthToken(r) {
			next.ServeHTTP(w, r)
			return
		}
		// 3. Show login form
		w.Header().Set("Cache-Control", "no-store,max-age=0")
		h, _ := json.Marshal(r.Header.Clone())
		b, _ := io.ReadAll(io.LimitReader(r.Body, 2000000)) // Only allow 20 Megabyte
		_ = r.Body.Close()
		if len(b) == 0 {
			// Maybe it's a form
			_ = r.ParseForm()
			b = []byte(r.PostForm.Encode())
		}
		render(w, r, templateLogin, &renderData{
			Data: map[string]interface{}{
				"loginmethod":  r.Method,
				"loginheaders": base64.StdEncoding.EncodeToString(h),
				"loginbody":    base64.StdEncoding.EncodeToString(b),
				"totp":         appConfig.User.TOTP != "",
			},
		})
	})
}

func checkAuthToken(r *http.Request) bool {
	if tokenCookie, err := r.Cookie("token"); err == nil {
		claims := &authClaims{}
		if tkn, err := jwt.ParseWithClaims(tokenCookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
			return jwtKey(), nil
		}); err == nil && tkn.Valid &&
			claims.TokenType == "login" &&
			checkUsernameTOTP(claims.Username, claims.TOTP) {
			return true
		}
	}
	return false
}

const loggedInKey requestContextKey = "loggedIn"

func checkLoggedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if checkAuthToken(r) {
			next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), loggedInKey, true)))
			return
		}
		next.ServeHTTP(rw, r)
	})
}

func checkIsLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !checkLogin(rw, r) {
			next.ServeHTTP(rw, r)
		}
	})
}

func checkLogin(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	if r.Header.Get(contentType) != contentTypeWWWForm {
		return false
	}
	if r.FormValue("loginaction") != "login" {
		return false
	}
	// Check credential
	if !checkCredentials(r.FormValue("username"), r.FormValue("password"), r.FormValue("token")) {
		serveError(w, r, "Incorrect credentials", http.StatusUnauthorized)
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
	tokenCookie, err := createTokenCookie()
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return true
	}
	req.AddCookie(tokenCookie)
	http.SetCookie(w, tokenCookie)
	// Serve original request
	d.ServeHTTP(w, req)
	return true
}

type authClaims struct {
	*jwt.StandardClaims
	TokenType string
	Username  string
	TOTP      bool
}

func createTokenCookie() (*http.Cookie, error) {
	expiration := time.Now().Add(7 * 24 * time.Hour)
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &authClaims{
		&jwt.StandardClaims{ExpiresAt: expiration.Unix()},
		"login",
		appConfig.User.Nick,
		appConfig.User.TOTP != "",
	}).SignedString(jwtKey())
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:     "token",
		Value:    tokenString,
		Expires:  expiration,
		Secure:   httpsConfigured(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}, nil
}

// Need to set auth middleware!
func serveLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)
}

func serveLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		MaxAge:   -1,
		Secure:   httpsConfigured(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}
