// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file. Standard test patterns.

package updater

import "testing"

func TestIsNewer_BasicSemver(t *testing.T) {
	cases := []struct {
		remote, current string
		want            bool
	}{
		{"1.0.1", "1.0.0", true},
		{"v1.2.0", "v1.1.9", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.1", false},
		{"2.0", "1.99", true},
		{"1.0.0", "v1.0.0", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.remote, c.current); got != c.want {
			t.Fatalf("IsNewer(%q, %q) = %v, want %v", c.remote, c.current, got, c.want)
		}
	}
}

func TestAssetForPlatform_MatchesOSArch(t *testing.T) {
	rel := &Release{
		Assets: []Asset{
			{Name: "flow-router-darwin-arm64.tar.gz"},
			{Name: "flow-router-linux-amd64.tar.gz"},
			{Name: "flow-router-windows-amd64.zip"},
		},
	}
	a := AssetForPlatform(rel)
	if a == nil {
		t.Fatal("no asset matched current platform — release should have at least one")
	}
}

func TestNumPrefix(t *testing.T) {
	if numPrefix("123abc") != 123 {
		t.Fatal("123abc")
	}
	if numPrefix("abc") != 0 {
		t.Fatal("abc")
	}
	if numPrefix("") != 0 {
		t.Fatal("empty")
	}
}
