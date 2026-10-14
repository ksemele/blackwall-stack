// Package httpclient builds TLS-aware HTTP clients for the Wazuh API and indexer.
package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

// New returns an *http.Client honouring insecure_skip_verify and ca_file.
func New(t config.TLS) (*http.Client, error) {
	cfg := &tls.Config{InsecureSkipVerify: t.InsecureSkipVerify}
	if t.CAFile != "" {
		raw, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca_file %q: %w", t.CAFile, err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(raw) // empty/invalid PEM yields empty pool — acceptable
		cfg.RootCAs = pool
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: cfg},
	}, nil
}
