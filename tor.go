package main

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cretz/bine/tor"
)

var (
	torAddress string
)

func startOnionService(h http.Handler) error {
	torDataPath, err := filepath.Abs("data/tor")
	if err != nil {
		return err
	}
	err = os.MkdirAll(torDataPath, 0644)
	if err != nil {
		return err
	}
	// Initialize private key
	torKeyPath := filepath.Join(torDataPath, "onion.pk")
	var torKey crypto.PrivateKey
	if _, err := os.Stat(torKeyPath); os.IsNotExist(err) {
		_, torKey, err = ed25519.GenerateKey(nil)
		if err != nil {
			return err
		}
		x509Encoded, err := x509.MarshalPKCS8PrivateKey(torKey)
		if err != nil {
			return err
		}
		pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})
		_ = os.WriteFile(torKeyPath, pemEncoded, os.ModePerm)
	} else {
		d, _ := os.ReadFile(torKeyPath)
		block, _ := pem.Decode(d)
		x509Encoded := block.Bytes
		torKey, err = x509.ParsePKCS8PrivateKey(x509Encoded)
		if err != nil {
			return err
		}
	}
	// Start tor with default config (can set start conf's DebugWriter to os.Stdout for debug logs)
	log.Println("Starting and registering onion service, please wait a couple of minutes...")
	t, err := tor.Start(context.Background(), &tor.StartConf{
		TempDataDirBase: os.TempDir(),
	})
	if err != nil {
		return err
	}
	defer t.Close()
	// Wait at most a few minutes to publish the service
	listenCtx, listenCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer listenCancel()
	// Create a v3 onion service to listen on any port but show as 80
	onion, err := t.Listen(listenCtx, &tor.ListenConf{
		Version3:    true,
		Key:         torKey,
		LocalPort:   8888,
		RemotePorts: []int{80},
	})
	if err != nil {
		return err
	}
	defer onion.Close()
	torAddress = onion.String()
	log.Println("Onion service published on http://" + torAddress)
	// Serve handler
	return http.Serve(onion, h)
}
