package wazuh

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

func TestBuildHTTPClient_InsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c, err := buildHTTPClient(config.TLS{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestBuildHTTPClient_VerifyFailsWithoutSkip(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c, err := buildHTTPClient(config.TLS{})
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
	if _, err := c.Get(srv.URL); err == nil {
		t.Fatal("Get: expected TLS verify failure")
	}
}

func TestBuildHTTPClient_CAFileMissing(t *testing.T) {
	_, err := buildHTTPClient(config.TLS{CAFile: "/nope/missing.crt"})
	if err == nil {
		t.Fatal("buildHTTPClient: expected error for missing CA file")
	}
}

func TestBuildHTTPClient_CAFileLoaded(t *testing.T) {
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.crt")
	// Empty file is enough for the loader to succeed; we only check no error.
	if err := os.WriteFile(ca, []byte(""), 0o600); err != nil {
		t.Fatalf("write CA: %v", err)
	}
	if _, err := buildHTTPClient(config.TLS{CAFile: ca}); err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
}
