package gateway

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/quic-go/quic-go/http3"
)

func TestLoadWebTransportServerTLSConfigLoadsConfiguredCertificate(t *testing.T) {
	certs := writeWebTransportTestCerts(t, []string{"simops-moq-gateway"})

	cfg, fingerprint, err := LoadWebTransportServerTLSConfig(certs.serverCertFile, certs.serverKeyFile)
	if err != nil {
		t.Fatalf("LoadWebTransportServerTLSConfig() error = %v", err)
	}

	if got, want := cfg.MinVersion, uint16(tls.VersionTLS13); got != want {
		t.Fatalf("MinVersion = %v, want %v", got, want)
	}
	if !containsString(cfg.NextProtos, http3.NextProtoH3) {
		t.Fatalf("NextProtos = %#v, want %q", cfg.NextProtos, http3.NextProtoH3)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("configured certificates = %d, want 1", len(cfg.Certificates))
	}

	wantFingerprint := sha256.Sum256(certs.serverDER)
	if fingerprint != hex.EncodeToString(wantFingerprint[:]) {
		t.Fatalf("fingerprint = %q, want %q", fingerprint, hex.EncodeToString(wantFingerprint[:]))
	}
}

func TestLoadWebTransportClientTLSConfigTrustsLocalCAAndRejectsWrongHost(t *testing.T) {
	certs := writeWebTransportTestCerts(t, []string{"simops-moq-gateway"})

	cfg, err := LoadWebTransportClientTLSConfig(certs.caCertFile, "simops-moq-gateway")
	if err != nil {
		t.Fatalf("LoadWebTransportClientTLSConfig() error = %v", err)
	}
	if !containsString(cfg.NextProtos, http3.NextProtoH3) {
		t.Fatalf("NextProtos = %#v, want %q", cfg.NextProtos, http3.NextProtoH3)
	}
	if got, want := cfg.ServerName, "simops-moq-gateway"; got != want {
		t.Fatalf("ServerName = %q, want %q", got, want)
	}
	verifyOptions := x509.VerifyOptions{
		DNSName: cfg.ServerName,
		Roots:   cfg.RootCAs,
	}
	if _, err := certs.serverCert.Verify(verifyOptions); err != nil {
		t.Fatalf("expected local CA to verify WebTransport server cert: %v", err)
	}

	wrongHostConfig, err := LoadWebTransportClientTLSConfig(certs.caCertFile, "wrong-host")
	if err != nil {
		t.Fatalf("LoadWebTransportClientTLSConfig(wrong host) error = %v", err)
	}
	verifyOptions.DNSName = wrongHostConfig.ServerName
	if _, err := certs.serverCert.Verify(verifyOptions); err == nil {
		t.Fatal("expected WebTransport server cert verification to reject wrong host")
	}
}

func TestLoadWebTransportClientTLSConfigRejectsInvalidCA(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(caFile, []byte("not a PEM certificate"), 0o600); err != nil {
		t.Fatalf("write invalid CA: %v", err)
	}

	_, err := LoadWebTransportClientTLSConfig(caFile, "")
	if err == nil || !strings.Contains(err.Error(), "CA") {
		t.Fatalf("LoadWebTransportClientTLSConfig() error = %v, want CA parse failure", err)
	}
}

type webTransportTestCerts struct {
	caCertFile     string
	serverCertFile string
	serverKeyFile  string
	serverDER      []byte
	serverCert     *x509.Certificate
}

func writeWebTransportTestCerts(t *testing.T, dnsNames []string) webTransportTestCerts {
	t.Helper()
	dir := t.TempDir()
	now := time.Now()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Radiant Local Test CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate server key: %v", err)
	}
	serverTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "radiant-simops-local-webtransport"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, &caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create server cert: %v", err)
	}
	serverCert, err := x509.ParseCertificate(serverDER)
	if err != nil {
		t.Fatalf("parse server cert: %v", err)
	}

	caCertFile := filepath.Join(dir, "ca.crt")
	serverCertFile := filepath.Join(dir, "server.crt")
	serverKeyFile := filepath.Join(dir, "server.key")
	writePEMFile(t, caCertFile, "CERTIFICATE", caDER)
	writePEMFile(t, serverCertFile, "CERTIFICATE", serverDER)
	writePEMFile(t, serverKeyFile, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverKey))

	return webTransportTestCerts{
		caCertFile:     caCertFile,
		serverCertFile: serverCertFile,
		serverKeyFile:  serverKeyFile,
		serverDER:      serverDER,
		serverCert:     serverCert,
	}
}

func writePEMFile(t *testing.T, path string, blockType string, der []byte) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer file.Close()
	if err := pem.Encode(file, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
