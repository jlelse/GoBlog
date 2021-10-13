package main

import (
	"crypto/tls"
	"net"
	"net/http"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"tailscale.com/client/tailscale"
)

func (a *goBlog) getTCPListener(s *http.Server) (net.Listener, error) {
	if a.tailscaleEnabled() {
		// Tailscale listener
		return a.getTailscaleListener(s.Addr)
	} else if s.Addr == ":443" && a.cfg.Server.PublicHTTPS {
		// Listener with public HTTPS
		hosts := []string{a.cfg.Server.publicHostname}
		if shn := a.cfg.Server.shortPublicHostname; shn != "" {
			hosts = append(hosts, shn)
		}
		if mhn := a.cfg.Server.mediaHostname; mhn != "" {
			hosts = append(hosts, mhn)
		}
		acmeDir := acme.LetsEncryptURL
		// Uncomment for Staging Let's Encrypt
		// acmeDir = "https://acme-staging-v02.api.letsencrypt.org/directory"
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(hosts...),
			Cache:      &httpsCache{db: a.db},
			Client:     &acme.Client{DirectoryURL: acmeDir},
		}
		return m.Listener(), nil
	} else if s.Addr == ":443" && a.cfg.Server.TailscaleHTTPS {
		// Listener with Tailscale TLS config
		ln, err := net.Listen("tcp", s.Addr)
		if err != nil {
			return nil, err
		}
		return tls.NewListener(ln, &tls.Config{
			GetCertificate: tailscale.GetCertificate,
		}), nil
	} else {
		// Default
		return net.Listen("tcp", s.Addr)
	}
}

func (a *goBlog) listenAndServe(s *http.Server) error {
	listener, err := a.getTCPListener(s)
	if err != nil {
		return err
	}
	return s.Serve(listener)
}
