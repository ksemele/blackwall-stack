package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetSyscheckLastScan_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscheck/001/last_scan":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(syscheckScanResponse{Data: struct {
				AffectedItems []SyscheckScan `json:"affected_items"`
			}{AffectedItems: []SyscheckScan{
				{Start: "2026-05-18T00:00:00Z", End: "2026-05-18T00:05:00Z"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	scan, err := c.GetSyscheckLastScan(context.Background(), "001")
	if err != nil {
		t.Fatalf("GetSyscheckLastScan: %v", err)
	}
	if scan.Start != "2026-05-18T00:00:00Z" || scan.End != "2026-05-18T00:05:00Z" {
		t.Errorf("scan = %+v", scan)
	}
}

func TestGetSyscheckLastScan_NonOK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	if _, err := c.GetSyscheckLastScan(context.Background(), "001"); err == nil {
		t.Fatal("expected error on non-200")
	}
}

func TestListSyscheckEntries_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscheck/001":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			if r.URL.Query().Get("limit") != "500" {
				t.Errorf("limit = %q, want 500", r.URL.Query().Get("limit"))
			}
			_ = json.NewEncoder(w).Encode(syscheckEntriesResponse{Data: struct {
				AffectedItems []SyscheckEntry `json:"affected_items"`
			}{AffectedItems: []SyscheckEntry{
				{File: "/etc/passwd", Type: "modified", MD5: "abc123"},
				{File: "/etc/shadow", Type: "added", SHA256: "deadbeef"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	entries, err := c.ListSyscheckEntries(context.Background(), "001")
	if err != nil {
		t.Fatalf("ListSyscheckEntries: %v", err)
	}
	if len(entries) != 2 || entries[0].File != "/etc/passwd" || entries[1].Type != "added" {
		t.Errorf("entries = %+v", entries)
	}
}

func TestListSyscheckEntries_NonOK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	if _, err := c.ListSyscheckEntries(context.Background(), "001"); err == nil {
		t.Fatal("expected error on non-200")
	}
}
