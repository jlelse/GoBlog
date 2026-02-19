//go:build !skipIntegration

package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationAcmeWithPebble(t *testing.T) {
	t.Parallel()

	requireDocker(t)

	// Allocate the HTTPS port before starting Pebble so we can configure it as
	// Pebble's tlsPort for TLS-ALPN-01 validation.
	goblogPort := getFreePort(t)
	tlsAddr := fmt.Sprintf("127.0.0.1:%d", goblogPort)

	// Start Pebble configured to validate TLS-ALPN-01 challenges directly on the host
	// (goblog.example resolves to host-gateway inside Pebble's container).
	pebbleDirURL := startPebble(t, "goblog.example", goblogPort)
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
		// Use an HTTP client that trusts Pebble's self-signed TLS
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 30 * time.Second,
		},
	}
	app.cfg.Server.PublicAddress = "https://goblog.example"
	app.cfg.Server.Port = goblogPort
	app.cfg.Server.PublicHTTPS = true
	app.cfg.Server.AcmeDir = pebbleDirURL

	require.NoError(t, app.initConfig(false))
	// Disable HTTP redirect server — TLS-ALPN-01 doesn't need port 80
	app.cfg.Server.HttpsRedirect = false
	require.NoError(t, app.initTemplateStrings())

	// Pre-initialize the cert manager with the Pebble-configured HTTP client
	cm := app.getCertManager()
	require.NotNil(t, cm)

	// Use GoBlog's own startServer which sets up the full middleware stack and
	// the HTTPS server. TLS-ALPN-01 challenges are served on the HTTPS port
	// itself, so no separate challenge server is needed.
	go func() { _ = app.startServer() }()

	// Wait for the HTTPS server to be reachable before running subtests
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", tlsAddr, time.Second)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, time.Minute, time.Second, "HTTPS server should be reachable")

	dialTLS := func(serverName string) (*tls.Conn, error) {
		return tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", tlsAddr, &tls.Config{
			ServerName:         serverName,
			InsecureSkipVerify: true, // Pebble issues certs from its own CA
		})
	}
	var initialSerial string

	t.Run("Certificate issuance", func(t *testing.T) {
		// Trigger a TLS handshake which causes the cert manager to obtain a certificate from Pebble
		var conn *tls.Conn
		require.Eventually(t, func() bool {
			c, dialErr := dialTLS("goblog.example")
			if dialErr != nil {
				t.Logf("TLS dial attempt: %v", dialErr)
				return false
			}
			conn = c
			return true
		}, time.Minute, time.Second, "Should eventually obtain a TLS certificate")
		require.NotNil(t, conn)
		defer conn.Close()

		// Verify the certificate has the expected SAN
		state := conn.ConnectionState()
		require.NotEmpty(t, state.PeerCertificates)
		cert := state.PeerCertificates[0]
		assert.Contains(t, cert.DNSNames, "goblog.example", "Certificate should contain goblog.example SAN")
		initialSerial = cert.SerialNumber.String()
		require.NotEmpty(t, initialSerial)
	})

	t.Run("Certificate renewal", func(t *testing.T) {
		require.NotEmpty(t, initialSerial)

		currentCert, err := cm.loadCert("goblog.example")
		require.NoError(t, err)
		require.NotNil(t, currentCert)

		// Wait two seconds to ensure the renewed cert has a different NotAfter timestamp
		time.Sleep(2 * time.Second)

		renewedCert, err := cm.renewCert("goblog.example", currentCert)
		require.NoError(t, err)
		require.NotNil(t, renewedCert)
		cm.certCache.Store("goblog.example", renewedCert)

		persistedCert, err := cm.loadCert("goblog.example")
		require.NoError(t, err)
		require.NotNil(t, persistedCert)
		assert.Equal(t, renewedCert.tlsCert.Leaf.SerialNumber.String(), persistedCert.tlsCert.Leaf.SerialNumber.String(), "Renewed certificate should have the same serial")
		assert.NotEqual(t, renewedCert.tlsCert.Leaf.NotAfter, currentCert.tlsCert.Leaf.NotAfter, "Renewed cert should have different validity")

		assert.Eventually(t, func() bool {
			conn, dialErr := dialTLS("goblog.example")
			if dialErr != nil {
				t.Logf("TLS dial attempt after renewal: %v", dialErr)
				return false
			}
			defer conn.Close()
			state := conn.ConnectionState()
			if len(state.PeerCertificates) == 0 {
				return false
			}
			servedNotAfter := state.PeerCertificates[0].NotAfter
			return servedNotAfter.Equal(renewedCert.tlsCert.Leaf.NotAfter)
		}, time.Minute, time.Second, "HTTPS should serve the renewed certificate")
	})

	t.Run("HTTPS serves content", func(t *testing.T) {
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					ServerName:         "goblog.example",
					InsecureSkipVerify: true,
				},
			},
			Timeout: 15 * time.Second,
		}

		require.Eventually(t, func() bool {
			resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/", goblogPort))
			if err != nil {
				return false
			}
			defer resp.Body.Close()
			return resp.StatusCode == http.StatusOK
		}, time.Minute, time.Second, "HTTPS should serve content after certificate issuance")
	})

	t.Run("Host whitelist rejects unknown hosts", func(t *testing.T) {
		conn, err := dialTLS("unknown.example")
		if conn != nil {
			conn.Close()
		}
		assert.Error(t, err, "Connection with non-whitelisted host should be rejected")
	})

	t.Run("Certificate cache persistence", func(t *testing.T) {
		// Verify that the certificate is cached in the database
		data, err := app.db.retrievePersistentCache("https_goblog.example")
		require.NoError(t, err)
		assert.NotNil(t, data, "Certificate data should be cached in the database")
		assert.Greater(t, len(data), 0, "Cached certificate data should not be empty")
	})

	t.Run("Account persistence", func(t *testing.T) {
		// Verify that the ACME account key is persisted; the registration is
		// recovered on demand via ResolveAccountByKey() and not stored locally.
		keyData, err := app.db.retrievePersistentCache(acmeAccountKeyDBKey)
		require.NoError(t, err)
		assert.NotNil(t, keyData, "ACME account key should be persisted")
	})

	t.Cleanup(func() {
		cm.Close()
		app.shutdown.ShutdownAndWait()
	})
}

// startPebble starts a Pebble ACME test server on the given Docker network.
// hostname is mapped to host-gateway inside the Pebble container so Pebble can
// reach GoBlog's HTTPS server directly on the host.
// tlsChallengePort is GoBlog's HTTPS port; Pebble uses it for TLS-ALPN-01 validation.
// It returns the ACME directory URL accessible from the host.
func startPebble(t *testing.T, hostname string, tlsChallengePort int) (dirURL string) {
	t.Helper()

	// Create Docker network
	netName := fmt.Sprintf("goblog-acme-net-%s", uuid.New().String())
	runDocker(t, "network", "create", netName)
	t.Cleanup(func() {
		_ = exec.Command("docker", "network", "rm", netName).Run()
	})

	containerName := fmt.Sprintf("goblog-pebble-%s", uuid.New().String())
	wfePort := getFreePort(t)
	mgmtPort := getFreePort(t)

	// Write a custom config so Pebble validates TLS-ALPN-01 on our dynamically-chosen port.
	pebbleConfig := fmt.Sprintf(`{
  "pebble": {
    "listenAddress": "0.0.0.0:14000",
    "managementListenAddress": "0.0.0.0:15000",
    "certificate": "test/certs/localhost/cert.pem",
    "privateKey": "test/certs/localhost/key.pem",
    "httpPort": 5002,
    "tlsPort": %d,
    "ocspResponderURL": "",
    "externalAccountBindingRequired": false
  }
}`, tlsChallengePort)
	configFile := filepath.Join(t.TempDir(), "pebble-config.json")
	require.NoError(t, os.WriteFile(configFile, []byte(pebbleConfig), 0644))

	runDocker(t,
		"run", "-d", "--rm",
		"--name", containerName,
		"--network", netName,
		"--network-alias", "pebble",
		// Resolve the challenge hostname to the host so Pebble can reach GoBlog directly
		"--add-host", fmt.Sprintf("%s:host-gateway", hostname),
		"-p", fmt.Sprintf("127.0.0.1:%d:14000", wfePort),
		"-p", fmt.Sprintf("127.0.0.1:%d:15000", mgmtPort),
		"-e", "PEBBLE_VA_NOSLEEP=1",
		"-e", "PEBBLE_WFE_NONCEREJECT=0",
		"-v", fmt.Sprintf("%s:/custom-pebble-config.json:ro", configFile),
		"ghcr.io/letsencrypt/pebble:latest",
		"-config", "/custom-pebble-config.json",
	)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	// Wait for Pebble to be ready
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 2 * time.Second,
	}
	dirURL = fmt.Sprintf("https://127.0.0.1:%d/dir", wfePort)
	require.Eventually(t, func() bool {
		resp, err := client.Get(dirURL)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, time.Minute, time.Second, "Pebble should become ready")

	return dirURL
}
