package main

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"

	"tailscale.com/client/tailscale"
)

func (a *goBlog) getTCPListener(s *http.Server) (net.Listener, error) {
	if a.tailscaleEnabled() {
		// Tailscale listener
		return a.getTailscaleListener(s.Addr)
	} else if s.Addr == ":443" && a.cfg.Server.PublicHTTPS {
		m := a.getAutocertManager()
		if m == nil {
			return nil, errors.New("autocert not initialized")
		}
		return a.getAutocertManager().Listener(), nil
	} else if s.Addr == ":443" && a.cfg.Server.TailscaleHTTPS {
		// Listener with Tailscale TLS config
		ln, err := net.Listen("tcp", s.Addr)
		if err != nil {
			return nil, err
		}
		return tls.NewListener(ln, &tls.Config{
			GetCertificate: tailscale.GetCertificate,
			MinVersion:     tls.VersionTLS12,
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
