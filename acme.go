package main

import (
	"encoding/base64"

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
		hosts := []string{a.cfg.Server.publicHostname}
		if shn := a.cfg.Server.shortPublicHostname; shn != "" {
			hosts = append(hosts, shn)
		}
		if mhn := a.cfg.Server.mediaHostname; mhn != "" {
			hosts = append(hosts, mhn)
		}
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
