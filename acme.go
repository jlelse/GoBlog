package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/tlsalpn01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	singleflightx "github.com/samber/go-singleflightx"
)

func (a *goBlog) getCertManager() *certManager {
	if !a.cfg.Server.PublicHTTPS {
		return nil
	}
	if a.certMgr != nil {
		return a.certMgr
	}
	a.certMgrInit.Do(func() {
		hostMap := make(map[string]struct{})
		for _, h := range append([]string{
			a.cfg.Server.publicHost,
			a.cfg.Server.shortPublicHost,
			a.cfg.Server.mediaHost,
		}, a.cfg.Server.altHosts...) {
			if h != "" {
				hostMap[strings.ToLower(h)] = struct{}{}
			}
		}

		acmeDir := lego.LEDirectoryProduction
		if a.cfg.Server.AcmeDir != "" {
			acmeDir = a.cfg.Server.AcmeDir
		}

		cm := &certManager{
			db:         a.db,
			hosts:      hostMap,
			acmeDir:    acmeDir,
			eabKid:     a.cfg.Server.AcmeEabKid,
			eabHmac:    a.cfg.Server.AcmeEabKey,
			httpClient: a.httpClient,
			tlsChall:   &tlsALPN01Provider{},
			stopCh:     make(chan struct{}),
		}
		go cm.renewLoop()
		a.certMgr = cm
	})
	return a.certMgr
}

// certManager manages ACME certificate lifecycle using lego.
type certManager struct {
	db         *database
	hosts      map[string]struct{}
	acmeDir    string
	eabKid     string
	eabHmac    string
	httpClient *http.Client

	tlsChall *tlsALPN01Provider

	certCache   sync.Map
	obtainGroup singleflightx.Group[string, *cachedCert]

	stopCh chan struct{}
}

type cachedCert struct {
	tlsCert       *tls.Certificate
	privateKeyPEM []byte
	certPEM       []byte
}

// acmeUser implements registration.User for lego.
type acmeUser struct {
	key          *ecdsa.PrivateKey
	registration *registration.Resource
}

func (u *acmeUser) GetEmail() string                        { return "" }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

func (cm *certManager) TLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: cm.GetCertificate,
		NextProtos:     []string{tlsalpn01.ACMETLS1Protocol, "h2", "http/1.1"},
	}
}

func (cm *certManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := strings.ToLower(strings.TrimSuffix(hello.ServerName, "."))

	// Handle TLS-ALPN-01 challenge: ACME validator connects with "acme-tls/1" ALPN
	for _, proto := range hello.SupportedProtos {
		if proto == tlsalpn01.ACMETLS1Protocol {
			cert := cm.tlsChall.GetCertificate(host)
			if cert == nil {
				return nil, errors.New("acme: no TLS-ALPN-01 challenge cert for: " + host)
			}
			return cert, nil
		}
	}

	if _, ok := cm.hosts[host]; !ok {
		return nil, errors.New("acme: host not allowed: " + host)
	}

	if cached, ok := cm.certCache.Load(host); ok {
		cachedCert := cached.(*cachedCert)
		if isCertValid(cachedCert) {
			return cachedCert.tlsCert, nil
		}
	}

	// Deduplicate concurrent cache misses for the same host: only one goroutine
	// performs the DB lookup + ACME issuance while others wait for the result.
	cert, err, _ := cm.obtainGroup.Do(host, func() (*cachedCert, error) {
		return cm.loadOrIssueCert(host)
	})
	if err != nil {
		return nil, fmt.Errorf("acme: obtain cert for %s: %w", host, err)
	}
	cm.certCache.Store(host, cert)
	return cert.tlsCert, nil
}

func (cm *certManager) Close() {
	close(cm.stopCh)
}

func (cm *certManager) renewLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-cm.stopCh:
			return
		case <-ticker.C:
			cm.certCache.Range(func(host, cert any) bool {
				cm.refreshCert(host.(string), cert.(*cachedCert))
				return true
			})
		}
	}
}

func (cm *certManager) loadOrIssueCert(host string) (*cachedCert, error) {
	c, err := cm.loadCert(host)
	if err == nil && isCertValid(c) {
		return c, nil
	}
	return cm.renewOrObtainCert(host, c)
}

func (cm *certManager) refreshCert(host string, cert *cachedCert) {
	if !needsRenewal(cert) {
		return
	}
	if cert, err := cm.renewOrObtainCert(host, cert); err == nil {
		cm.certCache.Store(host, cert)
	}
}

func (cm *certManager) newLegoClient() (*lego.Client, error) {
	user, err := cm.loadOrCreateKey()
	if err != nil {
		return nil, err
	}

	config := lego.NewConfig(user)
	config.CADirURL = cm.acmeDir
	config.Certificate.KeyType = certcrypto.EC256
	if cm.httpClient != nil {
		config.HTTPClient = cm.httpClient
	}

	client, err := lego.NewClient(config)
	if err != nil {
		return nil, err
	}

	if err := client.Challenge.SetTLSALPN01Provider(cm.tlsChall); err != nil {
		return nil, err
	}

	// Resolve or register the account — no need to persist the registration
	// separately; ResolveAccountByKey() (RFC 8555 §7.3.1) recovers it from
	// the ACME server using only the saved private key.
	reg, err := client.Registration.ResolveAccountByKey()
	if err != nil {
		if cm.eabKid != "" && cm.eabHmac != "" {
			reg, err = client.Registration.RegisterWithExternalAccountBinding(registration.RegisterEABOptions{
				TermsOfServiceAgreed: true,
				Kid:                  cm.eabKid,
				HmacEncoded:          cm.eabHmac,
			})
		} else {
			reg, err = client.Registration.Register(registration.RegisterOptions{
				TermsOfServiceAgreed: true,
			})
		}
		if err != nil {
			return nil, err
		}
	}
	user.registration = reg

	return client, nil
}

func (cm *certManager) obtainCert(host string) (*cachedCert, error) {
	client, err := cm.newLegoClient()
	if err != nil {
		return nil, err
	}

	resource, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: []string{host},
		Bundle:  true,
	})
	if err != nil {
		return nil, err
	}

	return cm.cacheAndParseResource(host, resource)
}

func (cm *certManager) renewCert(host string, current *cachedCert) (*cachedCert, error) {
	client, err := cm.newLegoClient()
	if err != nil {
		return nil, err
	}

	resource, err := client.Certificate.RenewWithOptions(certificate.Resource{
		Domain:      host,
		PrivateKey:  current.privateKeyPEM,
		Certificate: current.certPEM,
	}, &certificate.RenewOptions{
		Bundle: true,
	})
	if err != nil {
		return nil, err
	}

	if len(resource.PrivateKey) == 0 {
		resource.PrivateKey = current.privateKeyPEM
	}

	return cm.cacheAndParseResource(host, resource)
}

func (cm *certManager) cacheAndParseResource(host string, resource *certificate.Resource) (*cachedCert, error) {
	// autocert-compatible format: key PEM first, then certificate chain PEM
	bundle := append(resource.PrivateKey, resource.Certificate...)
	if err := cm.saveCert(host, bundle); err != nil {
		return nil, err
	}
	return parseCert(bundle)
}

func (cm *certManager) renewOrObtainCert(host string, current *cachedCert) (*cachedCert, error) {
	return renewOrObtainCert(host, current, cm.renewCert, cm.obtainCert)
}

func renewOrObtainCert(host string, current *cachedCert, renew func(string, *cachedCert) (*cachedCert, error), obtain func(string) (*cachedCert, error)) (*cachedCert, error) {
	if current != nil {
		if cert, err := renew(host, current); err == nil {
			return cert, nil
		}
	}
	return obtain(host)
}

func isCertValid(cert *cachedCert) bool {
	return cert != nil && cert.tlsCert != nil && cert.tlsCert.Leaf != nil && time.Now().Before(cert.tlsCert.Leaf.NotAfter)
}

func needsRenewal(cert *cachedCert) bool {
	if cert == nil || cert.tlsCert == nil || cert.tlsCert.Leaf == nil {
		return false
	}
	leaf := cert.tlsCert.Leaf
	return time.Until(leaf.NotAfter) < renewThreshold(leaf)
}

func (cm *certManager) saveCert(host string, data []byte) error {
	return cm.db.cachePersistently("https_"+host, data)
}

func (cm *certManager) loadCert(host string) (*cachedCert, error) {
	data, err := cm.db.retrievePersistentCache("https_" + host)
	if err != nil || data == nil {
		return nil, err
	}
	return parseCert(data)
}

// parseCert parses a PEM bundle in autocert-compatible format: the first block
// is the private key ("EC PRIVATE KEY"), the remaining blocks are the cert chain.
func parseCert(data []byte) (*cachedCert, error) {
	keyBlock, certPEM := pem.Decode(data)
	if keyBlock == nil || !strings.Contains(keyBlock.Type, "PRIVATE") {
		return nil, errors.New("acme: invalid cert cache: missing private key")
	}
	keyPEM := pem.EncodeToMemory(keyBlock)
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, err
	}
	tlsCert.Leaf = leaf
	return &cachedCert{
		tlsCert:       &tlsCert,
		privateKeyPEM: keyPEM,
		certPEM:       certPEM,
	}, nil
}

// renewThreshold returns how much time before expiry the certificate should be
// renewed: 30% of its total lifetime, so shorter-lived certs are renewed sooner.
func renewThreshold(leaf *x509.Certificate) time.Duration {
	return leaf.NotAfter.Sub(leaf.NotBefore) * 30 / 100
}

// acmeAccountKeyDBKey is compatible with autocert's cache key so existing
// account keys are reused automatically after migrating from autocert.
const acmeAccountKeyDBKey = "https_acme_account+key"

func (cm *certManager) loadOrCreateKey() (*acmeUser, error) {
	user := &acmeUser{}

	keyData, err := cm.db.retrievePersistentCache(acmeAccountKeyDBKey)
	if err != nil {
		return nil, err
	}
	if keyData != nil {
		user.key, err = parseECPrivateKey(keyData)
		if err != nil {
			return nil, err
		}
	} else {
		user.key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		keyPEM, err := encodeECPrivateKey(user.key)
		if err != nil {
			return nil, err
		}
		if err := cm.db.cachePersistently(acmeAccountKeyDBKey, keyPEM); err != nil {
			return nil, err
		}
	}

	return user, nil
}

func parseECPrivateKey(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func encodeECPrivateKey(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}

// tlsALPN01Provider implements challenge.Provider for TLS-ALPN-01, storing
// challenge certificates that are served via GetCertificate for integration
// with the server's existing TLS listener.
var _ challenge.Provider = (*tlsALPN01Provider)(nil)

type tlsALPN01Provider struct {
	certs sync.Map
}

func (p *tlsALPN01Provider) Present(domain, token, keyAuth string) error {
	cert, err := tlsalpn01.ChallengeCert(domain, keyAuth)
	if err != nil {
		return err
	}
	p.certs.Store(domain, cert)
	return nil
}

func (p *tlsALPN01Provider) CleanUp(domain, token, keyAuth string) error {
	p.certs.Delete(domain)
	return nil
}

func (p *tlsALPN01Provider) GetCertificate(domain string) *tls.Certificate {
	cert, ok := p.certs.Load(domain)
	if !ok {
		return nil
	}
	return cert.(*tls.Certificate)
}
