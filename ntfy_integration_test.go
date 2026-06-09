//go:build !skipIntegration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationNtfy(t *testing.T) {
	t.Parallel()
	requireDocker(t)

	ntfyPort := getFreePort(t)
	containerName := fmt.Sprintf("goblog-ntfy-%s", uuid.New().String())

	ntfyConfig := fmt.Sprintf(`base-url: "http://localhost:%d"
listen: "0.0.0.0:80"
cache-file: "/tmp/ntfy-cache.db"
cache-duration: "12h"
`, ntfyPort)
	configFile := filepath.Join(t.TempDir(), "server.yml")
	require.NoError(t, os.WriteFile(configFile, []byte(ntfyConfig), 0644))

	runDocker(t,
		"run", "-d", "--rm",
		"--name", containerName,
		"-p", fmt.Sprintf("127.0.0.1:%d:80", ntfyPort),
		"-v", fmt.Sprintf("%s:/etc/ntfy/server.yml:ro", configFile),
		"docker.io/binwiederhier/ntfy:latest",
		"serve",
	)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	ntfyBaseURL := fmt.Sprintf("http://127.0.0.1:%d", ntfyPort)
	waitForHTTP(t, ntfyBaseURL+"/v1/health", time.Minute)

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: newHTTPClient(),
	}
	app.cfg.Server.PublicAddress = fmt.Sprintf("http://localhost:%d", app.cfg.Server.Port)
	app.cfg.Notifications = &configNotifications{
		Ntfy: &configNtfy{
			Enabled: true,
			Topic:   "test-integration",
			Server:  ntfyBaseURL,
		},
	}
	require.NoError(t, app.initConfig(false))

	t.Run("notification is received by ntfy server", func(t *testing.T) {
		app.sendNotification("hello from goblog")

		var msg ntfyMessage
		require.Eventually(t, func() bool {
			var err error
			msg, err = ntfyPollMessage(ntfyBaseURL, "test-integration", nil)
			return err == nil && msg.Message != ""
		}, 10*time.Second, 500*time.Millisecond, "ntfy should receive the notification")

		assert.Equal(t, "hello from goblog", msg.Message)
	})

	t.Run("notification with special characters", func(t *testing.T) {
		app.cfg.Notifications.Ntfy.Topic = "test-special"
		app.sendNotification("special: <b>html</b> & \"quotes\" 'test'")

		require.Eventually(t, func() bool {
			msg, err := ntfyPollMessage(ntfyBaseURL, "test-special", nil)
			return err == nil && msg.Message == "special: <b>html</b> & \"quotes\" 'test'"
		}, 10*time.Second, 500*time.Millisecond)
	})

	t.Run("multiple notifications are all delivered", func(t *testing.T) {
		app.cfg.Notifications.Ntfy.Topic = "test-batch"
		for i := range 3 {
			app.sendNotification(fmt.Sprintf("batch message %d", i))
		}

		var messages []ntfyMessage
		require.Eventually(t, func() bool {
			var err error
			messages, err = ntfyPollMessages(ntfyBaseURL, "test-batch", nil)
			return err == nil && len(messages) >= 3
		}, 15*time.Second, 500*time.Millisecond, "all 3 batch messages should be delivered")

		assert.Equal(t, "batch message 0", messages[0].Message)
		assert.Equal(t, "batch message 1", messages[1].Message)
		assert.Equal(t, "batch message 2", messages[2].Message)
	})
}

func TestIntegrationNtfyWithAuth(t *testing.T) {
	t.Parallel()
	requireDocker(t)

	ntfyPort := getFreePort(t)
	containerName := fmt.Sprintf("goblog-ntfy-auth-%s", uuid.New().String())

	ntfyConfig := fmt.Sprintf(`base-url: "http://localhost:%d"
listen: "0.0.0.0:80"
cache-file: "/tmp/ntfy-cache.db"
cache-duration: "12h"
auth-file: "/tmp/ntfy-user.db"
auth-default-access: "deny-all"
`, ntfyPort)
	configFile := filepath.Join(t.TempDir(), "server.yml")
	require.NoError(t, os.WriteFile(configFile, []byte(ntfyConfig), 0644))

	runDocker(t,
		"run", "-d", "--rm",
		"--name", containerName,
		"-p", fmt.Sprintf("127.0.0.1:%d:80", ntfyPort),
		"-v", fmt.Sprintf("%s:/etc/ntfy/server.yml:ro", configFile),
		"docker.io/binwiederhier/ntfy:latest",
		"serve",
	)
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	ntfyBaseURL := fmt.Sprintf("http://127.0.0.1:%d", ntfyPort)
	waitForHTTP(t, ntfyBaseURL+"/v1/health", time.Minute)

	ntfyUser := "testuser"
	ntfyPass := "testpassword123"
	ntfyCreateUser(t, containerName, ntfyUser, ntfyPass)

	creds := &ntfyAuth{User: ntfyUser, Pass: ntfyPass}

	t.Run("authenticated notification is delivered", func(t *testing.T) {
		app := &goBlog{
			cfg:        createDefaultTestConfig(t),
			httpClient: newHTTPClient(),
		}
		app.cfg.Notifications = &configNotifications{
			Ntfy: &configNtfy{
				Enabled: true,
				Topic:   "auth-test",
				Server:  ntfyBaseURL,
				User:    ntfyUser,
				Pass:    ntfyPass,
			},
		}
		require.NoError(t, app.initConfig(false))

		app.sendNotification("authenticated message")

		var msg ntfyMessage
		require.Eventually(t, func() bool {
			var err error
			msg, err = ntfyPollMessage(ntfyBaseURL, "auth-test", creds)
			return err == nil && msg.Message != ""
		}, 10*time.Second, 500*time.Millisecond, "authenticated notification should be received")

		assert.Equal(t, "authenticated message", msg.Message)
	})

	t.Run("unauthenticated notification is rejected", func(t *testing.T) {
		err := requests.
			URL(ntfyBaseURL + "/unauth-test").
			Method(http.MethodPost).
			BodyReader(bytes.NewReader([]byte("should fail"))).
			Client(&http.Client{Timeout: 5 * time.Second}).
			Fetch(t.Context())
		assert.Error(t, err, "unauthenticated request to auth-required server should fail")
	})
}

type ntfyMessage struct {
	ID      string `json:"id"`
	Time    int64  `json:"time"`
	Message string `json:"message"`
	Topic   string `json:"topic"`
}

type ntfyAuth struct {
	User string
	Pass string
}

func ntfyPollMessage(server, topic string, auth *ntfyAuth) (ntfyMessage, error) {
	messages, err := ntfyPollMessages(server, topic, auth)
	if err != nil {
		return ntfyMessage{}, err
	}
	if len(messages) == 0 {
		return ntfyMessage{}, fmt.Errorf("no messages received")
	}
	return messages[0], nil
}

func ntfyPollMessages(server, topic string, auth *ntfyAuth) ([]ntfyMessage, error) {
	var messages []ntfyMessage
	req := requests.
		URL(fmt.Sprintf("%s/%s/json", server, topic)).
		Param("poll", "1").
		Param("since", "0").
		Client(newHTTPClient()).
		Handle(func(resp *http.Response) error {
			defer resp.Body.Close()
			dec := json.NewDecoder(resp.Body)
			for dec.More() {
				var msg ntfyMessage
				if err := dec.Decode(&msg); err != nil {
					return err
				}
				messages = append(messages, msg)
			}
			return nil
		})
	if auth != nil {
		req = req.BasicAuth(auth.User, auth.Pass)
	}
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := req.Fetch(timeoutCtx); err != nil {
		return nil, err
	}
	return messages, nil
}

func ntfyCreateUser(t *testing.T, containerName, username, password string) {
	t.Helper()
	cmd := exec.Command("docker", "exec", "-e", "NTFY_PASSWORD="+password, containerName, "ntfy", "user", "add", username)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "ntfy user add: %s", string(output))

	cmd = exec.Command("docker", "exec", containerName, "ntfy", "access", username, "*", "rw")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "ntfy access: %s", string(output))
}
