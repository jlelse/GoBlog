package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/dchest/captcha"
	"github.com/dgrijalva/jwt-go"
)

func captchaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check JWT
		claims := &captchaClaims{}
		if captchaCookie, err := r.Cookie("captcha"); err == nil {
			if tkn, err := jwt.ParseWithClaims(captchaCookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
				return jwtKey(), nil
			}); err == nil && tkn.Valid && claims.TokenType == "captcha" {
				next.ServeHTTP(w, r)
				return
			}
		}
		// 2. Show Captcha
		w.WriteHeader(http.StatusUnauthorized)
		h, _ := json.Marshal(r.Header.Clone())
		b, _ := ioutil.ReadAll(io.LimitReader(r.Body, 2000000)) // Only allow 20 Megabyte
		_ = r.Body.Close()
		if len(b) == 0 {
			// Maybe it's a form
			_ = r.ParseForm()
			b = []byte(r.PostForm.Encode())
		}
		render(w, templateCaptcha, &renderData{
			Data: map[string]string{
				"captchamethod":  r.Method,
				"captchaheaders": base64.StdEncoding.EncodeToString(h),
				"captchabody":    base64.StdEncoding.EncodeToString(b),
				"captchaid":      captcha.New(),
			},
		})
	})
}

func checkIsCaptcha(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !checkCaptcha(rw, r) {
			next.ServeHTTP(rw, r)
		}
	})
}

func checkCaptcha(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodPost &&
		r.Header.Get(contentType) == contentTypeWWWForm &&
		r.FormValue("captchaaction") == "captcha" {
		// Do original request
		captchabody, _ := base64.StdEncoding.DecodeString(r.FormValue("captchabody"))
		req, _ := http.NewRequest(r.FormValue("captchamethod"), r.RequestURI, bytes.NewReader(captchabody))
		// Copy original headers
		captchaheaders, _ := base64.StdEncoding.DecodeString(r.FormValue("captchaheaders"))
		var headers http.Header
		_ = json.Unmarshal(captchaheaders, &headers)
		for k, v := range headers {
			req.Header[k] = v
		}
		// Check captcha
		if captcha.VerifyString(r.FormValue("captchaid"), r.FormValue("digits")) {
			// Create cookie
			captchaCookie, err := createCaptchaCookie()
			if err != nil {
				serveError(w, r, err.Error(), http.StatusInternalServerError)
				return true
			}
			// Add cookie to original request
			req.AddCookie(captchaCookie)
			// Send cookie
			http.SetCookie(w, captchaCookie)
		}
		// Serve original request
		d.ServeHTTP(w, req)
		return true
	}
	return false
}

type captchaClaims struct {
	*jwt.StandardClaims
	TokenType string
}

func createCaptchaCookie() (*http.Cookie, error) {
	expiration := time.Now().Add(24 * time.Hour)
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &captchaClaims{
		&jwt.StandardClaims{ExpiresAt: expiration.Unix()},
		"captcha",
	}).SignedString(jwtKey())
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:     "captcha",
		Value:    tokenString,
		Expires:  expiration,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}, nil
}
