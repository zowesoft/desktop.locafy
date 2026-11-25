package network

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"
)

// Authority issues the local certificates the proxy serves. It is Locafy's own
// CA, stored beside the engine's data, and it never leaves the machine.
type Authority struct {
	directory string
}

// NewAuthority returns an authority whose files live under directory.
func NewAuthority(directory string) *Authority {
	return &Authority{directory: directory}
}

// CAPath is the certificate a user trusts once. It exists only after Ensure.
func (a *Authority) CAPath() string {
	return filepath.Join(a.directory, "ca.crt")
}

func (a *Authority) keyPath() string {
	return filepath.Join(a.directory, "ca.key")
}

func (a *Authority) leafCertPath() string {
	return filepath.Join(a.directory, "domains.crt")
}

func (a *Authority) leafKeyPath() string {
	return filepath.Join(a.directory, "domains.key")
}

// Ensure creates the CA if it is missing and returns its certificate. A CA that
// exists is never replaced: replacing it would silently invalidate every trust
// the user has already granted.
func (a *Authority) Ensure() (*x509.Certificate, error) {
	if certificate, err := a.loadCA(); err == nil {
		return certificate, nil
	}

	if err := os.MkdirAll(a.directory, 0o700); err != nil {
		return nil, fmt.Errorf("certificate directory: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate authority key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Locafy Local Development CA", Organization: []string{"Locafy"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create authority certificate: %w", err)
	}

	if err := writePEM(a.CAPath(), "CERTIFICATE", der, 0o600); err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("encode authority key: %w", err)
	}
	if err := writePEM(a.keyPath(), "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return nil, err
	}

	return a.loadCA()
}

// Leaf issues a certificate naming exactly the given hostnames, or reuses the
// existing one when it already names them and is still valid. The returned
// certificate is ready to serve TLS.
func (a *Authority) Leaf(hostnames []string) (tls.Certificate, error) {
	authority, err := a.Ensure()
	if err != nil {
		return tls.Certificate{}, err
	}

	sorted := append([]string(nil), hostnames...)
	slices.Sort(sorted)
	slices.Compact(sorted)

	if certificate, err := a.loadLeaf(authority, sorted); err == nil {
		return certificate, nil
	}

	caKey, err := a.loadCAKey()
	if err != nil {
		return tls.Certificate{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate domain key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: sorted[0], Organization: []string{"Locafy"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     sorted,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, authority, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create domain certificate: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("encode domain key: %w", err)
	}
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(a.leafCertPath(), leafPEM, 0o600); err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(a.leafKeyPath(), keyPEM, 0o600); err != nil {
		return tls.Certificate{}, err
	}

	return tls.X509KeyPair(leafPEM, keyPEM)
}

// TrustCommand is the command a user runs to trust the CA, and the reason when
// no command can be run for them. Linux is deliberately manual: its system store
// needs root, and Locafy never escalates on the user's behalf.
func (a *Authority) TrustCommand() (program string, args []string, explanation string, err error) {
	path := a.CAPath()
	if _, statErr := os.Stat(path); statErr != nil {
		if _, err := a.Ensure(); err != nil {
			return "", nil, "", err
		}
	}

	switch runtime.GOOS {
	case "windows":
		return "certutil", []string{"-user", "-addstore", "Root", path},
			"This adds the certificate to your user's trusted root store. It does not need administrator rights.", nil
	case "darwin":
		return "security", []string{"add-trusted-cert", "-r", "trustRoot", "-k",
				filepath.Join(os.Getenv("HOME"), "Library/Keychains/login.keychain-db"), path},
			"This adds the certificate to your login keychain as a trusted root.", nil
	default:
		return "", nil, "",
			fmt.Errorf("on Linux, trust the certificate yourself: add %s to your distribution's trust store (for example, copy it to /usr/local/share/ca-certificates and run update-ca-certificates)", path)
	}
}

func (a *Authority) loadCA() (*x509.Certificate, error) {
	raw, err := os.ReadFile(a.CAPath())
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("authority certificate is not PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

func (a *Authority) loadCAKey() (*ecdsa.PrivateKey, error) {
	raw, err := os.ReadFile(a.keyPath())
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("authority key is not PEM")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

// loadLeaf reuses a leaf only when it still names the same hostnames and has not
// expired, so a removed domain stops being trusted and a renewed one keeps
// working without a manual step.
func (a *Authority) loadLeaf(authority *x509.Certificate, hostnames []string) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(a.leafCertPath())
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM, err := os.ReadFile(a.leafKeyPath())
	if err != nil {
		return tls.Certificate{}, err
	}
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return tls.Certificate{}, err
	}
	if time.Now().After(leaf.NotAfter.Add(-24 * time.Hour)) {
		return tls.Certificate{}, fmt.Errorf("domain certificate is about to expire")
	}
	if !slices.Equal(leaf.DNSNames, hostnames) {
		return tls.Certificate{}, fmt.Errorf("domain certificate names a different set of hostnames")
	}
	if err := leaf.CheckSignatureFrom(authority); err != nil {
		return tls.Certificate{}, err
	}
	return certificate, nil
}

func writePEM(path string, kind string, der []byte, mode os.FileMode) error {
	encoded := pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der})
	if err := os.WriteFile(path, encoded, mode); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("certificate serial: %w", err)
	}
	return serial, nil
}
