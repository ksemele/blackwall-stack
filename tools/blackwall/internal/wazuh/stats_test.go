package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetAgentStats_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/agents/001/stats/agent":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(agentStatsResponse{Data: struct {
				AffectedItems []AgentStats `json:"affected_items"`
			}{AffectedItems: []AgentStats{
				{Events: 12345, EventsDropped: 0, EventsPerSecond: 3.2, Bytes: 98765},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	stats, err := c.GetAgentStats(context.Background(), "001")
	if err != nil {
		t.Fatalf("GetAgentStats: %v", err)
	}
	if stats.Events != 12345 || stats.Bytes != 98765 {
		t.Errorf("stats = %+v", stats)
	}
}

func TestGetAgentStats_NonOK(t *testing.T) {
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

	if _, err := c.GetAgentStats(context.Background(), "001"); err == nil {
		t.Fatal("expected error on non-200")
	}
}

func TestGetLogcollectorStats_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/agents/001/stats/logcollector":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(logcollectorStatsResponse{Data: struct {
				AffectedItems []LogcollectorStats `json:"affected_items"`
			}{AffectedItems: []LogcollectorStats{
				{Events: 500, EventsDropped: 5, EventsPerSecond: 1.1, Bytes: 2048},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	stats, err := c.GetLogcollectorStats(context.Background(), "001")
	if err != nil {
		t.Fatalf("GetLogcollectorStats: %v", err)
	}
	if stats.Events != 500 || stats.EventsDropped != 5 {
		t.Errorf("stats = %+v", stats)
	}
}

func TestGetLogcollectorStats_NonOK(t *testing.T) {
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

	if _, err := c.GetLogcollectorStats(context.Background(), "001"); err == nil {
		t.Fatal("expected error on non-200")
	}
}
