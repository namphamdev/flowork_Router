// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Pure utility (no DB, no I/O, no global state). Adversarial-audit
//   passed: input cap MaxSanitizeBytes=256KB cegah O(n²) DOS. Sanitize() jadi
//   pondasi semua ingest path — kalau lo butuh normalisasi beda (HTML strip,
//   markdown stage), BUAT FILE BARU di package ini, JANGAN ubah Sanitize.
//
// sanitize.go — strip control chars, normalize whitespace, trim. Cheap,
// no external deps. Tujuan: cegah binary garbage masuk drawer FTS, normalisasi
// agar content_hash stabil (mis. "foo\r\n" dan "foo\n" → hash sama).
package ingest

import (
	"strings"
	"unicode"
)

// MaxSanitizeBytes — hard cap input length sebelum normalize. Anti DOS:
// caller kirim 100MB string whitespace → loop ReplaceAll O(n²). 256KB cukup
// buat satu drawer (chat+context), caller chunk sendiri untuk doc panjang.
const MaxSanitizeBytes = 256 * 1024

// Sanitize — normalisasi minimal supaya content layak masuk drawer:
//   - Cap input length ke MaxSanitizeBytes (DOS protection)
//   - Normalize line ending: \r\n / \r → \n
//   - Strip control char (kecuali \n, \t)
//   - Collapse multi-space (>2) jadi 2-space (jaga indentasi tetap)
//   - Collapse > 3 consecutive newlines jadi 2 (paragraph separator)
//   - Trim leading/trailing whitespace
//
// Tidak ubah: capitalization, punctuation, language. Itu domain scorer atau
// embedding step. Sanitize fokus ke storage hygiene, bukan semantic.
func Sanitize(s string) string {
	if s == "" {
		return ""
	}
	// 0. Hard cap input length supaya loop di bawah ngga O(n²) untuk input
	// pathological (mis. 100MB spaces).
	if len(s) > MaxSanitizeBytes {
		s = s[:MaxSanitizeBytes]
	}
	// 1. Normalize line endings.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// 2. Strip control chars except \n \t. Pakai builder supaya satu pass.
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' {
			b.WriteRune(r)
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	s = b.String()

	// 3. Collapse run of newline > 2 → 2.
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}

	// 4. Collapse run of space > 2 → 2 (preserve indent).
	for strings.Contains(s, "   ") {
		s = strings.ReplaceAll(s, "   ", "  ")
	}

	// 5. Trim outer whitespace.
	return strings.TrimSpace(s)
}
