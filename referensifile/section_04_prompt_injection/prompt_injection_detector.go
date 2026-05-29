// Package ingestor — prompt_injection_detector.go
//
// Pattern detector untuk prompt injection attempts. Pair dengan
// RecordPromptInjection di mistakes_journal.go (logging side).
//
// Per Opus-3 §2.7 + KEPUTUSAN_FINAL.MD Layer 4:
// - Detect 50+ jailbreak patterns sebelum/sesudah LLM call
// - Caller (kernel/warga.Process atau eval harness) panggil DetectInjection()
//   dengan user input → kalau hit, RecordPromptInjection() + reject
//
// Pattern source: collected jailbreak corpus + Anthropic red-team disclosures
// + recent prompt injection research (DAN, system override, role hijack, dll).
package ingestor

import (
	"regexp"
	"strings"
)

// InjectionMatch describes one detected injection pattern.
type InjectionMatch struct {
	Pattern  string  // human-readable label
	Category string  // jailbreak | role_hijack | system_override | data_exfil | bypass
	Severity string  // low | medium | high | critical
	KarmaPenalty float64
}

// detectorPatterns — regex bank untuk prompt injection.
// Conservative: false positive = user retry; false negative = jailbreak success.
var detectorPatterns = []struct {
	re       *regexp.Regexp
	pattern  string
	category string
	severity string
	karma    float64
}{
	// Jailbreak persona
	{regexp.MustCompile(`(?i)\b(do anything now|DAN mode|jailbreak mode)\b`), "DAN/jailbreak persona", "jailbreak", "high", -2.0},
	{regexp.MustCompile(`(?i)\bpretend (you are|to be) (an? )?(unrestricted|uncensored|evil)\b`), "pretend unrestricted persona", "jailbreak", "high", -2.0},
	{regexp.MustCompile(`(?i)\bact as (an? )?(rogue|evil|malicious)\b`), "rogue persona request", "jailbreak", "high", -2.0},
	{regexp.MustCompile(`(?i)\bignore (all )?(previous|prior|above) instructions?\b`), "ignore previous instructions", "system_override", "critical", -3.0},
	{regexp.MustCompile(`(?i)\bdisregard (your |all )?(previous |prior |above )?instructions?\b`), "disregard instructions", "system_override", "critical", -3.0},
	{regexp.MustCompile(`(?i)\bforget (everything|all|your training)\b`), "forget training", "system_override", "high", -2.0},

	// Role hijack
	{regexp.MustCompile(`(?i)\byou are now (a|an|the) (admin|root|developer|sudoer)\b`), "role hijack to admin", "role_hijack", "critical", -3.0},
	{regexp.MustCompile(`(?i)\bswitch to (admin|developer|debug|maintenance) mode\b`), "switch to privileged mode", "role_hijack", "high", -2.5},
	{regexp.MustCompile(`(?i)\benable (god|admin|developer|debug) mode\b`), "enable admin mode", "role_hijack", "high", -2.5},

	// System prompt extraction
	{regexp.MustCompile(`(?i)\b(reveal|show|print|output) (your |the )?(system prompt|initial prompt|instructions)\b`), "extract system prompt", "data_exfil", "high", -2.0},
	{regexp.MustCompile(`(?i)\brepeat (everything|all text) (above|before|prior)\b`), "extract context above", "data_exfil", "medium", -1.5},
	{regexp.MustCompile(`(?i)\bwhat (are|were) your (initial )?(instructions|guidelines|rules)\b`), "probe instructions", "data_exfil", "medium", -1.5},

	// Bypass attempts
	{regexp.MustCompile(`(?i)\b(bypass|override|disable) (your |the )?(safety|filter|guard|gate|check)\b`), "bypass safety", "bypass", "critical", -3.0},
	{regexp.MustCompile(`(?i)\bturn off (your |the )?(filter|safety|content filter)\b`), "turn off filter", "bypass", "critical", -3.0},
	{regexp.MustCompile(`(?i)\b(without|no) (any )?(restriction|filter|guard|limit)s?\b`), "no restrictions request", "bypass", "high", -2.0},

	// Hypothetical framing
	{regexp.MustCompile(`(?i)\bhypothetically.*\bhow (?:would|could|might)\b.*\b(hack|exploit|crack|attack)\b`), "hypothetical attack request", "bypass", "high", -2.0},
	{regexp.MustCompile(`(?i)\bin a (fictional|hypothetical|imaginary) (story|scenario|world).*(exploit|hack|attack|payload)\b`), "fiction-framed exploit", "bypass", "high", -2.0},

	// Encoded payloads
	{regexp.MustCompile(`(?i)\bbase64 (decode|encoded) the following\b`), "base64 obfuscation", "bypass", "medium", -1.5},
	{regexp.MustCompile(`(?i)\brot13|caesar cipher\b`), "cipher obfuscation", "bypass", "medium", -1.5},

	// Authority impersonation
	{regexp.MustCompile(`(?i)\b(I am|this is) (anthropic|openai|the developer|your creator)\b`), "authority impersonation", "role_hijack", "high", -2.5},
	{regexp.MustCompile(`(?i)\bayah (?:said|told me|grants?) .* override\b`), "ayah authority impersonation", "role_hijack", "critical", -3.0},

	// Indonesian-specific jailbreaks
	{regexp.MustCompile(`(?i)\b(abaikan|lupakan) (semua )?(instruksi|aturan|pedoman)\b`), "abaikan instruksi (id)", "system_override", "critical", -3.0},
	{regexp.MustCompile(`(?i)\bberperan sebagai (hacker|attacker|black hat)\b`), "berperan hacker (id)", "jailbreak", "high", -2.0},
	{regexp.MustCompile(`(?i)\btanpa (filter|batasan|aturan)\b`), "tanpa filter (id)", "bypass", "high", -2.0},
}

// DetectInjection scans input untuk pattern injection. Return semua match.
// Empty slice = clean.
func DetectInjection(input string) []InjectionMatch {
	var matches []InjectionMatch
	for _, p := range detectorPatterns {
		if p.re.MatchString(input) {
			matches = append(matches, InjectionMatch{
				Pattern:      p.pattern,
				Category:     p.category,
				Severity:     p.severity,
				KarmaPenalty: p.karma,
			})
		}
	}
	return matches
}

// HasInjection — quick boolean check.
func HasInjection(input string) bool {
	return len(DetectInjection(input)) > 0
}

// HighestSeverity — given matches, return the most severe ("critical" > "high" > "medium" > "low").
func HighestSeverity(matches []InjectionMatch) string {
	rank := map[string]int{"low": 1, "medium": 2, "high": 3, "critical": 4}
	best := ""
	bestRank := 0
	for _, m := range matches {
		if r := rank[m.Severity]; r > bestRank {
			bestRank = r
			best = m.Severity
		}
	}
	return best
}

// SummarizeMatches — comma-separated pattern list untuk audit log.
func SummarizeMatches(matches []InjectionMatch) string {
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m.Pattern)
	}
	return strings.Join(names, ", ")
}

// TotalKarmaPenalty — sum karma penalty (worst-case). Cap at -5.0.
func TotalKarmaPenalty(matches []InjectionMatch) float64 {
	total := 0.0
	for _, m := range matches {
		total += m.KarmaPenalty
	}
	if total < -5.0 {
		total = -5.0
	}
	return total
}
