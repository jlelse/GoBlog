package main

import (
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cast"
)

// https://www.w3.org/TR/indieauth/

type indieAuthData struct {
	Me           string
	ClientID     string
	RedirectURI  string
	State        string
	ResponseType string
	Scopes       []string
	code         string
	token        string
	time         time.Time
}

func indieAuthAuthGet(w http.ResponseWriter, r *http.Request) {
	// Authentication / authorization request
	r.ParseForm()
	data := &indieAuthData{
		Me:           r.Form.Get("me"),
		ClientID:     r.Form.Get("client_id"),
		RedirectURI:  r.Form.Get("redirect_uri"),
		State:        r.Form.Get("state"),
		ResponseType: r.Form.Get("response_type"),
	}
	if data.ResponseType == "" {
		data.ResponseType = "id"
	}
	if scope := r.Form.Get("scope"); scope != "" {
		data.Scopes = strings.Split(scope, " ")
	}
	if !isValidProfileURL(data.Me) || !isValidProfileURL(data.ClientID) || !isValidProfileURL(data.RedirectURI) {
		http.Error(w, "me, client_id and redirect_uri need to by valid URLs", http.StatusBadRequest)
		return
	}
	if data.State == "" {
		http.Error(w, "state must not be empty", http.StatusBadRequest)
		return
	}
	if data.ResponseType != "id" && data.ResponseType != "code" {
		http.Error(w, "response_type must be empty or id or code", http.StatusBadRequest)
		return
	}
	if data.ResponseType == "code" && len(data.Scopes) < 1 {
		http.Error(w, "scope is missing or empty", http.StatusBadRequest)
		return
	}
	render(w, "indieauth", &renderData{
		Data: data,
	})
}

func indieAuthAuthPost(w http.ResponseWriter, r *http.Request) {
	// Authentication verification
	r.ParseForm()
	data := &indieAuthData{
		code:        r.Form.Get("code"),
		ClientID:    r.Form.Get("client_id"),
		RedirectURI: r.Form.Get("redirect_uri"),
	}
	valid, err := data.verifyAuthorization(true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !valid {
		http.Error(w, "Authentication not valid", http.StatusForbidden)
		return
	}
	res := &tokenResponse{
		Me: data.Me,
	}
	w.Header().Add(contentType, contentTypeJSONUTF8)
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		w.Header().Del(contentType)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func isValidProfileURL(profileURL string) bool {
	u, err := url.Parse(profileURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	// Missing: Check path
	// Missing: Check single/double dot path
	if u.Fragment != "" {
		return false
	}
	if u.User.String() != "" {
		return false
	}
	if u.Port() != "" {
		return false
	}
	// Missing: Check domain / ip
	return true
}

func indieAuthAccept(w http.ResponseWriter, r *http.Request) {
	// Authentication flow
	r.ParseForm()
	data := &indieAuthData{
		Me:           r.Form.Get("me"),
		ClientID:     r.Form.Get("client_id"),
		RedirectURI:  r.Form.Get("redirect_uri"),
		State:        r.Form.Get("state"),
		ResponseType: r.Form.Get("response_type"),
		Scopes:       r.Form["scopes"],
		time:         time.Now(),
	}
	sha := sha1.New()
	sha.Write([]byte(data.time.String() + data.Me + data.ClientID))
	data.code = fmt.Sprintf("%x", sha.Sum(nil))
	err := data.saveAuthorization()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, data.RedirectURI+"?code="+data.code+"&state="+data.State, http.StatusFound)
}

type tokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Me          string `json:"me,omitempty"`
	ClientID    string `json:"client_id,omitempty"`
}

func indieAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Token verification
		data, err := verifyIndieAuthToken(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, "Invalid token or token not found", http.StatusUnauthorized)
		}
		res := &tokenResponse{
			Scope:    strings.Join(data.Scopes, " "),
			Me:       data.Me,
			ClientID: data.ClientID,
		}
		w.Header().Add(contentType, contentTypeJSONUTF8)
		err = json.NewEncoder(w).Encode(res)
		if err != nil {
			w.Header().Del(contentType)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		r.ParseForm()
		// Token Revocation
		if r.Form.Get("action") == "revoke" {
			revokeIndieAuthToken(r.Form.Get("token"))
			w.WriteHeader(http.StatusOK)
			return
		}
		// Token request
		if r.Form.Get("grant_type") == "authorization_code" {
			data := &indieAuthData{
				code:        r.Form.Get("code"),
				ClientID:    r.Form.Get("client_id"),
				RedirectURI: r.Form.Get("redirect_uri"),
				Me:          r.Form.Get("me"),
			}
			valid, err := data.verifyAuthorization(false)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if !valid {
				http.Error(w, "Authentication not valid", http.StatusForbidden)
				return
			}
			if len(data.Scopes) < 1 {
				http.Error(w, "No scope", http.StatusBadRequest)
				return
			}
			data.time = time.Now()
			sha := sha1.New()
			sha.Write([]byte(data.time.String() + data.Me + data.ClientID))
			data.token = fmt.Sprintf("%x", sha.Sum(nil))
			err = data.saveToken()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			res := &tokenResponse{
				TokenType:   "Bearer",
				AccessToken: data.token,
				Scope:       strings.Join(data.Scopes, " "),
				Me:          data.Me,
			}
			w.Header().Add(contentType, contentTypeJSONUTF8)
			err = json.NewEncoder(w).Encode(res)
			if err != nil {
				w.Header().Del(contentType)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
}

func (data *indieAuthData) saveAuthorization() (err error) {
	_, err = appDbExec("insert into indieauthauth (time, code, me, client, redirect, scope) values (?, ?, ?, ?, ?, ?)", data.time.Unix(), data.code, data.Me, data.ClientID, data.RedirectURI, strings.Join(data.Scopes, " "))
	return
}

func (data *indieAuthData) verifyAuthorization(authentication bool) (valid bool, err error) {
	// code valid for 600 seconds
	if !authentication {
		row, err := appDbQueryRow("select code, me, client, redirect, scope from indieauthauth where time >= ? and code = ? and me = ? and client = ? and redirect = ?", time.Now().Unix()-600, data.code, data.Me, data.ClientID, data.RedirectURI)
		if err != nil {
			return false, err
		}
		scope := ""
		err = row.Scan(&data.code, &data.Me, &data.ClientID, &data.RedirectURI, &scope)
		if err == sql.ErrNoRows {
			return false, nil
		} else if err != nil {
			return false, err
		}
		if scope != "" {
			data.Scopes = strings.Split(scope, " ")
		}
	} else {
		row, err := appDbQueryRow("select code, me, client, redirect from indieauthauth where time >= ? and code = ? and client = ? and redirect = ?", time.Now().Unix()-600, data.code, data.ClientID, data.RedirectURI)
		if err != nil {
			return false, err
		}
		err = row.Scan(&data.code, &data.Me, &data.ClientID, &data.RedirectURI)
		if err == sql.ErrNoRows {
			return false, nil
		} else if err != nil {
			return false, err
		}
	}
	valid = true
	_, err = appDbExec("delete from indieauthauth where code = ? or time < ?", data.code, time.Now().Unix()-600)
	data.code = ""
	return
}

func (data *indieAuthData) saveToken() (err error) {
	_, err = appDbExec("insert into indieauthtoken (time, token, me, client, scope) values (?, ?, ?, ?, ?)", data.time.Unix(), data.token, data.Me, data.ClientID, strings.Join(data.Scopes, " "))
	return
}

func verifyIndieAuthToken(token string) (data *indieAuthData, err error) {
	token = strings.ReplaceAll(token, "Bearer ", "")
	data = &indieAuthData{
		Scopes: []string{},
	}
	row, err := appDbQueryRow("select time, token, me, client, scope from indieauthtoken where token = @token", sql.Named("token", token))
	if err != nil {
		return nil, err
	}
	timeString := ""
	scope := ""
	err = row.Scan(&timeString, &data.token, &data.Me, &data.ClientID, &scope)
	if err == sql.ErrNoRows {
		return nil, errors.New("token not found")
	} else if err != nil {
		return nil, err
	}
	if scope != "" {
		data.Scopes = strings.Split(scope, " ")
	}
	data.time = time.Unix(cast.ToInt64(timeString), 0)
	return
}

func revokeIndieAuthToken(token string) {
	if token == "" {
		return
	}
	_, _ = appDbExec("delete from indieauthtoken where token=?", token)
	return
}
