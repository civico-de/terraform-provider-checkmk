package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A pinned version must not make the client ask the site for one: creating the client
// then works against a site that is not reachable yet.
func TestNewClientWithOptions_PinnedVersionSkipsDetection(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer srv.Close()

	pinned, err := ParseVersion("2.4.0p10")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	c, err := NewClientWithOptions(srv.URL, "automation", "secret", &ClientOptions{Version: pinned, MaxRetries: 1})
	if err != nil {
		t.Fatalf("client creation with a pinned version must not fail: %v", err)
	}
	if requests != 0 {
		t.Fatalf("expected no request during client creation, got %d", requests)
	}
	if c.Version.String() != pinned.String() {
		t.Fatalf("expected version %s, got %s", pinned, c.Version)
	}
}

// Without a pin the client still asks the site, so an unreachable site fails creation.
func TestNewClientWithOptions_DetectsVersionByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/check_mk/api/1.0/version" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versions":{"checkmk":"2.3.0p5"}}`))
	}))
	defer srv.Close()

	c, err := NewClientWithOptions(srv.URL, "automation", "secret", &ClientOptions{MaxRetries: 1})
	if err != nil {
		t.Fatalf("client creation: %v", err)
	}
	if c.Version.String() != "2.3.0p5" {
		t.Fatalf("expected detected version 2.3.0p5, got %s", c.Version)
	}
}
