package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type indieAuthTokenResponse struct {
	Me               string `json:"me"`
	ClientID         string `json:"client_id"`
	Scope            string `json:"scope"`
	IssuedBy         string `json:"issued_by"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	StatusCode       int
}

func checkIndieAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearerToken := r.Header.Get("Authorization")
		if len(bearerToken) == 0 {
			if accessToken := r.URL.Query().Get("access_token"); len(accessToken) > 0 {
				bearerToken = "Bearer " + accessToken
			}
		}
		tokenResponse, err := verifyIndieAuthAccessToken(bearerToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if tokenResponse.StatusCode != http.StatusOK {
			http.Error(w, "Failed to retrieve authentication information", http.StatusUnauthorized)
			return
		}
		authorized := false
		for _, allowed := range appConfig.Micropub.AuthAllowed {
			if err := compareHostnames(tokenResponse.Me, allowed); err == nil {
				authorized = true
				break
			}
		}
		if !authorized {
			http.Error(w, "Forbidden", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
		return
	})
}

func verifyIndieAuthAccessToken(bearerToken string) (*indieAuthTokenResponse, error) {
	if len(bearerToken) == 0 {
		return nil, errors.New("no token")
	}
	req, err := http.NewRequest("GET", appConfig.Micropub.TokenEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add(contentType, contentTypeWWWForm)
	req.Header.Add("Authorization", bearerToken)
	req.Header.Add("Accept", contentTypeJSON)
	c := http.Client{
		Timeout: time.Duration(10 * time.Second),
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	tokenRes := indieAuthTokenResponse{StatusCode: resp.StatusCode}
	err = json.NewDecoder(resp.Body).Decode(&tokenRes)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return &tokenRes, nil
}

func compareHostnames(a string, allowed string) error {
	h1, err := url.Parse(a)
	if err != nil {
		return err
	}
	if strings.ToLower(h1.Hostname()) != strings.ToLower(allowed) {
		return fmt.Errorf("hostnames do not match, %s is not %s", h1, allowed)
	}
	return nil
}
