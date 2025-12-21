package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	webauthnCredSettingsKey   = "webauthncred"
	webauthnIdSettingsKey     = "webauthnid"
	settingsDeletePasskeyPath = "/deletepasskey"
	settingsRenamePasskeyPath = "/renamepasskey"
)

func (a *goBlog) initWebAuthn() error {
	wconfig := &webauthn.Config{
		RPDisplayName:        "GoBlog",
		RPID:                 a.cfg.Server.publicHostname,
		RPOrigins:            []string{a.getFullAddress("/")},
		EncodeUserIDAsString: true,
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    5 * time.Minute,
				TimeoutUVD: 5 * time.Minute,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    5 * time.Minute,
				TimeoutUVD: 5 * time.Minute,
			},
		},
	}
	webAuthn, err := webauthn.New(wconfig)
	if err != nil {
		return err
	}
	a.webAuthn = webAuthn
	return nil
}

func (a *goBlog) beginWebAuthnRegistration(w http.ResponseWriter, r *http.Request) {
	options, session, err := a.webAuthn.BeginRegistration(a.getWebAuthnUser())
	if err != nil {
		a.debug("failed to begin webauthn registration", "err", err)
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	a.initSessionStores()
	ses, err := a.webauthnSessions.New(r, "wa")
	if err != nil {
		a.debug("failed to create new webauthn registration session", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	sessionJsonBytes, err := json.Marshal(session)
	if err != nil {
		a.debug("failed to marshal webauthn session to json", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	ses.Values["session"] = string(sessionJsonBytes)
	_ = ses.Save(r, w)
	w.WriteHeader(http.StatusOK)
	a.respondWithMinifiedJson(w, options)
}

func (a *goBlog) finishWebAuthnRegistration(w http.ResponseWriter, r *http.Request) {
	a.initSessionStores()
	ses, err := a.webauthnSessions.Get(r, "wa")
	if err != nil {
		a.debug("failed to get webauthn session", "err", err)
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	sessionJson, ok := ses.Values["session"]
	if !ok || sessionJson == "" {
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJson.(string)), &session); err != nil {
		a.debug("failed to unmarshal webauthn session data", "err", err)
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	user := a.getWebAuthnUser()
	credential, err := a.webAuthn.FinishRegistration(user, session, r)
	if err != nil {
		a.debug("failed to finish webauthn registration", "err", err)
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	// Generate a unique ID for the passkey based on credential ID
	passkeyID := base64.RawURLEncoding.EncodeToString(credential.ID)
	// Save to passkeys table
	if err := a.savePasskey(passkeyID, "Passkey", credential); err != nil {
		a.error("failed to save webauthn credentials", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	a.webauthnSessions.Delete(r, w, ses)
	w.WriteHeader(http.StatusOK)
}

func (a *goBlog) beginWebAuthnLogin(w http.ResponseWriter, r *http.Request) {
	options, session, err := a.webAuthn.BeginLogin(a.getWebAuthnUser())
	if err != nil {
		a.debug("failed to begin webauthn login", "err", err)
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	a.initSessionStores()
	ses, err := a.webauthnSessions.New(r, "wa")
	if err != nil {
		a.debug("failed to create new webauthn login session", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	sessionJsonBytes, err := json.Marshal(session)
	if err != nil {
		a.debug("failed to marshal webauthn session to json", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	ses.Values["session"] = string(sessionJsonBytes)
	_ = ses.Save(r, w)
	w.WriteHeader(http.StatusOK)
	a.respondWithMinifiedJson(w, options)
}

func (a *goBlog) finishWebAuthnLogin(w http.ResponseWriter, r *http.Request) {
	a.initSessionStores()
	ses, err := a.webauthnSessions.Get(r, "wa")
	if err != nil {
		a.debug("failed to get webauthn session", "err", err)
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	sessionJson, ok := ses.Values["session"]
	if !ok || sessionJson == "" {
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJson.(string)), &session); err != nil {
		a.debug("failed to unmarshal webauthn session data", "err", err)
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	user := a.getWebAuthnUser()
	credential, err := a.webAuthn.FinishLogin(user, session, r)
	if err != nil {
		a.debug("failed to finish webauthn login", "err", err)
		a.serveError(w, r, "", http.StatusBadRequest)
		return
	}
	// Update the credential in the database (for counter updates)
	passkeyID := base64.RawURLEncoding.EncodeToString(credential.ID)
	if pk, _ := a.getPasskey(passkeyID); pk != nil {
		// Update existing passkey
		if err := a.savePasskey(passkeyID, pk.Name, credential); err != nil {
			a.debug("failed to update webauthn credentials", "err", err)
		}
	}
	a.webauthnSessions.Delete(r, w, ses)
	// Also set login cookie
	loginSes, err := a.loginSessions.Get(r, "l")
	if err != nil {
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	loginSes.Values["login"] = true
	_ = a.loginSessions.Save(r, w, loginSes)
	// Write header, login successful
	w.WriteHeader(http.StatusOK)
}

func (a *goBlog) settingsDeletePasskey(w http.ResponseWriter, r *http.Request) {
	passkeyID := r.FormValue("passkeyid")
	if passkeyID == "" {
		a.serveError(w, r, "Passkey ID is required", http.StatusBadRequest)
		return
	}
	if err := a.deletePasskeyByID(passkeyID); err != nil {
		a.debug("failed to delete passkey", "err", err)
		a.serveError(w, r, "failed to delete passkey", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, ".", http.StatusFound)
}

func (a *goBlog) settingsRenamePasskey(w http.ResponseWriter, r *http.Request) {
	passkeyID := r.FormValue("passkeyid")
	passkeyName := r.FormValue("passkeyname")
	if passkeyID == "" || passkeyName == "" {
		a.serveError(w, r, "Missing passkey ID or name", http.StatusBadRequest)
		return
	}
	if err := a.renamePasskey(passkeyID, passkeyName); err != nil {
		a.debug("failed to rename passkey", "err", err)
		a.serveError(w, r, "failed to rename passkey", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, ".", http.StatusFound)
}

type webAuthnUser struct {
	a *goBlog
}

func (a *goBlog) getWebAuthnUser() *webAuthnUser {
	return &webAuthnUser{a: a}
}

func (u *webAuthnUser) WebAuthnID() []byte {
	id, _ := u.a.getSettingValue(webauthnIdSettingsKey)
	if id == "" {
		id = randomString(32)
		if err := u.a.saveSettingValue(webauthnIdSettingsKey, id); err != nil {
			u.a.error("failed to save webauthnid settings value", "err", err)
		}
	}
	return []byte(id)
}

func (u *webAuthnUser) WebAuthnName() string {
	return u.a.cfg.User.Name
}

func (u *webAuthnUser) WebAuthnDisplayName() string {
	return u.a.cfg.User.Name
}

func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	creds, err := u.a.getWebAuthnCredentials()
	if err != nil {
		u.a.error("failed to read webauthn credentials from db", "err", err)
		return nil
	}
	return creds
}

// hasWebAuthnCredential checks if any passkeys are registered
func (a *goBlog) hasWebAuthnCredential() bool {
	if a.db == nil {
		return false
	}
	hasPasskeys, err := a.hasPasskeys()
	return err == nil && hasPasskeys
}
