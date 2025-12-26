package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
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
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
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

// passkey represents a stored WebAuthn credential
type passkey struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Credential string    `json:"credential"`
	Created    time.Time `json:"created"`
}

// getPasskeys returns all stored passkeys
func (a *goBlog) getPasskeys() ([]*passkey, error) {
	rows, err := a.db.Query("select id, name, credential, created from passkeys order by created desc")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var passkeys []*passkey
	for rows.Next() {
		var pk passkey
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
func (a *goBlog) getPasskey(id string) (*passkey, error) {
	row, err := a.db.QueryRow("select id, name, credential, created from passkeys where id = @id", sql.Named("id", id))
	if err != nil {
		return nil, err
	}
	var pk passkey
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

// appPassword represents a stored app password
type appPassword struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
}

// generateAppPassword creates a secure random app password
func generateAppPassword() (string, error) {
	return generateSecurePassword(40)
}

// getAppPasswords returns all stored app passwords
func (a *goBlog) getAppPasswords() ([]*appPassword, error) {
	rows, err := a.db.Query("select id, name, created from app_passwords order by created desc")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var passwords []*appPassword
	for rows.Next() {
		var ap appPassword
		var createdUnix int64
		err = rows.Scan(&ap.ID, &ap.Name, &createdUnix)
		if err != nil {
			return nil, err
		}
		ap.Created = time.Unix(createdUnix, 0)
		passwords = append(passwords, &ap)
	}
	return passwords, nil
}

// getAppPasswordHashes returns all stored app password hashes
func (a *goBlog) getAppPasswordHashes() ([]string, error) {
	rows, err := a.db.Query("select hash from app_passwords")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hashes []string
	for rows.Next() {
		var hash string
		err = rows.Scan(&hash)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

// createAppPassword creates a new app password and returns the plaintext password (only shown once)
func (a *goBlog) createAppPassword(name string) (id, password string, err error) {
	id = uuid.NewString()
	password, err = generateAppPassword()
	if err != nil {
		return "", "", err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return "", "", err
	}
	_, err = a.db.Exec(
		"insert into app_passwords (id, name, hash, created) values (@id, @name, @hash, @created)",
		sql.Named("id", id),
		sql.Named("name", name),
		sql.Named("hash", hash),
		sql.Named("created", time.Now().Unix()),
	)
	if err != nil {
		return "", "", err
	}
	return id, password, nil
}

// checkAppPassword verifies a password against stored app passwords
func (a *goBlog) checkAppPassword(password string) (bool, error) {
	passwordHashes, err := a.getAppPasswordHashes()
	if err != nil {
		return false, err
	}
	for _, hash := range passwordHashes {
		if checkPasswordHash(password, hash) {
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
	migrated, _ := a.getBooleanSettingValue(authMigratedSettingsKey, false)
	return migrated
}

// setAuthMigrated marks that auth has been migrated from config to database
func (a *goBlog) setAuthMigrated() error {
	return a.saveBooleanSettingValue(authMigratedSettingsKey, true)
}

// hasDeprecatedConfig checks if deprecated auth config options are still present
func (a *goBlog) hasDeprecatedConfig() bool {
	return a.cfg.User.Password != "" || a.cfg.User.TOTP != "" || len(a.cfg.User.AppPasswords) > 0
}

// generateInitialPassword generates a secure random password for first-time setup
func generateInitialPassword() (string, error) {
	return generateSecurePassword(20)
}

// migrateAuthFromConfig migrates authentication data from config to database
func (a *goBlog) migrateAuthFromConfig(logging bool) error {
	if a.isAuthMigrated() {
		return nil // Already migrated
	}

	// Migrate password if set in config, or generate a new one on first run
	hasPwd, _ := a.hasPassword()
	if !hasPwd {
		if a.cfg.User.Password != "" {
			// Migrate password from config
			if err := a.setPassword(a.cfg.User.Password); err != nil {
				return err
			}
			if logging {
				a.info("Migrated password from config to database")
			}
		} else {
			// Generate a secure initial password on first run
			initialPassword, err := generateInitialPassword()
			if err != nil {
				return err
			}
			if err := a.setPassword(initialPassword); err != nil {
				return err
			}
			if logging {
				a.info("Generated initial password for first-time setup. Please change it via Settings or CLI.",
					"username", a.cfg.User.Nick,
					"password", initialPassword,
				)
			}
		}
	}

	// Migrate TOTP if set in config
	if a.cfg.User.TOTP != "" {
		hasTOTP, _ := a.hasTOTP()
		if !hasTOTP {
			if err := a.setTOTPSecret(a.cfg.User.TOTP); err != nil {
				return err
			}
			if logging {
				a.info("Migrated TOTP from config to database")
			}
		}
	}

	// Migrate app passwords if set in config
	for _, apw := range a.cfg.User.AppPasswords {
		hash, err := hashPassword(apw.Password)
		if err != nil {
			return err
		}
		_, err = a.db.Exec(
			"insert into app_passwords (id, name, hash, created) values (@id, @name, @hash, @created)",
			sql.Named("id", uuid.NewString()),
			sql.Named("name", apw.Username),
			sql.Named("hash", hash),
			sql.Named("created", time.Now().Unix()),
		)
		if err != nil {
			return err
		}
		if logging {
			a.info("Migrated app password from config to database", "name", apw.Username)
		}
	}

	// Migrate legacy passkey to new passkeys table
	if err := a.migrateLegacyPasskey(); err != nil {
		a.error("Failed to migrate legacy passkey", "err", err)
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
			a.error("Failed to delete legacy passkey setting", "err", err)
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
		a.error("Failed to delete legacy passkey setting after migration", "err", err)
	}
	a.info("Migrated legacy passkey to new passkeys table")
	return nil
}
