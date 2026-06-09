//go:build !skipIntegration

package main

import (
	"net"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/stretchr/testify/require"
)

func requireDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not installed")
	}
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skipf("docker not available: %v", err)
	}
}

func getFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func runDocker(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "docker %s: %s", strings.Join(args, " "), string(output))
	return strings.TrimSpace(string(output))
}

func waitForHTTP(t *testing.T, endpoint string, timeout time.Duration) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	require.Eventually(t, func() bool {
		req, err := requests.URL(endpoint).Method(http.MethodGet).Request(t.Context())
		if err != nil {
			return false
		}
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusInternalServerError
	}, timeout, 2*time.Second)
}
