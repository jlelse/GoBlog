package main

import (
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cast"
	"go.goblog.app/app/pkgs/contenttype"
)

// https://www.w3.org/TR/indieauth/
// https://indieauth.spec.indieweb.org/

type indieAuthData struct {
	ClientID    string
	RedirectURI string
	State       string
	Scopes      []string
	code        string
	token       string
	time        time.Time
}

func (a *goBlog) indieAuthRequest(w http.ResponseWriter, r *http.Request) {
	// Authorization request
	if err := r.ParseForm(); err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	data := &indieAuthData{
		ClientID:    r.Form.Get("client_id"),
		RedirectURI: r.Form.Get("redirect_uri"),
		State:       r.Form.Get("state"),
	}
	if rt := r.Form.Get("response_type"); rt != "code" && rt != "id" && rt != "" {
		a.serveError(w, r, "response_type must be code", http.StatusBadRequest)
		return
	}
	if scope := r.Form.Get("scope"); scope != "" {
		data.Scopes = strings.Split(scope, " ")
	}
	if !isValidProfileURL(data.ClientID) || !isValidProfileURL(data.RedirectURI) {
		a.serveError(w, r, "client_id and redirect_uri need to by valid URLs", http.StatusBadRequest)
		return
	}
	if data.State == "" {
		a.serveError(w, r, "state must not be empty", http.StatusBadRequest)
		return
	}
	a.render(w, r, "indieauth", &renderData{
		Data: data,
	})
}

func isValidProfileURL(profileURL string) bool {
	u, err := url.Parse(profileURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
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

func (a *goBlog) indieAuthAccept(w http.ResponseWriter, r *http.Request) {
	// Authentication flow
	if err := r.ParseForm(); err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	data := &indieAuthData{
		ClientID:    r.Form.Get("client_id"),
		RedirectURI: r.Form.Get("redirect_uri"),
		State:       r.Form.Get("state"),
		Scopes:      r.Form["scopes"],
		time:        time.Now().UTC(),
	}
	sha := sha1.New()
	if _, err := sha.Write([]byte(data.time.Format(time.RFC3339) + data.ClientID)); err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	data.code = fmt.Sprintf("%x", sha.Sum(nil))
	err := a.db.saveAuthorization(data)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
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

func (a *goBlog) indieAuthVerification(w http.ResponseWriter, r *http.Request) {
	// Authorization verification
	if err := r.ParseForm(); err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	data := &indieAuthData{
		code:        r.Form.Get("code"),
		ClientID:    r.Form.Get("client_id"),
		RedirectURI: r.Form.Get("redirect_uri"),
	}
	valid, err := a.db.verifyAuthorization(data)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	if !valid {
		a.serveError(w, r, "Authentication not valid", http.StatusForbidden)
		return
	}
	b, _ := json.Marshal(tokenResponse{
		Me: a.getFullAddress("") + "/", // MUST contain a path component / trailing slash
	})
	w.Header().Set(contentType, contenttype.JSONUTF8)
	_, _ = a.min.Write(w, contenttype.JSON, b)
}

func (a *goBlog) indieAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Token verification
		data, err := a.db.verifyIndieAuthToken(r.Header.Get("Authorization"))
		if err != nil {
			a.serveError(w, r, "Invalid token or token not found", http.StatusUnauthorized)
			return
		}
		res := &tokenResponse{
			Scope:    strings.Join(data.Scopes, " "),
			Me:       a.getFullAddress("") + "/", // MUST contain a path component / trailing slash
			ClientID: data.ClientID,
		}
		b, _ := json.Marshal(res)
		w.Header().Set(contentType, contenttype.JSONUTF8)
		_, _ = a.min.Write(w, contenttype.JSON, b)
		return
	} else if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			a.serveError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		// Token Revocation
		if r.Form.Get("action") == "revoke" {
			a.db.revokeIndieAuthToken(r.Form.Get("token"))
			w.WriteHeader(http.StatusOK)
			return
		}
		// Token request
		if r.Form.Get("grant_type") == "authorization_code" {
			data := &indieAuthData{
				code:        r.Form.Get("code"),
				ClientID:    r.Form.Get("client_id"),
				RedirectURI: r.Form.Get("redirect_uri"),
			}
			valid, err := a.db.verifyAuthorization(data)
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			if !valid {
				a.serveError(w, r, "Authentication not valid", http.StatusForbidden)
				return
			}
			if len(data.Scopes) < 1 {
				a.serveError(w, r, "No scope", http.StatusBadRequest)
				return
			}
			data.time = time.Now().UTC()
			sha := sha1.New()
			if _, err := sha.Write([]byte(data.time.Format(time.RFC3339) + data.ClientID)); err != nil {
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			data.token = fmt.Sprintf("%x", sha.Sum(nil))
			err = a.db.saveToken(data)
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			res := &tokenResponse{
				TokenType:   "Bearer",
				AccessToken: data.token,
				Scope:       strings.Join(data.Scopes, " "),
				Me:          a.getFullAddress("") + "/", // MUST contain a path component / trailing slash
			}
			b, _ := json.Marshal(res)
			w.Header().Set(contentType, contenttype.JSONUTF8)
			_, _ = a.min.Write(w, contenttype.JSON, b)
			return
		}
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
}

func (db *database) saveAuthorization(data *indieAuthData) (err error) {
	_, err = db.exec("insert into indieauthauth (time, code, client, redirect, scope) values (?, ?, ?, ?, ?)", data.time.Unix(), data.code, data.ClientID, data.RedirectURI, strings.Join(data.Scopes, " "))
	return
}

func (db *database) verifyAuthorization(data *indieAuthData) (valid bool, err error) {
	// code valid for 600 seconds
	row, err := db.queryRow("select code, client, redirect, scope from indieauthauth where time >= ? and code = ? and client = ? and redirect = ?", time.Now().Unix()-600, data.code, data.ClientID, data.RedirectURI)
	if err != nil {
		return false, err
	}
	scope := ""
	err = row.Scan(&data.code, &data.ClientID, &data.RedirectURI, &scope)
	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if scope != "" {
		data.Scopes = strings.Split(scope, " ")
	}
	valid = true
	_, err = db.exec("delete from indieauthauth where code = ? or time < ?", data.code, time.Now().Unix()-600)
	data.code = ""
	return
}

func (db *database) saveToken(data *indieAuthData) (err error) {
	_, err = db.exec("insert into indieauthtoken (time, token, client, scope) values (?, ?, ?, ?)", data.time.Unix(), data.token, data.ClientID, strings.Join(data.Scopes, " "))
	return
}

func (db *database) verifyIndieAuthToken(token string) (data *indieAuthData, err error) {
	token = strings.ReplaceAll(token, "Bearer ", "")
	data = &indieAuthData{
		Scopes: []string{},
	}
	row, err := db.queryRow("select time, token, client, scope from indieauthtoken where token = @token", sql.Named("token", token))
	if err != nil {
		return nil, err
	}
	timeString := ""
	scope := ""
	err = row.Scan(&timeString, &data.token, &data.ClientID, &scope)
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

func (db *database) revokeIndieAuthToken(token string) {
	if token != "" {
		_, _ = db.exec("delete from indieauthtoken where token=?", token)
	}
}
