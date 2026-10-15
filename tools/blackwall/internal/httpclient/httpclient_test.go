package httpclient

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

// tlsServer serves on 127.0.0.1 with a certificate issued for
// manager.example.test / 10.9.8.7 only (as behind an ssh tunnel), and returns
// the path of the issuing CA in PEM.
func tlsServer(t *testing.T) (url, caFile string) {
	t.Helper()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test root"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, _ := x509.ParseCertificate(caDER)
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leafTpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "manager"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"manager.example.test"}, IPAddresses: []net.IP{net.ParseIP("10.9.8.7")},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTpl, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}}}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	caFile = filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return srv.URL, caFile
}

func TestNew_ServerNameVerifiesTunnelledCert(t *testing.T) {
	url, ca := tlsServer(t)
	cases := []struct {
		name string
		tls  config.TLS
		ok   bool
	}{
		{"dns name", config.TLS{CAFile: ca, ServerName: "manager.example.test"}, true},
		{"ip san", config.TLS{CAFile: ca, ServerName: "10.9.8.7"}, true},
		{"no server name (127.0.0.1 not in cert)", config.TLS{CAFile: ca}, false},
		{"wrong name", config.TLS{CAFile: ca, ServerName: "other.example.test"}, false},
		{"insecure ignores names", config.TLS{InsecureSkipVerify: true, ServerName: "other.example.test"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := New(c.tls)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := h.Get(url)
			if err == nil {
				resp.Body.Close()
			}
			if (err == nil) != c.ok {
				t.Errorf("Get err = %v, want ok=%v", err, c.ok)
			}
		})
	}
}
