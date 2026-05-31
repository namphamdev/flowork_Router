package main

import (
	"context"
	"strings"
	"testing"
)

// Tests the SSRF guard used by the MCP HTTP gateway + media/provider base-URL
// fetchers. The contract: block link-local / cloud-metadata targets, but ALLOW
// loopback + LAN (legitimate local-server use for a single-user tool).
func TestBlockMetadataURL(t *testing.T) {
	ctx := context.Background()

	// Must be BLOCKED — link-local / cloud-metadata range (169.254.0.0/16).
	blocked := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://169.254.169.254/",
		"https://169.254.0.1/x",
	}
	for _, u := range blocked {
		if _, err := blockMetadataURL(ctx, u); err == nil {
			t.Errorf("expected %q to be BLOCKED, got nil error", u)
		}
	}

	// Must be ALLOWED — loopback/LAN literal IPs (local servers) + empty.
	allowed := []string{
		"http://127.0.0.1:2402/v1",
		"http://192.168.1.50:11434/api",
		"https://10.0.0.5/x",
		"", // empty → no-op, caller handles
	}
	for _, u := range allowed {
		if _, err := blockMetadataURL(ctx, u); err != nil {
			t.Errorf("expected %q to be ALLOWED, got err: %v", u, err)
		}
	}

	// Bad scheme rejected (defense-in-depth: no file://, gopher://, etc.).
	if _, err := blockMetadataURL(ctx, "file:///etc/passwd"); err == nil {
		t.Error("expected file:// scheme to be rejected")
	}
	if _, err := blockMetadataURL(ctx, "gopher://x/y"); err == nil {
		t.Error("expected gopher:// scheme to be rejected")
	}
	// Error message mentions the guard so operators can diagnose 403s.
	if _, err := blockMetadataURL(ctx, "http://169.254.169.254/"); err == nil || !strings.Contains(err.Error(), "link-local") {
		t.Errorf("expected link-local error message, got: %v", err)
	}
}
