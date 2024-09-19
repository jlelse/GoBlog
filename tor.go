package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.goblog.app/app/pkgs/tor"
	"go.goblog.app/app/pkgs/utils"
)

const torUsedKey contextKey = "tor"

func (a *goBlog) startOnionService(h http.Handler) error {
	torDataPath, err := filepath.Abs("data/tor")
	if err != nil {
		return err
	}
	// Initialize private key
	torKey, err := a.createTorPrivateKey(torDataPath)
	if err != nil {
		return err
	}
	// Start tor
	a.info("Starting and registering onion service")
	listener, addr, cancel, err := tor.StartTor(torKey, 80)
	if err != nil {
		return err
	}
	a.torAddress = "http://" + addr
	a.torHostname = addr
	a.info("Onion service published", "address", a.torAddress)
	// Clear cache
	a.cache.purge()
	// Serve handler
	s := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 1 * time.Minute,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		BaseContext: func(_ net.Listener) context.Context {
			return context.WithValue(context.Background(), torUsedKey, true)
		},
	}
	a.shutdown.Add(a.shutdownServer(s, "tor"))
	a.shutdown.Add(func() {
		if err := cancel(); err != nil {
			a.error("failed to shutdown tor", "err", err)
		}
	})
	if err = s.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (*goBlog) createTorPrivateKey(torDataPath string) (ed25519.PrivateKey, error) {
	torKeyPath := filepath.Join(torDataPath, "onion.pk")
	var torKey ed25519.PrivateKey
	if _, err := os.Stat(torKeyPath); os.IsNotExist(err) {
		// Tor private key not found, create it
		_, torKey, err = ed25519.GenerateKey(nil)
		if err != nil {
			return nil, err
		}
		x509Encoded, err := x509.MarshalPKCS8PrivateKey(torKey)
		if err != nil {
			return nil, err
		}
		pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})
		if err := utils.SaveToFileWithMode(bytes.NewReader(pemEncoded), torKeyPath, 0777, 0600); err != nil {
			return nil, err
		}
	} else {
		// Tor private key found, load it
		d, _ := os.ReadFile(torKeyPath)
		block, _ := pem.Decode(d)
		x509Encoded := block.Bytes
		parsedTorKey, err := x509.ParsePKCS8PrivateKey(x509Encoded)
		if err != nil {
			return nil, err
		}
		ok := false
		torKey, ok = parsedTorKey.(ed25519.PrivateKey)
		if !ok {
			return nil, errors.New("could not parse Tor key as ed25519 private key")
		}
	}
	return torKey, nil
}
