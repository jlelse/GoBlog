//go:build !skipIntegration

package main

import (
"context"
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

// Domains for testing
newDomain := "newgoblog.example"
oldDomain := "goblog.example"
port := getFreePort(t)

// Create Docker network
netName := fmt.Sprintf("goblog-net-%d", time.Now().UnixNano())
runDocker(t, "network", "create", netName)
t.Cleanup(func() {
exec.Command("docker", "network", "rm", netName).Run()
})

// Create Caddy reverse proxy for old domain
oldProxyName := fmt.Sprintf("goblog-proxy-old-%d", time.Now().UnixNano())
runDocker(t,
", "-d", "--rm",
ame", oldProxyName,
ame", oldDomain,
etwork", netName,
etwork-alias", oldDomain,
ternal:host-gateway",
:2",
"reverse-proxy", "--from", ":80", "--to", fmt.Sprintf("host.docker.internal:%d", port),
)
t.Cleanup(func() {
exec.Command("docker", "rm", "-f", oldProxyName).Run()
})

// Create Caddy reverse proxy for new domain
newProxyName := fmt.Sprintf("goblog-proxy-new-%d", time.Now().UnixNano())
runDocker(t,
", "-d", "--rm",
ame", newProxyName,
ame", newDomain,
etwork", netName,
etwork-alias", newDomain,
ternal:host-gateway",
:2",
"reverse-proxy", "--from", ":80", "--to", fmt.Sprintf("host.docker.internal:%d", port),
)
t.Cleanup(func() {
exec.Command("docker", "rm", "-f", newProxyName).Run()
})

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
", "-d", "--rm",
ame", containerName,
etwork", netName,
tf("%d:%d", gtsPort, gtsPort),
tf("%s:/config/config.yaml", gtsConfigPath),
"/gotosocial/storage",
ess/gotosocial:0.20.3",
fig-path", "/config/config.yaml", "server", "start",
)
t.Cleanup(func() {
exec.Command("docker", "rm", "-f", containerName).Run()
})

gtsBaseURL := fmt.Sprintf("http://127.0.0.1:%d", gtsPort)
waitForHTTP(t, gtsBaseURL+"/api/v1/instance", 2*time.Minute)

// Create test user
runDocker(t,
tainerName,
fig-path", "/config/config.yaml",
", "account", "create",
ame", gtsTestUsername,
gtsTestPassword,
)

clientID, clientSecret := gtsRegisterApp(t, gtsBaseURL)
accessToken := gtsAuthorizeToken(t, gtsBaseURL, clientID, clientSecret, gtsTestEmail, gtsTestPassword)
mc := mastodon.NewClient(&mastodon.Config{Server: gtsBaseURL, AccessToken: accessToken})
mc.Client = http.Client{Timeout: time.Minute}

// === PHASE 1: Start GoBlog without alternate domain (using OLD domain) ===
t.Log("=== PHASE 1: Starting GoBlog with OLD domain only ===")
app := &goBlog{
      createDefaultTestConfig(t),
t: newHttpClient(),
}

// Configure to use OLD domain initially (no alternate domains)
app.cfg.Server.PublicAddress = "http://" + oldDomain
app.cfg.Server.AlternateDomains = []string{} // No alternate domains yet
app.cfg.Server.Port = port
app.cfg.ActivityPub.Enabled = true

// Initialize the app
require.NoError(t, app.initConfig(false))
require.NoError(t, app.initTemplateStrings())
require.NoError(t, app.initActivityPub())

server := &http.Server{
            fmt.Sprintf(":%d", port),
dler:           app.buildRouter(),
ute,
}
listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
require.NoError(t, err)
app.shutdown.Add(app.shutdownServer(server, "integration server"))
go func() {
server.Serve(listener)
}()

// Wait for GoBlog to be ready on old domain
require.Eventually(t, func() bool {
"acct:default@" + oldDomain
exec.Command("docker", "run", "--rm", "--network", netName, "docker.io/alpine/curl", "-sS", "-m", "2", "-G", "--data-urlencode", fmt.Sprintf("resource=%s", acct), "http://"+oldDomain+"/.well-known/webfinger")
:= cmd.CombinedOutput()
 err == nil && strings.Contains(string(out), acct)
}, time.Minute, time.Second)

t.Log("GoBlog is running on old domain")

// Follow GoBlog on OLD domain
goBlogAcctOld := fmt.Sprintf("%s@%s", app.cfg.DefaultBlog, oldDomain)
t.Logf("Following GoBlog account on old domain: %s", goBlogAcctOld)
searchResultsOld, err := mc.Search(context.Background(), goBlogAcctOld, true)
require.NoError(t, err)
require.NotNil(t, searchResultsOld)
require.Greater(t, len(searchResultsOld.Accounts), 0)
lookupOld := searchResultsOld.Accounts[0]
_, err = mc.AccountFollow(context.Background(), lookupOld.ID)
require.NoError(t, err)

// Verify that GoBlog has the GoToSocial user as a follower
require.Eventually(t, func() bool {
:= app.db.apGetAllFollowers(app.cfg.DefaultBlog)
!= nil {
 false
 len(followers) >= 1 && strings.Contains(followers[0].follower, fmt.Sprintf("/users/%s", gtsTestUsername))
}, time.Minute, time.Second)

t.Log("Successfully following old domain account")

// === PHASE 2: Stop server, update config, restart ===
t.Log("=== PHASE 2: Stopping server to update config ===")

// Gracefully shutdown the server
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
require.NoError(t, server.Shutdown(shutdownCtx))
app.shutdown.ShutdownAndWait()

t.Log("Server stopped")

// Update config: Change to new domain and add old domain as alternate
t.Log("=== PHASE 3: Updating config and restarting with new domain ===")
app.cfg.Server.PublicAddress = "http://" + newDomain
app.cfg.Server.AlternateDomains = []string{oldDomain}

// Re-initialize with new config
require.NoError(t, app.initConfig(false))
require.NoError(t, app.initTemplateStrings())
require.NoError(t, app.initActivityPub())

// Start server again
server = &http.Server{
            fmt.Sprintf(":%d", port),
dler:           app.buildRouter(),
ute,
}
listener, err = net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
require.NoError(t, err)
app.shutdown.Add(app.shutdownServer(server, "integration server"))
go func() {
server.Serve(listener)
}()
t.Cleanup(func() {
.ShutdownAndWait()
})

// Wait for GoBlog to be ready on new domain
require.Eventually(t, func() bool {
"acct:default@" + newDomain
exec.Command("docker", "run", "--rm", "--network", netName, "docker.io/alpine/curl", "-sS", "-m", "2", "-G", "--data-urlencode", fmt.Sprintf("resource=%s", acct), "http://"+newDomain+"/.well-known/webfinger")
:= cmd.CombinedOutput()
 err == nil && strings.Contains(string(out), acct)
}, time.Minute, time.Second)

t.Log("GoBlog restarted on new domain")

// Verify old domain still works (as alternate)
require.Eventually(t, func() bool {
"acct:default@" + oldDomain
exec.Command("docker", "run", "--rm", "--network", netName, "docker.io/alpine/curl", "-sS", "-m", "2", "-G", "--data-urlencode", fmt.Sprintf("resource=%s", acct), "http://"+oldDomain+"/.well-known/webfinger")
:= cmd.CombinedOutput()
 err == nil && strings.Contains(string(out), acct)
}, time.Minute, time.Second)

t.Log("Old domain still works as alternate")

// === PHASE 4: Perform domain move ===
t.Log("=== PHASE 4: Performing domain move ===")
err = app.apDomainMove(oldDomain, newDomain)
require.NoError(t, err)

// Verify movedTo was set for the blog
movedTo, err := app.getApMovedTo(app.cfg.DefaultBlog)
require.NoError(t, err)
expectedMovedTo := fmt.Sprintf("http://%s", newDomain)
assert.Equal(t, expectedMovedTo, movedTo, "movedTo should be set to new domain actor")

t.Log("Domain move completed, movedTo is set")

// Wait for Move activities to be processed
time.Sleep(10 * time.Second)

// === PHASE 5: Verify the move worked ===
t.Log("=== PHASE 5: Verifying move worked ===")

// Check that GTS user now follows the NEW domain (automatic migration)
goBlogAcctNew := fmt.Sprintf("%s@%s", app.cfg.DefaultBlog, newDomain)

// Search for the new domain account from GTS user's perspective
require.Eventually(t, func() bool {
ew, searchErr := mc.Search(context.Background(), goBlogAcctNew, true)
!= nil || len(searchResultsNew.Accounts) == 0 {
new domain failed or no results: %v", searchErr)
 false
ewDomainAccount := searchResultsNew.Accounts[0]
d new domain account: %s", newDomainAccount.Acct)

if user is now following the new domain account
ships, relErr := mc.GetAccountRelationships(context.Background(), []string{string(newDomainAccount.ID)})
!= nil || len(relationships) == 0 {
get relationships: %v", relErr)
 false
g := relationships[0].Following
g new domain: %v", following)
 following
}, 60*time.Second, 3*time.Second, "GTS user should now follow new domain after Move")

t.Log("Follower successfully migrated to new domain")

// Verify webfinger works for both domains
goBlogAcctOldWebfinger := "acct:default@" + oldDomain
cmdOld := exec.Command("docker", "run", "--rm", "--network", netName,
e/curl", "-sS", "-m", "5", "-G",
code", fmt.Sprintf("resource=%s", goBlogAcctOldWebfinger),
+"/.well-known/webfinger")
outOld, err := cmdOld.CombinedOutput()
require.NoError(t, err, "Old domain webfinger failed: %s", string(outOld))
assert.Contains(t, string(outOld), oldDomain, "Old domain webfinger should contain old domain")

goBlogAcctNewWebfinger := "acct:default@" + newDomain
cmdNew := exec.Command("docker", "run", "--rm", "--network", netName,
e/curl", "-sS", "-m", "5", "-G",
code", fmt.Sprintf("resource=%s", goBlogAcctNewWebfinger),
ewDomain+"/.well-known/webfinger")
outNew, err := cmdNew.CombinedOutput()
require.NoError(t, err, "New domain webfinger failed: %s", string(outNew))
assert.Contains(t, string(outNew), newDomain, "New domain webfinger should contain new domain")

// Verify actor endpoints return correct IRIs and movedTo
cmdActorOld := exec.Command("docker", "run", "--rm", "--network", netName,
e/curl", "-sS", "-m", "5",
application/activity+json",
)
outActorOld, err := cmdActorOld.CombinedOutput()
require.NoError(t, err, "Old domain actor fetch failed: %s", string(outActorOld))
assert.Contains(t, string(outActorOld), oldDomain, "Old domain actor should have old domain in ID")
assert.Contains(t, string(outActorOld), newDomain, "Old domain actor should have new domain in alsoKnownAs or movedTo")
assert.Contains(t, string(outActorOld), "movedTo", "Old domain actor should have movedTo field")

cmdActorNew := exec.Command("docker", "run", "--rm", "--network", netName,
e/curl", "-sS", "-m", "5",
application/activity+json",
ewDomain)
outActorNew, err := cmdActorNew.CombinedOutput()
require.NoError(t, err, "New domain actor fetch failed: %s", string(outActorNew))
assert.Contains(t, string(outActorNew), newDomain, "New domain actor should have new domain in ID")
assert.Contains(t, string(outActorNew), oldDomain, "New domain actor should have old domain in alsoKnownAs")

t.Log("✓ Domain move integration test completed successfully")
t.Log("✓ Started with old domain only")
t.Log("✓ Followed old domain")
t.Log("✓ Restarted with new domain and old as alternate")
t.Log("✓ Domain move executed successfully")
t.Log("✓ movedTo field is set")
t.Log("✓ Follower migrated to new domain")
t.Log("✓ Both domains are accessible")
t.Log("✓ Webfinger works for both domains")
}
