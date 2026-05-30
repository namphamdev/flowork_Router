// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package mcpsecurity

import (
	"sort"
	"testing"
)

// helper: snapshot + restore the package state around a mutation test.
func withCleanList(t *testing.T, fn func()) {
	t.Helper()
	mu.Lock()
	orig := make(map[string]struct{}, len(allowed))
	for k, v := range allowed {
		orig[k] = v
	}
	mu.Unlock()
	defer func() {
		mu.Lock()
		allowed = orig
		mu.Unlock()
	}()
	fn()
}

func TestIsAllowed_DefaultEntries(t *testing.T) {
	for _, cmd := range []string{"npx", "node", "uvx", "python", "python3", "bunx", "bun", "deno", "pnpm", "yarn"} {
		if !IsAllowed(cmd) {
			t.Errorf("default allowlist missing %q", cmd)
		}
	}
}

func TestIsAllowed_RejectsArbitraryCommand(t *testing.T) {
	for _, bad := range []string{"rm", "curl", "bash", "sh", "../node", "evil-binary"} {
		if IsAllowed(bad) {
			t.Errorf("dangerous command should be blocked: %q", bad)
		}
	}
}

func TestIsAllowed_MatchesFullPath(t *testing.T) {
	cases := []string{
		"/usr/local/bin/npx",
		"/opt/python3",
		"/home/user/.nvm/versions/node/v22.0.0/bin/node",
	}
	for _, p := range cases {
		if !IsAllowed(p) {
			t.Errorf("absolute path should resolve to basename: %q", p)
		}
	}
}

func TestIsAllowed_StripsWindowsExtensions(t *testing.T) {
	cases := []string{
		"npx.cmd",
		"C:\\node\\node.exe",
		"yarn.cmd",
		"bunx.exe",
	}
	for _, p := range cases {
		if !IsAllowed(p) {
			t.Errorf("Windows extension should be stripped: %q", p)
		}
	}
}

func TestIsAllowed_EmptyRejected(t *testing.T) {
	if IsAllowed("") {
		t.Fatal("empty command must be rejected")
	}
}

func TestIsAllowed_CaseInsensitive(t *testing.T) {
	for _, c := range []string{"NPX", "Node", "PyThOn3"} {
		if !IsAllowed(c) {
			t.Errorf("matching should be case-insensitive: %q", c)
		}
	}
}

func TestAllow_AddsRuntimeEntry(t *testing.T) {
	withCleanList(t, func() {
		if IsAllowed("ruby") {
			t.Fatal("precondition: ruby should not start allowed")
		}
		Allow("ruby")
		if !IsAllowed("ruby") {
			t.Fatal("Allow() did not persist runtime entry")
		}
	})
}

func TestAllow_IgnoresBlank(t *testing.T) {
	withCleanList(t, func() {
		before := len(List())
		Allow("")
		Allow("   ")
		if len(List()) != before {
			t.Fatal("blank Allow() must not modify list")
		}
	})
}

func TestSet_ReplacesEntireList(t *testing.T) {
	withCleanList(t, func() {
		Set([]string{"only-this"})
		if !IsAllowed("only-this") {
			t.Fatal("Set() entry missing")
		}
		if IsAllowed("npx") {
			t.Fatal("Set() should have wiped defaults")
		}
	})
}

func TestDefaults_ReturnsCopy(t *testing.T) {
	d := Defaults()
	if len(d) < 7 {
		t.Fatalf("default list too short: %d entries", len(d))
	}
	d[0] = "mutated"
	// Re-fetch must be unchanged.
	again := Defaults()
	if again[0] == "mutated" {
		t.Fatal("Defaults() must return a copy, not the underlying slice")
	}
}

func TestList_SnapshotShape(t *testing.T) {
	l := List()
	sort.Strings(l)
	if len(l) < 7 {
		t.Fatalf("List() shrinking: %d entries", len(l))
	}
}
