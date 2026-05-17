package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListSCAPolicies_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/sca/001":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(scaPoliciesResponse{Data: struct {
				AffectedItems []SCAPolicy `json:"affected_items"`
			}{AffectedItems: []SCAPolicy{
				{PolicyID: "cis_debian10", Name: "CIS Debian 10", Pass: 42, Fail: 8, Score: 84},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	policies, err := c.ListSCAPolicies(context.Background(), "001")
	if err != nil {
		t.Fatalf("ListSCAPolicies: %v", err)
	}
	if len(policies) != 1 || policies[0].PolicyID != "cis_debian10" || policies[0].Score != 84 {
		t.Errorf("policies = %+v", policies)
	}
}

func TestListSCAPolicies_NonOK(t *testing.T) {
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

	if _, err := c.ListSCAPolicies(context.Background(), "001"); err == nil {
		t.Fatal("expected error on non-200")
	}
}

func TestListSCAChecks_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/sca/001/checks/cis_debian10":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(scaChecksResponse{Data: struct {
				AffectedItems []SCACheck `json:"affected_items"`
			}{AffectedItems: []SCACheck{
				{ID: 1001, PolicyID: "cis_debian10", Title: "Ensure SSH is configured", Result: "passed"},
				{ID: 1002, PolicyID: "cis_debian10", Title: "Ensure root login is disabled", Result: "failed"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	checks, err := c.ListSCAChecks(context.Background(), "001", "cis_debian10")
	if err != nil {
		t.Fatalf("ListSCAChecks: %v", err)
	}
	if len(checks) != 2 || checks[0].Result != "passed" || checks[1].ID != 1002 {
		t.Errorf("checks = %+v", checks)
	}
}

func TestListSCAChecks_NonOK(t *testing.T) {
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

	if _, err := c.ListSCAChecks(context.Background(), "001", "cis_debian10"); err == nil {
		t.Fatal("expected error on non-200")
	}
}
