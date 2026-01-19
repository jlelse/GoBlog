//go:build integration
// +build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestActivityPubWithGoToSocial(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ActivityPub integration test in short mode")
	}
	requireDocker(t)

	goBlogPort := getFreePort(t)
	gtsPort := getFreePort(t)

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: newHttpClient(),
	}
	app.cfg.Server.PublicAddress = fmt.Sprintf("http://host.docker.internal:%d", goBlogPort)
	app.cfg.Server.Port = goBlogPort
	app.cfg.ActivityPub.Enabled = true
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateStrings())
	require.NoError(t, app.initActivityPub())
	app.reloadRouter()

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", goBlogPort),
		Handler:           app.d,
		ReadHeaderTimeout: time.Minute,
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", goBlogPort))
	require.NoError(t, err)
	app.shutdown.Add(app.shutdownServer(server, "integration server"))
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		app.shutdown.ShutdownAndWait()
	})

	webfingerURL := fmt.Sprintf("http://127.0.0.1:%d/.well-known/webfinger?resource=acct:%s@%s", goBlogPort, app.cfg.DefaultBlog, app.cfg.Server.publicHostname)
	waitForHTTP(t, webfingerURL, 2*time.Minute)

	gtsDir := t.TempDir()
	gtsDataDir := filepath.Join(gtsDir, "data")
	gtsStorageDir := filepath.Join(gtsDataDir, "storage")
	require.NoError(t, os.MkdirAll(gtsStorageDir, 0o777))
	require.NoError(t, os.Chmod(gtsDataDir, 0o777))
	require.NoError(t, os.Chmod(gtsStorageDir, 0o777))
	gtsConfigPath := filepath.Join(gtsDir, "config.yaml")
	gtsConfig := fmt.Sprintf(`host: "127.0.0.1"
protocol: "http"
bind-address: "0.0.0.0"
port: %d
db-type: "sqlite"
db-address: "/data/sqlite.db"
storage-local-base-path: "/data/storage"
`, gtsPort)
	require.NoError(t, os.WriteFile(gtsConfigPath, []byte(gtsConfig), 0o644))

	containerName := fmt.Sprintf("goblog-gts-%d", time.Now().UnixNano())
	runDocker(t,
		"run", "-d", "--rm",
		"--name", containerName,
		"--add-host", "host.docker.internal:host-gateway",
		"-p", fmt.Sprintf("%d:%d", gtsPort, gtsPort),
		"-v", fmt.Sprintf("%s:/config/config.yaml", gtsConfigPath),
		"-v", fmt.Sprintf("%s:/data", gtsDataDir),
		"docker.io/superseriousbusiness/gotosocial:latest",
		"--config-path", "/config/config.yaml", "server", "start",
	)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	gtsBaseURL := fmt.Sprintf("http://127.0.0.1:%d", gtsPort)
	waitForHTTP(t, gtsBaseURL+"/api/v1/instance", 10*time.Minute)

	gtsPassword := "GtsPassword123!@#"
	runDocker(t,
		"exec", containerName,
		"/gotosocial/gotosocial",
		"--config-path", "/config/config.yaml",
		"admin", "account", "create",
		"--username", "gtsuser",
		"--email", "gtsuser@example.com",
		"--password", gtsPassword,
	)

	httpClient := &http.Client{Timeout: time.Minute}
	clientID, clientSecret := gtsRegisterApp(t, httpClient, gtsBaseURL)
	accessToken := gtsPasswordToken(t, httpClient, gtsBaseURL, clientID, clientSecret, gtsPassword)

	goBlogActor := fmt.Sprintf("http://host.docker.internal:%d", goBlogPort)
	accountID := gtsFollow(t, httpClient, gtsBaseURL, accessToken, goBlogActor)

	require.Eventually(t, func() bool {
		followers, err := app.db.apGetAllFollowers(app.cfg.DefaultBlog)
		if err != nil || len(followers) != 1 {
			return false
		}
		return strings.Contains(followers[0].follower, "/users/gtsuser")
	}, 2*time.Minute, 2*time.Second)

	post := &post{
		Content: "Hello from GoBlog via GoToSocial!",
	}
	require.NoError(t, app.createPost(post))
	postURL := app.fullPostURL(post)

	require.Eventually(t, func() bool {
		statuses, err := gtsAccountStatuses(t, httpClient, gtsBaseURL, accessToken, accountID)
		if err != nil {
			return false
		}
		for _, status := range statuses {
			if status.URI == postURL || status.URL == postURL || strings.Contains(status.Content, "Hello from GoBlog via GoToSocial!") {
				return true
			}
		}
		return false
	}, 3*time.Minute, 5*time.Second)
}

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
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, nil)
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

type gtsAppResponse struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func gtsRegisterApp(t *testing.T, client *http.Client, baseURL string) (string, string) {
	t.Helper()
	values := url.Values{
		"client_name":   {"goblog-activitypub-test"},
		"redirect_uris": {"urn:ietf:wg:oauth:2.0:oob"},
		"scopes":        {"read write follow"},
		"website":       {"https://goblog.app"},
	}
	resp := doFormRequest(t, client, http.MethodPost, baseURL+"/api/v1/apps", values, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var payload gtsAppResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.NotEmpty(t, payload.ClientID)
	require.NotEmpty(t, payload.ClientSecret)
	return payload.ClientID, payload.ClientSecret
}

type gtsTokenResponse struct {
	AccessToken string `json:"access_token"`
}

func gtsPasswordToken(t *testing.T, client *http.Client, baseURL, clientID, clientSecret, password string) string {
	t.Helper()
	values := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"password"},
		"redirect_uri":  {"urn:ietf:wg:oauth:2.0:oob"},
		"username":      {"gtsuser"},
		"password":      {password},
		"scope":         {"read write follow"},
	}
	resp := doFormRequest(t, client, http.MethodPost, baseURL+"/oauth/token", values, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var payload gtsTokenResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.NotEmpty(t, payload.AccessToken)
	return payload.AccessToken
}

func gtsFollow(t *testing.T, client *http.Client, baseURL, token, target string) string {
	t.Helper()
	values := url.Values{
		"uri": {target},
	}
	resp := doFormRequest(t, client, http.MethodPost, baseURL+"/api/v1/follows", values, token)
	defer resp.Body.Close()
	require.Contains(t, []int{http.StatusOK, http.StatusAccepted}, resp.StatusCode)
	var payload gtsAccount
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.NotEmpty(t, payload.ID)
	return payload.ID
}

type gtsAccount struct {
	ID string `json:"id"`
}

type gtsStatus struct {
	URI     string `json:"uri"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

func gtsAccountStatuses(t *testing.T, client *http.Client, baseURL, token, accountID string) ([]gtsStatus, error) {
	t.Helper()
	endpoint := fmt.Sprintf("%s/api/v1/accounts/%s/statuses", baseURL, accountID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("account statuses status %d: %s", resp.StatusCode, string(body))
	}
	var statuses []gtsStatus
	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func doFormRequest(t *testing.T, client *http.Client, method, endpoint string, values url.Values, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, endpoint, strings.NewReader(values.Encode()))
	require.NoError(t, err)
	req.Header.Set(contentType, "application/x-www-form-urlencoded")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}
