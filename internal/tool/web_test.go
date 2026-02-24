package tool

import (
	"net"
	"testing"
)

func TestValidateWebFetchURL_BlocksUnsupportedScheme(t *testing.T) {
	if _, err := validateWebFetchURL("file:///etc/passwd"); err == nil {
		t.Fatalf("expected unsupported scheme to be blocked")
	}
}

func TestValidateWebFetchURL_BlocksLocalhost(t *testing.T) {
	if _, err := validateWebFetchURL("http://localhost:8080"); err == nil {
		t.Fatalf("expected localhost to be blocked")
	}
}

func TestValidateWebFetchURL_BlocksPrivateResolvedAddress(t *testing.T) {
	oldLookup := webFetchLookupIP
	webFetchLookupIP = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.0.0.42")}, nil
	}
	t.Cleanup(func() { webFetchLookupIP = oldLookup })

	if _, err := validateWebFetchURL("https://example.internal/path"); err == nil {
		t.Fatalf("expected private resolution to be blocked")
	}
}

func TestValidateWebFetchURL_AllowsPublicResolvedAddress(t *testing.T) {
	oldLookup := webFetchLookupIP
	webFetchLookupIP = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}
	t.Cleanup(func() { webFetchLookupIP = oldLookup })

	if _, err := validateWebFetchURL("https://example.com/path"); err != nil {
		t.Fatalf("expected public host to be allowed, got error: %v", err)
	}
}
