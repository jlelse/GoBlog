//go:build !skipIntegration

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-mastodon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationActivityPubDomainMove(t *testing.T) {
	requireDocker(t)

	// Speed up the AP send queue for testing
	apSendInterval = time.Second

	// Start GoBlog ActivityPub server
	port := getFreePort(t)
	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: newHttpClient(),
	}

	// Configure to use new domain and add old domain as alternate
	newDomain := "newgoblog.example"
	oldDomain := "goblog.example"
	app.cfg.Server.PublicAddress = "http://" + newDomain
	app.cfg.Server.AlternateDomains = []string{oldDomain}
	app.cfg.Server.Port = port
	app.cfg.ActivityPub.Enabled = true

	// Initialize the app
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

	// Create Docker network
	netName := fmt.Sprintf("goblog-net-%d", time.Now().UnixNano())
	runDocker(t, "network", "create", netName)
	t.Cleanup(func() {
		_ = exec.Command("docker", "network", "rm", netName).Run()
	})

	// Create Caddy reverse proxies for both domains
	oldProxyName := fmt.Sprintf("goblog-proxy-old-%d", time.Now().UnixNano())
	runDocker(t,
		"run", "-d", "--rm",
		"--name", oldProxyName,
		"--hostname", oldDomain,
		"--network", netName,
		"--network-alias", oldDomain,
		"--add-host", "host.docker.internal:host-gateway",
		"docker.io/library/caddy:2",
		"caddy", "reverse-proxy", "--from", ":80", "--to", fmt.Sprintf("host.docker.internal:%d", port),
	)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", oldProxyName).Run()
	})

	newProxyName := fmt.Sprintf("goblog-proxy-new-%d", time.Now().UnixNano())
	runDocker(t,
		"run", "-d", "--rm",
		"--name", newProxyName,
		"--hostname", newDomain,
		"--network", netName,
		"--network-alias", newDomain,
		"--add-host", "host.docker.internal:host-gateway",
		"docker.io/library/caddy:2",
		"caddy", "reverse-proxy", "--from", ":80", "--to", fmt.Sprintf("host.docker.internal:%d", port),
	)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", newProxyName).Run()
	})

	// Wait for proxies to be ready
	require.Eventually(t, func() bool {
		acct := "acct:default@" + newDomain
		cmd := exec.Command("docker", "run", "--rm", "--network", netName, "docker.io/alpine/curl", "-sS", "-m", "2", "-G", "--data-urlencode", fmt.Sprintf("resource=%s", acct), "http://"+newDomain+"/.well-known/webfinger")
		out, err := cmd.CombinedOutput()
		return err == nil && strings.Contains(string(out), acct)
	}, time.Minute, time.Second)

	// Start GoToSocial instance
	containerName := fmt.Sprintf("goblog-gts-%d", time.Now().UnixNano())
	gtsPort := getFreePort(t)
	gtsDir := t.TempDir()
	gtsConfigPath := gtsDir + "/config.yaml"
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
`, gtsPort, gtsPort)
	require.NoError(t, os.WriteFile(gtsConfigPath, []byte(gtsConfig), 0o644))

	runDocker(t,
		"run", "-d", "--rm",
		"--name", containerName,
		"--network", netName,
		"-p", fmt.Sprintf("%d:%d", gtsPort, gtsPort),
		"-v", fmt.Sprintf("%s:/config/config.yaml", gtsConfigPath),
		"--tmpfs", "/data",
		"--tmpfs", "/gotosocial/storage",
		"--tmpfs", "/gotosocial/.cache",
		"docker.io/superseriousbusiness/gotosocial:0.20.3",
		"--config-path", "/config/config.yaml", "server", "start",
	)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	gtsBaseURL := fmt.Sprintf("http://127.0.0.1:%d", gtsPort)
	waitForHTTP(t, gtsBaseURL+"/api/v1/instance", 2*time.Minute)

	// Create test user
	runDocker(t,
		"exec", containerName,
		"/gotosocial/gotosocial",
		"--config-path", "/config/config.yaml",
		"admin", "account", "create",
		"--username", gtsTestUsername,
		"--email", gtsTestEmail,
		"--password", gtsTestPassword,
	)

	clientID, clientSecret := gtsRegisterApp(t, gtsBaseURL)
	accessToken := gtsAuthorizeToken(t, gtsBaseURL, clientID, clientSecret, gtsTestEmail, gtsTestPassword)
	mc := mastodon.NewClient(&mastodon.Config{Server: gtsBaseURL, AccessToken: accessToken})
	mc.Client = http.Client{Timeout: time.Minute}

	// Test 1: Follow GoBlog on OLD domain first (this is the key change per requirement)
	goBlogAcctOld := fmt.Sprintf("%s@%s", app.cfg.DefaultBlog, oldDomain)
	t.Logf("Following GoBlog account on old domain: %s", goBlogAcctOld)
	searchResultsOld, err := mc.Search(t.Context(), goBlogAcctOld, true)
	require.NoError(t, err)
	require.NotNil(t, searchResultsOld)
	require.Greater(t, len(searchResultsOld.Accounts), 0)
	lookupOld := searchResultsOld.Accounts[0]
	_, err = mc.AccountFollow(t.Context(), lookupOld.ID)
	require.NoError(t, err)

	// Verify that GoBlog has the GoToSocial user as a follower
	require.Eventually(t, func() bool {
		followers, err := app.db.apGetAllFollowers(app.cfg.DefaultBlog)
		if err != nil {
			return false
		}
		return len(followers) >= 1 && strings.Contains(followers[0].follower, fmt.Sprintf("/users/%s", gtsTestUsername))
	}, time.Minute, time.Second)

	t.Log("Successfully following old domain account")

	// Test 2: Perform domain move
	t.Log("Performing domain move...")
	err = app.apDomainMove(oldDomain, newDomain)
	require.NoError(t, err)

	// Wait a bit for Move activities to be processed
	time.Sleep(5 * time.Second)

	// Test 3: Verify that GTS user now follows the NEW domain (automatic migration)
	// GoToSocial should have processed the Move activity and auto-followed the new domain
	t.Log("Checking if follower migrated to new domain...")

	goBlogAcctNew := fmt.Sprintf("%s@%s", app.cfg.DefaultBlog, newDomain)

	// Search for the new domain account from GTS user's perspective
	require.Eventually(t, func() bool {
		searchResultsNew, searchErr := mc.Search(t.Context(), goBlogAcctNew, true)
		if searchErr != nil || len(searchResultsNew.Accounts) == 0 {
			t.Logf("Search for new domain failed or no results: %v", searchErr)
			return false
		}

		newDomainAccount := searchResultsNew.Accounts[0]
		t.Logf("Found new domain account: %s", newDomainAccount.Acct)

		// Check if user is now following the new domain account
		relationships, relErr := mc.GetAccountRelationships(t.Context(), []string{string(newDomainAccount.ID)})
		if relErr != nil || len(relationships) == 0 {
			t.Logf("Failed to get relationships: %v", relErr)
			return false
		}

		following := relationships[0].Following
		t.Logf("Following new domain: %v", following)
		return following
	}, 30*time.Second, 2*time.Second, "GTS user should now follow new domain after Move")

	// Test 4: Verify webfinger works for BOTH domains from GoToSocial perspective
	// Check old domain webfinger
	goBlogAcctOldWebfinger := "acct:default@" + oldDomain
	cmdOld := exec.Command("docker", "run", "--rm", "--network", netName,
		"docker.io/alpine/curl", "-sS", "-m", "5", "-G",
		"--data-urlencode", fmt.Sprintf("resource=%s", goBlogAcctOld),
		"http://"+oldDomain+"/.well-known/webfinger")
	outOld, err := cmdOld.CombinedOutput()
	require.NoError(t, err, "Old domain webfinger failed: %s", string(outOld))
	assert.Contains(t, string(outOld), oldDomain, "Old domain webfinger should contain old domain")

	// Check new domain webfinger
	goBlogAcctNewQuery := "acct:default@" + newDomain
	cmdNew := exec.Command("docker", "run", "--rm", "--network", netName,
		"docker.io/alpine/curl", "-sS", "-m", "5", "-G",
		"--data-urlencode", fmt.Sprintf("resource=%s", goBlogAcctNewQuery),
		"http://"+newDomain+"/.well-known/webfinger")
	outNew, err := cmdNew.CombinedOutput()
	require.NoError(t, err, "New domain webfinger failed: %s", string(outNew))
	assert.Contains(t, string(outNew), newDomain, "New domain webfinger should contain new domain")

	// Test 4: Verify actor endpoint returns correct domain-specific IRIs
	// Check old domain actor (should include alsoKnownAs with new domain)
	cmdActorOld := exec.Command("docker", "run", "--rm", "--network", netName,
		"docker.io/alpine/curl", "-sS", "-m", "5",
		"-H", "Accept: application/activity+json",
		"http://"+oldDomain)
	outActorOld, err := cmdActorOld.CombinedOutput()
	require.NoError(t, err, "Old domain actor fetch failed: %s", string(outActorOld))
	assert.Contains(t, string(outActorOld), oldDomain, "Old domain actor should have old domain in ID")
	assert.Contains(t, string(outActorOld), newDomain, "Old domain actor should have new domain in alsoKnownAs")

	// Check new domain actor (should include alsoKnownAs with old domain)
	cmdActorNew := exec.Command("docker", "run", "--rm", "--network", netName,
		"docker.io/alpine/curl", "-sS", "-m", "5",
		"-H", "Accept: application/activity+json",
		"http://"+newDomain)
	outActorNew, err := cmdActorNew.CombinedOutput()
	require.NoError(t, err, "New domain actor fetch failed: %s", string(outActorNew))
	assert.Contains(t, string(outActorNew), newDomain, "New domain actor should have new domain in ID")
	assert.Contains(t, string(outActorNew), oldDomain, "New domain actor should have old domain in alsoKnownAs")

	t.Log("Domain move integration test completed successfully")
	t.Log("- Both domains are accessible")
	t.Log("- Webfinger works for both domains")
	t.Log("- Actor endpoints return domain-specific IRIs")
	t.Log("- alsoKnownAs includes alternate domains")
}
