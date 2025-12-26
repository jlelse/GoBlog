package main

import (
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_passwordHashing(t *testing.T) {
	t.Run("Hash and verify password", func(t *testing.T) {
		password := "testPassword123!"
		hash, err := hashPassword(password)
		require.NoError(t, err)
		require.NotEmpty(t, hash)

		// Correct password should verify
		assert.True(t, checkPasswordHash(password, hash))

		// Wrong password should not verify
		assert.False(t, checkPasswordHash("wrongPassword", hash))
	})

	t.Run("Empty password returns empty hash", func(t *testing.T) {
		hash, err := hashPassword("")
		require.NoError(t, err)
		assert.Empty(t, hash)
	})

	t.Run("Empty hash returns false", func(t *testing.T) {
		assert.False(t, checkPasswordHash("anyPassword", ""))
	})
}

func Test_authDb_password(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	// Clear the default password to trigger automatic password generation
	app.cfg.User.Password = ""
	err := app.initConfig(false)
	require.NoError(t, err)

	t.Run("Initial state generates password automatically", func(t *testing.T) {
		// A password is now generated automatically on first run
		hasPwd, err := app.hasPassword()
		require.NoError(t, err)
		assert.True(t, hasPwd)
	})

	t.Run("Set and check password", func(t *testing.T) {
		err := app.setPassword("testPassword123")
		require.NoError(t, err)

		hasPwd, err := app.hasPassword()
		require.NoError(t, err)
		assert.True(t, hasPwd)

		valid, err := app.checkPassword("testPassword123")
		require.NoError(t, err)
		assert.True(t, valid)

		valid, err = app.checkPassword("wrongPassword")
		require.NoError(t, err)
		assert.False(t, valid)
	})
}

func Test_generateInitialPassword(t *testing.T) {
	password1, err := generateInitialPassword()
	require.NoError(t, err)
	require.NotEmpty(t, password1)

	password2, err := generateInitialPassword()
	require.NoError(t, err)
	require.NotEmpty(t, password2)

	// Each password should be unique
	assert.NotEqual(t, password1, password2)

	// Password should be 20 characters long
	assert.Equal(t, 20, len(password1))
}

func Test_authDb_totp(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	t.Run("Initial state has no TOTP", func(t *testing.T) {
		hasTOTP, err := app.hasTOTP()
		require.NoError(t, err)
		assert.False(t, hasTOTP)
	})

	t.Run("Set and check TOTP secret", func(t *testing.T) {
		testSecret := "JBSWY3DPEHPK3PXP"
		err := app.setTOTPSecret(testSecret)
		require.NoError(t, err)

		hasTOTP, err := app.hasTOTP()
		require.NoError(t, err)
		assert.True(t, hasTOTP)

		secret, err := app.getTOTPSecret()
		require.NoError(t, err)
		assert.Equal(t, testSecret, secret)
	})

	t.Run("Delete TOTP", func(t *testing.T) {
		err := app.deleteTOTP()
		require.NoError(t, err)

		hasTOTP, err := app.hasTOTP()
		require.NoError(t, err)
		assert.False(t, hasTOTP)
	})
}

func Test_authDb_passkeys(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	t.Run("Initial state has no passkeys", func(t *testing.T) {
		hasPasskeys, err := app.hasPasskeys()
		require.NoError(t, err)
		assert.False(t, hasPasskeys)

		passkeys, err := app.getPasskeys()
		require.NoError(t, err)
		assert.Len(t, passkeys, 0)
	})

	t.Run("Save and retrieve passkey", func(t *testing.T) {
		testCred := &webauthn.Credential{
			ID:              []byte("test-credential-id"),
			PublicKey:       []byte("test-public-key"),
			AttestationType: "none",
		}

		err := app.savePasskey("pk1", "My First Passkey", testCred)
		require.NoError(t, err)

		hasPasskeys, err := app.hasPasskeys()
		require.NoError(t, err)
		assert.True(t, hasPasskeys)

		passkeys, err := app.getPasskeys()
		require.NoError(t, err)
		require.Len(t, passkeys, 1)
		assert.Equal(t, "pk1", passkeys[0].ID)
		assert.Equal(t, "My First Passkey", passkeys[0].Name)
	})

	t.Run("Rename passkey", func(t *testing.T) {
		err := app.renamePasskey("pk1", "Renamed Passkey")
		require.NoError(t, err)

		pk, err := app.getPasskey("pk1")
		require.NoError(t, err)
		require.NotNil(t, pk)
		assert.Equal(t, "Renamed Passkey", pk.Name)
	})

	t.Run("Add multiple passkeys", func(t *testing.T) {
		testCred2 := &webauthn.Credential{
			ID:              []byte("test-credential-id-2"),
			PublicKey:       []byte("test-public-key-2"),
			AttestationType: "none",
		}

		err := app.savePasskey("pk2", "My Second Passkey", testCred2)
		require.NoError(t, err)

		passkeys, err := app.getPasskeys()
		require.NoError(t, err)
		assert.Len(t, passkeys, 2)
	})

	t.Run("Get WebAuthn credentials", func(t *testing.T) {
		creds, err := app.getWebAuthnCredentials()
		require.NoError(t, err)
		assert.Len(t, creds, 2)
	})

	t.Run("Delete passkey", func(t *testing.T) {
		err := app.deletePasskeyByID("pk1")
		require.NoError(t, err)

		passkeys, err := app.getPasskeys()
		require.NoError(t, err)
		assert.Len(t, passkeys, 1)
		assert.Equal(t, "pk2", passkeys[0].ID)
	})
}

func Test_authDb_appPasswords(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	t.Run("Initial state has no app passwords", func(t *testing.T) {
		passwords, err := app.getAppPasswords()
		require.NoError(t, err)
		assert.Len(t, passwords, 0)
	})

	t.Run("Create app password", func(t *testing.T) {
		id, token, err := app.createAppPassword("My App")
		require.NoError(t, err)
		require.NotEmpty(t, id)
		require.NotEmpty(t, token)

		passwords, err := app.getAppPasswords()
		require.NoError(t, err)
		require.Len(t, passwords, 1)
		assert.Equal(t, "My App", passwords[0].Name)
	})

	t.Run("Check app password", func(t *testing.T) {
		id, password, err := app.createAppPassword("Another App")
		require.NoError(t, err)
		require.NotEmpty(t, id)

		// Correct password should verify
		valid, err := app.checkAppPassword(password)
		require.NoError(t, err)
		assert.True(t, valid)

		// Wrong password should not verify
		valid, err = app.checkAppPassword("wrong-password")
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("Delete app password", func(t *testing.T) {
		passwords, err := app.getAppPasswords()
		require.NoError(t, err)
		initialCount := len(passwords)

		err = app.deleteAppPassword(passwords[0].ID)
		require.NoError(t, err)

		passwords, err = app.getAppPasswords()
		require.NoError(t, err)
		assert.Len(t, passwords, initialCount-1)
	})
}

func Test_totpEnabled(t *testing.T) {
	// Note: After migration is complete, config-based TOTP is ignored
	// This test verifies that database-based TOTP works correctly

	t.Run("TOTP disabled when no secret in db", func(t *testing.T) {
		app := &goBlog{
			cfg: createDefaultTestConfig(t),
		}
		// Clear password so no migration happens
		app.cfg.User.Password = ""
		err := app.initConfig(false)
		require.NoError(t, err)
		assert.False(t, app.totpEnabled())
	})

	t.Run("TOTP enabled when secret in database", func(t *testing.T) {
		app := &goBlog{
			cfg: createDefaultTestConfig(t),
		}
		app.cfg.User.Password = ""
		err := app.initConfig(false)
		require.NoError(t, err)

		err = app.setTOTPSecret("JBSWY3DPEHPK3PXP")
		require.NoError(t, err)
		assert.True(t, app.totpEnabled())
	})

	t.Run("Config TOTP migrates to database", func(t *testing.T) {
		app := &goBlog{
			cfg: createDefaultTestConfig(t),
		}
		app.cfg.User.Password = ""
		app.cfg.User.TOTP = "TESTMIGRATION"
		err := app.initConfig(false)
		require.NoError(t, err)

		// Should be enabled because it was migrated to database
		assert.True(t, app.totpEnabled())

		// Verify it's in the database
		secret, err := app.getTOTPSecret()
		require.NoError(t, err)
		assert.Equal(t, "TESTMIGRATION", secret)
	})
}

func Test_generateAppPassword(t *testing.T) {
	password1, err := generateAppPassword()
	require.NoError(t, err)
	require.NotEmpty(t, password1)

	password2, err := generateAppPassword()
	require.NoError(t, err)
	require.NotEmpty(t, password2)

	// Each password should be unique
	assert.NotEqual(t, password1, password2)

	// Password should be 40 characters long
	assert.Equal(t, 40, len(password1))
}
