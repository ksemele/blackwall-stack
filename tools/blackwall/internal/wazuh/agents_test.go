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

func TestGetAgent_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
				Token string `json:"token"`
			}{Token: "tok"}})
		case "/agents":
			if r.URL.Query().Get("agents_list") != "001" {
				t.Errorf("agents_list = %q, want 001", r.URL.Query().Get("agents_list"))
			}
			_ = json.NewEncoder(w).Encode(singleAgentResponse{Data: struct {
				AffectedItems []Agent `json:"affected_items"`
			}{AffectedItems: []Agent{
				{ID: "001", Name: "host-01", Status: "active"},
			}}})
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	got, err := c.GetAgent(context.Background(), "001")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.ID != "001" {
		t.Errorf("ID = %q, want 001", got.ID)
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
				Token string `json:"token"`
			}{Token: "tok"}})
		case "/agents":
			// Empty affected_items list = not found.
			_ = json.NewEncoder(w).Encode(singleAgentResponse{})
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	_, err := c.GetAgent(context.Background(), "999")
	if err != ErrAgentNotFound {
		t.Errorf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestDoAuthed_RetriesOn401(t *testing.T) {
	calls := 0
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
				Token string `json:"token"`
			}{Token: "tok"}})
		case "/agents":
			calls++
			if calls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(agentsResponse{})
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	if _, err := c.ListAgents(context.Background()); err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if calls != 2 {
		t.Errorf("agents calls = %d, want 2 (initial + retry)", calls)
	}
}
