package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost = 12
	// Settings keys for auth data
	passwordHashSettingsKey = "passwordhash"
	totpSecretSettingsKey   = "totpsecret"
)

// Password functions

// hashPassword creates a bcrypt hash of the password
func hashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// checkPasswordHash compares a password against a bcrypt hash
func checkPasswordHash(password, hash string) bool {
	if hash == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// getPasswordHash returns the stored password hash from settings table
func (a *goBlog) getPasswordHash() (string, error) {
	return a.getSettingValue(passwordHashSettingsKey)
}

// setPasswordHash stores the password hash in settings table
func (a *goBlog) setPasswordHash(hash string) error {
	return a.saveSettingValue(passwordHashSettingsKey, hash)
}

// setPassword hashes and stores the password
func (a *goBlog) setPassword(password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return a.setPasswordHash(hash)
}

// checkPassword verifies the password against the stored hash
func (a *goBlog) checkPassword(password string) (bool, error) {
	hash, err := a.getPasswordHash()
	if err != nil {
		return false, err
	}
	return checkPasswordHash(password, hash), nil
}

// hasPassword checks if a password has been set
func (a *goBlog) hasPassword() (bool, error) {
	hash, err := a.getPasswordHash()
	if err != nil {
		return false, err
	}
	return hash != "", nil
}

// TOTP functions

// getTOTPSecret returns the stored TOTP secret from settings table
func (a *goBlog) getTOTPSecret() (string, error) {
	return a.getSettingValue(totpSecretSettingsKey)
}

// setTOTPSecret stores the TOTP secret in settings table
func (a *goBlog) setTOTPSecret(secret string) error {
	return a.saveSettingValue(totpSecretSettingsKey, secret)
}

// hasTOTP checks if TOTP is configured
func (a *goBlog) hasTOTP() (bool, error) {
	secret, err := a.getTOTPSecret()
	if err != nil {
		return false, err
	}
	return secret != "", nil
}

// deleteTOTP removes the TOTP secret
func (a *goBlog) deleteTOTP() error {
	return a.setTOTPSecret("")
}

// Passkey (WebAuthn) functions

// Passkey represents a stored WebAuthn credential
type Passkey struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Credential string    `json:"credential"`
	Created    time.Time `json:"created"`
}

// getPasskeys returns all stored passkeys
func (a *goBlog) getPasskeys() ([]*Passkey, error) {
	rows, err := a.db.Query("select id, name, credential, created from passkeys order by created desc")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var passkeys []*Passkey
	for rows.Next() {
		var pk Passkey
		var createdUnix int64
		err = rows.Scan(&pk.ID, &pk.Name, &pk.Credential, &createdUnix)
		if err != nil {
			return nil, err
		}
		pk.Created = time.Unix(createdUnix, 0)
		passkeys = append(passkeys, &pk)
	}
	return passkeys, nil
}

// getPasskey returns a specific passkey by ID
func (a *goBlog) getPasskey(id string) (*Passkey, error) {
	row, err := a.db.QueryRow("select id, name, credential, created from passkeys where id = @id", sql.Named("id", id))
	if err != nil {
		return nil, err
	}
	var pk Passkey
	var createdUnix int64
	err = row.Scan(&pk.ID, &pk.Name, &pk.Credential, &createdUnix)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	pk.Created = time.Unix(createdUnix, 0)
	return &pk, nil
}

// savePasskey stores a new passkey or updates an existing one
func (a *goBlog) savePasskey(id, name string, cred *webauthn.Credential) error {
	credBytes, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(
		`insert into passkeys (id, name, credential, created) values (@id, @name, @credential, @created)
		 on conflict (id) do update set name = @name, credential = @credential`,
		sql.Named("id", id),
		sql.Named("name", name),
		sql.Named("credential", string(credBytes)),
		sql.Named("created", time.Now().Unix()),
	)
	return err
}

// renamePasskey updates the name of a passkey
func (a *goBlog) renamePasskey(id, name string) error {
	_, err := a.db.Exec(
		"update passkeys set name = @name where id = @id",
		sql.Named("id", id),
		sql.Named("name", name),
	)
	return err
}

// deletePasskey removes a passkey by ID
func (a *goBlog) deletePasskeyByID(id string) error {
	_, err := a.db.Exec("delete from passkeys where id = @id", sql.Named("id", id))
	return err
}

// hasPasskeys checks if any passkeys are registered
func (a *goBlog) hasPasskeys() (bool, error) {
	row, err := a.db.QueryRow("select count(*) from passkeys")
	if err != nil {
		return false, err
	}
	var count int
	err = row.Scan(&count)
	return count > 0, err
}

// getWebAuthnCredentials returns all WebAuthn credentials for authentication
func (a *goBlog) getWebAuthnCredentials() ([]webauthn.Credential, error) {
	passkeys, err := a.getPasskeys()
	if err != nil {
		return nil, err
	}
	var creds []webauthn.Credential
	for _, pk := range passkeys {
		var cred webauthn.Credential
		if err := json.Unmarshal([]byte(pk.Credential), &cred); err != nil {
			continue // Skip invalid credentials
		}
		creds = append(creds, cred)
	}
	return creds, nil
}

// App Password functions

// AppPassword represents a stored app password
type AppPassword struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TokenHash string    `json:"-"`
	Created   time.Time `json:"created"`
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// generateAppPasswordID generates a unique ID for an app password
func generateAppPasswordID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// getAppPasswords returns all stored app passwords
func (a *goBlog) getAppPasswords() ([]*AppPassword, error) {
	rows, err := a.db.Query("select id, name, token_hash, created from app_passwords order by created desc")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var passwords []*AppPassword
	for rows.Next() {
		var ap AppPassword
		var createdUnix int64
		err = rows.Scan(&ap.ID, &ap.Name, &ap.TokenHash, &createdUnix)
		if err != nil {
			return nil, err
		}
		ap.Created = time.Unix(createdUnix, 0)
		passwords = append(passwords, &ap)
	}
	return passwords, nil
}

// createAppPassword creates a new app password and returns the plaintext token (only shown once)
func (a *goBlog) createAppPassword(name string) (id, token string, err error) {
	token, err = generateSecureToken()
	if err != nil {
		return "", "", err
	}
	hash, err := hashPassword(token)
	if err != nil {
		return "", "", err
	}
	id, err = generateAppPasswordID()
	if err != nil {
		return "", "", err
	}
	_, err = a.db.Exec(
		"insert into app_passwords (id, name, token_hash, created) values (@id, @name, @hash, @created)",
		sql.Named("id", id),
		sql.Named("name", name),
		sql.Named("hash", hash),
		sql.Named("created", time.Now().Unix()),
	)
	if err != nil {
		return "", "", err
	}
	return id, token, nil
}

// checkAppPassword verifies a token against stored app passwords
func (a *goBlog) checkAppPasswordToken(token string) (bool, error) {
	passwords, err := a.getAppPasswords()
	if err != nil {
		return false, err
	}
	for _, ap := range passwords {
		if checkPasswordHash(token, ap.TokenHash) {
			return true, nil
		}
	}
	return false, nil
}

// deleteAppPassword removes an app password by ID
func (a *goBlog) deleteAppPassword(id string) error {
	_, err := a.db.Exec("delete from app_passwords where id = @id", sql.Named("id", id))
	return err
}

// Auth migration functions

const authMigratedSettingsKey = "auth_migrated"

// isAuthMigrated checks if auth has been migrated from config to database
func (a *goBlog) isAuthMigrated() bool {
	val, err := a.getSettingValue(authMigratedSettingsKey)
	return err == nil && val == "1"
}

// setAuthMigrated marks that auth has been migrated from config to database
func (a *goBlog) setAuthMigrated() error {
	return a.saveSettingValue(authMigratedSettingsKey, "1")
}

// hasDeprecatedConfig checks if deprecated auth config options are still present
func (a *goBlog) hasDeprecatedConfig() bool {
	return a.cfg.User.Password != "" || a.cfg.User.TOTP != "" || len(a.cfg.User.AppPasswords) > 0
}

// migrateAuthFromConfig migrates authentication data from config to database
func (a *goBlog) migrateAuthFromConfig() error {
	if a.isAuthMigrated() {
		return nil // Already migrated
	}

	// Migrate password if set in config
	if a.cfg.User.Password != "" {
		hasPwd, _ := a.hasPassword()
		if !hasPwd {
			if err := a.setPassword(a.cfg.User.Password); err != nil {
				return err
			}
			a.info("Migrated password from config to database")
		}
	}

	// Migrate TOTP if set in config
	if a.cfg.User.TOTP != "" {
		hasTOTP, _ := a.hasTOTP()
		if !hasTOTP {
			if err := a.setTOTPSecret(a.cfg.User.TOTP); err != nil {
				return err
			}
			a.info("Migrated TOTP from config to database")
		}
	}

	// Migrate app passwords if set in config
	for _, apw := range a.cfg.User.AppPasswords {
		// Create an app password with the original password as token
		hash, err := hashPassword(apw.Password)
		if err != nil {
			return err
		}
		id, err := generateAppPasswordID()
		if err != nil {
			return err
		}
		_, err = a.db.Exec(
			"insert into app_passwords (id, name, token_hash, created) values (@id, @name, @hash, @created)",
			sql.Named("id", id),
			sql.Named("name", apw.Username),
			sql.Named("hash", hash),
			sql.Named("created", time.Now().Unix()),
		)
		if err != nil {
			return err
		}
		a.info("Migrated app password from config to database", "name", apw.Username)
	}

	// Migrate legacy passkey to new passkeys table
	if err := a.migrateLegacyPasskey(); err != nil {
		a.debug("Failed to migrate legacy passkey", "err", err)
	}

	// Mark as migrated
	return a.setAuthMigrated()
}

// migrateLegacyPasskey migrates the old single passkey from settings to the new passkeys table
func (a *goBlog) migrateLegacyPasskey() error {
	// Check if there's a legacy passkey
	jsonStr, err := a.getSettingValue(webauthnCredSettingsKey)
	if err != nil || jsonStr == "" {
		return nil // No legacy passkey to migrate
	}

	// Check if passkeys table already has entries
	hasPasskeys, _ := a.hasPasskeys()
	if hasPasskeys {
		// Already have passkeys, just delete the legacy one
		if err := a.deleteSettingValue(webauthnCredSettingsKey); err != nil {
			a.debug("Failed to delete legacy passkey setting", "err", err)
		}
		return nil
	}

	// Parse the legacy credential
	var cred webauthn.Credential
	if err := json.Unmarshal([]byte(jsonStr), &cred); err != nil {
		return err
	}

	// Create a new passkey from the legacy credential
	passkeyID := base64.RawURLEncoding.EncodeToString(cred.ID)
	if err := a.savePasskey(passkeyID, "Passkey", &cred); err != nil {
		return err
	}

	// Delete the legacy setting
	if err := a.deleteSettingValue(webauthnCredSettingsKey); err != nil {
		a.debug("Failed to delete legacy passkey setting after migration", "err", err)
	}
	a.info("Migrated legacy passkey to new passkeys table")
	return nil
}
