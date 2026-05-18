package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// TestCacheHit verifies that two successive GetHardware calls on the same
// agent ID hit the HTTP server only once.
func TestCacheHit(t *testing.T) {
	var hits atomic.Int32
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/hardware":
			hits.Add(1)
			_ = json.NewEncoder(w).Encode(hardwareResponse{Data: struct {
				AffectedItems []Hardware `json:"affected_items"`
			}{AffectedItems: []Hardware{{BoardSerial: "SN-001", CPUName: "TestCPU", CPUCores: 4}}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})

	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	hw1, err := c.GetHardware(context.Background(), "001")
	if err != nil {
		t.Fatalf("first GetHardware: %v", err)
	}
	hw2, err := c.GetHardware(context.Background(), "001")
	if err != nil {
		t.Fatalf("second GetHardware: %v", err)
	}

	if hw1.BoardSerial != hw2.BoardSerial {
		t.Errorf("cache returned different value: %v vs %v", hw1, hw2)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("HTTP server hit %d times, want 1 (cache miss)", n)
	}
}

// TestCacheTTLExpiry verifies that after expiry the cache goes back to the
// server. We manipulate the entry's expiresAt directly to avoid a real sleep.
func TestCacheTTLExpiry(t *testing.T) {
	var hits atomic.Int32
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/hardware":
			hits.Add(1)
			_ = json.NewEncoder(w).Encode(hardwareResponse{Data: struct {
				AffectedItems []Hardware `json:"affected_items"`
			}{AffectedItems: []Hardware{{BoardSerial: "SN-EXPIRED", CPUName: "TestCPU", CPUCores: 2}}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})

	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	// Populate cache.
	if _, err := c.GetHardware(context.Background(), "001"); err != nil {
		t.Fatalf("first GetHardware: %v", err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("expected 1 hit after first call, got %d", n)
	}

	// Manually expire the cache entry.
	c.cacheMu.Lock()
	e := c.cache["hardware:001"]
	e.expiresAt = time.Now().Add(-time.Second)
	c.cache["hardware:001"] = e
	c.cacheMu.Unlock()

	// Should fetch again.
	if _, err := c.GetHardware(context.Background(), "001"); err != nil {
		t.Fatalf("second GetHardware after expiry: %v", err)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("HTTP server hit %d times after expiry, want 2", n)
	}
}

// TestCacheMissOnError verifies that a fetch error does not populate the cache,
// so the next call retries the HTTP server.
func TestCacheMissOnError(t *testing.T) {
	var hits atomic.Int32
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/hardware":
			hits.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})

	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	_, err1 := c.GetHardware(context.Background(), "001")
	if err1 == nil {
		t.Fatal("expected error on 500, got nil")
	}
	_, err2 := c.GetHardware(context.Background(), "001")
	if err2 == nil {
		t.Fatal("expected second error on 500, got nil")
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("HTTP server hit %d times, want 2 (no cache on error)", n)
	}
}

// TestCacheKeyIsolation verifies that different agent IDs maintain independent
// cache entries and each triggers its own HTTP hit.
func TestCacheKeyIsolation(t *testing.T) {
	var hits001, hits002 atomic.Int32
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/hardware":
			hits001.Add(1)
			_ = json.NewEncoder(w).Encode(hardwareResponse{Data: struct {
				AffectedItems []Hardware `json:"affected_items"`
			}{AffectedItems: []Hardware{{BoardSerial: "SN-001"}}}})
		case "/syscollector/002/hardware":
			hits002.Add(1)
			_ = json.NewEncoder(w).Encode(hardwareResponse{Data: struct {
				AffectedItems []Hardware `json:"affected_items"`
			}{AffectedItems: []Hardware{{BoardSerial: "SN-002"}}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})

	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	hw1, err := c.GetHardware(context.Background(), "001")
	if err != nil {
		t.Fatalf("GetHardware 001: %v", err)
	}
	hw2, err := c.GetHardware(context.Background(), "002")
	if err != nil {
		t.Fatalf("GetHardware 002: %v", err)
	}

	if hw1.BoardSerial != "SN-001" {
		t.Errorf("001 board serial = %q, want SN-001", hw1.BoardSerial)
	}
	if hw2.BoardSerial != "SN-002" {
		t.Errorf("002 board serial = %q, want SN-002", hw2.BoardSerial)
	}
	if hits001.Load() != 1 || hits002.Load() != 1 {
		t.Errorf("hits: 001=%d 002=%d, want 1 each", hits001.Load(), hits002.Load())
	}
}

// TestInvalidateCache verifies that InvalidateCache causes the next call to
// bypass the cache and hit the HTTP server.
func TestInvalidateCache(t *testing.T) {
	var hits atomic.Int32
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/hardware":
			hits.Add(1)
			_ = json.NewEncoder(w).Encode(hardwareResponse{Data: struct {
				AffectedItems []Hardware `json:"affected_items"`
			}{AffectedItems: []Hardware{{BoardSerial: "SN-FRESH"}}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})

	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	if _, err := c.GetHardware(context.Background(), "001"); err != nil {
		t.Fatalf("first GetHardware: %v", err)
	}
	// Second call should use cache (hits stays at 1).
	if _, err := c.GetHardware(context.Background(), "001"); err != nil {
		t.Fatalf("second GetHardware: %v", err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("expected 1 hit before invalidate, got %d", n)
	}

	c.InvalidateCache()

	// After invalidation, should hit server again.
	if _, err := c.GetHardware(context.Background(), "001"); err != nil {
		t.Fatalf("third GetHardware: %v", err)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("expected 2 hits after invalidate, got %d", n)
	}
}
