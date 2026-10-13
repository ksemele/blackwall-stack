package wazuh

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

func newServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func subFromServer(srv *httptest.Server, user, pw string) config.WazuhSubsystem {
	// strip http:// and split host:port for our client which expects no scheme.
	addr := strings.TrimPrefix(srv.URL, "http://")
	host, port := splitHostPort(addr)
	return config.WazuhSubsystem{
		Server:   host,
		Port:     port,
		User:     user,
		Password: pw,
		TLS:      config.TLS{InsecureSkipVerify: true},
	}
}

func splitHostPort(addr string) (string, int) {
	// httptest URLs are always host:port with numeric port.
	host, p, _ := strings.Cut(addr, ":")
	port := 0
	for _, c := range p {
		port = port*10 + int(c-'0')
	}
	return host, port
}

func TestAuthenticate_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/security/user/authenticate" {
			t.Errorf("path = %q, want /security/user/authenticate", r.URL.Path)
		}
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("wazuh:secret"))
		if r.Header.Get("Authorization") != want {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), want)
		}
		_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
			Token string `json:"token"`
		}{Token: "fake-jwt"}})
	})

	c, err := New(subFromServer(srv, "wazuh", "secret"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Override the scheme via the unexported field; in production code
	// the scheme is always https. For testing we expose insecureScheme.
	c.insecureScheme = "http"

	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if c.token != "fake-jwt" {
		t.Errorf("token = %q, want fake-jwt", c.token)
	}
}

func TestAuthenticate_401(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	c, _ := New(subFromServer(srv, "wazuh", "wrong"))
	c.insecureScheme = "http"
	if err := c.Authenticate(context.Background()); err == nil {
		t.Fatal("Authenticate: expected error")
	}
}
