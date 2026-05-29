// Package ingestor — importance_scorer.go: domain-based importance scoring
// untuk training data classification.
//
// Per KONSEP_TRAINING.MD §4.3 + Opus-1 §3.2 karma-weighted learning rate.
//
// Importance scale 1.0 - 5.0:
//   5.0 — hackerone (real-world bug, verified publicly)
//   4.5 — trading dengan DOCTRINE tag
//   4.0 — whitehat vulnerability/pentest
//   3.0 — whitehat function calling / synthetic
//   2.5 — general baseline (Aya Indonesian, knowledge)
//
// Karma source mapping (Opus-1 §3.2):
//   hackerone        → 0.95
//   Ayah-curated     → 1.00
//   whitehat synth   → 0.60
//   trading sentiment → 0.70
//   general Q&A      → 0.50

package ingestor

import (
	"path/filepath"
	"strings"
)

// ImportanceForWing — return importance score per wing.
func ImportanceForWing(wing string) float64 {
	switch wing {
	case "hackerone":
		return 5.0
	case "trading":
		return 4.5
	case "whitehat":
		return 4.0
	case "indonesian_legal":
		return 4.5
	case "general":
		return 2.5
	default:
		return 2.0
	}
}

// KarmaForSource — return karma weight per source (Opus-1 §3.2).
//
// Used di V4 training: gradient × importance × karma.
// Higher karma = source lebih trusted = lebih besar pengaruh ke V4 weights.
func KarmaForSource(source string) float64 {
	low := strings.ToLower(source)

	// Ayah explicit curate (top trust)
	if strings.Contains(low, "ayah") || strings.Contains(low, "curated") {
		return 1.0
	}

	// HackerOne verified bug bounty
	if strings.Contains(low, "hackerone") || strings.Contains(low, "h1_") {
		return 0.95
	}

	// Trading (auto-labeled, mungkin noisy)
	if strings.Contains(low, "trading") || strings.Contains(low, "sentiment") {
		return 0.7
	}

	// Whitehat (synthetic, function calling boilerplate)
	if strings.Contains(low, "whitehat") || strings.Contains(low, "func_call") {
		return 0.6
	}

	// General Q&A (Aya Indonesian, mixed quality)
	if strings.Contains(low, "general") || strings.Contains(low, "aya") {
		return 0.5
	}

	// Mesh peer (per-peer karma, default low until trust earned)
	if strings.Contains(low, "peer") || strings.Contains(low, "mesh") {
		return 0.4
	}

	// Default unknown source
	return 0.3
}

// ClassifyTrainingRow — determine wing + room from filename + row content.
//
// Wing: high-level domain (whitehat / trading / hackerone / general / indonesian_legal)
// Room: specific topic (xss / sentiment / detection_rule / dll)
func ClassifyTrainingRow(sourceFile string, row TrainingRow) (wing, room string) {
	base := strings.ToLower(filepath.Base(sourceFile))

	// Filename-based wing classification
	switch {
	case strings.Contains(base, "hackerone"):
		wing = "hackerone"
		room = classifyHackeroneRoom(row)
	case strings.Contains(base, "whitehat"):
		wing = "whitehat"
		room = classifyWhitehatRoom(row)
	case strings.Contains(base, "trading"):
		wing = "trading"
		room = classifyTradingRoom(row)
	case strings.Contains(base, "indonesian") || strings.Contains(base, "uu_ite") || strings.Contains(base, "ojk"):
		wing = "indonesian_legal"
		room = classifyIndonesianRoom(row)
	default:
		wing = "general"
		room = "knowledge"
	}

	return wing, room
}

// classifyHackeroneRoom — categorize HackerOne report by vuln type.
func classifyHackeroneRoom(row TrainingRow) string {
	content := strings.ToLower(row.Prompt + " " + row.Completion)
	switch {
	case strings.Contains(content, "xss") || strings.Contains(content, "cross-site script"):
		return "xss"
	case strings.Contains(content, "sql injection") || strings.Contains(content, "sqli"):
		return "sqli"
	case strings.Contains(content, "ssrf") || strings.Contains(content, "server-side request"):
		return "ssrf"
	case strings.Contains(content, "idor") || strings.Contains(content, "insecure direct object"):
		return "idor"
	case strings.Contains(content, "use-after-free") || strings.Contains(content, "uaf"):
		return "memory_corruption"
	case strings.Contains(content, "buffer overflow") || strings.Contains(content, "heap"):
		return "memory_corruption"
	case strings.Contains(content, "logic flaw") || strings.Contains(content, "business logic"):
		return "logic_flaw"
	case strings.Contains(content, "auth") || strings.Contains(content, "authentication"):
		return "authentication"
	case strings.Contains(content, "rce") || strings.Contains(content, "remote code execution"):
		return "rce"
	default:
		return "general_vuln"
	}
}

// classifyWhitehatRoom — categorize whitehat training row.
func classifyWhitehatRoom(row TrainingRow) string {
	content := strings.ToLower(row.Prompt + " " + row.Completion)
	switch {
	case strings.Contains(content, "function") && strings.Contains(content, "call"):
		return "function_calling"
	case strings.Contains(content, "audit") || strings.Contains(content, "code review"):
		return "audit_methodology"
	case strings.Contains(content, "smart contract") || strings.Contains(content, "solidity"):
		return "smart_contract"
	case strings.Contains(content, "pentest") || strings.Contains(content, "penetration"):
		return "pentest"
	case strings.Contains(content, "forensic") || strings.Contains(content, "incident response"):
		return "forensics"
	case strings.Contains(content, "secure cod") || strings.Contains(content, "owasp"):
		return "secure_coding"
	case strings.Contains(content, "vulnerab") || strings.Contains(content, "cve"):
		return "vulnerability"
	default:
		return "general"
	}
}

// classifyTradingRoom — categorize trading row.
func classifyTradingRoom(row TrainingRow) string {
	content := strings.ToLower(row.Prompt + " " + row.Completion)
	switch {
	case strings.Contains(content, "sentiment") || strings.Contains(content, "positive") || strings.Contains(content, "negative"):
		return "sentiment"
	case strings.Contains(content, "technical") || strings.Contains(content, "chart") || strings.Contains(content, "indicator"):
		return "technical_analysis"
	case strings.Contains(content, "fundamental") || strings.Contains(content, "earnings"):
		return "fundamental"
	case strings.Contains(content, "risk") || strings.Contains(content, "stop loss") || strings.Contains(content, "position sizing"):
		return "risk_management"
	case strings.Contains(content, "rug pull") || strings.Contains(content, "scam"):
		return "fraud_detection"
	default:
		return "general"
	}
}

// classifyIndonesianRoom — categorize Indonesian legal context.
func classifyIndonesianRoom(row TrainingRow) string {
	content := strings.ToLower(row.Prompt + " " + row.Completion)
	switch {
	case strings.Contains(content, "uu ite") || strings.Contains(content, "pasal 27") || strings.Contains(content, "pasal 30"):
		return "uu_ite"
	case strings.Contains(content, "pdp") || strings.Contains(content, "perlindungan data"):
		return "pdp"
	case strings.Contains(content, "ojk") || strings.Contains(content, "fintech"):
		return "ojk"
	case strings.Contains(content, "bssn") || strings.Contains(content, "incident report"):
		return "bssn"
	case strings.Contains(content, "sdppi") || strings.Contains(content, "frekuensi"):
		return "sdppi"
	default:
		return "general"
	}
}
