package main

import (
	"encoding/base64"

	"github.com/samber/lo"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

func (a *goBlog) getAutocertManager() *autocert.Manager {
	if !a.cfg.Server.PublicHTTPS {
		return nil
	}
	if a.autocertManager != nil {
		return a.autocertManager
	}
	// Not initialized yet
	a.autocertInit.Do(func() {
		// Create hosts whitelist
		hosts := []string{a.cfg.Server.publicHost, a.cfg.Server.shortPublicHost, a.cfg.Server.mediaHost}
		hosts = append(hosts, a.cfg.Server.altHosts...)
		hosts = lo.Filter(hosts, func(v string, _ int) bool { return v != "" })
		// Create autocert manager
		acmeDir := acme.LetsEncryptURL
		if a.cfg.Server.AcmeDir != "" {
			acmeDir = a.cfg.Server.AcmeDir
		}
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(hosts...),
			Cache:      &httpsCache{db: a.db},
			Client:     &acme.Client{DirectoryURL: acmeDir, HTTPClient: a.httpClient},
		}
		// Set external account binding
		if a.cfg.Server.AcmeEabKid != "" && a.cfg.Server.AcmeEabKey != "" {
			key, err := base64.RawURLEncoding.DecodeString(a.cfg.Server.AcmeEabKey)
			if err != nil {
				return
			}
			m.ExternalAccountBinding = &acme.ExternalAccountBinding{
				KID: a.cfg.Server.AcmeEabKid,
				Key: key,
			}
		}
		// Save
		a.autocertManager = m
	})
	// Return
	return a.autocertManager
}
