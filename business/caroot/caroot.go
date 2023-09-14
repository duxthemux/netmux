package caroot

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path"
	"sync"
	"time"
)

const (
	DefaultPerm        = 0o600
	DefaultRsaKeySize  = 4096
	DefaultYearCertDur = 1000
)

//nolint:gochecknoglobals
var rootDir = "certs"

type CA struct {
	ca    *x509.Certificate
	catls tls.Certificate
	start int64
	mtx   sync.Mutex
}

func New() *CA {
	return &CA{}
}

func exists(s string) bool {
	_, err := os.Stat(s)

	return !os.IsNotExist(err)
}

func createDefaultCert() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization:  []string{"Netmux"},
			CommonName:    "Netmux Root CA",
			Country:       []string{"na"},
			Province:      []string{"na"},
			Locality:      []string{"na"},
			StreetAddress: []string{"na"},
			PostalCode:    []string{"na"},
		},
		NotBefore:             time.Now().Add(time.Hour * -24),
		NotAfter:              time.Now().AddDate(DefaultYearCertDur, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
}

//nolint:funlen,cyclop
func (c *CA) Init(rdir string, installca func(ca string)) error {
	if rdir != "" {
		rootDir = rdir
	}

	_ = os.MkdirAll(rootDir, os.ModePerm)

	slog.Info(fmt.Sprintf("USING ROOT CA as: %s", rootDir))

	c.start = time.Now().Unix()

	//nolint:nestif
	if !exists(path.Join(rootDir, "ca.cer")) {
		slog.Info("Initiating new CA CERT")

		c.ca = createDefaultCert()

		priv, _ := rsa.GenerateKey(rand.Reader, DefaultRsaKeySize)

		pub := &priv.PublicKey

		caBytes, err := x509.CreateCertificate(rand.Reader, c.ca, c.ca, pub, priv)
		if err != nil {
			return fmt.Errorf("error creeating certificate: %w", err)
		}

		out := &bytes.Buffer{}

		if err = pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: caBytes}); err != nil {
			return fmt.Errorf("error encoding ca: %w", err)
		}

		cert := out.Bytes()

		err = os.WriteFile(path.Join(rootDir, "ca.cer"), cert, DefaultPerm)
		if err != nil {
			return fmt.Errorf("error writing ca.cer: %w", err)
		}

		keyOut, err := os.OpenFile(path.Join(rootDir, "ca.key"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, DefaultPerm)
		if err != nil {
			return fmt.Errorf("could not open ca.key: %w", err)
		}

		if err = pem.Encode(
			keyOut,
			&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
			return fmt.Errorf("error encoding rsa key: %w", err)
		}

		_ = keyOut.Close()

		slog.Info("written key.pem")

		if installca != nil {
			slog.Info("Installing CA")
			installca(path.Join(rootDir, "ca.cer"))
		}
	}

	var err error

	// Load CA
	c.catls, err = tls.LoadX509KeyPair(path.Join(rootDir, "ca.cer"), path.Join(rootDir, "ca.key"))
	if err != nil {
		return fmt.Errorf("error lading x509 keypair: %w", err)
	}

	c.ca, err = x509.ParseCertificate(c.catls.Certificate[0])
	if err != nil {
		return fmt.Errorf("error parsing x509 certificate: %w", err)
	}

	return nil
}

func (c *CA) GenCertForDomain(domain string) ([]byte, []byte, error) {
	c.mtx.Lock()

	time.Sleep(time.Nanosecond)

	ser := time.Now().UnixNano()

	c.mtx.Unlock()

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(ser),
		Subject: pkix.Name{
			Organization: []string{"Digital Circle"},
			Country:      []string{"BR"},
			CommonName:   domain,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	const KeySize = 2048

	cert.DNSNames = append(cert.DNSNames, domain)

	priv, err := rsa.GenerateKey(rand.Reader, KeySize)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating rsa key: %w", err)
	}

	pub := &priv.PublicKey

	// Sign the certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, c.ca, pub, c.catls.PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating x509 cert: %w", err)
	}
	// Public key
	certBuffer := &bytes.Buffer{}
	if err = pem.Encode(certBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return nil, nil, fmt.Errorf("error encoding certificate %w", err)
	}

	keyBuffer := &bytes.Buffer{}

	if err = pem.Encode(
		keyBuffer,
		&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return nil, nil, fmt.Errorf("error encoding rsa key: %w", err)
	}

	return keyBuffer.Bytes(), certBuffer.Bytes(), nil
}

func (c *CA) GenCertFilesForDomain(domain string, dir string) error {
	key, cert, err := c.GenCertForDomain(domain)
	if err != nil {
		return err
	}

	_ = os.MkdirAll(dir, os.ModePerm)

	keyfile := path.Join(dir, domain+".key")

	certfile := path.Join(dir, domain+".cer")

	err = os.WriteFile(keyfile, key, DefaultPerm)
	if err != nil {
		return fmt.Errorf("error writing key: %w", err)
	}

	err = os.WriteFile(certfile, cert, DefaultPerm)
	if err != nil {
		return fmt.Errorf("error writing cert: %w", err)
	}

	return nil
}

func (c *CA) GenCertFilesForDomainInRootDir(d string) error {
	return c.GenCertFilesForDomain(d, rootDir)
}

func (c *CA) GetCertFromRoot(domain string) (tls.Certificate, error) {
	cer, err := tls.LoadX509KeyPair(path.Join(rootDir, domain+".cer"), path.Join(rootDir, domain+".key"))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("error loading cert: %w", err)
	}

	return cer, nil
}

func (c *CA) GetOrGenFromRoot(domain string) (tls.Certificate, error) {
	if exists(path.Join(rootDir, domain+".cer")) {
		return c.GetCertFromRoot(domain)
	}

	if err := c.GenCertFilesForDomainInRootDir(domain); err != nil {
		return tls.Certificate{}, err
	}

	return c.GetCertFromRoot(domain)
}

func (c *CA) GetCATLS() tls.Certificate {
	return c.catls
}

func (c *CA) CaCerBytes() ([]byte, error) {
	bs, err := os.ReadFile("ca.cer")
	if err != nil {
		return nil, fmt.Errorf("error reading ca.cer: %w", err)
	}

	return bs, nil
}
