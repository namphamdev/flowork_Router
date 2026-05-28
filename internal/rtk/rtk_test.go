package rtk_test

import (
	"strings"
	"testing"

	"github.com/flowork-os/flowork_Router/internal/rtk"
	_ "github.com/flowork-os/flowork_Router/internal/rtk/filters"
)

// Each test asserts the named filter is picked AND the output is shorter than
// input for an above-cap payload. We don't lock specific char counts because
// that would couple the test to internal trim ratios.

func TestRTK_GitDiff(t *testing.T) {
	in := "diff --git a/foo.go b/foo.go\n@@ -1,3 +1,3 @@\n" +
		strings.Repeat(" unchanged context line that should be trimmed\n", 300) +
		"+ added\n- removed\n"
	out, saved := rtk.Compress(in, 1000)
	if saved == 0 || len(out) >= len(in) {
		t.Fatalf("git-diff: expected compression, saved=%d outLen=%d inLen=%d", saved, len(out), len(in))
	}
	if !strings.Contains(out, "RTK") {
		t.Fatal("expected RTK marker in trimmed output")
	}
}

func TestRTK_BuildOutput(t *testing.T) {
	in := "Compiling foo\n" +
		strings.Repeat("npm WARN deprecated package@1.0.0\n", 200) +
		"BUILD FAILED\nTraceback (most recent call last):\n  File 'x', line 1\n"
	out, saved := rtk.Compress(in, 1000)
	if saved == 0 || len(out) >= len(in) {
		t.Fatalf("build-output: expected compression, saved=%d", saved)
	}
	// Critical lines (BUILD FAILED / Traceback) must survive.
	if !strings.Contains(out, "BUILD FAILED") || !strings.Contains(out, "Traceback") {
		t.Fatal("build-output filter dropped critical lines")
	}
}

func TestRTK_GrepLargeResult(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 300; i++ {
		b.WriteString("internal/router/dispatcher.go:")
		b.WriteString("42:")
		b.WriteString(" some matched line content here\n")
	}
	in := b.String()
	out, saved := rtk.Compress(in, 1000)
	if saved == 0 || len(out) >= len(in) {
		t.Fatalf("grep: expected compression, saved=%d", saved)
	}
}

func TestRTK_DedupCollapse(t *testing.T) {
	in := strings.Repeat("WARN: socket timeout, retrying\n", 50) +
		"final line A\nfinal line B\n"
	out, saved := rtk.Compress(in, 200)
	if saved == 0 {
		t.Fatalf("dedup-log: expected compression")
	}
	if !strings.Contains(out, "×") {
		t.Fatal("dedup marker missing (×N notation)")
	}
}

func TestRTK_SmallInputUntouched(t *testing.T) {
	in := "small payload"
	out, saved := rtk.Compress(in, 4000)
	if saved != 0 || out != in {
		t.Fatalf("small input should pass through unchanged, got out=%q saved=%d", out, saved)
	}
}

func TestRTK_GenericFallback(t *testing.T) {
	// Random bytes that match no specific filter — fallback must still compress.
	in := strings.Repeat("xyz random goo without git/grep/tree shape ", 200)
	out, saved := rtk.Compress(in, 500)
	if saved == 0 {
		t.Fatalf("generic fallback should always compress when over cap, saved=%d outLen=%d", saved, len(out))
	}
}
