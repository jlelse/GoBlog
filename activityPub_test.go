package main

import (
	"crypto/x509"
	"encoding/pem"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_loadActivityPubPrivateKey(t *testing.T) {

	app := &goBlog{
		cfg: &config{
			Db: &configDb{
				File: filepath.Join(t.TempDir(), "test.db"),
			},
		},
	}
	_ = app.initDatabase(false)

	// Generate
	err := app.loadActivityPubPrivateKey()
	require.NoError(t, err)

	assert.NotNil(t, app.apPrivateKey)

	oldEncodedKey := x509.MarshalPKCS1PrivateKey(app.apPrivateKey)
	oldPemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: oldEncodedKey})

	// Reset and reload
	err = app.loadActivityPubPrivateKey()
	require.NoError(t, err)

	assert.NotNil(t, app.apPrivateKey)

	newEncodedKey := x509.MarshalPKCS1PrivateKey(app.apPrivateKey)
	newPemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: newEncodedKey})

	assert.Equal(t, string(oldPemEncoded), string(newPemEncoded))

}
