package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// selfSignedCert generates a minimal self-signed ECDSA certificate for testing
// and returns the PEM bundle in autocert-compatible format: key block first,
// then certificate block(s).
func selfSignedCert(t *testing.T, domain string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		DNSNames:     []string{domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(90 * 24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return append(keyPEM, certPEM...)
}

func Test_acmeCertCache(t *testing.T) {
	app := &goBlog{cfg: &config{}}
	db, err := app.openDatabase("file::memory:?cache=shared", false, false)
	require.NoError(t, err)
	cm := &certManager{db: db}

	t.Run("Round-trip", func(t *testing.T) {
		bundle := selfSignedCert(t, "example.com")
		require.NoError(t, cm.saveCert("example.com", bundle))

		loaded, err := cm.loadCert("example.com")
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Contains(t, loaded.tlsCert.Leaf.DNSNames, "example.com")
		assert.True(t, time.Now().Before(loaded.tlsCert.Leaf.NotAfter))
		assert.NotEmpty(t, loaded.privateKeyPEM)
		assert.NotEmpty(t, loaded.certPEM)
	})

	t.Run("Autocert compatibility", func(t *testing.T) {
		// Simulate a certificate stored by the old autocert-based implementation:
		// same PEM bundle format, same "https_" DB key prefix.
		bundle := selfSignedCert(t, "legacy.example.com")
		require.NoError(t, db.cachePersistently("https_legacy.example.com", bundle))

		loaded, err := cm.loadCert("legacy.example.com")
		require.NoError(t, err)
		require.NotNil(t, loaded)
		assert.Contains(t, loaded.tlsCert.Leaf.DNSNames, "legacy.example.com")
	})

	t.Run("Missing entry returns nil", func(t *testing.T) {
		loaded, err := cm.loadCert("notfound.example.com")
		require.NoError(t, err)
		assert.Nil(t, loaded)
	})
}

func Test_renewOrObtainCert(t *testing.T) {
	t.Run("Uses renewal when possible", func(t *testing.T) {
		renewed := &cachedCert{}
		renewCalls := 0
		obtainCalls := 0

		cert, err := renewOrObtainCert("example.com", &cachedCert{}, func(host string, current *cachedCert) (*cachedCert, error) {
			renewCalls++
			require.Equal(t, "example.com", host)
			require.NotNil(t, current)
			return renewed, nil
		}, func(host string) (*cachedCert, error) {
			obtainCalls++
			return nil, nil
		})
		require.NoError(t, err)
		require.Same(t, renewed, cert)
		assert.Equal(t, 1, renewCalls)
		assert.Equal(t, 0, obtainCalls)
	})

	t.Run("Falls back to obtain when renewal fails", func(t *testing.T) {
		obtained := &cachedCert{}
		renewCalls := 0
		obtainCalls := 0

		cert, err := renewOrObtainCert("example.com", &cachedCert{}, func(host string, current *cachedCert) (*cachedCert, error) {
			renewCalls++
			return nil, errors.New("renew failed")
		}, func(host string) (*cachedCert, error) {
			obtainCalls++
			return obtained, nil
		})
		require.NoError(t, err)
		require.Same(t, obtained, cert)
		assert.Equal(t, 1, renewCalls)
		assert.Equal(t, 1, obtainCalls)
	})

	t.Run("Obtains directly without existing certificate", func(t *testing.T) {
		obtained := &cachedCert{}
		obtainCalls := 0

		cert, err := renewOrObtainCert("example.com", nil, func(host string, current *cachedCert) (*cachedCert, error) {
			t.Fatal("renew should not be called without existing certificate")
			return nil, nil
		}, func(host string) (*cachedCert, error) {
			obtainCalls++
			return obtained, nil
		})
		require.NoError(t, err)
		require.Same(t, obtained, cert)
		assert.Equal(t, 1, obtainCalls)
	})
}
