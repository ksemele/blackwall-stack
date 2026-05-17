package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetRootcheckLastScan_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/rootcheck/001/last_scan":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(rootcheckScanResponse{Data: struct {
				AffectedItems []RootcheckScan `json:"affected_items"`
			}{AffectedItems: []RootcheckScan{
				{Start: "2026-05-18T01:00:00Z", End: "2026-05-18T01:10:00Z"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	scan, err := c.GetRootcheckLastScan(context.Background(), "001")
	if err != nil {
		t.Fatalf("GetRootcheckLastScan: %v", err)
	}
	if scan.Start != "2026-05-18T01:00:00Z" || scan.End != "2026-05-18T01:10:00Z" {
		t.Errorf("scan = %+v", scan)
	}
}

func TestGetRootcheckLastScan_NonOK(t *testing.T) {
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

	if _, err := c.GetRootcheckLastScan(context.Background(), "001"); err == nil {
		t.Fatal("expected error on non-200")
	}
}

func TestListRootcheckFindings_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/rootcheck/001":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			if r.URL.Query().Get("limit") != "500" {
				t.Errorf("limit = %q, want 500", r.URL.Query().Get("limit"))
			}
			_ = json.NewEncoder(w).Encode(rootcheckFindingsResponse{Data: struct {
				AffectedItems []RootcheckFinding `json:"affected_items"`
			}{AffectedItems: []RootcheckFinding{
				{Log: "No passwd lock", Status: "outstanding"},
				{Log: "SSH root login allowed", Status: "solved"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	findings, err := c.ListRootcheckFindings(context.Background(), "001")
	if err != nil {
		t.Fatalf("ListRootcheckFindings: %v", err)
	}
	if len(findings) != 2 || findings[0].Status != "outstanding" || findings[1].Log != "SSH root login allowed" {
		t.Errorf("findings = %+v", findings)
	}
}

func TestListRootcheckFindings_NonOK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	if _, err := c.ListRootcheckFindings(context.Background(), "001"); err == nil {
		t.Fatal("expected error on non-200")
	}
}
