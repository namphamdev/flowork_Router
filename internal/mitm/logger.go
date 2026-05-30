// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — MITM proxy module.

// MITM request/response dumper.

package mitm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

var dumpCounter uint64

func dumpDir() string { return filepath.Join(MITMDir(), "dumps") }

// ClearDumpDir wipes all previously written dump files. Called at MITM start to
// keep disk usage bounded.
func ClearDumpDir() {
	dir := dumpDir()
	_ = os.RemoveAll(dir)
	_ = os.MkdirAll(dir, 0o700)
}

// DumpRequest writes <ts>-<n>-<host>-req.txt with method, URL, headers, body.
// Returns the dump filename (without dir) and a writer for the matching
// response file via CreateResponseDumper.
func DumpRequest(host, method, urlStr string, headers map[string]string, body []byte) string {
	for _, blk := range LogBlacklistURLParts {
		if strings.Contains(urlStr, blk) {
			return ""
		}
	}
	n := atomic.AddUint64(&dumpCounter, 1)
	ts := time.Now().UTC().Format("20060102T150405.000Z")
	name := fmt.Sprintf("%s-%04d-%s-req.txt", ts, n, sanitizeFile(host))
	_ = os.MkdirAll(dumpDir(), 0o700)
	full := filepath.Join(dumpDir(), name)
	var b strings.Builder
	b.WriteString(method + " " + urlStr + "\n")
	for k, v := range headers {
		b.WriteString(k + ": " + v + "\n")
	}
	b.WriteString("\n")
	b.Write(body)
	_ = os.WriteFile(full, []byte(b.String()), 0o600)
	return name
}

// CreateResponseDumper returns a writer for the response payload, paired with
// the request dump returned by DumpRequest.
func CreateResponseDumper(requestDumpName string) func(status int, headers map[string]string, body []byte) {
	if requestDumpName == "" {
		return func(int, map[string]string, []byte) {}
	}
	respName := strings.TrimSuffix(requestDumpName, "-req.txt") + "-resp.txt"
	full := filepath.Join(dumpDir(), respName)
	return func(status int, headers map[string]string, body []byte) {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("HTTP %d\n", status))
		for k, v := range headers {
			b.WriteString(k + ": " + v + "\n")
		}
		b.WriteString("\n")
		b.Write(body)
		_ = os.WriteFile(full, []byte(b.String()), 0o600)
	}
}

func sanitizeFile(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
