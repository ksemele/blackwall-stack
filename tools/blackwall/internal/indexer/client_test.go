package indexer

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

// serve starts an HTTP test server and returns a client pointed at it.
func serve(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	p, _ := strconv.Atoi(port)
	c, err := New(config.IndexerSubsystem{Server: host, Port: p, User: "admin", Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	c.scheme = "http"
	return c
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSearchAlerts_RequestAndDecode(t *testing.T) {
	var gotBody map[string]any
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/wazuh-alerts-*/_search" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if u, p, ok := r.BasicAuth(); !ok || u != "admin" || p != "pw" {
			t.Errorf("basic auth = %q/%q/%v", u, p, ok)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write(fixture(t, "search_alerts.json"))
	})

	alerts, err := c.SearchAlerts(context.Background(), AlertQuery{MinLevel: 5})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["size"] != float64(200) {
		t.Errorf("request size = %v", gotBody["size"])
	}
	if len(alerts) != 2 {
		t.Fatalf("len = %d, want 2", len(alerts))
	}
	a := alerts[0]
	if a.ID != "1791935883.1688070" || a.Rule.ID != "17101" || a.Rule.Level != 9 {
		t.Errorf("alert = %+v", a)
	}
	if a.Agent.Name != "black-host-01" || a.Agent.ID != "002" || a.Data["dstuser"] != "root" {
		t.Errorf("agent/data = %+v / %v", a.Agent, a.Data)
	}
	if len(a.Rule.MITRE.Tactic) != 4 || a.Rule.MITRE.ID[0] != "T1078" || a.Rule.FiredTimes != 8 {
		t.Errorf("rule = %+v", a.Rule)
	}
	if !a.Time.Equal(time.Date(2026, 10, 13, 23, 58, 3, 908_000_000, time.UTC)) {
		t.Errorf("time = %v (zero means the +0000 format was not parsed)", a.Time)
	}
	if !alerts[1].Time.Before(a.Time) {
		t.Error("results must keep the server's newest-first order")
	}
}

func TestSearchAlerts_EmptyIsNotAnError(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture(t, "search_empty.json"))
	})
	alerts, err := c.SearchAlerts(context.Background(), AlertQuery{})
	if err != nil || len(alerts) != 0 {
		t.Errorf("alerts=%v err=%v, want empty and nil", alerts, err)
	}
}

func TestSearchAlerts_Unauthorized(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	})
	_, err := c.SearchAlerts(context.Background(), AlertQuery{})
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("err = %v, want HTTP 401 with body", err)
	}
}

func TestClient_Addr(t *testing.T) {
	c, _ := New(config.IndexerSubsystem{Server: "127.0.0.1", Port: 9200})
	if c.Addr() != "127.0.0.1:9200" {
		t.Errorf("Addr = %q", c.Addr())
	}
}
