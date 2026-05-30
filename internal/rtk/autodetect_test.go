// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package rtk_test

import (
	"strings"
	"testing"

	"github.com/flowork-os/flowork_Router/internal/rtk"
	_ "github.com/flowork-os/flowork_Router/internal/rtk/filters" // register all filters
)

// helper: compress with an aggressive cap so detection runs even on small inputs.
func compress(text string) (string, int) {
	return rtk.Compress(text, 32) // tiny cap forces compression path
}

func TestAutoDetect_GitDiffWins(t *testing.T) {
	header := "diff --git a/foo.go b/foo.go\n@@ -1,500 +1,500 @@\n"
	// Long context block (dropped by git-diff filter — single-space prefix).
	body := strings.Repeat(" context line stays untouched in upstream\n", 200)
	in := header + body + "-removed line\n+added line\n" + body
	out, saved := compress(in)
	if saved == 0 {
		t.Fatalf("expected compression, got %d saved", saved)
	}
	if !strings.Contains(out, "diff --git") {
		t.Fatalf("git-diff filter dropped the header: %s", out[:200])
	}
}

func TestAutoDetect_BuildOutputBeforePorcelain(t *testing.T) {
	// Cargo "Compiling x.rs" lines also match porcelain pattern.  Build-output
	// must win, otherwise the wrong compactor strips them.
	in := strings.Repeat("   Compiling crate1 v0.1.0\n   Compiling crate2 v0.2.0\n", 30) +
		"    Finished release [optimized] target(s) in 1.5s\n"
	out, saved := compress(in)
	if saved == 0 {
		t.Fatalf("expected build-output compression, got 0")
	}
	if strings.Contains(out, "git-status") {
		t.Fatalf("misclassified as git-status")
	}
	// build-output trims duplicate Compiling lines but preserves the Finished marker.
	if !strings.Contains(out, "Finished") {
		t.Fatalf("Finished marker lost: %s", out)
	}
}

func TestAutoDetect_GrepLines(t *testing.T) {
	in := strings.Repeat("src/main.go:42:fmt.Println(\"x\")\n", 200)
	out, saved := compress(in)
	if saved == 0 {
		t.Fatalf("grep should compact")
	}
	if !strings.Contains(out, "src/main.go") {
		t.Fatalf("grep filter dropped path data: %s", out[:200])
	}
}

func TestAutoDetect_LsListing(t *testing.T) {
	in := `total 24
-rw-r--r-- 1 user user 100 May 28 a.txt
drwxr-xr-x 2 user user 200 May 28 dir
-rwxr-xr-x 1 user user 300 May 28 b.sh
` + strings.Repeat("-rw-r--r-- 1 user user 100 May 28 file.txt\n", 100)
	_, saved := compress(in)
	if saved == 0 {
		t.Fatalf("ls listing should be compacted")
	}
}

func TestAutoDetect_TreeOutput(t *testing.T) {
	in := strings.Repeat(`├── src
│   ├── a.go
│   └── b.go
└── README.md
`, 30)
	_, saved := compress(in)
	if saved == 0 {
		t.Fatalf("tree should be compacted")
	}
}

func TestAutoDetect_NoFalsePositive_ShortPlain(t *testing.T) {
	// Tiny plain text — no filter should apply, Compress returns input as-is.
	in := "Hello, world.\n"
	out, saved := compress(in)
	if saved != 0 || out != in {
		t.Fatalf("small input must be untouched: got saved=%d out=%q", saved, out)
	}
}

func TestAutoDetect_FallbackToDedupLog(t *testing.T) {
	// Many duplicate generic lines — should fall through to dedup-log, NOT
	// misclassify as ls / grep / tree.
	in := strings.Repeat("hello world\n", 200)
	out, saved := compress(in)
	if saved == 0 {
		t.Fatalf("dedup-log should kick in for duplicate noise")
	}
	if len(out) >= len(in) {
		t.Fatalf("dedup-log did not reduce size")
	}
}
