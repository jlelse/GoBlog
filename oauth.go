package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	oauthMetadataPath          = "/.well-known/oauth-authorization-server"
	oauthCreateAppPath         = "/api/v1/apps"
	oauthAuthorizePath         = "/oauth/authorize"
	oauthTokenPath             = "/oauth/token" //nolint:gosec
	oauthRevokePath            = "/oauth/revoke"
	oauthVerifyCredentialsPath = "/api/v1/accounts/verify_credentials"

	oauthScope contextKey = "oauthScope"
)

var (
	errInvalidOAuthApp = errors.New("invalid client_id or client_secret")
	errInvalidToken    = errors.New("invalid token or token not found")
	errInvalidCode     = errors.New("invalid code or code not found")

	oauthGrantTypes           = []string{"authorization_code"}
	oauthSupportedScopes      = []string{"profile", "read", "write", "create", "update", "delete", "undelete", "media"}
	oauthCodeChallengeMethods = []string{"S256"}
)

func (a *goBlog) oauthMetadata(w http.ResponseWriter, r *http.Request) {
	issuerURL := a.getIssuerURL(r)
	baseURL := strings.TrimSuffix(issuerURL, "/")
	resp := map[string]any{
		"issuer":                                         issuerURL,
		"authorization_endpoint":                         baseURL + oauthAuthorizePath,
		"token_endpoint":                                 baseURL + oauthTokenPath,
		"revocation_endpoint":                            baseURL + oauthRevokePath,
		"scopes_supported":                               oauthSupportedScopes,
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          oauthGrantTypes,
		"code_challenge_methods_supported":               oauthCodeChallengeMethods,
		"token_endpoint_auth_methods_supported":          []string{"client_secret_basic", "client_secret_post"},
		"introspection_endpoint":                         baseURL + oauthTokenPath,
		"introspection_endpoint_auth_methods_supported":  []string{"none"},
		"revocation_endpoint_auth_methods_supported":     []string{"none"},
		"authorization_response_iss_parameter_supported": true,
	}
	if a.apEnabled() {
		resp["app_registration_endpoint"] = baseURL + oauthCreateAppPath
	}
	a.respondWithMinifiedJSON(w, resp)
}

func (a *goBlog) oauthCreateApp(w http.ResponseWriter, r *http.Request) {
	clientName := r.FormValue("client_name")
	redirectURIs := r.FormValue("redirect_uris")
	if clientName == "" || redirectURIs == "" {
		a.serveError(w, r, "Missing required parameters", http.StatusUnprocessableEntity)
		return
	}

	scopes := r.FormValue("scopes")
	if scopes == "" {
		scopes = "read"
	}

	website := r.FormValue("website")
	secret := generateOAuthSecret()
	id, err := a.db.oauthCreateApp(clientName, secret, redirectURIs, scopes, website)
	if err != nil {
		a.serveError(w, r, "Failed to create app", http.StatusInternalServerError)
		return
	}

	redirectURIList := splitFields(redirectURIs)
	scopeList := splitFields(scopes)
	resp := map[string]any{
		"client_id":                id,
		"client_secret":            secret,
		"name":                     clientName,
		"website":                  website,
		"redirect_uris":            redirectURIList,
		"scopes":                   scopeList,
		"client_secret_expires_at": 0,
	}

	a.respondWithMinifiedJSON(w, resp)
}

func (a *goBlog) oauthShowAuthorize(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	responseType := r.URL.Query().Get("response_type")

	if responseType != "code" || clientID == "" || redirectURI == "" {
		a.serveError(w, r, "Missing required parameters", http.StatusBadRequest)
		return
	}

	if err := a.getOAuthApp(clientID, redirectURI); err != nil {
		a.serveError(w, r, "Invalid client_id or redirect_uri", http.StatusBadRequest)
		return
	}

	codeChallenge := r.URL.Query().Get("code_challenge")
	if isOAuthURLClientID(clientID) && codeChallenge == "" {
		a.serveError(w, r, "PKCE is required for URL-based clients", http.StatusBadRequest)
		return
	}

	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")
	if codeChallengeMethod != "" && !slices.Contains(oauthCodeChallengeMethods, codeChallengeMethod) {
		a.serveError(w, r, "Unsupported code_challenge_method", http.StatusBadRequest)
		return
	}

	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "read"
	}

	state := r.URL.Query().Get("state")
	appName, appWebsite := a.lookupOAuthAppNameAndWebsite(clientID)
	_, bc := a.getBlog(r)
	a.render(w, r, a.renderOAuthAuthorize, &renderData{
		Blog: bc,
		Data: &oauthAuthorizeData{
			ClientID:            clientID,
			RedirectURI:         redirectURI,
			Scope:               scope,
			State:               state,
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: codeChallengeMethod,
			AppName:             appName,
			AppWebsite:          appWebsite,
			Scopes:              strings.Split(scope, " "),
		},
	})
}

func (a *goBlog) oauthHandleAuthorize(w http.ResponseWriter, r *http.Request) {
	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	if clientID == "" || redirectURI == "" {
		a.serveError(w, r, "Missing required parameters", http.StatusBadRequest)
		return
	}

	if err := a.getOAuthApp(clientID, redirectURI); err != nil {
		a.serveError(w, r, "Invalid client_id or redirect_uri", http.StatusBadRequest)
		return
	}

	codeChallenge := r.FormValue("code_challenge")
	if isOAuthURLClientID(clientID) && codeChallenge == "" {
		a.serveError(w, r, "PKCE is required for URL-based clients", http.StatusBadRequest)
		return
	}

	codeChallengeMethod := r.FormValue("code_challenge_method")
	if codeChallengeMethod != "" && !slices.Contains(oauthCodeChallengeMethods, codeChallengeMethod) {
		a.serveError(w, r, "Unsupported code_challenge_method", http.StatusBadRequest)
		return
	}
	if codeChallenge != "" && codeChallengeMethod == "" {
		codeChallengeMethod = "S256"
	}

	scope := r.FormValue("scope")
	code := uuid.NewString()
	_, err := a.db.Exec(
		"insert into indieauthauth (time, code, client, redirect, scope, challenge, challengemethod) values (?, ?, ?, ?, ?, ?, ?)",
		time.Now().UTC().Unix(), code, clientID, redirectURI, scope, codeChallenge, codeChallengeMethod,
	)
	if err != nil {
		a.serveError(w, r, "Failed to create authorization code", http.StatusInternalServerError)
		return
	}

	state := r.FormValue("state")
	query := url.Values{}
	query.Set("code", code)
	if state != "" {
		query.Set("state", state)
	}
	query.Set("iss", a.getIssuerURL(r))
	http.Redirect(w, r, redirectURI+"?"+query.Encode(), http.StatusFound)
}

func (a *goBlog) oauthToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil { //nolint:gosec
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	// IndieAuth access token introspection (current spec): POST with `token` parameter.
	// https://indieauth.spec.indieweb.org/#access-token-verification
	if r.FormValue("grant_type") == "" && r.FormValue("token") != "" {
		a.oauthIntrospectToken(w, r)
		return
	}

	// Client credentials may be supplied via the request body (client_secret_post)
	// or via HTTP Basic auth (client_secret_basic), as advertised in the server
	// metadata. Form values take precedence when present.
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	if clientID == "" {
		if basicID, basicSecret, ok := r.BasicAuth(); ok {
			clientID = basicID
			clientSecret = basicSecret
		}
	}
	if clientID == "" {
		a.serveOAuthError(w, r, "invalid_client", "missing client_id", http.StatusUnauthorized)
		return
	}

	app, isURLClient, err := a.getOAuthAppForToken(clientID, clientSecret)
	if err != nil {
		a.serveOAuthError(w, r, "invalid_client", err.Error(), http.StatusUnauthorized)
		return
	}

	switch r.FormValue("grant_type") {
	case "authorization_code":
		a.oauthTokenAuthorizationCode(w, r, app, isURLClient)
	default:
		a.serveOAuthError(w, r, "unsupported_grant_type", "unsupported grant type", http.StatusBadRequest)
	}
}

// getOAuthApp validates the client_id and redirect_uri combination.
// For URL-based clients (IndieAuth), the redirect_uri scheme/host/port must match the client_id.
func (a *goBlog) getOAuthApp(clientID, redirectURI string) error {
	if isOAuthURLClientID(clientID) {
		return oauthRedirectURIOriginMatches(clientID, redirectURI)
	}
	app, err := a.db.oauthGetApp(clientID)
	if err != nil {
		return errors.New("unknown client_id")
	}
	if !oauthRedirectURIMatches(app.RedirectURIs, redirectURI) {
		return errors.New("invalid redirect_uri")
	}
	return nil
}

// getOAuthAppForToken returns the app data for a token request, supporting both registered apps (UUID) and URL-based clients (IndieAuth).
// For URL-based clients, no client_secret is required (PKCE is used).
// The second return value is true if the client_id is a URL-based client (IndieAuth style).
func (a *goBlog) getOAuthAppForToken(clientID, clientSecret string) (*oauthApp, bool, error) {
	if isOAuthURLClientID(clientID) {
		return &oauthApp{
			ID:     clientID,
			Secret: clientSecret,
		}, true, nil
	}
	if clientSecret == "" {
		return nil, false, errors.New("missing client_secret")
	}
	app, err := a.db.oauthGetApp(clientID)
	if err != nil {
		return nil, false, errors.New("unknown client")
	}
	if app.Secret != clientSecret {
		return nil, false, errors.New("invalid client_secret")
	}
	return app, false, nil
}

// isOAuthURLClientID reports whether the given client_id is an IndieAuth URL identifier.
// It applies basic IndieAuth canonicalization (IndieAuth §3.4): the URL must have http or https
// scheme, a non-empty path, no single/double-dot path segments and no fragment. The host is
// compared case-insensitively. More thorough checks (host name, port) belong at the place that
// actually uses the URL.
func isOAuthURLClientID(clientID string) bool {
	if !strings.HasPrefix(clientID, "http://") && !strings.HasPrefix(clientID, "https://") {
		return false
	}
	u, err := url.Parse(clientID)
	if err != nil {
		return false
	}
	if u.Fragment != "" || u.RawFragment != "" {
		return false
	}
	if u.Path == "" {
		return false
	}
	for seg := range strings.SplitSeq(u.Path, "/") {
		if seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

func (a *goBlog) lookupOAuthAppNameAndWebsite(clientID string) (string, string) {
	if isOAuthURLClientID(clientID) {
		// Try to fetch the client_id URL to get its name
		return clientID, clientID
	}
	app, err := a.db.oauthGetApp(clientID)
	if err != nil {
		return "", ""
	}
	return app.Name, app.Website
}

func (a *goBlog) oauthTokenAuthorizationCode(w http.ResponseWriter, r *http.Request, app *oauthApp, isURLClient bool) {
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	if code == "" || redirectURI == "" {
		a.serveOAuthError(w, r, "invalid_request", "missing required parameters", http.StatusBadRequest)
		return
	}

	if isURLClient {
		if err := oauthRedirectURIOriginMatches(app.ID, redirectURI); err != nil {
			a.serveOAuthError(w, r, "invalid_grant", "redirect_uri does not match client_id origin", http.StatusBadRequest)
			return
		}
	} else if !oauthRedirectURIMatches(app.RedirectURIs, redirectURI) {
		a.serveOAuthError(w, r, "invalid_grant", "redirect_uri mismatch", http.StatusBadRequest)
		return
	}

	data, err := a.db.oauthGetAuthRequest(code)
	if errors.Is(err, errInvalidCode) {
		a.serveOAuthError(w, r, "invalid_grant", "invalid or expired code", http.StatusBadRequest)
		return
	} else if err != nil {
		a.serveError(w, r, "Internal server error", http.StatusInternalServerError)
		return
	}

	if data.ClientID != app.ID {
		a.serveOAuthError(w, r, "invalid_grant", "client_id mismatch", http.StatusBadRequest)
		return
	}

	codeVerifier := r.FormValue("code_verifier")
	if isURLClient || data.CodeChallenge != "" {
		if codeVerifier == "" {
			a.serveOAuthError(w, r, "invalid_grant", "missing code_verifier for PKCE", http.StatusBadRequest)
			return
		}
		expectedChallenge := sha256Challenge(codeVerifier, data.CodeChallengeMethod)
		if expectedChallenge != data.CodeChallenge {
			a.serveOAuthError(w, r, "invalid_grant", "PKCE verification failed", http.StatusBadRequest)
			return
		}
	}

	token := uuid.NewString()
	_, err = a.db.Exec(
		"insert into indieauthtoken (time, token, client, scope) values (?, ?, ?, ?)",
		time.Now().UTC().Unix(), token, app.ID, strings.Join(data.Scopes, " "),
	)
	if err != nil {
		a.serveError(w, r, "Failed to create token", http.StatusInternalServerError)
		return
	}

	meURL := a.getIssuerURL(r)
	resp := map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"scope":        strings.Join(data.Scopes, " "),
		"created_at":   time.Now().UTC().Unix(),
		"me":           meURL,
	}
	a.respondWithMinifiedJSON(w, resp)
}

func (a *goBlog) oauthRevoke(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	if token == "" {
		a.serveError(w, r, "missing token", http.StatusBadRequest)
		return
	}

	// Client authentication is optional for revocation, but when credentials
	// are supplied (via body or HTTP Basic auth) they must be valid.
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	if clientID == "" {
		if basicID, basicSecret, ok := r.BasicAuth(); ok {
			clientID = basicID
			clientSecret = basicSecret
		}
	}
	if clientID != "" && clientSecret != "" {
		app, _, err := a.getOAuthAppForToken(clientID, clientSecret)
		if err != nil {
			a.serveOAuthError(w, r, "invalid_client", err.Error(), http.StatusUnauthorized)
			return
		}
		_ = app
	}

	a.db.oauthRevokeToken(token)
	w.WriteHeader(http.StatusOK)
}

// oauthTokenVerification is the IndieAuth access token verification endpoint (legacy GET).
// https://indieauth.spec.indieweb.org/#access-token-verification-request
func (a *goBlog) oauthTokenVerification(w http.ResponseWriter, r *http.Request) {
	a.respondWithIntrospection(w, r, r.Header.Get("Authorization"))
}

// oauthIntrospectToken handles IndieAuth access token introspection (POST).
// https://indieauth.spec.indieweb.org/#access-token-verification
// The token to introspect is passed as the `token` form parameter.
func (a *goBlog) oauthIntrospectToken(w http.ResponseWriter, r *http.Request) {
	a.respondWithIntrospection(w, r, r.FormValue("token"))
}

// respondWithIntrospection writes the IndieAuth introspection response for the given token.
func (a *goBlog) respondWithIntrospection(w http.ResponseWriter, r *http.Request, token string) {
	data, err := a.db.oauthVerifyToken(token)
	var res map[string]any
	if errors.Is(err, errInvalidToken) {
		res = map[string]any{
			"active": false,
		}
	} else if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	} else {
		meURL := a.getIssuerURL(r)
		res = map[string]any{
			"active":    true,
			"me":        meURL,
			"client_id": data.ClientID,
			"scope":     strings.Join(data.Scopes, " "),
		}
	}
	a.respondWithMinifiedJSON(w, res)
}

func (a *goBlog) oauthVerifyCredentials(w http.ResponseWriter, r *http.Request) {
	bearer := r.Header.Get("Authorization")
	data, err := a.db.oauthVerifyToken(bearer)
	if err != nil {
		a.serveError(w, r, "unauthorized", http.StatusUnauthorized)
		return
	}

	if !oauthScopeGrants(data.Scopes, "profile") && !oauthScopeGrants(data.Scopes, "read:accounts") {
		a.serveError(w, r, "insufficient scope", http.StatusForbidden)
		return
	}

	blogName := a.cfg.DefaultBlog
	blog := a.cfg.Blogs[blogName]
	acct := a.apUserHandle[blogName]
	apIri := a.apIri(blog)
	if altAddr, ok := r.Context().Value(altAddressKey).(string); ok && altAddr != "" {
		apIri = a.apIriForAddress(blog, altAddr)
	}

	resp := map[string]any{
		"id":           apIri,
		"username":     blogName,
		"acct":         strings.TrimPrefix(acct, "@"),
		"display_name": a.cfg.User.Name,
		"locked":       false,
		"bot":          false,
		"note":         "",
		"url":          apIri,
		"avatar":       a.getFullAddress(a.profileImagePath(profileImageFormatJPEG, 256, 0)),
	}

	a.respondWithMinifiedJSON(w, resp)
}

func (a *goBlog) serveOAuthError(w http.ResponseWriter, _ *http.Request, erro, description string, status int) {
	w.WriteHeader(status)
	a.respondWithMinifiedJSON(w, map[string]any{
		"error":             erro,
		"error_description": description,
	})
}

func (a *goBlog) checkOAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		if bearer == "" {
			bearer = "Bearer " + r.URL.Query().Get("access_token")
		}
		data, err := a.db.oauthVerifyToken(bearer)
		if err != nil {
			a.serveError(w, r, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), oauthScope, strings.Join(data.Scopes, " "))))
	})
}

func addAllOAuthScopes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), oauthScope, "create update delete undelete media")))
	})
}

func (a *goBlog) getIssuerURL(r *http.Request) string {
	meURL := a.getInstanceRootURL()
	if altAddr, ok := r.Context().Value(altAddressKey).(string); ok && altAddr != "" {
		meURL = getFullAddressStatic(altAddr, "") + "/"
	} else if ia := a.cfg.Server.IndieAuthAddress; ia != "" {
		meURL = getFullAddressStatic(ia, "") + "/"
	}
	return meURL
}

func oauthRedirectURIMatches(allowedURIs, redirectURI string) bool {
	return slices.Contains(splitFields(allowedURIs), redirectURI)
}

// oauthRedirectURIOriginMatches reports whether the redirect_uri's scheme, host and port
// match those of the client_id. IndieAuth §5.2: "If the URL scheme, host or port of the
// `redirect_uri` in the request do not match that of the `client_id`, then the authorization
// endpoint SHOULD verify that the requested `redirect_uri` matches one of the redirect URLs
// published by the client".
// Without a published list, scheme/host/port equality is the strongest check we can do
// without fetching the client_id URL.
func oauthRedirectURIOriginMatches(clientID, redirectURI string) error {
	cu, err := url.Parse(clientID)
	if err != nil {
		return errors.New("invalid client_id")
	}
	ru, err := url.Parse(redirectURI)
	if err != nil {
		return errors.New("invalid redirect_uri")
	}
	if !strings.EqualFold(cu.Scheme, ru.Scheme) || !strings.EqualFold(cu.Hostname(), ru.Hostname()) || cu.Port() != ru.Port() {
		return errors.New("redirect_uri does not match client_id origin")
	}
	return nil
}

// oauthScopeGrants reports whether the granted scopes authorize the requested
// permission. Mastodon-style hierarchical scopes are supported: a top-level
// scope like "read" grants any granular "read:*" permission, while a granular
// scope only grants itself (or sub-permissions that share its prefix).
func oauthScopeGrants(granted []string, permission string) bool {
	for _, s := range granted {
		if s == permission {
			return true
		}
		if root, _, found := strings.Cut(permission, ":"); found && s == root {
			return true
		}
	}
	return false
}

func splitFields(s string) []string {
	fields := strings.Fields(s)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func generateOAuthSecret() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func sha256Challenge(verifier, method string) string {
	if method != "S256" {
		return verifier
	}
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

type oauthAuthorizeData struct {
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	AppName             string
	AppWebsite          string
	Scopes              []string
}

type oauthApp struct {
	ID           string
	Name         string
	Secret       string
	RedirectURIs string
	Scopes       string
	Website      string
	Created      int64
}

type oauthAuthRequest struct {
	ClientID            string
	RedirectURI         string
	Scopes              []string
	CodeChallenge       string
	CodeChallengeMethod string
}

func (db *database) oauthCreateApp(name, secret, redirectURIs, scopes, website string) (string, error) {
	id := uuid.NewString()
	_, err := db.Exec(
		"insert into fediverseapps (id, name, secret, redirect_uris, scopes, website, created) values (?, ?, ?, ?, ?, ?, ?)",
		id, name, secret, redirectURIs, scopes, website, time.Now().UTC().Unix(),
	)
	return id, err
}

func (db *database) oauthGetApp(id string) (*oauthApp, error) {
	row, err := db.QueryRow("select id, name, secret, redirect_uris, scopes, website, created from fediverseapps where id = ?", id)
	if err != nil {
		return nil, err
	}
	app := &oauthApp{}
	var website sql.NullString
	err = row.Scan(&app.ID, &app.Name, &app.Secret, &app.RedirectURIs, &app.Scopes, &website, &app.Created)
	if err == sql.ErrNoRows {
		return nil, errInvalidOAuthApp
	} else if err != nil {
		return nil, err
	}
	app.Website = website.String
	return app, nil
}

func (db *database) oauthGetAuthRequest(code string) (data *oauthAuthRequest, err error) {
	maxAge := time.Now().UTC().Add(-10 * time.Minute).Unix()
	row, err := db.QueryRow("select client, redirect, scope, challenge, challengemethod from indieauthauth where time >= ? and code = ?", maxAge, code)
	if err != nil {
		return nil, err
	}
	data = &oauthAuthRequest{}
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
	_, _ = db.Exec("delete from indieauthauth where code = ? or time < ?", code, maxAge)
	return data, nil
}

func (db *database) oauthVerifyToken(token string) (data *oauthAuthRequest, err error) {
	token = strings.TrimPrefix(token, "Bearer ")
	data = &oauthAuthRequest{Scopes: []string{}}
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
	return data, nil
}

func (db *database) oauthRevokeToken(token string) {
	if token != "" {
		_, _ = db.Exec("delete from indieauthtoken where token=?", token)
	}
}
