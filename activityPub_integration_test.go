//go:build !skipIntegration

package main

import (
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

	// Speed up the AP send queue for testing
	apSendInterval = time.Second

	// Start GoBlog ActivityPub server and GoToSocial instance
	gb := startApIntegrationServer(t)
	_, mc := startGoToSocialInstance(t, gb.cfg.Server.Port)

	goBlogAcct := fmt.Sprintf("%s@%s", gb.cfg.DefaultBlog, gb.cfg.Server.publicHostname)

	// Search for GoBlog account on GoToSocial and follow it
	searchResults, err := mc.Search(t.Context(), goBlogAcct, true)
	require.NoError(t, err)
	require.NotNil(t, searchResults)
	require.Greater(t, len(searchResults.Accounts), 0)
	lookup := searchResults.Accounts[0]
	_, err = mc.AccountFollow(t.Context(), lookup.ID)
	require.NoError(t, err)

	// Verify that GoBlog has the GoToSocial user as a follower
	require.Eventually(t, func() bool {
		followers, err := gb.db.apGetAllFollowers(gb.cfg.DefaultBlog)
		if err != nil {
			return false
		}
		return len(followers) >= 1 && strings.Contains(followers[0].follower, fmt.Sprintf("/users/%s", gtsTestUsername))
	}, time.Minute, time.Second)

	// Verify that GoToSocial received the follow accept activity
	require.Eventually(t, func() bool {
		rs, err := mc.GetAccountRelationships(t.Context(), []string{string(lookup.ID)})
		if err != nil {
			return false
		}
		if len(rs) == 0 {
			return false
		}
		return rs[0].Following
	}, time.Minute, time.Second)

	// Update blog title and check that GoToSocial received the update
	gb.cfg.Blogs[gb.cfg.DefaultBlog].Title = "GoBlog ActivityPub Test Blog Updated"
	gb.apSendProfileUpdates()

	require.Eventually(t, func() bool {
		account, err := mc.GetAccount(t.Context(), lookup.ID)
		if err != nil {
			return false
		}
		return strings.Contains(account.DisplayName, "GoBlog ActivityPub Test Blog Updated")
	}, time.Minute, time.Second)

	// Create a post on GoBlog and check that it appears on GoToSocial
	post := &post{
		Content: "Hello from GoBlog to GoToSocial!",
	}
	require.NoError(t, gb.createPost(post))
	postURL := gb.fullPostURL(post)

	require.Eventually(t, func() bool {
		statuses, err := mc.GetAccountStatuses(t.Context(), lookup.ID, nil)
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

	// Update the post on GoBlog and verify the update appears on GoToSocial
	post.Content = "Updated content from GoBlog to GoToSocial!"
	require.NoError(t, gb.replacePost(post, post.Path, statusPublished, visibilityPublic, false))

	var statusId mastodon.ID
	require.Eventually(t, func() bool {
		statuses, err := mc.GetAccountStatuses(t.Context(), lookup.ID, nil)
		if err != nil {
			return false
		}
		for _, status := range statuses {
			if strings.Contains(status.Content, "Updated content from GoBlog to GoToSocial!") {
				statusId = status.ID
				return true
			}
		}
		return false
	}, time.Minute, time.Second)

	// Favorite the post on GoToSocial and verify GoBlog creates a notification
	_, err = mc.Favourite(t.Context(), statusId)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		notifications, err := gb.db.getNotifications(&notificationsRequestConfig{limit: 10})
		if err != nil {
			return false
		}
		for _, n := range notifications {
			if strings.Contains(n.Text, "liked") && strings.Contains(n.Text, post.Path) {
				return true
			}
		}
		return false
	}, time.Minute, time.Second)

	// Announce the post on GoToSocial and verify GoBlog creates a notification
	_, err = mc.Reblog(t.Context(), statusId)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		notifications, err := gb.db.getNotifications(&notificationsRequestConfig{limit: 10})
		if err != nil {
			return false
		}
		for _, n := range notifications {
			if strings.Contains(n.Text, "announced") && strings.Contains(n.Text, post.Path) {
				return true
			}
		}
		return false
	}, time.Minute, time.Second)

	// Delete the post on GoBlog and verify it is removed from GoToSocial
	require.NoError(t, gb.deletePost(post.Path))
	require.Eventually(t, func() bool {
		statuses, err := mc.GetAccountStatuses(t.Context(), lookup.ID, nil)
		if err != nil {
			return false
		}
		for _, status := range statuses {
			if status.URI == postURL || status.URL == postURL || strings.Contains(status.Content, "Updated content from GoBlog to GoToSocial!") {
				return false
			}
		}
		return true
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

func startGoToSocialInstance(t *testing.T, goblogPort int) (*goToSocialInstance, *mastodon.Client) {
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
cache:
  memory-target: "50MiB"
`, port, port)
	require.NoError(t, os.WriteFile(gtsConfigPath, []byte(gtsConfig), 0o644))

	// Start GoToSocial Docker container on the test network
	runDocker(t,
		"run", "-d", "--rm",
		"--name", containerName,
		"--network", netName,
		"-p", fmt.Sprintf("%d:%d", port, port),
		"-v", fmt.Sprintf("%s:/config/config.yaml", gtsConfigPath),
		"--tmpfs", "/data",
		"--tmpfs", "/gotosocial/storage",
		"--tmpfs", "/gotosocial/.cache",
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

	clientID, clientSecret := gtsRegisterApp(t, gts.baseURL)
	accessToken := gtsAuthorizeToken(t, gts.baseURL, clientID, clientSecret, gtsTestEmail, gtsTestPassword)
	mc := mastodon.NewClient(&mastodon.Config{Server: gts.baseURL, AccessToken: accessToken})
	mc.Client = http.Client{Timeout: time.Minute}

	return gts, mc
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

func gtsRegisterApp(t *testing.T, baseURL string) (string, string) {
	t.Helper()
	appCfg := &mastodon.AppConfig{
		Server:       baseURL,
		ClientName:   "goblog-activitypub-test",
		RedirectURIs: "urn:ietf:wg:oauth:2.0:oob",
		Scopes:       "read write follow",
		Website:      "https://goblog.app",
	}
	app, err := mastodon.RegisterApp(t.Context(), appCfg)
	require.NoError(t, err)
	require.NotEmpty(t, app.ClientID)
	require.NotEmpty(t, app.ClientSecret)
	return app.ClientID, app.ClientSecret
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
		}).Fetch(t.Context())
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
		}).Fetch(t.Context())
	require.NoError(t, err)

	// Step 3: Get authorization page
	err = requests.URL(authorizeURL).Client(client).Fetch(t.Context())
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
		}).Fetch(t.Context())
	require.NoError(t, err)

	// Step 5: Retrieve authorization code from out-of-band page
	var code string
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err = requests.URL(oobURL).Client(client).
		ToBytesBuffer(buf).
		Fetch(t.Context())
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
	err = requests.URL(baseURL + "/oauth/token").BodyForm(tokenData).ToJSON(&tokenResult).Fetch(t.Context())
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
