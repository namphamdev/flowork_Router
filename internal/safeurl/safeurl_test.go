package safeurl

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestIsPublic_PrivateRanges(t *testing.T) {
	bad := []string{
		"127.0.0.1", "127.1.2.3", "::1",
		"10.0.0.1", "10.255.255.254",
		"172.16.0.1", "172.31.255.254",
		"192.168.0.1", "192.168.255.254",
		"169.254.169.254", // AWS metadata
		"100.64.0.1",      // CGNAT
		"100.127.255.254", // CGNAT high
		"0.0.0.0",
		"fc00::1", "fd00::1", // ULA
		"fe80::1", // link-local v6
		"ff02::1", // multicast v6
		"224.0.0.1",
	}
	for _, s := range bad {
		ip := net.ParseIP(s)
		if IsPublic(ip) {
			t.Errorf("expected %s to be NOT public", s)
		}
	}
}

func TestIsPublic_PublicAddresses(t *testing.T) {
	good := []string{
		"8.8.8.8", "1.1.1.1", "151.101.0.1",
		"2606:4700::6810:84e5", // Cloudflare v6
	}
	for _, s := range good {
		ip := net.ParseIP(s)
		if !IsPublic(ip) {
			t.Errorf("expected %s to be public", s)
		}
	}
}

func TestValidate_RejectsScheme(t *testing.T) {
	ctx := context.Background()
	for _, raw := range []string{
		"file:///etc/passwd",
		"gopher://example.com/",
		"ftp://example.com/",
		"javascript:alert(1)",
	} {
		if _, err := Validate(ctx, raw); err == nil {
			t.Errorf("expected scheme rejection for %s", raw)
		}
	}
}

func TestValidate_RejectsLoopbackLiteral(t *testing.T) {
	ctx := context.Background()
	if _, err := Validate(ctx, "http://127.0.0.1:2402/admin"); err == nil {
		t.Fatal("loopback literal should be blocked")
	}
}

func TestValidate_RejectsMetadataLiteral(t *testing.T) {
	ctx := context.Background()
	if _, err := Validate(ctx, "http://169.254.169.254/latest/meta-data/"); err == nil {
		t.Fatal("AWS metadata literal should be blocked")
	}
}

func TestValidate_RejectsRFC1918Literal(t *testing.T) {
	ctx := context.Background()
	for _, raw := range []string{
		"http://10.0.0.1/",
		"https://192.168.1.1/",
		"http://172.16.0.1/",
	} {
		if _, err := Validate(ctx, raw); err == nil {
			t.Errorf("private literal should be blocked: %s", raw)
		}
	}
}

func TestValidate_EmptyHostAndMalformed(t *testing.T) {
	ctx := context.Background()
	if _, err := Validate(ctx, "http:///foo"); err == nil {
		t.Fatal("missing host should error")
	}
	if _, err := Validate(ctx, "not a url"); err == nil {
		t.Fatal("malformed should error")
	}
}

func TestValidate_AcceptsPublicLiteral(t *testing.T) {
	ctx := context.Background()
	u, err := Validate(ctx, "https://8.8.8.8/")
	if err != nil {
		t.Fatalf("public literal rejected: %v", err)
	}
	if u.Host != "8.8.8.8" {
		t.Errorf("host mismatch: %s", u.Host)
	}
}

func TestValidate_ErrorMessageContainsBlocked(t *testing.T) {
	ctx := context.Background()
	_, err := Validate(ctx, "http://127.0.0.1/")
	if err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("expected non-public error, got: %v", err)
	}
}
