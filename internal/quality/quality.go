// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 5 (Quality gate phase 1) DONE + adversarial-audit
//   passed. API stable: Result struct, Check(content), detectRepetition
//   helper. Pure heuristic, no DB / no I/O / no state. Phase 2 extension
//   (embedding-based semantic dup, LLM coherence) → tambah function/file
//   baru di package ini, JANGAN modify ini.
//
// Package quality — heuristic content quality gate for brain ingest.
//
// PURPOSE:
//   Filter low-quality content sebelum jadi drawer. Cover:
//     - Length (terlalu pendek / panjang)
//     - Repetition pattern (string repeat → spam)
//     - Whitespace ratio (mostly blank)
//     - Char diversity (terlalu sedikit unique char → garbage / single-char spam)
//
// Phase 1 scope: pure heuristic, no DB / no embedding / no LLM call. Cepat,
// deterministik, anti-dep. Caller invoke `Check(content)` SEBELUM
// `ingest.Submit()` kalau mau gate. Library standalone — ingest.Submit
// (locked) ngga otomatis call quality; caller bertanggung jawab.
//
// Defer (phase 2/3):
//   - Embedding-based semantic duplicate (butuh embedding worker)
//   - LLM coherence judge (expensive)
//   - Language detect (lang ID library separate)
//
// Source: flowork_Router/roadmap.md Section 5.

package quality

import (
	"strings"
	"unicode"
)

// AlgoVersion — bumped saat ada perubahan signifikan ke heuristic.
// JSON shape future-proof: caller bisa pisahin handling per version.
const AlgoVersion = "v1"

// Result — outcome single check. `Allowed=true` → content layak; `false`
// → reject dengan `Reason`. `Score` 0.0-1.0 (higher = more quality signal).
type Result struct {
	AlgoVersion string  `json:"algo_version"`
	Allowed     bool    `json:"allowed"`
	Reason      string  `json:"reason,omitempty"`
	Score       float64 `json:"score"`
	// Sub-scores untuk transparency / future tuning.
	LengthScore     float64 `json:"length_score"`
	RepetitionScore float64 `json:"repetition_score"`
	WhitespaceScore float64 `json:"whitespace_score"`
	DiversityScore  float64 `json:"diversity_score"`
}

// Threshold konstanta — di-tune empirically. Anti runaway: hard cap input
// untuk algoritma O(n) loops.
//
// maxRepetitionPct = 0.30: caught short repeating pattern lebih reliable
// (audit Section 5 finding — 0.50 missed "halohalohalo" yang 3-gram
// rotation cuma capai ~40% frequency).
const (
	minLengthBytes    = 20
	maxLengthBytes    = 256 * 1024
	maxRepetitionPct  = 0.30 // > 30% same 3-gram = spam pattern
	maxWhitespacePct  = 0.80 // > 80% whitespace = low signal
	minDiversityChars = 5    // < 5 unique non-space char = garbage
	maxDiversityCount = 20   // > 20 unique → cap di full score (early-exit)
	overallThreshold  = 0.5  // composite score < 0.5 → reject
)

// Check — invoke heuristic. Cheap (< 1ms untuk 4KB content typical).
// Empty content auto-reject.
//
// ⚠️ UTF-8 limitation: detectRepetition pakai byte-level 3-gram. Untuk
// CJK / emoji (multi-byte rune), 3-byte chunk bisa split mid-rune →
// repetition detect underestimate. Acceptable untuk Indonesian/English
// content. Phase 2 fix via rune-level chunk.
func Check(content string) Result {
	r := Result{AlgoVersion: AlgoVersion}

	if content == "" {
		r.Reason = "content empty"
		return r
	}

	// 1. Length check
	n := len(content)
	switch {
	case n < minLengthBytes:
		r.Reason = "content too short"
		r.LengthScore = 0
	case n > maxLengthBytes:
		r.Reason = "content too long"
		r.LengthScore = 0
	default:
		// Sweet spot: 100-10000 bytes. Outside → lower score.
		switch {
		case n < 100:
			r.LengthScore = float64(n) / 100.0
		case n <= 10000:
			r.LengthScore = 1.0
		default:
			// Linear taper to 0.5 at maxLengthBytes.
			r.LengthScore = 1.0 - 0.5*float64(n-10000)/float64(maxLengthBytes-10000)
		}
	}
	if r.Reason != "" {
		return r
	}

	// 2. Whitespace ratio + 3. Char diversity — single pass collect both.
	// Audit fix: gabung 2 separate range loops jadi satu. Plus early-exit
	// unique map saat >= maxDiversityCount supaya CPU bounded.
	wsCount := 0
	runeCount := 0
	unique := make(map[rune]struct{}, maxDiversityCount+1)
	diversityCapped := false
	for _, c := range content {
		runeCount++
		if unicode.IsSpace(c) {
			wsCount++
			continue
		}
		if !diversityCapped {
			unique[c] = struct{}{}
			if len(unique) >= maxDiversityCount {
				diversityCapped = true
			}
		}
	}
	if runeCount == 0 {
		runeCount = 1 // div by zero defense
	}
	wsRatio := float64(wsCount) / float64(runeCount)
	if wsRatio > maxWhitespacePct {
		r.Reason = "content mostly whitespace"
		return r
	}
	r.WhitespaceScore = 1.0 - wsRatio

	if len(unique) < minDiversityChars {
		r.Reason = "content low diversity (likely garbage/spam)"
		return r
	}
	// Score: cap at maxDiversityCount unique chars = full score.
	div := float64(len(unique))
	if div > maxDiversityCount {
		div = maxDiversityCount
	}
	r.DiversityScore = div / float64(maxDiversityCount)

	// 4. Repetition pattern — detect substring 3+ chars yang repeat banyak.
	// Algoritma sederhana: scan 3-gram, hitung max repeat. Cap input length
	// supaya O(n) tetap cepat.
	repPct := detectRepetition(content)
	if repPct > maxRepetitionPct {
		r.Reason = "content high repetition (likely spam)"
		return r
	}
	r.RepetitionScore = 1.0 - repPct

	// Composite score — average of 4 sub-scores.
	r.Score = (r.LengthScore + r.WhitespaceScore + r.DiversityScore + r.RepetitionScore) / 4.0
	r.Allowed = r.Score >= overallThreshold
	if !r.Allowed {
		r.Reason = "composite quality score below threshold"
	}
	return r
}

// detectRepetition — fraction content yang represented by most-frequent
// 3-gram. Output 0.0 (no repetition) to 1.0 (entire content repeats).
//
// Algoritma: extract all 3-gram, count max frequency, normalize by total
// 3-grams. Bukan perfect (skip whitespace gap), tapi cukup untuk spam
// signal real (mis. "halohalohalohalo" detect, "ngantuk semua" pass).
func detectRepetition(content string) float64 {
	if len(content) < 6 {
		return 0
	}
	// Cap input untuk perf. 16KB cukup signal sample.
	const maxScan = 16 * 1024
	if len(content) > maxScan {
		content = content[:maxScan]
	}
	// Strip whitespace untuk fokus signal char.
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, content)
	if len(cleaned) < 6 {
		return 0
	}

	counts := map[string]int{}
	total := 0
	for i := 0; i+3 <= len(cleaned); i++ {
		gram := cleaned[i : i+3]
		counts[gram]++
		total++
	}
	if total == 0 {
		return 0
	}
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}
	return float64(maxCount) / float64(total)
}
