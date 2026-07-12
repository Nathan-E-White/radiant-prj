package gateway

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/quic-go/quic-go/http3"
)

const webTransportLocalCertCommonName = "radiant-simops-local-webtransport"

func LoadWebTransportServerTLSConfig(certFile string, keyFile string) (*tls.Config, string, error) {
	certFile = strings.TrimSpace(certFile)
	keyFile = strings.TrimSpace(keyFile)
	if certFile == "" && keyFile == "" {
		return generateLocalWebTransportServerTLSConfig()
	}
	if certFile == "" || keyFile == "" {
		return nil, "", fmt.Errorf("WebTransport TLS cert and key files must be supplied together")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, "", fmt.Errorf("load WebTransport TLS certificate: %w", err)
	}
	if len(cert.Certificate) == 0 {
		return nil, "", fmt.Errorf("WebTransport TLS certificate did not contain a leaf certificate")
	}

	return newWebTransportServerTLSConfig(cert), certificateFingerprint(cert.Certificate[0]), nil
}

func LoadWebTransportClientTLSConfig(caCertFile string, serverName string) (*tls.Config, error) {
	caCertFile = strings.TrimSpace(caCertFile)
	serverName = strings.TrimSpace(serverName)

	var roots *x509.CertPool
	if caCertFile != "" {
		caBytes, err := os.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("read WebTransport CA certificate: %w", err)
		}
		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM(caBytes) {
			return nil, fmt.Errorf("WebTransport CA file did not contain a PEM certificate")
		}
	}

	cfg := &tls.Config{
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{http3.NextProtoH3},
		RootCAs:    roots,
	}
	if serverName != "" {
		cfg.ServerName = serverName
	}
	return cfg, nil
}

func generateLocalWebTransportServerTLSConfig() (*tls.Config, string, error) {
	certKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, "", err
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: webTransportLocalCertCommonName,
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "simops-moq-gateway"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &certKey.PublicKey, certKey)
	if err != nil {
		return nil, "", err
	}
	return newWebTransportServerTLSConfig(tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  certKey,
	}), certificateFingerprint(der), nil
}

func newWebTransportServerTLSConfig(cert tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{http3.NextProtoH3},
		MinVersion:   tls.VersionTLS13,
	}
}

func certificateFingerprint(der []byte) string {
	fingerprint := sha256.Sum256(der)
	return hex.EncodeToString(fingerprint[:])
}
