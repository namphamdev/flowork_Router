// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Pure heuristic (deterministic, no I/O, no state). API stable:
//   Score(content, sourceType) → 0.0–10.0 clamped. Section 2 (Re-score worker)
//   akan TAMBAH function baru `Rescore(drawerID, retrievalCount)` di file lain
//   — JANGAN ubah Score() ini, downstream test bisa drift.
//
// score.go — heuristic importance scoring untuk drawer baru. Output 0-10.
//
// Anti over-engineer: cuma signal sederhana (panjang, signal word, source
// reputation). Re-score job lebih kompleks (frequency-of-retrieval) di
// roadmap section 2 (Importance scorer).
package ingest

import (
	"strings"
)

// signalWords — kata yang sering muncul di knowledge important. Mostly
// code/process keyword. Lower-case match.
var signalWords = []string{
	"bug", "fix", "incident", "rootcause", "root cause", "regression",
	"security", "vulnerability", "cve", "exploit", "patch", "mitigation",
	"deprecated", "breaking change", "migration", "rollback",
	"decision", "rfc", "design doc", "architecture",
	"policy", "constitution", "governance", "amendment",
	"warning", "caution", "important", "critical",
	"todo", "fixme", "hack", "xxx",
}

// sourceTypeBoost — reputation per source taxonomy. Manual admin > federation,
// chat lower (kemungkinan ephemeral). Range 0-2.
var sourceTypeBoost = map[string]float64{
	"manual":      2.0, // explicit admin submit
	"doc":         1.5, // import dari source-of-truth file
	"federation":  1.0, // dari peer sync (trust netral)
	"chat":        0.5, // kompoundkan dari interaksi (banyak noise)
	"compounding": 0.5,
	"":            1.0, // default
}

// Score — heuristic importance 0-10. Komponen:
//
//	base                = 3.0    (DB default)
//	source reputation   = 0..2   (sourceTypeBoost)
//	signal words        = +0.5 / hit (max 2.0)
//	length penalty/boost:
//	    < 50 char        → -1.5  (terlalu pendek, sering noise)
//	    50–200 char      → 0     (neutral)
//	    200–1500 char    → +1.0  (rich content)
//	    > 1500 char      → +0.5  (panjang ok tapi tidak >2K bonus, biasanya copy-paste)
//
// Hasil clamp ke [0.5, 10.0].
func Score(content, sourceType string) float64 {
	if content == "" {
		return 0.5
	}
	score := 3.0

	if b, ok := sourceTypeBoost[strings.ToLower(sourceType)]; ok {
		score += b
	} else {
		score += sourceTypeBoost[""]
	}

	lower := strings.ToLower(content)
	hits := 0
	for _, w := range signalWords {
		if strings.Contains(lower, w) {
			hits++
			if hits >= 4 {
				break
			}
		}
	}
	score += float64(hits) * 0.5

	switch n := len(content); {
	case n < 50:
		score -= 1.5
	case n < 200:
		// no change
	case n < 1500:
		score += 1.0
	default:
		score += 0.5
	}

	if score < 0.5 {
		score = 0.5
	}
	if score > 10.0 {
		score = 10.0
	}
	return score
}
