package main

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_loadActivityPubPrivateKey(t *testing.T) {

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	err := app.initConfig(false)
	require.NoError(t, err)
	require.NotNil(t, app.db)

	// Generate
	err = app.loadActivityPubPrivateKey()
	require.NoError(t, err)

	assert.NotNil(t, app.apPrivateKey)
	assert.NotEmpty(t, app.apPubKeyBytes)

	oldEncodedKey := x509.MarshalPKCS1PrivateKey(app.apPrivateKey)
	oldPemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: oldEncodedKey})

	// Reset and reload
	err = app.loadActivityPubPrivateKey()
	require.NoError(t, err)

	assert.NotNil(t, app.apPrivateKey)
	assert.NotEmpty(t, app.apPubKeyBytes)

	newEncodedKey := x509.MarshalPKCS1PrivateKey(app.apPrivateKey)
	newPemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: newEncodedKey})

	assert.Equal(t, string(oldPemEncoded), string(newPemEncoded))

}

func Test_webfinger(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)

	app.prepareWebfinger()

	req := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:default@example.com", nil)
	rec := httptest.NewRecorder()

	app.apHandleWebfinger(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
