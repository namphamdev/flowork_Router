// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Provider executor HTTP call.

// Cursor x-cursor-checksum header generator (Jyh cipher).
// The Cursor API rejects requests without a valid checksum derived from
// the current timestamp + the caller's machine id. This file ports the
// algorithm + a header bundle builder.

package executors

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"time"
)

const cursorBase64Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// CursorHashed64Hex returns the lowercase 64-char hex of sha256(input+salt).
// Used to derive deterministic machine ids and client keys from the access
// token when the caller didn't supply one.
func CursorHashed64Hex(input, salt string) string {
	sum := sha256.Sum256([]byte(input + salt))
	return hex.EncodeToString(sum[:])
}

// GenerateCursorChecksum produces the x-cursor-checksum value for the given
// machineId. The format is `<base64-jyh-cipher>{machineId}` where the prefix
// is the timestamp obfuscated with a rolling XOR + URL-safe base64.
func GenerateCursorChecksum(machineID string) string {
	// timestamp = floor(now_ms / 1_000_000); same scale as the JS source.
	ts := time.Now().UnixMilli() / 1_000_000

	bytes := [6]byte{
		byte((ts >> 40) & 0xFF),
		byte((ts >> 32) & 0xFF),
		byte((ts >> 24) & 0xFF),
		byte((ts >> 16) & 0xFF),
		byte((ts >> 8) & 0xFF),
		byte(ts & 0xFF),
	}

	// Jyh cipher: XOR-then-shift rolling key.
	t := byte(165)
	for i := 0; i < len(bytes); i++ {
		bytes[i] = byte((int(bytes[i]^t) + (i % 256)) & 0xFF)
		t = bytes[i]
	}

	// URL-safe base64 (no padding) using the same alphabet as upstream.
	var encoded []byte
	for i := 0; i < len(bytes); i += 3 {
		a := bytes[i]
		var b, c byte
		if i+1 < len(bytes) {
			b = bytes[i+1]
		}
		if i+2 < len(bytes) {
			c = bytes[i+2]
		}
		encoded = append(encoded, cursorBase64Alphabet[a>>2])
		encoded = append(encoded, cursorBase64Alphabet[((a&3)<<4)|(b>>4)])
		if i+1 < len(bytes) {
			encoded = append(encoded, cursorBase64Alphabet[((b&15)<<2)|(c>>6)])
		}
		if i+2 < len(bytes) {
			encoded = append(encoded, cursorBase64Alphabet[c&63])
		}
	}
	return string(encoded) + machineID
}

// BuildCursorHeaders returns the full header map Cursor's ConnectRPC
// endpoint expects. machineID is optional — when empty it's derived from
// the access token. ghostMode toggles the x-ghost-mode privacy header.
func BuildCursorHeaders(accessToken, machineID string, ghostMode bool) map[string]string {
	// Some tokens are prefixed "userId::actualToken" — strip the prefix
	// before any derivation so the deterministic ids match Cursor's own.
	cleanToken := accessToken
	for i := 0; i < len(cleanToken)-1; i++ {
		if cleanToken[i] == ':' && cleanToken[i+1] == ':' {
			cleanToken = cleanToken[i+2:]
			break
		}
	}

	if machineID == "" {
		machineID = CursorHashed64Hex(cleanToken, "machineId")
	}
	sessionID := cursorUUIDv5DNS(cleanToken)
	clientKey := CursorHashed64Hex(cleanToken, "")
	checksum := GenerateCursorChecksum(machineID)

	osName := "linux"
	switch runtime.GOOS {
	case "windows":
		osName = "windows"
	case "darwin":
		osName = "macos"
	}
	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "aarch64"
	}

	ghost := "false"
	if ghostMode {
		ghost = "true"
	}

	return map[string]string{
		"authorization":               "Bearer " + cleanToken,
		"connect-accept-encoding":     "gzip",
		"connect-protocol-version":    "1",
		"content-type":                "application/connect+proto",
		"user-agent":                  "connect-es/1.6.1",
		"x-amzn-trace-id":             "Root=" + randomUUIDStr(),
		"x-client-key":                clientKey,
		"x-cursor-checksum":           checksum,
		"x-cursor-client-version":     "3.1.0",
		"x-cursor-client-type":        "ide",
		"x-cursor-client-os":          osName,
		"x-cursor-client-arch":        arch,
		"x-cursor-client-device-type": "desktop",
		"x-cursor-config-version":     randomUUIDStr(),
		"x-cursor-timezone":           "UTC",
		"x-ghost-mode":                ghost,
		"x-request-id":                randomUUIDStr(),
		"x-session-id":                sessionID,
	}
}

// cursorUUIDv5DNS produces a UUID v5 from name in the DNS namespace.
// The DNS namespace is 6ba7b810-9dad-11d1-80b4-00c04fd430c8 per RFC 4122.
// RFC 4122 v5 mandates SHA-1 (not SHA-256).
func cursorUUIDv5DNS(name string) string {
	dnsNs := []byte{
		0x6b, 0xa7, 0xb8, 0x10,
		0x9d, 0xad,
		0x11, 0xd1,
		0x80, 0xb4,
		0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8,
	}
	h := sha1.New()
	h.Write(dnsNs)
	h.Write([]byte(name))
	hash := h.Sum(nil)
	// Set version (5) and variant (RFC 4122) bits.
	hash[6] = (hash[6] & 0x0F) | 0x50
	hash[8] = (hash[8] & 0x3F) | 0x80
	return formatUUID(hash[:16])
}

func formatUUID(b []byte) string {
	const hexd = "0123456789abcdef"
	out := make([]byte, 36)
	dashes := map[int]bool{8: true, 13: true, 18: true, 23: true}
	bi := 0
	for i := 0; i < 36; i++ {
		if dashes[i] {
			out[i] = '-'
			continue
		}
		out[i] = hexd[b[bi]>>4]
		i++
		out[i] = hexd[b[bi]&0xF]
		bi++
	}
	return string(out)
}

// randomUUIDStr returns a random v4 UUID. Used for per-request trace ids.
func randomUUIDStr() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0F) | 0x40 // v4
	b[8] = (b[8] & 0x3F) | 0x80 // variant
	return formatUUID(b[:])
}
