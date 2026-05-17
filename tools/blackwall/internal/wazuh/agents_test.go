package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListAgents_OK(t *testing.T) {
	authed := false
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authed = true
			_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
				Token string `json:"token"`
			}{Token: "tok"}})
		case "/agents":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(agentsResponse{Data: struct {
				AffectedItems []Agent `json:"affected_items"`
			}{AffectedItems: []Agent{
				{ID: "000", Name: "master", Status: "active"},
				{ID: "001", Name: "host-01", Status: "disconnected"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	agents, err := c.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if !authed {
		t.Error("client did not authenticate before listing")
	}
	if len(agents) != 2 || agents[0].ID != "000" || agents[1].ID != "001" {
		t.Errorf("agents = %+v", agents)
	}
}

func TestListAgents_ContextCancel(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Block forever; cancellation should terminate the request.
		<-r.Context().Done()
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"
	c.token = "tok" // skip Authenticate to keep test focused on the cancel path
	c.tokenExp = neverExpire()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.ListAgents(ctx); err == nil {
		t.Fatal("ListAgents: expected error on cancelled context")
	}
}
