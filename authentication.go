package main

import (
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
)

func checkCredentials(username, password string) bool {
	return username == appConfig.User.Nick && password == appConfig.User.Password
}

func jwtKey() []byte {
	return []byte(appConfig.Server.JWTSecret)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check basic auth
		username, password, basicauth := r.BasicAuth()
		if basicauth && checkCredentials(username, password) {
			next.ServeHTTP(w, r)
			return
		}
		// 2. Check JWT
		if tokenCookie, err := r.Cookie("token"); err == nil {
			if tkn, err := jwt.Parse(tokenCookie.Value, func(t *jwt.Token) (interface{}, error) {
				return jwtKey(), nil
			}); err == nil && tkn.Valid {
				next.ServeHTTP(w, r)
				return
			}
		}
		// 3. Show login form
		w.WriteHeader(http.StatusUnauthorized)
		h, _ := json.Marshal(r.Header.Clone())
		b, _ := ioutil.ReadAll(io.LimitReader(r.Body, 2000000)) // Only allow 20 Megabyte
		_ = r.Body.Close()
		if len(b) == 0 {
			// Maybe it's a form
			_ = r.ParseForm()
			b = []byte(r.PostForm.Encode())
		}
		render(w, templateLogin, &renderData{
			Data: map[string]string{
				"loginmethod":  r.Method,
				"loginheaders": base64.StdEncoding.EncodeToString(h),
				"loginbody":    base64.StdEncoding.EncodeToString(b),
			},
		})
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
		r.FormValue("loginaction") == "login" &&
		checkCredentials(r.FormValue("username"), r.FormValue("password")) {
		// Do original request
		loginbody, _ := base64.StdEncoding.DecodeString(r.FormValue("loginbody"))
		req, _ := http.NewRequest(r.FormValue("loginmethod"), r.RequestURI, bytes.NewReader(loginbody))
		// Copy original headers
		loginheaders, _ := base64.StdEncoding.DecodeString(r.FormValue("loginheaders"))
		var headers http.Header
		json.Unmarshal(loginheaders, &headers)
		for k, v := range headers {
			req.Header[k] = v
		}
		// Set basic auth
		req.SetBasicAuth(r.FormValue("username"), r.FormValue("password"))
		// Send cookie
		sendTokenCookie(w, r)
		// Serve original request
		d.ServeHTTP(w, req)
		return true
	}
	return false
}

func sendTokenCookie(w http.ResponseWriter, r *http.Request) {
	expiration := time.Now().Add(7 * 24 * time.Hour)
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.StandardClaims{ExpiresAt: expiration.Unix()}).SignedString(jwtKey())
	if err != nil {
		serveError(w, r, "Failed to sign JWT", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    tokenString,
		Expires:  expiration,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	return
}
