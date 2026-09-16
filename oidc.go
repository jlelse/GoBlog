package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"golang.org/x/oauth2"
)

const (
	oidcLoginPath    = "/oidc/login"
	oidcCallbackPath = "/oidc/callback"

	settingsDeleteOIDCPath = "/deleteoidc"

	// Settings keys for the linked OIDC account
	oidcSubjectSettingsKey = "oidcsubject"
	oidcIssuerSettingsKey  = "oidcissuer"
)

// oidcClient bundles the discovered provider configuration.
type oidcClient struct {
	verifier *oidc.IDTokenVerifier
	config   oauth2.Config
}

// oidcEnabled reports whether OIDC login is configured.
func (a *goBlog) oidcEnabled() bool {
	cfg := a.cfg.OIDC
	return cfg != nil && cfg.Enabled && cfg.Issuer != "" && cfg.ClientID != ""
}

// OIDC account link storage

// getOIDCSubject returns the linked OIDC subject, if any.
func (a *goBlog) getOIDCSubject() (string, error) {
	return a.getSettingValue(oidcSubjectSettingsKey)
}

// hasOIDCLink reports whether an OIDC account is linked.
func (a *goBlog) hasOIDCLink() bool {
	if !a.oidcEnabled() {
		return false
	}
	subject, err := a.getOIDCSubject()
	return err == nil && subject != ""
}

// linkOIDC links the current account to an OIDC subject.
func (a *goBlog) linkOIDC(subject string) error {
	if !a.oidcEnabled() {
		return errors.New("oidc is not enabled")
	}
	if err := a.saveSettingValue(oidcSubjectSettingsKey, subject); err != nil {
		return err
	}
	return a.saveSettingValue(oidcIssuerSettingsKey, a.cfg.OIDC.Issuer)
}

// unlinkOIDC removes the OIDC account link.
func (a *goBlog) unlinkOIDC() error {
	if err := a.deleteSettingValue(oidcSubjectSettingsKey); err != nil {
		return err
	}
	return a.deleteSettingValue(oidcIssuerSettingsKey)
}

// initOIDC creates one OIDC client per configured address (main public
// address and alt addresses), each with a redirect URI for that address,
// so login works from any of them. Cookie and callback then stay on the
// same host, like the WebAuthn per-address approach.
func (a *goBlog) initOIDC() error {
	if !a.oidcEnabled() {
		return nil
	}
	// Initialize the provider discovery once
	cfg := a.cfg.OIDC
	dctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(dctx, cfg.Issuer)
	if err != nil {
		return err
	}
	addresses := append([]string{a.cfg.Server.PublicAddress}, a.cfg.Server.AltAddresses...)
	a.oidcClients = map[string]*oidcClient{}
	for _, address := range addresses {
		a.oidcClients[address] = &oidcClient{
			verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
			config: oauth2.Config{
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
				Endpoint:     provider.Endpoint(),
				RedirectURL:  getFullAddressStatic(address, oidcCallbackPath),
				Scopes:       []string{oidc.ScopeOpenID},
			},
		}
	}
	return nil
}

// getOIDCClientForRequest returns the OIDC client matching the requesting address.
func (a *goBlog) getOIDCClientForRequest(r *http.Request) *oidcClient {
	if altAddress, ok := r.Context().Value(altAddressKey).(string); ok && altAddress != "" {
		if client, ok := a.oidcClients[altAddress]; ok && client != nil {
			return client
		}
	}
	return a.oidcClients[a.cfg.Server.PublicAddress]
}

// Handlers

// serveOIDCLogin starts the OIDC authorization code flow. When the request is
// authenticated, the resulting account is linked instead of used for login.
// Linking is only allowed for POST requests, so it can't be triggered by
// cross-site GET navigations. GET requests start a login, which is used by the
// OIDC login popup from the login page.
func (a *goBlog) serveOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if !a.oidcEnabled() {
		a.serveError(w, r, "OIDC is not enabled", http.StatusNotFound)
		return
	}
	// Linking requires an authenticated POST request
	isLink := a.isLoggedIn(r)
	if isLink && r.Method != http.MethodPost {
		a.serveError(w, r, "", http.StatusMethodNotAllowed)
		return
	}
	// Linking requires an authenticated session, otherwise it's a login attempt
	a.initSessionStores()
	ses, err := a.oidcSessions.New(r, "o")
	if err != nil {
		a.debug("failed to create oidc session", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	state := randomString(32)
	ses.Values["state"] = state
	nonce := randomString(32)
	ses.Values["nonce"] = nonce
	verifier := oauth2.GenerateVerifier()
	ses.Values["verifier"] = verifier
	// Linking requires POST, unauthenticated GET starts a login popup
	ses.Values["link"] = isLink
	if err = a.oidcSessions.Save(r, w, ses); err != nil {
		a.debug("failed to save oidc session", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	authURL := a.getOIDCClientForRequest(r).config.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
	http.Redirect(w, r, authURL, http.StatusFound)
}

// serveOIDCCallback handles the redirect from the OIDC provider.
func (a *goBlog) serveOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if !a.oidcEnabled() {
		a.serveError(w, r, "OIDC is not enabled", http.StatusNotFound)
		return
	}
	a.initSessionStores()
	ses, err := a.oidcSessions.Get(r, "o")
	if err != nil {
		a.debug("failed to get oidc session", "err", err)
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	state, stateSaved := ses.Values["state"].(string)
	nonce, _ := ses.Values["nonce"].(string)
	verifier, _ := ses.Values["verifier"].(string)
	link, linkSaved := ses.Values["link"].(bool)
	_ = a.oidcSessions.Delete(r, w, ses)
	if !stateSaved || r.URL.Query().Get("state") != state {
		a.serveError(w, r, "Invalid OIDC state", http.StatusBadRequest)
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		a.serveError(w, r, "OIDC error: "+errParam, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	oidcClient := a.getOIDCClientForRequest(r)
	oauth2Token, err := oidcClient.config.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		a.debug("failed to exchange oidc code", "err", err)
		a.serveError(w, r, "Failed to exchange OIDC code", http.StatusBadRequest)
		return
	}
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		a.serveError(w, r, "Missing OIDC ID token", http.StatusBadRequest)
		return
	}
	idToken, err := oidcClient.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		a.debug("failed to verify oidc id token", "err", err)
		a.serveError(w, r, "Failed to verify OIDC ID token", http.StatusBadRequest)
		return
	}
	if idToken.Nonce != nonce {
		a.serveError(w, r, "Invalid OIDC nonce", http.StatusBadRequest)
		return
	}
	if linkSaved && link {
		// Re-verify authentication at callback time, the user might have logged out during the flow
		if !a.isLoggedIn(r) {
			a.serveError(w, r, "Not logged in", http.StatusUnauthorized)
			return
		}
		// Link the OIDC account to the user
		if err = a.linkOIDC(idToken.Subject); err != nil {
			a.debug("failed to link oidc account", "err", err)
			a.serveError(w, r, "Failed to link OIDC account", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, settingsPath, http.StatusFound)
		return
	}
	// The link is only valid for the issuer it was created for
	storedIssuer, err := a.getSettingValue(oidcIssuerSettingsKey)
	if err != nil {
		a.debug("failed to read linked oidc account", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	if storedIssuer != a.cfg.OIDC.Issuer {
		a.serveError(w, r, "This OIDC link is not valid for the configured provider", http.StatusForbidden)
		return
	}
	subject, err := a.getOIDCSubject()
	if err != nil {
		a.debug("failed to read linked oidc account", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	if subject == "" || subject != idToken.Subject {
		a.serveError(w, r, "This OIDC account is not linked to GoBlog", http.StatusForbidden)
		return
	}
	loginSes, err := a.loginSessions.Get(r, "l")
	if err != nil {
		a.debug("failed to get login session", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	loginSes.Values["login"] = true
	if err = a.loginSessions.Save(r, w, loginSes); err != nil {
		a.debug("failed to save login session", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	// Notify the opener (popup flow) or redirect home (fallback flow)
	a.render(w, r, a.renderOIDCLoggedIn, &renderData{})
}

// renderOIDCLoggedIn notifies the opener via oidc.js and closes the popup, or
// redirects home when the login didn't happen in a popup.
func (a *goBlog) renderOIDCLoggedIn(hb *htmlbuilder.HTMLBuilder, rd *renderData) {
	hb.WriteElementOpen("main")
	hb.WriteElementOpen("p")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "oidcloginpopup"))
	hb.WriteElementClose("p")
	hb.WriteElementOpen("p")
	hb.WriteElementOpen("a", "href", "/")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "backhome"))
	hb.WriteElementClose("a")
	hb.WriteElementClose("p")
	hb.WriteElementOpen("script", "src", a.assetFileName("js/oidc.js"), "integrity", a.assetFileHash("js/oidc.js"))
	hb.WriteElementClose("script")
	hb.WriteElementClose("main")
}

// settingsDeleteOIDC removes the linked OIDC account.
func (a *goBlog) settingsDeleteOIDC(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	// Prevent locking the user out: require another way of logging in
	hasPassword, _ := a.hasPassword()
	hasPasskeys, _ := a.hasPasskeys()
	if !hasPassword && !hasPasskeys {
		a.serveError(w, r, "Cannot unlink OIDC without a password or passkey", http.StatusBadRequest)
		return
	}
	if err := a.unlinkOIDC(); err != nil {
		a.debug("failed to unlink oidc account", "err", err)
		a.serveError(w, r, "Failed to unlink OIDC account", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, bc.getRelativePath(settingsPath), http.StatusFound)
}
