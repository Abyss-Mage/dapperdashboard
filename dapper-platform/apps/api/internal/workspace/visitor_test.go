package workspace

import (
	"net/http/httptest"
	"testing"
)

// Through Caddy and the gateway the header reads "client, 127.0.0.1": each visitor must keep their own key.
func TestVisitorIsTheClientNotTheProxy(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:5000"
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 127.0.0.1")
	if got := visitor(r); got != "203.0.113.7" {
		t.Fatalf("visitor = %q, want the client address", got)
	}
	r.RemoteAddr = "198.51.100.9:5000"
	r.Header.Set("X-Forwarded-For", "10.0.0.1")
	if got := visitor(r); got != "198.51.100.9" {
		t.Fatalf("a non-proxy peer must not be able to choose its own key, got %q", got)
	}
}
