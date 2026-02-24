package channel

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIP_UsesForwardedOnlyFromPrivateProxy(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.RemoteAddr = "10.0.0.2:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	ip := clientIP(req)
	if ip != "203.0.113.7" {
		t.Fatalf("expected forwarded client IP from trusted proxy, got %q", ip)
	}
}

func TestClientIP_IgnoresForwardedFromPublicRemote(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.RemoteAddr = "8.8.8.8:44321"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	ip := clientIP(req)
	if ip != "8.8.8.8" {
		t.Fatalf("expected remote IP when proxy is untrusted, got %q", ip)
	}
}

func TestGoogleOAuthState_CreateAndConsume(t *testing.T) {
	w := &WebChannel{}
	state, err := w.createGoogleOAuthState("sess-1")
	if err != nil {
		t.Fatalf("createGoogleOAuthState: %v", err)
	}

	if ok := w.consumeGoogleOAuthState(state, "sess-1"); !ok {
		t.Fatalf("expected oauth state to validate once")
	}
	if ok := w.consumeGoogleOAuthState(state, "sess-1"); ok {
		t.Fatalf("expected oauth state replay to fail")
	}
}

func TestGoogleOAuthState_RejectsExpired(t *testing.T) {
	w := &WebChannel{}
	w.googleOAuthStates.Store("expired", googleOAuthState{
		SessionToken: "sess-2",
		ExpiresAt:    time.Now().Add(-time.Minute),
	})

	if ok := w.consumeGoogleOAuthState("expired", "sess-2"); ok {
		t.Fatalf("expected expired oauth state to be rejected")
	}
}
