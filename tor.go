package main

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/cretz/bine/tor"
	"github.com/go-chi/chi/v5/middleware"
)

const torUsedKey contextKey = "tor"

func (a *goBlog) startOnionService(h http.Handler) error {
	torDataPath, err := filepath.Abs("data/tor")
	if err != nil {
		return err
	}
	err = os.MkdirAll(torDataPath, 0777)
	if err != nil {
		return err
	}
	// Initialize private key
	torKey, err := a.createTorPrivateKey(torDataPath)
	if err != nil {
		return err
	}
	// Start tor
	log.Println("Starting and registering onion service, please wait a couple of minutes...")
	t, err := tor.Start(context.Background(), &tor.StartConf{
		TempDataDirBase: os.TempDir(),
		NoAutoSocksPort: true,
		ExtraArgs:       a.torExtraArgs(),
	})
	if err != nil {
		return err
	}
	defer func() {
		_ = t.Close()
	}()
	// Wait at most a few minutes to publish the service
	listenCtx, listenCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer listenCancel()
	// Create a v3 onion service to listen on any port but show as 80
	onion, err := t.Listen(listenCtx, &tor.ListenConf{
		Key:          torKey,
		RemotePorts:  []int{80},
		NonAnonymous: a.cfg.Server.TorSingleHop,
	})
	if err != nil {
		return err
	}
	defer func() {
		_ = onion.Close()
	}()
	a.torAddress = "http://" + onion.String()
	torUrl, _ := url.Parse(a.torAddress)
	a.torHostname = torUrl.Hostname()
	log.Println("Onion service published on " + a.torAddress)
	// Clear cache
	a.cache.purge()
	// Serve handler
	s := &http.Server{
		Handler:           middleware.WithValue(torUsedKey, true)(h),
		ReadHeaderTimeout: 1 * time.Minute,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
	}
	a.shutdown.Add(shutdownServer(s, "tor"))
	if err = s.Serve(onion); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (*goBlog) createTorPrivateKey(torDataPath string) (crypto.PrivateKey, error) {
	torKeyPath := filepath.Join(torDataPath, "onion.pk")
	var torKey crypto.PrivateKey
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
		err = os.WriteFile(torKeyPath, pemEncoded, 0600)
		if err != nil {
			return nil, err
		}
	} else {
		// Tor private key found, load it
		d, _ := os.ReadFile(torKeyPath)
		block, _ := pem.Decode(d)
		x509Encoded := block.Bytes
		torKey, err = x509.ParsePKCS8PrivateKey(x509Encoded)
		if err != nil {
			return nil, err
		}
	}
	return torKey, nil
}

func (a *goBlog) torExtraArgs() []string {
	s := []string{"--SocksPort", "0"}
	if a.cfg.Server.TorSingleHop {
		s = append(s, "--HiddenServiceNonAnonymousMode", "1", "--HiddenServiceSingleHopMode", "1")
	}
	return s
}
