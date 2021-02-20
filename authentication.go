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
)

func checkCredentials(username, password string) bool {
	return username == appConfig.User.Nick && password == appConfig.User.Password
}

func checkUsername(username string) bool {
	return username == appConfig.User.Nick
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
		// 1. Check BasicAuth
		if username, password, ok := r.BasicAuth(); ok && checkCredentials(username, password) {
			next.ServeHTTP(w, r)
			return
		}
		// 2. Check JWT
		if checkAuthToken(r) {
			next.ServeHTTP(w, r)
			return
		}
		// 3. Show login form
		w.WriteHeader(http.StatusUnauthorized)
		h, _ := json.Marshal(r.Header.Clone())
		b, _ := io.ReadAll(io.LimitReader(r.Body, 2000000)) // Only allow 20 Megabyte
		_ = r.Body.Close()
		if len(b) == 0 {
			// Maybe it's a form
			_ = r.ParseForm()
			b = []byte(r.PostForm.Encode())
		}
		render(w, r, templateLogin, &renderData{
			Data: map[string]string{
				"loginmethod":  r.Method,
				"loginheaders": base64.StdEncoding.EncodeToString(h),
				"loginbody":    base64.StdEncoding.EncodeToString(b),
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
			checkUsername(claims.Username) {
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
	if r.Method == http.MethodPost &&
		r.Header.Get(contentType) == contentTypeWWWForm &&
		r.FormValue("loginaction") == "login" {
		// Do original request
		loginbody, _ := base64.StdEncoding.DecodeString(r.FormValue("loginbody"))
		req, _ := http.NewRequest(r.FormValue("loginmethod"), r.RequestURI, bytes.NewReader(loginbody))
		// Copy original headers
		loginheaders, _ := base64.StdEncoding.DecodeString(r.FormValue("loginheaders"))
		var headers http.Header
		_ = json.Unmarshal(loginheaders, &headers)
		for k, v := range headers {
			req.Header[k] = v
		}
		// Check credential
		if checkCredentials(r.FormValue("username"), r.FormValue("password")) {
			tokenCookie, err := createTokenCookie(r.FormValue("username"))
			if err != nil {
				serveError(w, r, err.Error(), http.StatusInternalServerError)
				return true
			}
			// Add cookie to original request
			req.AddCookie(tokenCookie)
			// Send cookie
			http.SetCookie(w, tokenCookie)
		}
		// Serve original request
		d.ServeHTTP(w, req)
		return true
	}
	return false
}

type authClaims struct {
	*jwt.StandardClaims
	TokenType string
	Username  string
}

func createTokenCookie(username string) (*http.Cookie, error) {
	expiration := time.Now().Add(7 * 24 * time.Hour)
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &authClaims{
		&jwt.StandardClaims{ExpiresAt: expiration.Unix()},
		"login",
		username,
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
