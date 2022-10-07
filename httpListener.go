package main

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"

	"tailscale.com/client/tailscale"
)

func (a *goBlog) getTCPListener(serverAddr string) (net.Listener, error) {
	if a.tailscaleEnabled() {
		// Tailscale listener
		return a.getTailscaleListener(serverAddr)
	} else if serverAddr == ":443" && a.cfg.Server.PublicHTTPS {
		m := a.getAutocertManager()
		if m == nil {
			return nil, errors.New("autocert not initialized")
		}
		return a.getAutocertManager().Listener(), nil
	} else if serverAddr == ":443" && a.cfg.Server.TailscaleHTTPS {
		// Listener with Tailscale TLS config
		ln, err := net.Listen("tcp", serverAddr)
		if err != nil {
			return nil, err
		}
		tailscaleLC := &tailscale.LocalClient{}
		return tls.NewListener(ln, &tls.Config{
			GetCertificate: tailscaleLC.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}), nil
	} else {
		// Default
		return net.Listen("tcp", serverAddr)
	}
}

func (a *goBlog) listenAndServe(s *http.Server) error {
	listener, err := a.getTCPListener(s.Addr)
	if err != nil {
		return err
	}
	return s.Serve(listener)
}
