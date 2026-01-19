//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

const (
	gtsTestEmail    = "gtsuser@example.com"
	gtsTestPassword = "GtsPassword123!@#"
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

	gtsEmail := gtsTestEmail
	gtsPassword := gtsTestPassword
	runDocker(t,
		"exec", containerName,
		"/gotosocial/gotosocial",
		"--config-path", "/config/config.yaml",
		"admin", "account", "create",
		"--username", "gtsuser",
		"--email", gtsEmail,
		"--password", gtsPassword,
	)

	httpClient := &http.Client{Timeout: time.Minute}
	clientID, clientSecret := gtsRegisterApp(t, httpClient, gtsBaseURL)
	accessToken := gtsAuthorizeToken(t, gtsBaseURL, clientID, clientSecret, gtsEmail, gtsPassword)

	goBlogAcct := fmt.Sprintf("%s@%s", app.cfg.DefaultBlog, app.cfg.Server.publicHostname)
	waitForHTTP(t, webfingerURL, 2*time.Minute)
	lookup := gtsLookupAccount(t, httpClient, gtsBaseURL, accessToken, goBlogAcct)
	gtsFollowAccount(t, httpClient, gtsBaseURL, accessToken, lookup.ID)

	require.Eventually(t, func() bool {
		app.cfg.Server.PublicAddress = fmt.Sprintf("http://127.0.0.1:%d", goBlogPort)
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
		statuses, err := gtsAccountStatuses(t, httpClient, gtsBaseURL, accessToken, lookup.ID)
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

type gtsLookupResponse struct {
	ID string `json:"id"`
}

func gtsLookupAccount(t *testing.T, client *http.Client, baseURL, token, acct string) gtsLookupResponse {
	t.Helper()
	query := url.Values{
		"acct": {acct},
	}
	resp := doFormRequest(t, client, http.MethodGet, baseURL+"/api/v1/accounts/lookup?"+query.Encode(), nil, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return gtsLookupResponse{}
	}
	var payload gtsLookupResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	return payload
}

func gtsFollowAccount(t *testing.T, client *http.Client, baseURL, token, accountID string) {
	t.Helper()
	if accountID == "" {
		t.Skip("gotosocial account lookup not available")
	}
	resp := doFormRequest(t, client, http.MethodPost, fmt.Sprintf("%s/api/v1/accounts/%s/follow", baseURL, accountID), url.Values{}, token)
	defer resp.Body.Close()
	require.Contains(t, []int{http.StatusOK, http.StatusAccepted}, resp.StatusCode)
}

func gtsAuthorizeToken(t *testing.T, baseURL, clientID, clientSecret, email, password string) string {
	t.Helper()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{
		Timeout: time.Minute,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	query := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {"urn:ietf:wg:oauth:2.0:oob"},
		"response_type": {"code"},
		"scope":         {"read write follow"},
	}
	authURL := baseURL + "/oauth/authorize?" + query.Encode()

	resp := doFormRequest(t, client, http.MethodGet, authURL, nil, "")
	defer resp.Body.Close()
	require.Contains(t, []int{http.StatusFound, http.StatusSeeOther}, resp.StatusCode)
	signInURL := resp.Header.Get("Location")
	require.NotEmpty(t, signInURL)
	if strings.HasPrefix(signInURL, "/") {
		signInURL = baseURL + signInURL
	}

	signInValues := url.Values{
		"username": {email},
		"password": {password},
	}
	resp = doFormRequest(t, client, http.MethodPost, signInURL, signInValues, "")
	defer resp.Body.Close()
	require.Contains(t, []int{http.StatusFound, http.StatusSeeOther}, resp.StatusCode)

	authorizeURL := resp.Header.Get("Location")
	require.NotEmpty(t, authorizeURL)
	if strings.HasPrefix(authorizeURL, "/") {
		authorizeURL = baseURL + authorizeURL
	}

	resp = doFormRequest(t, client, http.MethodGet, authorizeURL, nil, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = doFormRequest(t, client, http.MethodPost, authorizeURL, url.Values{}, "")
	defer resp.Body.Close()
	require.Contains(t, []int{http.StatusFound, http.StatusSeeOther}, resp.StatusCode)
	oobURL := resp.Header.Get("Location")
	require.NotEmpty(t, oobURL)
	if strings.HasPrefix(oobURL, "/") {
		oobURL = baseURL + oobURL
	}

	resp = doFormRequest(t, client, http.MethodGet, oobURL, nil, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	code := strings.TrimSpace(extractCode(string(body)))
	require.NotEmpty(t, code)

	tokenValues := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {"urn:ietf:wg:oauth:2.0:oob"},
		"code":          {code},
	}
	resp = doFormRequest(t, client, http.MethodPost, baseURL+"/oauth/token", tokenValues, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var payload gtsTokenResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.NotEmpty(t, payload.AccessToken)
	return payload.AccessToken
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
	var body io.Reader
	if values != nil {
		body = strings.NewReader(values.Encode())
	}
	req, err := http.NewRequestWithContext(context.Background(), method, endpoint, body)
	require.NoError(t, err)
	if values != nil {
		req.Header.Set(contentType, "application/x-www-form-urlencoded")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	return resp
}

func extractCode(body string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return ""
	}
	code := strings.TrimSpace(doc.Find("code").First().Text())
	if code == "" {
		return ""
	}
	return code
}
