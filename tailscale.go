package main

import (
	"crypto/tls"
	"net/http"

	"tailscale.com/client/tailscale"
)

func (a *goBlog) startTailscaleHttps(s *http.Server) error {
	s.Addr = ":https"
	s.TLSConfig = &tls.Config{
		GetCertificate: tailscale.GetCertificate,
	}
	if err := s.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
