//go:build !skipIntegration

package main

import (
	"context"
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
	"github.com/carlmjohnson/requests"
	"github.com/mattn/go-mastodon"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/bufferpool"
)

const (
	gtsTestEmail    = "gtsuser@example.com"
	gtsTestUsername = "gtsuser"
	gtsTestPassword = "GtsPassword123!@#"
)

func TestIntegrationActivityPubWithGoToSocial(t *testing.T) {
	requireDocker(t)

	gb := startApIntegrationServer(t)
	gts := startGoToSocialInstance(t, gb.cfg.Server.Port)

	httpClient := &http.Client{Timeout: time.Minute}
	clientID, clientSecret := gtsRegisterApp(t, gts.baseURL)
	accessToken := gtsAuthorizeToken(t, gts.baseURL, clientID, clientSecret, gtsTestEmail, gtsTestPassword)

	goBlogAcct := fmt.Sprintf("%s@%s", gb.cfg.DefaultBlog, gb.cfg.Server.publicHostname)

	// Search for GoBlog account on GoToSocial and follow it
	lookup := gtsLookupAccount(t, httpClient, gts.baseURL, accessToken, goBlogAcct)
	require.NotNil(t, lookup, "gotosocial account lookup failed for %s", goBlogAcct)
	gtsFollowAccount(t, httpClient, gts.baseURL, accessToken, lookup.ID)

	require.Eventually(t, func() bool {
		followers, err := gb.db.apGetAllFollowers(gb.cfg.DefaultBlog)
		if err != nil {
			return false
		}
		return len(followers) >= 1 && strings.Contains(followers[0].follower, fmt.Sprintf("/users/%s", gtsTestUsername))
	}, time.Minute, time.Second)

	// Create a post on GoBlog and check that it appears on GoToSocial
	post := &post{
		Content: "Hello from GoBlog to GoToSocial!",
	}
	require.NoError(t, gb.createPost(post))
	postURL := gb.fullPostURL(post)

	require.Eventually(t, func() bool {
		statuses, err := gtsAccountStatuses(t, httpClient, gts.baseURL, accessToken, lookup.ID)
		if err != nil {
			return false
		}
		for _, status := range statuses {
			if status.URI == postURL || status.URL == postURL || strings.Contains(status.Content, "Hello from GoBlog to GoToSocial!") {
				return true
			}
		}
		return false
	}, time.Minute, time.Second)
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

func startApIntegrationServer(t *testing.T) *goBlog {
	t.Helper()
	port := getFreePort(t)
	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: newHttpClient(),
	}
	// Externally expose GoBlog as goblog.example (proxied to the test port)
	app.cfg.Server.PublicAddress = "http://goblog.example"
	app.cfg.Server.Port = port
	app.cfg.ActivityPub.Enabled = true
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateStrings())
	require.NoError(t, app.initActivityPub())

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           app.buildRouter(),
		ReadHeaderTimeout: time.Minute,
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	require.NoError(t, err)
	app.shutdown.Add(app.shutdownServer(server, "integration server"))
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		app.shutdown.ShutdownAndWait()
	})

	return app
}

type goToSocialInstance struct {
	baseURL       string
	containerName string
	port          int
	networkName   string
}

func startGoToSocialInstance(t *testing.T, goblogPort int) *goToSocialInstance {
	t.Helper()

	// Create Docker network for container DNS resolution
	netName := fmt.Sprintf("goblog-net-%d", time.Now().UnixNano())
	runDocker(t, "network", "create", netName)
	t.Cleanup(func() {
		_ = exec.Command("docker", "network", "rm", netName).Run()
	})

	// Create caddy reverse proxy to forward goblog.example to GoBlog test port
	proxyName := fmt.Sprintf("goblog-proxy-%d", time.Now().UnixNano())
	runDocker(t,
		"run", "-d", "--rm",
		"--name", proxyName,
		"--hostname", "goblog.example",
		"--network", netName,
		"--network-alias", "goblog.example",
		"--add-host", "host.docker.internal:host-gateway",
		"docker.io/library/caddy:2",
		"caddy", "reverse-proxy", "--from", ":80", "--to", fmt.Sprintf("host.docker.internal:%d", goblogPort),
	)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", proxyName).Run()
	})

	// Wait for proxy to be ready
	require.Eventually(t, func() bool {
		acct := "acct:default@goblog.example"
		cmd := exec.Command("docker", "run", "--rm", "--network", netName, "docker.io/alpine/curl", "-sS", "-m", "2", "-G", "--data-urlencode", fmt.Sprintf("resource=%s", acct), "http://goblog.example/.well-known/webfinger")
		out, err := cmd.CombinedOutput()
		return err == nil && strings.Contains(string(out), acct)
	}, time.Minute, time.Second)

	// Create config and data directories
	containerName := fmt.Sprintf("goblog-gts-%d", time.Now().UnixNano())
	port := getFreePort(t)
	gtsDir := t.TempDir()
	gtsDataDir := filepath.Join(gtsDir, "data")
	gtsStorageDir := filepath.Join(gtsDataDir, "storage")
	require.NoError(t, os.MkdirAll(gtsStorageDir, 0o777))
	require.NoError(t, os.Chmod(gtsDataDir, 0o777))
	require.NoError(t, os.Chmod(gtsStorageDir, 0o777))
	gtsConfigPath := filepath.Join(gtsDir, "config.yaml")
	gtsConfig := fmt.Sprintf(`host: "127.0.0.1:%d"
protocol: "http"
bind-address: "0.0.0.0"
port: %d
db-type: "sqlite"
db-address: "/data/sqlite.db"
storage-local-base-path: "/data/storage"
http-client:
  insecure-outgoing: true
  allow-ips:
    - 0.0.0.0/0
trusted-proxies:
  - "0.0.0.0/0"
`, port, port)
	require.NoError(t, os.WriteFile(gtsConfigPath, []byte(gtsConfig), 0o644))

	// Start GoToSocial Docker container on the test network
	runDocker(t,
		"run", "-d", "--rm",
		"--name", containerName,
		"--network", netName,
		"-p", fmt.Sprintf("%d:%d", port, port),
		"-v", fmt.Sprintf("%s:/config/config.yaml", gtsConfigPath),
		"-v", fmt.Sprintf("%s:/data", gtsDataDir),
		"docker.io/superseriousbusiness/gotosocial:0.20.2",
		"--config-path", "/config/config.yaml", "server", "start",
	)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})
	gts := &goToSocialInstance{
		baseURL:       fmt.Sprintf("http://127.0.0.1:%d", port),
		containerName: containerName,
		port:          port,
		networkName:   netName,
	}

	// Wait for GoToSocial to be ready
	waitForHTTP(t, gts.baseURL+"/api/v1/instance", 2*time.Minute)

	// Create admin account
	runDocker(t,
		"exec", gts.containerName,
		"/gotosocial/gotosocial",
		"--config-path", "/config/config.yaml",
		"admin", "account", "create",
		"--username", gtsTestUsername,
		"--email", gtsTestEmail,
		"--password", gtsTestPassword,
	)

	return gts
}

func waitForHTTP(t *testing.T, endpoint string, timeout time.Duration) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	require.Eventually(t, func() bool {
		req, err := requests.URL(endpoint).Method(http.MethodGet).Request(context.Background())
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

func gtsRegisterApp(t *testing.T, baseURL string) (string, string) {
	t.Helper()
	appCfg := &mastodon.AppConfig{
		Server:       baseURL,
		ClientName:   "goblog-activitypub-test",
		RedirectURIs: "urn:ietf:wg:oauth:2.0:oob",
		Scopes:       "read write follow",
		Website:      "https://goblog.app",
	}
	app, err := mastodon.RegisterApp(context.Background(), appCfg)
	require.NoError(t, err)
	require.NotEmpty(t, app.ClientID)
	require.NotEmpty(t, app.ClientSecret)
	return app.ClientID, app.ClientSecret
}

func gtsLookupAccount(t *testing.T, client *http.Client, baseURL, token, acct string) *mastodon.Account {
	t.Helper()
	mc := mastodon.NewClient(&mastodon.Config{Server: baseURL, AccessToken: token})
	mc.Client = *client
	results, err := mc.Search(context.Background(), acct, true)
	if err != nil || results == nil || len(results.Accounts) == 0 {
		return nil
	}
	return results.Accounts[0]
}

func gtsFollowAccount(t *testing.T, client *http.Client, baseURL, token string, accountID mastodon.ID) {
	t.Helper()
	if accountID == "" {
		t.Skip("gotosocial account lookup not available")
	}
	mc := mastodon.NewClient(&mastodon.Config{Server: baseURL, AccessToken: token})
	mc.Client = *client
	_, err := mc.AccountFollow(context.Background(), accountID)
	require.NoError(t, err)
}

func gtsAccountStatuses(t *testing.T, client *http.Client, baseURL, token string, accountID mastodon.ID) ([]*mastodon.Status, error) {
	t.Helper()
	mc := mastodon.NewClient(&mastodon.Config{Server: baseURL, AccessToken: token})
	mc.Client = *client
	mStatuses, err := mc.GetAccountStatuses(context.Background(), accountID, nil)
	if err != nil {
		return nil, err
	}
	return mStatuses, nil
}

// gtsAuthorizeToken performs the OAuth2 authorization code flow to get an access token.
// This simulates a user logging in via web browser and authorizing the application.
func gtsAuthorizeToken(t *testing.T, baseURL, clientID, clientSecret, email, password string) string {
	t.Helper()

	// Create HTTP client with cookie jar to maintain session state
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{
		Timeout: time.Minute,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects automatically
		},
	}

	// Step 1: Initiate OAuth authorization flow
	query := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {"urn:ietf:wg:oauth:2.0:oob"},
		"response_type": {"code"},
		"scope":         {"read write follow"},
	}
	var signInURL string
	err = requests.URL(baseURL + "/oauth/authorize").Params(query).Client(client).
		AddValidator(requests.CheckStatus(http.StatusSeeOther)).
		Handle(func(resp *http.Response) error {
			defer resp.Body.Close()
			signInURL = resp.Header.Get("Location")
			require.NotEmpty(t, signInURL)
			if strings.HasPrefix(signInURL, "/") {
				signInURL = baseURL + signInURL
			}
			return nil
		}).Fetch(context.Background())
	require.NoError(t, err)

	// Step 2: Submit login credentials
	signInValues := url.Values{
		"username": {email},
		"password": {password},
	}
	var authorizeURL string
	err = requests.URL(signInURL).Client(client).BodyForm(signInValues).
		AddValidator(requests.CheckStatus(http.StatusFound)).
		Handle(func(resp *http.Response) error {
			defer resp.Body.Close()
			authorizeURL = resp.Header.Get("Location")
			require.NotEmpty(t, authorizeURL)
			if strings.HasPrefix(authorizeURL, "/") {
				authorizeURL = baseURL + authorizeURL
			}
			return nil
		}).Fetch(context.Background())
	require.NoError(t, err)

	// Step 3: Get authorization page
	err = requests.URL(authorizeURL).Client(client).Fetch(context.Background())
	require.NoError(t, err)

	// Step 4: Approve authorization request
	var oobURL string
	err = requests.URL(authorizeURL).Client(client).BodyForm(url.Values{}).
		AddValidator(requests.CheckStatus(http.StatusFound)).
		Handle(func(resp *http.Response) error {
			defer resp.Body.Close()
			oobURL = resp.Header.Get("Location")
			require.NotEmpty(t, oobURL)
			if strings.HasPrefix(oobURL, "/") {
				oobURL = baseURL + oobURL
			}
			return nil
		}).Fetch(context.Background())
	require.NoError(t, err)

	// Step 5: Retrieve authorization code from out-of-band page
	var code string
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err = requests.URL(oobURL).Client(client).
		ToBytesBuffer(buf).
		Fetch(context.Background())
	code = extractCode(buf)
	require.NotEmpty(t, code)
	require.NoError(t, err)

	// Step 6: Exchange authorization code for access token
	tokenData := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {"urn:ietf:wg:oauth:2.0:oob"},
		"grant_type":    {"authorization_code"},
		"code":          {code},
	}
	var tokenResult struct {
		AccessToken string `json:"access_token"`
	}
	err = requests.URL(baseURL + "/oauth/token").BodyForm(tokenData).ToJSON(&tokenResult).Fetch(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, tokenResult.AccessToken)

	return tokenResult.AccessToken
}

func extractCode(body io.Reader) string {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return ""
	}
	code := strings.TrimSpace(doc.Find("code").First().Text())
	if code == "" {
		return ""
	}
	return code
}
