package main

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.hacdias.com/indielib/indieauth"
)

// TODOs:
// - Expire tokens after a while
// - Userinfo endpoint

const indieAuthPath = "/indieauth"
const indieAuthTokenSubpath = "/token"
const indieAuthTokenRevocationSubpath = "/revoke"

// https://www.w3.org/TR/indieauth/
// https://indieauth.spec.indieweb.org/

var (
	errInvalidToken = errors.New("invalid token or token not found")
	errInvalidCode  = errors.New("invalid code or code not found")
)

// Server Metadata
// https://indieauth.spec.indieweb.org/#x4-1-1-indieauth-server-metadata
func (a *goBlog) indieAuthMetadata(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{
		"issuer":                 a.getInstanceRootURL(),
		"authorization_endpoint": a.getFullAddress(indieAuthPath),
		"token_endpoint":         a.getFullAddress(indieAuthPath + indieAuthTokenSubpath),
		"introspection_endpoint": a.getFullAddress(indieAuthPath + indieAuthTokenSubpath),
		"revocation_endpoint":    a.getFullAddress(indieAuthPath + indieAuthTokenRevocationSubpath),
		"revocation_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                           []string{"create", "update", "delete", "undelete", "media"},
		"code_challenge_methods_supported":           indieauth.CodeChallengeMethods,
	}
	a.respondWithMinifiedJson(w, resp)
}

// Parse Authorization Request
// https://indieauth.spec.indieweb.org/#authorization-request
func (a *goBlog) indieAuthRequest(w http.ResponseWriter, r *http.Request) {
	iareq, err := a.ias.ParseAuthorization(r)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	// Render page that let's the user authorize the app
	a.render(w, r, a.renderIndieAuth, &renderData{
		Data: iareq,
	})
}

// The user accepted the authorization request
// Authorization response
// https://indieauth.spec.indieweb.org/#authorization-response
func (a *goBlog) indieAuthAccept(w http.ResponseWriter, r *http.Request) {
	iareq, err := a.ias.ParseAuthorization(r)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	// Save the authorization request
	code, err := a.db.indieAuthSaveAuthRequest(iareq)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Build a redirect
	query := url.Values{}
	query.Set("code", code)
	query.Set("state", iareq.State)
	query.Set("iss", a.getInstanceRootURL())
	query.Set("me", a.getInstanceRootURL())
	http.Redirect(w, r, iareq.RedirectURI+"?"+query.Encode(), http.StatusFound)
}

// authorization endpoint
// https://indieauth.spec.indieweb.org/#redeeming-the-authorization-code
// The client only exchanges the authorization code for the user's profile URL
func (a *goBlog) indieAuthVerificationAuth(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	a.indieAuthVerification(w, r, false)
}

// token endpoint
// https://indieauth.spec.indieweb.org/#redeeming-the-authorization-code
// The client exchanges the authorization code for an access token and the user's profile URL
func (a *goBlog) indieAuthVerificationToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	// Token Revocation (old way)
	if r.Form.Get("action") == "revoke" {
		a.db.indieAuthRevokeToken(r.Form.Get("token"))
		return
	}
	// Token request
	a.indieAuthVerification(w, r, true)
}

// Token Revocation (new way)
// https://indieauth.spec.indieweb.org/#token-revocation-p-4
func (a *goBlog) indieAuthTokenRevokation(_ http.ResponseWriter, r *http.Request) {
	a.db.indieAuthRevokeToken(r.Form.Get("token"))
}

// Verify the authorization request with or without token response
func (a *goBlog) indieAuthVerification(w http.ResponseWriter, r *http.Request, withToken bool) {
	// Get code and retrieve auth request
	code := r.Form.Get("code")
	if code == "" {
		a.serveError(w, r, "missing code parameter", http.StatusBadRequest)
		return
	}
	data, err := a.db.indieAuthGetAuthRequest(code)
	if errors.Is(err, errInvalidCode) {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	} else if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Check grant type
	if grantType := r.Form.Get("grant_type"); grantType != "" && grantType != "authorization_code" {
		a.serveError(w, r, "unknown grant type", http.StatusBadRequest)
		return
	}
	// Validate token exchange
	if err = a.ias.ValidateTokenExchange(data, r); err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	// Generate response
	resp := map[string]any{
		"me": a.getInstanceRootURL(),
	}
	if withToken {
		// Generate and save token
		token, err := a.db.indieAuthSaveToken(data)
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		// Add token to response
		resp["token_type"] = "Bearer"
		resp["access_token"] = token
		resp["scope"] = strings.Join(data.Scopes, " ")
	}
	a.respondWithMinifiedJson(w, resp)
}

// Save the authorization request and return the code
func (db *database) indieAuthSaveAuthRequest(data *indieauth.AuthenticationRequest) (string, error) {
	// Generate a code to identify the request
	code := uuid.NewString()
	// Save the request
	_, err := db.Exec(
		"insert into indieauthauth (time, code, client, redirect, scope, challenge, challengemethod) values (?, ?, ?, ?, ?, ?, ?)",
		time.Now().UTC().Unix(), code, data.ClientID, data.RedirectURI, strings.Join(data.Scopes, " "), data.CodeChallenge, data.CodeChallengeMethod,
	)
	return code, err
}

// Retrieve the auth request from the database to continue the authorization process
func (db *database) indieAuthGetAuthRequest(code string) (data *indieauth.AuthenticationRequest, err error) {
	// code valid for 10 minutes
	maxAge := time.Now().UTC().Add(-10 * time.Minute).Unix()
	// Query the database
	row, err := db.QueryRow("select client, redirect, scope, challenge, challengemethod from indieauthauth where time >= ? and code = ?", maxAge, code)
	if err != nil {
		return nil, err
	}
	data = &indieauth.AuthenticationRequest{}
	var scope string
	err = row.Scan(&data.ClientID, &data.RedirectURI, &scope, &data.CodeChallenge, &data.CodeChallengeMethod)
	if err == sql.ErrNoRows {
		return nil, errInvalidCode
	} else if err != nil {
		return nil, err
	}
	if scope != "" {
		data.Scopes = strings.Split(scope, " ")
	}
	// Delete the auth code and expired auth codes
	_, _ = db.Exec("delete from indieauthauth where code = ? or time < ?", code, maxAge)
	return data, nil
}

// Access token verification request (https://indieauth.spec.indieweb.org/#access-token-verification-request)
//
// GET request to the token endpoint to check if the access token is valid
func (a *goBlog) indieAuthTokenVerification(w http.ResponseWriter, r *http.Request) {
	data, err := a.db.indieAuthVerifyToken(r.Header.Get("Authorization"))
	var res map[string]any
	if errors.Is(err, errInvalidToken) {
		res = map[string]any{
			"active": false,
		}
	} else if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	} else {
		res = map[string]any{
			"active":    true,
			"me":        a.getInstanceRootURL(),
			"client_id": data.ClientID,
			"scope":     strings.Join(data.Scopes, " "),
		}
	}
	a.respondWithMinifiedJson(w, res)
}

// Checks the database for the token and returns the indieAuthData with client and scope.
//
// Returns errInvalidToken if the token is invalid.
func (db *database) indieAuthVerifyToken(token string) (data *indieauth.AuthenticationRequest, err error) {
	token = strings.ReplaceAll(token, "Bearer ", "")
	data = &indieauth.AuthenticationRequest{Scopes: []string{}}
	row, err := db.QueryRow("select client, scope from indieauthtoken where token = @token", sql.Named("token", token))
	if err != nil {
		return nil, err
	}
	var scope string
	err = row.Scan(&data.ClientID, &scope)
	if err == sql.ErrNoRows {
		return nil, errInvalidToken
	} else if err != nil {
		return nil, err
	}
	if scope != "" {
		data.Scopes = strings.Split(scope, " ")
	}
	return
}

// Save a new token to the database
func (db *database) indieAuthSaveToken(data *indieauth.AuthenticationRequest) (string, error) {
	token := uuid.NewString()
	_, err := db.Exec("insert into indieauthtoken (time, token, client, scope) values (?, ?, ?, ?)", time.Now().UTC().Unix(), token, data.ClientID, strings.Join(data.Scopes, " "))
	return token, err
}

// Revoke and delete the token from the database
func (db *database) indieAuthRevokeToken(token string) {
	if token != "" {
		_, _ = db.Exec("delete from indieauthtoken where token=?", token)
	}
}
