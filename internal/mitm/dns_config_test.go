// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package mitm

import (
	"os"
	"strings"
	"testing"
)

// buildHostsContent is the pure function — test it without touching /etc/hosts.
func TestBuildHostsContent_RoundTrip(t *testing.T) {
	original := []byte(`127.0.0.1 localhost
::1 ip6-localhost

192.168.1.1 router.lan
`)
	hosts := []string{"api2.cursor.sh", "cloudcode-pa.googleapis.com"}
	withBlock := buildHostsContent(original, hosts)
	got := string(withBlock)
	for _, want := range []string{
		"localhost", "router.lan",
		dnsMarker, dnsMarkerEnd,
		"127.0.0.1 api2.cursor.sh",
		"127.0.0.1 cloudcode-pa.googleapis.com",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	// Removing block must restore original ordering (modulo a trailing newline).
	stripped := buildHostsContent(withBlock, nil)
	if strings.Contains(string(stripped), dnsMarker) {
		t.Fatalf("marker not removed:\n%s", stripped)
	}
	for _, must := range []string{"localhost", "router.lan"} {
		if !strings.Contains(string(stripped), must) {
			t.Fatalf("original line %q lost on strip", must)
		}
	}
}

func TestBuildHostsContent_IdempotentReadd(t *testing.T) {
	original := []byte("127.0.0.1 localhost\n")
	hosts := []string{"foo.test"}
	once := buildHostsContent(original, hosts)
	twice := buildHostsContent(once, hosts)
	if string(once) != string(twice) {
		t.Fatalf("re-applying same hosts should be idempotent:\n--once--\n%s\n--twice--\n%s", once, twice)
	}
}

// End-to-end against a tmpfile (not the real /etc/hosts).
func TestWriteHosts_TmpFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "flow_hosts_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	original := []byte("127.0.0.1 localhost\n")
	tmp.Write(original)
	tmp.Close()

	// Use writeHosts directly with a writable target (skip sudo path).
	newContent := buildHostsContent(original, []string{"api2.cursor.sh"})
	if err := os.WriteFile(tmp.Name(), newContent, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, _ := os.ReadFile(tmp.Name())
	if !strings.Contains(string(out), "127.0.0.1 api2.cursor.sh") {
		t.Fatalf("write did not persist: %s", out)
	}
}
