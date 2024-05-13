package tor

import (
	"context"
	"crypto/ed25519"
	"crypto/sha512"
	"encoding/base32"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/sha3"
)

// https://github.com/torproject/torspec/blob/8961bb4d83fccb2b987f9899ca83aa430f84ab0c/rend-spec-v3.txt#L2259-L2281
func onionAddressFromEd25519(publicKey ed25519.PublicKey) string {
	checksum := sha3.Sum256(append(append([]byte(".onion checksum"), publicKey...), 0x03))
	checkdigits := checksum[:2]
	serviceID := base32.StdEncoding.EncodeToString(append(append(publicKey[:], checkdigits...), 0x03))
	return strings.ToLower(serviceID) + ".onion"
}

// https://github.com/torproject/torspec/blob/8961bb4d83fccb2b987f9899ca83aa430f84ab0c/rend-spec-v3.txt#L2391-L2449
func deriveSecretKey(privateKey ed25519.PrivateKey) []byte {
	hash := sha512.Sum512(privateKey[:32])
	hash[0] &= 248
	hash[31] &= 127
	hash[31] |= 64
	return hash[:]
}

// StartTor creates a temporary directory, writes the required files to it,
// and then creates a listener and starts the tor process to publish the onion service.
func StartTor(privateKey ed25519.PrivateKey, remotePort int) (net.Listener, string, func() error, error) {
	publicKey := privateKey.Public().(ed25519.PublicKey)
	onion := onionAddressFromEd25519(publicKey)

	// Create the hidden service directory
	dir, err := os.MkdirTemp("", "hidden_service_*")
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create directory: %v", err)
	}

	// Write the keys to a file
	secretKeyFilePath := filepath.Join(dir, "hs_ed25519_secret_key")
	secretKeyFileContent := append([]byte("== ed25519v1-secret: type0 ==\x00\x00\x00"), deriveSecretKey(privateKey)...)
	err = os.WriteFile(secretKeyFilePath, secretKeyFileContent, 0600)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to write private key: %v", err)
	}

	publicKeyFilePath := filepath.Join(dir, "hs_ed25519_public_key")
	publicKeyFileContent := append([]byte("== ed25519v1-public: type0 ==\x00\x00\x00"), publicKey...)
	err = os.WriteFile(publicKeyFilePath, publicKeyFileContent, 0600)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to write public key: %v", err)
	}

	hostnameFilePath := filepath.Join(dir, "hostname")
	err = os.WriteFile(hostnameFilePath, []byte(onion), 0600)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to write hostname: %v", err)
	}

	// Create listener
	listener, err := net.Listen("tcp4", "127.0.0.1:")
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create listener: %v", err)
	}

	// Start the Tor process
	cmd := exec.CommandContext(
		context.Background(),
		"tor", "--ignore-missing-torrc",
		// Hidden Service configs
		"--HiddenServiceDir", dir,
		"--HiddenServicePort", strconv.Itoa(remotePort)+" "+listener.Addr().String(),
		"--HiddenServiceVersion", "3",
		"--HiddenServiceEnableIntroDoSDefense", "1",
		// Disable SocksPort, we don't need it
		"--SocksPort", "0",
		// Limit to one process to reduce memory usage
		"--NumCPUs", "1",
		// Enable hardware acceleration, it might improve speed
		"--HardwareAccel", "1",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to start Tor: %v", err)
	}

	// Wait
	go cmd.Wait()

	return listener, onion, func() error {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		return nil
	}, nil
}
