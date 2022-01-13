package main

import (
	"crypto/tls"
	"errors"
	"log"
	"net"
	"os"
	"path/filepath"

	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

func (a *goBlog) tailscaleEnabled() bool {
	return a.cfg.Server != nil &&
		a.cfg.Server.Tailscale != nil &&
		a.cfg.Server.Tailscale.Enabled
}

func (a *goBlog) getTailscaleListener(addr string) (net.Listener, error) {
	if !a.tailscaleEnabled() {
		return nil, errors.New("tailscale not configured")
	}
	a.tsinit.Do(func() {
		tsconfig := a.cfg.Server.Tailscale
		if tsconfig.AuthKey != "" {
			// Set Auth Key
			_ = os.Setenv("TS_AUTHKEY", tsconfig.AuthKey)
		}
		// Enable Tailscale WIP code
		_ = os.Setenv("TAILSCALE_USE_WIP_CODE", "true")
		// Init server
		tailscaleDir := filepath.Join("data", "tailscale")
		_ = os.MkdirAll(tailscaleDir, 0777)
		a.tss = &tsnet.Server{
			Hostname: tsconfig.Hostname,
			Dir:      tailscaleDir,
			Logf: func(format string, args ...interface{}) {
				log.Printf("tailscale: "+format, args...)
			},
		}
	})
	ln, err := a.tss.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	// Tailscale HTTPS
	if addr == ":443" && a.cfg.Server.TailscaleHTTPS {
		ln = tls.NewListener(ln, &tls.Config{
			GetCertificate: tailscale.GetCertificate,
		})
	}
	return ln, nil
}
