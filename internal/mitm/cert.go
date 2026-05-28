// MITM Certificate Manager.

package mitm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	rootCAValidity = 5 * 365 * 24 * time.Hour
	leafValidity   = 365 * 24 * time.Hour
	rsaBits        = 2048
)

// CertManager hands out tls.Certificate values keyed by Server Name (SNI).
// Concurrency-safe; per-domain leaves are cached on disk + in memory.
type CertManager struct {
	dir      string
	rootCert *x509.Certificate
	rootKey  *rsa.PrivateKey
	rootPEM  []byte // for OS keychain install
	cacheMu  sync.RWMutex
	cache    map[string]*tls.Certificate
}

// NewCertManager loads (or creates on first use) the root CA under <dir>/mitm.
// dir is typically <dataDir> (resolved from FLOW_ROUTER_DATA env or homedir).
func NewCertManager(dir string) (*CertManager, error) {
	mitmDir := filepath.Join(dir, "mitm")
	if err := os.MkdirAll(filepath.Join(mitmDir, "leaves"), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir mitm: %w", err)
	}
	m := &CertManager{dir: mitmDir, cache: map[string]*tls.Certificate{}}
	if err := m.loadOrCreateRoot(); err != nil {
		return nil, err
	}
	return m, nil
}

// RootCAPEM returns the encoded root certificate for the user to install in
// the OS trust store. Empty unless NewCertManager succeeded.
func (m *CertManager) RootCAPEM() []byte {
	out := make([]byte, len(m.rootPEM))
	copy(out, m.rootPEM)
	return out
}

// GetCertificate fits crypto/tls.Config.GetCertificate signature. Caller is
// the TLS handshake; ServerName comes from the SNI client extension.
func (m *CertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := hello.ServerName
	if host == "" {
		return nil, fmt.Errorf("no SNI")
	}
	return m.issueLeaf(host)
}

// IssueLeaf returns a signed leaf for host (testable alias).
func (m *CertManager) IssueLeaf(host string) (*tls.Certificate, error) {
	return m.issueLeaf(host)
}

func (m *CertManager) issueLeaf(host string) (*tls.Certificate, error) {
	host = strings.ToLower(host)
	m.cacheMu.RLock()
	if c := m.cache[host]; c != nil {
		m.cacheMu.RUnlock()
		return c, nil
	}
	m.cacheMu.RUnlock()
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	if c := m.cache[host]; c != nil {
		return c, nil
	}

	// Try disk first
	certPath := filepath.Join(m.dir, "leaves", host+".pem")
	keyPath := filepath.Join(m.dir, "leaves", host+".key")
	if certPEM, err := os.ReadFile(certPath); err == nil {
		if keyPEM, err := os.ReadFile(keyPath); err == nil {
			if c, err := tls.X509KeyPair(certPEM, keyPEM); err == nil {
				m.cache[host] = &c
				return &c, nil
			}
		}
	}

	// Generate fresh
	key, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		return nil, fmt.Errorf("leaf key: %w", err)
	}
	serial := randomSerial()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host, Organization: []string{"flow_router MITM"}},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(leafValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host},
	}
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
		template.DNSNames = nil
	}
	der, err := x509.CreateCertificate(rand.Reader, template, m.rootCert, &key.PublicKey, m.rootKey)
	if err != nil {
		return nil, fmt.Errorf("sign leaf: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	_ = os.WriteFile(certPath, certPEM, 0o600)
	_ = os.WriteFile(keyPath, keyPEM, 0o600)
	c, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("pair leaf: %w", err)
	}
	m.cache[host] = &c
	return &c, nil
}

func (m *CertManager) loadOrCreateRoot() error {
	certPath := filepath.Join(m.dir, "rootCA.pem")
	keyPath := filepath.Join(m.dir, "rootCA.key")
	if certPEM, err := os.ReadFile(certPath); err == nil {
		if keyPEM, err := os.ReadFile(keyPath); err == nil {
			if c, k, ok := decodeRoot(certPEM, keyPEM); ok {
				m.rootCert = c
				m.rootKey = k
				m.rootPEM = certPEM
				return nil
			}
		}
	}
	// Generate
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("root key: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          randomSerial(),
		Subject:               pkix.Name{CommonName: "flow_router Root CA", Organization: []string{"flow_router"}},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(rootCAValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("self-sign root: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		return fmt.Errorf("write root cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return fmt.Errorf("write root key: %w", err)
	}
	c, _, ok := decodeRoot(certPEM, keyPEM)
	if !ok {
		return fmt.Errorf("decode just-written root failed")
	}
	m.rootCert = c
	m.rootKey = key
	m.rootPEM = certPEM
	return nil
}

func decodeRoot(certPEM, keyPEM []byte) (*x509.Certificate, *rsa.PrivateKey, bool) {
	certBlock, _ := pem.Decode(certPEM)
	keyBlock, _ := pem.Decode(keyPEM)
	if certBlock == nil || keyBlock == nil {
		return nil, nil, false
	}
	c, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, false
	}
	k, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, false
	}
	return c, k, true
}

func randomSerial() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	n, _ := rand.Int(rand.Reader, limit)
	return n
}
