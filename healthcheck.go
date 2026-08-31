package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	healthPath = "/health"
	pingPath   = "/ping"
)

func (a *goBlog) serveHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if failures := a.runHealthChecks(ctx); len(failures) > 0 {
		a.error("Health check failed", "failures", failures)
		http.Error(w, strings.Join(failures, "; "), http.StatusServiceUnavailable)
		return
	}
	http.Error(w, "ok", http.StatusOK)
}

func (a *goBlog) runHealthChecks(ctx context.Context) []string {
	var failures []string
	// Database
	if err := a.checkHealthDatabase(ctx); err != nil {
		failures = append(failures, "database: "+err.Error())
	}
	// Markdown rendering
	if err := a.checkHealthMarkdown(); err != nil {
		failures = append(failures, "markdown rendering: "+err.Error())
	}
	// Main server
	if err := a.checkHealthMainServer(ctx); err != nil {
		failures = append(failures, "main server: "+err.Error())
	}
	// imgproxy
	if err := a.checkImgproxyReachable(ctx); err != nil {
		failures = append(failures, "imgproxy: "+err.Error())
	}
	return failures
}

func (a *goBlog) checkHealthDatabase(ctx context.Context) error {
	if a.db == nil {
		return nil
	}
	if err := a.db.readDb.PingContext(ctx); err != nil {
		return fmt.Errorf("read database: %w", err)
	}
	if err := a.db.writeDb.PingContext(ctx); err != nil {
		return fmt.Errorf("write database: %w", err)
	}
	return nil
}

func (a *goBlog) checkHealthMarkdown() error {
	var buf bytes.Buffer
	if err := a.renderMarkdownToWriter(&buf, "GoBlog health check"); err != nil {
		return err
	}
	if !strings.Contains(buf.String(), "GoBlog health check") {
		return errors.New("rendered output missing expected content")
	}
	return nil
}

func (a *goBlog) checkHealthMainServer(ctx context.Context) error {
	if port := a.cfg.Server.Port; port != 0 {
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			return err
		}
		return conn.Close()
	}
	return nil
}

func (a *goBlog) healthCheckURL() string {
	if hca := a.cfg.Server.HealthCheckAddress; hca != "" {
		if !strings.HasPrefix(hca, "http://") && !strings.HasPrefix(hca, "https://") {
			hca = "http://" + hca
		}
		return hca + healthPath
	}
	return a.getFullAddress(pingPath)
}

func (a *goBlog) healthcheck() error {
	timeoutContext, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFunc()
	req, err := http.NewRequestWithContext(timeoutContext, http.MethodGet, a.healthCheckURL(), nil)
	if err != nil {
		return fmt.Errorf("failed to create healthcheck request: %w", err)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("healthcheck returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (a *goBlog) healthcheckExitCode() int {
	if err := a.healthcheck(); err != nil {
		a.error("Healthcheck failed", "err", err)
		return 1
	}
	return 0
}