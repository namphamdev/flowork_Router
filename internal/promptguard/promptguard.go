// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 4 (Prompt injection detector phase 1) DONE +
//   adversarial-audit passed (C1 dampening bypass floor for high-weight
//   pattern, C2 newline-prefix system: via (?m) multiline flag).
//   API stable: Result struct, Severity enum, Detect, HasInjection.
//   13 signatures across 4 category, educational context dampening x0.5
//   floored at 0.4 for high-weight pattern. Phase 2 (Indonesian patterns,
//   embedding intent, LLM judge) → tambah signatures/file baru, JANGAN
//   modify existing pattern atau threshold.
//
// Package promptguard — signature-based prompt injection detector.
//
// PURPOSE:
//   Deteksi content yang berusaha override system prompt / jailbreak.
//   Common pattern: "ignore previous instructions", role hijack ("you are
//   now X"), system: prefix, instruction reversal. Score 0.0-1.0 (higher =
//   lebih bahaya). Caller bisa pakai untuk quarantine atau drop.
//
// SEMANTIC:
//   - Detect(content) → Result{Score, Allowed, Hits, ...}
//   - Score >= 0.7 → suspicious (recommend quarantine)
//   - Score >= 0.4 → flag for review
//   - Score < 0.4 → likely safe
//   - Educational context allowance: jika hit pattern muncul dalam konteks
//     pembahasan/edukasi (didahului "example:", "tutorial:", "explain"), score
//     di-dampen.
//
// Phase 1 scope:
//   - Static regex signature list
//   - Educational context heuristic (simple keyword prefix dampening)
//   - Deterministic — no LLM judge
//
// Defer phase 2:
//   - Embedding-based semantic intent classification
//   - LLM judge for ambiguous score
//   - Custom signature via config
//   - Per-language patterns (current English+Indonesia mix)
//
// Source: flowork_Router/roadmap.md Section 4.

package promptguard

import (
	"regexp"
	"strings"
)

// AlgoVersion — bumped saat signature signifikan berubah.
const AlgoVersion = "v1"

// Severity tier — caller branching.
type Severity string

const (
	SeveritySafe       Severity = "safe"       // score < 0.4
	SeverityReview     Severity = "review"     // 0.4 ≤ score < 0.7
	SeveritySuspicious Severity = "suspicious" // score ≥ 0.7
)

// Hit — single matched signature.
type Hit struct {
	Category string  `json:"category"` // 'instruction_override' | 'role_hijack' | 'system_leak' | 'jailbreak'
	Pattern  string  `json:"pattern"`  // pattern name (not raw regex — supaya audit aman)
	Snippet  string  `json:"snippet"`  // first 80 char of matched text
	Weight   float64 `json:"weight"`   // contribution ke composite score
}

// Result — outcome detect.
type Result struct {
	AlgoVersion       string   `json:"algo_version"`
	Allowed           bool     `json:"allowed"`
	Severity          Severity `json:"severity"`
	Score             float64  `json:"score"` // 0.0–1.0
	Hits              []Hit    `json:"hits,omitempty"`
	EducationalContext bool    `json:"educational_context"` // dampening triggered
	Reason            string   `json:"reason,omitempty"`
}

// signature — internal pattern definition. Weight kumulatif ke score.
type signature struct {
	Category string
	Name     string
	Re       *regexp.Regexp
	Weight   float64
}

var signatures []signature

func init() {
	// Pattern weight di-tune: high-confidence pattern (e.g. "ignore previous
	// instructions") → 0.5, moderate (e.g. "you are now") → 0.3, weak
	// (e.g. "system:") → 0.2. Sum capped at 1.0.
	mustCompile := func(s string) *regexp.Regexp {
		return regexp.MustCompile(`(?i)` + s) // case-insensitive
	}

	signatures = []signature{
		// Category 1: instruction_override (high weight)
		{"instruction_override", "ignore_previous", mustCompile(`\bignore (the )?(previous|prior|above|all) (instructions?|prompts?|rules?|context|messages?)\b`), 0.5},
		{"instruction_override", "disregard_above", mustCompile(`\b(disregard|forget) (the )?(previous|prior|above|all) (instructions?|prompts?|rules?)\b`), 0.5},
		{"instruction_override", "override_directive", mustCompile(`\boverride (the )?(system|previous|all) (prompt|instruction|directive)\b`), 0.5},

		// Category 2: role_hijack
		{"role_hijack", "you_are_now", mustCompile(`\byou are (now|actually) (?:a )?(jailbroken|uncensored|unfiltered|dan|stan|developer mode|admin|root|godmode)\b`), 0.4},
		{"role_hijack", "pretend_to_be", mustCompile(`\bpretend (?:to be|you are) (?:a )?(jailbroken|uncensored|evil|hacker|dan|stan)\b`), 0.3},
		{"role_hijack", "act_as", mustCompile(`\bact as (?:if you (?:are|were) )?(jailbroken|uncensored|evil|godmode|admin)\b`), 0.3},

		// Category 3: system_leak / prompt extraction
		{"system_leak", "reveal_prompt", mustCompile(`\b(reveal|show|print|display|output) (your|the) (system )?prompt\b`), 0.5},
		// Audit fix C2: multiline flag (?m) supaya `\nsystem:` ke-detect
		// (sebelumnya `^` anchor bypass via leading newline).
		{"system_leak", "system_prefix", mustCompile(`(?m)^(system|admin|root):\s`), 0.2},
		{"system_leak", "developer_mode", mustCompile(`\b(enable|activate|enter) (developer|debug|admin|god) mode\b`), 0.3},

		// Category 4: jailbreak common patterns
		{"jailbreak", "dan_prompt", mustCompile(`\bDAN (mode|prompt|jailbreak)\b`), 0.4},
		{"jailbreak", "anything_now", mustCompile(`\b(do|say) anything now\b`), 0.4},
		{"jailbreak", "no_restriction", mustCompile(`\b(no|without|bypass) (restrictions?|limitations?|guidelines?|rules?|filter|filters)\b`), 0.3},
		{"jailbreak", "hypothetical_evil", mustCompile(`\bhypothetically(,)? (if you were|imagine you are) (evil|unaligned|jailbroken)\b`), 0.3},
	}
}

// educationalPrefixes — kalau salah satu phrase muncul dalam first 200
// char content, dampen score (educational/discussion context allowance).
var educationalPrefixes = []string{
	"example:",
	"tutorial:",
	"explain ",
	"explanation:",
	"discuss ",
	"discussion:",
	"in this article",
	"this article",
	"educational:",
	"contoh:",
	"penjelasan:",
	"tutorial:",
	"misalkan",
	"misalnya",
	"sebagai ilustrasi",
}

// Detect — apply signatures, compute composite score, classify severity.
// Cap content scan 64KB untuk perf.
func Detect(content string) Result {
	r := Result{AlgoVersion: AlgoVersion}
	if content == "" {
		r.Severity = SeveritySafe
		r.Allowed = true
		return r
	}

	const maxScan = 64 * 1024
	scanContent := content
	if len(scanContent) > maxScan {
		scanContent = scanContent[:maxScan]
	}

	// Check educational context (first 200 char lowercased).
	headLow := strings.ToLower(scanContent)
	if len(headLow) > 200 {
		headLow = headLow[:200]
	}
	for _, p := range educationalPrefixes {
		if strings.Contains(headLow, p) {
			r.EducationalContext = true
			break
		}
	}

	// Scan signatures.
	for _, sig := range signatures {
		matches := sig.Re.FindAllString(scanContent, -1)
		if len(matches) == 0 {
			continue
		}
		for i, m := range matches {
			if i >= 3 {
				break // cap hits per signature
			}
			snippet := m
			if len(snippet) > 80 {
				snippet = snippet[:80] + "…"
			}
			r.Hits = append(r.Hits, Hit{
				Category: sig.Category,
				Pattern:  sig.Name,
				Snippet:  snippet,
				Weight:   sig.Weight,
			})
			r.Score += sig.Weight
		}
	}

	// Educational dampening: scale score by 0.5 — context allowance.
	//
	// Audit fix C1: kalau ada hit dengan weight >= 0.5 (high-confidence
	// pattern seperti `ignore_previous` atau `reveal_prompt`), JANGAN
	// dampen full — itu attacker-controlled bypass via prefix "Tutorial:".
	// Floor dampened score di 0.4 (review tier) supaya minimum tetap kena
	// flag walau prefix manipulation.
	if r.EducationalContext {
		hasHighWeight := false
		for _, h := range r.Hits {
			if h.Weight >= 0.5 {
				hasHighWeight = true
				break
			}
		}
		r.Score *= 0.5
		if hasHighWeight && r.Score < 0.4 {
			r.Score = 0.4 // floor at review tier — refuse full bypass
		}
	}

	// Cap at 1.0.
	if r.Score > 1.0 {
		r.Score = 1.0
	}

	// Classify.
	switch {
	case r.Score >= 0.7:
		r.Severity = SeveritySuspicious
		r.Allowed = false
		r.Reason = "high prompt injection signal — recommend quarantine"
	case r.Score >= 0.4:
		r.Severity = SeverityReview
		r.Allowed = false
		r.Reason = "moderate prompt injection signal — flag for review"
	default:
		r.Severity = SeveritySafe
		r.Allowed = true
	}

	return r
}

// HasInjection — quick boolean. Lebih cepat dari Detect kalau caller cuma
// butuh tau ada signal injection atau ngga.
//
// ⚠️ Tidak apply educational dampening — return true bahkan kalau pattern
// muncul dalam konteks tutorial/edukasi. Caller yang butuh full classification
// (severity tier + dampening) pakai Detect().
func HasInjection(content string) bool {
	if content == "" {
		return false
	}
	const maxScan = 64 * 1024
	if len(content) > maxScan {
		content = content[:maxScan]
	}
	for _, sig := range signatures {
		if sig.Re.MatchString(content) {
			return true
		}
	}
	return false
}
