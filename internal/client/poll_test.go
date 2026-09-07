package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// CheckMK answers its wait-for-completion endpoints with a path-only Location
// header, so the poll loop must resolve it against the request URL.
func TestPollSelfRedirectingCompletion_RelativeLocation(t *testing.T) {
	const waitPath = "/cmk/check_mk/api/1.0/objects/activation_run/abc/actions/wait-for-completion/invoke"

	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		if len(requests) == 1 {
			w.Header().Set("Location", waitPath)
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := &Client{HTTPClient: server.Client()}
	if err := c.pollSelfRedirectingCompletion(context.Background(), server.URL+waitPath); err != nil {
		t.Fatalf("poll failed: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d: %v", len(requests), requests)
	}
	for _, path := range requests {
		if path != waitPath {
			t.Errorf("expected request to %s, got %s", waitPath, path)
		}
	}
}
