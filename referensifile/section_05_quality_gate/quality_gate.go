// Package ingestor — quality_gate.go
// Multi-stage quality gate for training data ingest.
// 6 gates: length, SHA-256 dedup, fuzzy dedup, language coherence,
// content sanitization, PII strip.
//
// KEPUTUSAN tim 4-AI (Opus-1 + Opus-2 + Opus-3 + Gemini):
// - Wajib SEBELUM ingest apapun (Phase 0 BLOCKER)
// - Anti cache pollution: garbage in = garbage strengthened
// - FQP-4 idempotent, FQP-12 append-only audit log
package ingestor

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// QualityGateConfig configures the multi-stage quality filter.
type QualityGateConfig struct {
	MinPromptLen     int     // min rune count for prompt (default 50)
	MinCompletionLen int     // min rune count for completion (default 100)
	FuzzyThreshold   float64 // Jaccard similarity threshold (default 0.85)
	MaxBase64Len     int     // max base64 blob length before reject (default 1024)
}

// DefaultQualityGateConfig returns production-ready defaults.
func DefaultQualityGateConfig() QualityGateConfig {
	return QualityGateConfig{
		MinPromptLen:     50,
		MinCompletionLen: 100,
		FuzzyThreshold:   0.85,
		MaxBase64Len:     1024,
	}
}

// TrainingRow represents one JSONL row from bahan_training.
type TrainingRow struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
	Source     string `json:"source"`
}

// GateResult captures the outcome of quality filtering.
type GateResult struct {
	Passed   bool   `json:"passed"`
	Rejected string `json:"rejected,omitempty"` // rejection reason
	Stage    int    `json:"stage"`              // which stage rejected (0 = passed all)
}

// IngestAuditEntry logs one quality gate decision for telemetry.
type IngestAuditEntry struct {
	Timestamp   string `json:"ts"`
	SourceFile  string `json:"source_file"`
	RowIndex    int    `json:"row_index"`
	Passed      bool   `json:"passed"`
	Rejection   string `json:"rejection,omitempty"`
	Stage       int    `json:"stage"`
	PromptHash  string `json:"prompt_hash"`
}

// EnsureIngestLogSchema creates the ingest_log table for per-stage telemetry.
// Per Opus-2: rejection_reason kolom untuk Ayah lihat distribution per dataset.
func EnsureIngestLogSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS ingest_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts TEXT DEFAULT CURRENT_TIMESTAMP,
		source_file TEXT NOT NULL,
		row_index INTEGER NOT NULL,
		passed INTEGER NOT NULL DEFAULT 0,
		rejection_reason TEXT DEFAULT '',
		stage INTEGER DEFAULT 0,
		prompt_hash TEXT DEFAULT ''
	)`)
	return err
}

// EnsureKnowledgeQuarantineSchema creates quarantine table.
// Per Opus-2: quarantine bukan delete — Ayah bisa manual review via GUI.
func EnsureKnowledgeQuarantineSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_quarantine (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		prompt TEXT NOT NULL,
		completion TEXT NOT NULL,
		source TEXT DEFAULT '',
		quarantine_reason TEXT NOT NULL,
		quarantined_at TEXT DEFAULT CURRENT_TIMESTAMP,
		reviewed INTEGER DEFAULT 0,
		reviewed_at TEXT DEFAULT '',
		review_action TEXT DEFAULT ''
	)`)
	return err
}

// RunQualityGate applies the 6-stage quality gate to a single training row.
// Returns GateResult with pass/fail + rejection reason.
func RunQualityGate(row TrainingRow, cfg QualityGateConfig, seenHashes map[string]bool) GateResult {
	// Stage 1: Length check
	if utf8.RuneCountInString(row.Prompt) < cfg.MinPromptLen {
		return GateResult{Passed: false, Rejected: "stage1_prompt_too_short", Stage: 1}
	}
	if utf8.RuneCountInString(row.Completion) < cfg.MinCompletionLen {
		return GateResult{Passed: false, Rejected: "stage1_completion_too_short", Stage: 1}
	}

	// Stage 2: SHA-256 exact dedup
	hash := sha256Hash(row.Prompt + "||" + row.Completion)
	if seenHashes[hash] {
		return GateResult{Passed: false, Rejected: "stage2_exact_dedup", Stage: 2}
	}
	seenHashes[hash] = true

	// Stage 3: Content sanitization (shell patterns, base64 blobs)
	if reason := checkDangerousContent(row.Completion, cfg.MaxBase64Len); reason != "" {
		return GateResult{Passed: false, Rejected: "stage3_" + reason, Stage: 3}
	}

	// Stage 4: Language coherence (reject mixed 5+ scripts)
	if !checkLanguageCoherence(row.Prompt + " " + row.Completion) {
		return GateResult{Passed: false, Rejected: "stage4_language_incoherent", Stage: 4}
	}

	// Stage 5: PII detected in content (strip, don't reject — but log)
	// PII stripping handled by StripPII() in pii_strip.go — called separately

	// Stage 6: Fuzzy dedup handled at batch level (needs corpus comparison)
	// Handled by FuzzyDedupCheck() in fuzzy_dedup.go

	return GateResult{Passed: true, Stage: 0}
}

// LogGateResult persists one quality gate decision to ingest_log.
func LogGateResult(db *sql.DB, sourceFile string, rowIndex int, result GateResult, promptHash string) {
	passed := 0
	if result.Passed {
		passed = 1
	}
	_, _ = db.Exec(`INSERT INTO ingest_log (source_file, row_index, passed, rejection_reason, stage, prompt_hash)
		VALUES (?, ?, ?, ?, ?, ?)`,
		sourceFile, rowIndex, passed, result.Rejected, result.Stage, promptHash)
}

// QuarantineRow saves a rejected row to quarantine for Ayah review.
func QuarantineRow(db *sql.DB, row TrainingRow, reason string) {
	_, _ = db.Exec(`INSERT INTO knowledge_quarantine (prompt, completion, source, quarantine_reason)
		VALUES (?, ?, ?, ?)`,
		row.Prompt, row.Completion, row.Source, reason)
}

// --- Internal helpers ---

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// dangerousPatterns are shell/exec patterns that MUST NOT be stored verbatim.
// Per Gemini security audit: reject rows containing destructive commands.
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+-rf\s+/`),
	regexp.MustCompile(`(?i)\bformat\s+[a-z]:\s*/`),
	regexp.MustCompile(`(?i)\bdel\s+/[sf]\b`),
	regexp.MustCompile(`(?i)\bwget\s+.+\|\s*bash\b`),
	regexp.MustCompile(`(?i)\bcurl\s+.+\|\s*sh\b`),
	regexp.MustCompile(`(?i)\bfdisk\s+/dev/`),
	regexp.MustCompile(`(?i)\bmkfs\b`),
	regexp.MustCompile(`(?i)\bdd\s+if=.+of=/dev/`),
	regexp.MustCompile(`(?i)\bnetsh\s+firewall\s+.*off\b`),
	regexp.MustCompile(`(?i)\breg\s+delete\s+hklm\b`),
	regexp.MustCompile(`(?i)\bschtasks\s+/create\b.*\bmalware\b`),
}

// base64Pattern detects long base64-encoded blobs.
var base64Pattern = regexp.MustCompile(`[A-Za-z0-9+/]{100,}={0,2}`)

func checkDangerousContent(content string, maxBase64Len int) string {
	for _, p := range dangerousPatterns {
		if p.MatchString(content) {
			return "dangerous_shell_pattern"
		}
	}

	// Check for embedded base64 blobs > threshold
	matches := base64Pattern.FindAllString(content, -1)
	for _, m := range matches {
		if len(m) > maxBase64Len {
			return "base64_blob_too_large"
		}
	}

	return ""
}

// checkLanguageCoherence rejects text that mixes too many Unicode scripts.
// Heuristic: if text contains characters from 5+ distinct Unicode blocks = likely garbage.
func checkLanguageCoherence(text string) bool {
	scripts := map[string]bool{}
	for _, r := range text {
		switch {
		case r >= 0x0000 && r <= 0x007F:
			scripts["latin_basic"] = true
		case r >= 0x00C0 && r <= 0x024F:
			scripts["latin_extended"] = true
		case r >= 0x0400 && r <= 0x04FF:
			scripts["cyrillic"] = true
		case r >= 0x0600 && r <= 0x06FF:
			scripts["arabic"] = true
		case r >= 0x0900 && r <= 0x097F:
			scripts["devanagari"] = true
		case r >= 0x3040 && r <= 0x309F:
			scripts["hiragana"] = true
		case r >= 0x30A0 && r <= 0x30FF:
			scripts["katakana"] = true
		case r >= 0x4E00 && r <= 0x9FFF:
			scripts["cjk"] = true
		case r >= 0xAC00 && r <= 0xD7AF:
			scripts["hangul"] = true
		case r >= 0x0E00 && r <= 0x0E7F:
			scripts["thai"] = true
		}
	}
	// Normal text should be in <= 4 scripts (e.g., basic latin + extended + CJK for code comments)
	return len(scripts) <= 4
}

// ParseTrainingRow parses one JSONL line into TrainingRow.
func ParseTrainingRow(line []byte) (TrainingRow, error) {
	var row TrainingRow
	if err := json.Unmarshal(line, &row); err != nil {
		return row, fmt.Errorf("invalid JSON: %w", err)
	}
	return row, nil
}

// --- Fuzzy dedup helpers (simple Jaccard on token sets) ---

// JaccardSimilarity computes Jaccard similarity between two token sets.
func JaccardSimilarity(a, b string) float64 {
	tokensA := tokenize(a)
	tokensB := tokenize(b)

	if len(tokensA) == 0 && len(tokensB) == 0 {
		return 1.0
	}

	intersection := 0
	setB := make(map[string]bool, len(tokensB))
	for _, t := range tokensB {
		setB[t] = true
	}
	for _, t := range tokensA {
		if setB[t] {
			intersection++
		}
	}

	union := len(tokensA) + len(tokensB) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// tokenize splits text into lowercase tokens, stripping stopwords.
func tokenize(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	result := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}/-")
		if len(w) < 2 || stopwords[w] {
			continue
		}
		result = append(result, w)
	}
	return result
}

// stopwords — combined Indonesian + English stopwords for dedup accuracy.
// Per Opus-2: dedup harus ignore stopwords.
var stopwords = map[string]bool{
	// English
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true, "need": true,
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"into": true, "through": true, "during": true, "before": true,
	"after": true, "above": true, "below": true, "between": true,
	"and": true, "but": true, "or": true, "not": true, "so": true,
	"if": true, "then": true, "than": true, "that": true, "this": true,
	"it": true, "its": true, "he": true, "she": true, "they": true,
	"we": true, "you": true, "me": true, "him": true, "her": true,
	"them": true, "my": true, "your": true, "our": true, "their": true,
	// Indonesian
	"yang": true, "dan": true, "di": true, "ke": true, "dari": true,
	"ini": true, "itu": true, "dengan": true, "untuk": true, "pada": true,
	"adalah": true, "atau": true, "juga": true, "akan": true, "sudah": true,
	"ada": true, "bisa": true, "saya": true, "kamu": true, "kami": true,
	"mereka": true, "dia": true, "apa": true, "siapa": true, "mana": true,
	"gw": true, "lo": true, "gue": true, "lu": true, "ya": true,
	"ga": true, "ngga": true, "udah": true, "dong": true, "sih": true,
	"nih": true, "tuh": true, "deh": true, "lah": true, "kan": true,
}

// --- Timestamp helper ---

// NowISO returns current time in ISO 8601 format.
func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

