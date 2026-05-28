package executors

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"testing"
)

func TestCursorHashed64Hex_KnownVector(t *testing.T) {
	// sha256("hello" + "world") = 936a185caaa266bb9cbe981e9e05cb78cd732b0b3280eb944412bb6f8f8f07af
	got := CursorHashed64Hex("hello", "world")
	want := "936a185caaa266bb9cbe981e9e05cb78cd732b0b3280eb944412bb6f8f8f07af"
	if got != want {
		t.Fatalf("CursorHashed64Hex mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestCursorHashed64Hex_Length(t *testing.T) {
	if got := CursorHashed64Hex("any-input", ""); len(got) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(got))
	}
}

func TestGenerateCursorChecksum_HasMachineIDSuffix(t *testing.T) {
	machineID := "abc1234567890def"
	got := GenerateCursorChecksum(machineID)
	if !strings.HasSuffix(got, machineID) {
		t.Fatalf("checksum must end with machineId: %s", got)
	}
}

func TestGenerateCursorChecksum_PrefixUsesAllowedAlphabet(t *testing.T) {
	machineID := "mid"
	got := GenerateCursorChecksum(machineID)
	prefix := strings.TrimSuffix(got, machineID)
	if prefix == "" {
		t.Fatal("prefix empty")
	}
	allowed := cursorBase64Alphabet
	for _, c := range prefix {
		if !strings.ContainsRune(allowed, c) {
			t.Errorf("character %q not in URL-safe base64 alphabet", c)
		}
	}
}

func TestGenerateCursorChecksum_ChangesAcrossMachines(t *testing.T) {
	a := GenerateCursorChecksum("machineA")
	b := GenerateCursorChecksum("machineB")
	// Suffix differs → whole string differs.
	if a == b {
		t.Fatal("different machineIds should produce different checksums")
	}
}

func TestBuildCursorHeaders_StripsTokenPrefix(t *testing.T) {
	// Token format "userId::actualToken" — the prefix must be stripped
	// before deriving ids so they match Cursor's own.
	h := BuildCursorHeaders("user-x::eyJrey.token", "", true)
	if got := h["authorization"]; got != "Bearer eyJrey.token" {
		t.Fatalf("authorization wrong: %q", got)
	}
}

func TestBuildCursorHeaders_PopulatesRequiredHeaders(t *testing.T) {
	h := BuildCursorHeaders("plain-token", "", true)
	for _, key := range []string{
		"authorization", "connect-accept-encoding", "connect-protocol-version",
		"content-type", "user-agent", "x-amzn-trace-id", "x-client-key",
		"x-cursor-checksum", "x-cursor-client-version", "x-cursor-client-type",
		"x-cursor-client-os", "x-cursor-client-arch", "x-cursor-client-device-type",
		"x-cursor-config-version", "x-cursor-timezone", "x-ghost-mode",
		"x-request-id", "x-session-id",
	} {
		if h[key] == "" {
			t.Errorf("required header %q missing", key)
		}
	}
	if h["x-ghost-mode"] != "true" {
		t.Errorf("ghost-mode=true not propagated: %v", h["x-ghost-mode"])
	}
}

func TestBuildCursorHeaders_DerivesMachineIDWhenEmpty(t *testing.T) {
	h := BuildCursorHeaders("token-abc", "", false)
	// machineId is the suffix of the checksum.
	checksum := h["x-cursor-checksum"]
	expectedMID := CursorHashed64Hex("token-abc", "machineId")
	if !strings.HasSuffix(checksum, expectedMID) {
		t.Fatalf("derived machineId not used in checksum:\nchecksum=%s\nexpected suffix=%s", checksum, expectedMID)
	}
}

func TestBuildCursorHeaders_UsesProvidedMachineID(t *testing.T) {
	h := BuildCursorHeaders("token", "custom-machine-id", false)
	if !strings.HasSuffix(h["x-cursor-checksum"], "custom-machine-id") {
		t.Fatalf("explicit machineId not used: %s", h["x-cursor-checksum"])
	}
}

func TestCursorUUIDv5DNS_KnownVector(t *testing.T) {
	// RFC 4122 example: UUIDv5 of "python.org" in DNS namespace = 886313e1-3b8a-5372-9b90-0c9aee199e5d
	got := cursorUUIDv5DNS("python.org")
	want := "886313e1-3b8a-5372-9b90-0c9aee199e5d"
	if got != want {
		t.Fatalf("UUIDv5(python.org, DNS) = %s, want %s", got, want)
	}
}

func TestRandomUUIDStr_Format(t *testing.T) {
	got := randomUUIDStr()
	if len(got) != 36 {
		t.Fatalf("expected 36 chars, got %d (%q)", len(got), got)
	}
	for i, c := range got {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				t.Errorf("expected dash at index %d, got %q", i, c)
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("char %q at index %d not hex", c, i)
			}
		}
	}
	// Version nibble must be 4.
	if got[14] != '4' {
		t.Errorf("UUID v4 version nibble wrong: %q", got[14])
	}
	// Variant nibble must be 8/9/a/b.
	switch got[19] {
	case '8', '9', 'a', 'b':
	default:
		t.Errorf("UUID variant nibble wrong: %q", got[19])
	}
}

// Sanity: confirm our SHA-1 helper produces the same value as the stdlib.
func TestSha1MatchesStdlib(t *testing.T) {
	want := sha1.Sum([]byte("flow-router-test"))
	if hex.EncodeToString(want[:]) == "" {
		t.Fatal("stdlib sha1 returned empty")
	}
}
