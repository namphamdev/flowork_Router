// Package ingestor — pii_strip.go
// PII (Personally Identifiable Information) stripping for training data.
//
// KEPUTUSAN tim 4-AI: mandatory BEFORE ingest.
// HackerOne data may contain victim emails, internal URLs, API keys.
// Per Gemini security audit + Antigravity §9 risk assessment.
//
// Pattern: replace PII with redaction tags, never delete rows.
// FQP-12: append-only (original content not recoverable from brain DB).
package ingestor

import (
	"regexp"
)

// PIIRedactionTag is the replacement string for detected PII.
const PIIRedactionTag = "[PII_REDACTED]"

// piiPatterns defines regex patterns for common PII types.
//
// ORDER MATTERS — pattern lebih spesifik (longer match) WAJIB dulu sebelum
// lebih general. Contoh: NIK Indonesia 16 digit harus run sebelum phone_intl
// karena phone bisa consume inner digit sequence yang start dengan 0 (rc190
// hotfix per ROADMAP_AKTIF §6.2 unit test).
var piiPatterns = []struct {
	Name    string
	Pattern *regexp.Regexp
	Replace string
}{
	{
		Name:    "email",
		Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
		Replace: "[EMAIL_REDACTED]",
	},
	{
		Name:    "ipv4",
		Pattern: regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`),
		Replace: "[IP_REDACTED]",
	},
	{
		Name:    "nik_indonesia",
		Pattern: regexp.MustCompile(`\b\d{16}\b`), // 16 digits — moved BEFORE phone (anti-overlap)
		Replace: "[NIK_REDACTED]",
	},
	{
		Name:    "phone_intl",
		Pattern: regexp.MustCompile(`(?:\+62|0)\d{9,13}`),
		Replace: "[PHONE_REDACTED]",
	},
	{
		Name:    "api_key_generic",
		Pattern: regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|access[_-]?token|secret[_-]?key|auth[_-]?token)\s*[=:]\s*["']?[A-Za-z0-9_\-]{20,}["']?`),
		Replace: "[API_KEY_REDACTED]",
	},
	{
		Name:    "aws_key",
		Pattern: regexp.MustCompile(`AKIA[A-Z0-9]{16}`),
		Replace: "[AWS_KEY_REDACTED]",
	},
	{
		Name:    "private_key_header",
		Pattern: regexp.MustCompile(`-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----`),
		Replace: "[PRIVATE_KEY_REDACTED]",
	},
	{
		Name:    "credit_card",
		Pattern: regexp.MustCompile(`\b(?:4\d{3}|5[1-5]\d{2}|3[47]\d{2}|6(?:011|5\d{2}))[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`),
		Replace: "[CC_REDACTED]",
	},
	{
		Name:    "ssn_us",
		Pattern: regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		Replace: "[SSN_REDACTED]",
	},
	{
		Name:    "btc_address",
		Pattern: regexp.MustCompile(`\b(?:1|3|bc1)[A-Za-z0-9]{25,42}\b`),
		Replace: "[CRYPTO_ADDR_REDACTED]",
	},
	{
		Name:    "eth_address",
		Pattern: regexp.MustCompile(`\b0x[a-fA-F0-9]{40}\b`),
		Replace: "[CRYPTO_ADDR_REDACTED]",
	},
	{
		Name:    "mnemonic_seed",
		Pattern: regexp.MustCompile(`(?i)\b(?:abandon|ability|able|about|above|absent)\b(?:\s+\w+){11,23}`),
		Replace: "[MNEMONIC_REDACTED]",
	},
}

// StripPII replaces all detected PII in text with redaction tags.
// Returns the cleaned text and count of redactions made.
func StripPII(text string) (string, int) {
	count := 0
	result := text

	for _, p := range piiPatterns {
		matches := p.Pattern.FindAllStringIndex(result, -1)
		if len(matches) > 0 {
			count += len(matches)
			result = p.Pattern.ReplaceAllString(result, p.Replace)
		}
	}

	return result, count
}

// StripPIIFromRow applies PII stripping to both prompt and completion.
func StripPIIFromRow(row *TrainingRow) int {
	var totalRedactions int

	cleaned, n := StripPII(row.Prompt)
	row.Prompt = cleaned
	totalRedactions += n

	cleaned, n = StripPII(row.Completion)
	row.Completion = cleaned
	totalRedactions += n

	return totalRedactions
}

// ContainsPII checks if text contains any PII patterns.
func ContainsPII(text string) bool {
	for _, p := range piiPatterns {
		if p.Pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// RedactExploitPayloads replaces literal exploit code with safe description.
// Per Gemini security audit: training data should contain KNOWLEDGE about exploits,
// not copy-pasteable exploit payloads.
func RedactExploitPayloads(text string) string {
	// Replace common exploit payload patterns
	replacements := []struct {
		Pattern *regexp.Regexp
		Replace string
	}{
		{
			// SQL injection payloads
			Pattern: regexp.MustCompile(`(?i)(?:'\s*(?:OR|AND)\s+(?:'?\d+'?\s*=\s*'?\d+'?|1\s*=\s*1)(?:\s*--)?)`),
			Replace: "[SQLI_PAYLOAD_REDACTED]",
		},
		{
			// XSS payloads
			Pattern: regexp.MustCompile(`<script[^>]*>.*?</script>`),
			Replace: "[XSS_PAYLOAD_REDACTED]",
		},
		{
			// Shell reverse shells
			Pattern: regexp.MustCompile(`(?i)(?:bash\s+-i\s+>&\s*/dev/tcp/|nc\s+-[elp]+\s+\d+\s+-e\s+/bin/)`),
			Replace: "[REVERSE_SHELL_REDACTED]",
		},
	}

	result := text
	for _, r := range replacements {
		result = r.Pattern.ReplaceAllString(result, r.Replace)
	}
	return result
}

// SanitizeTrainingRow applies full sanitization pipeline to a row.
// Order: PII strip → exploit payload redact.
func SanitizeTrainingRow(row *TrainingRow) int {
	redactions := StripPIIFromRow(row)
	row.Completion = RedactExploitPayloads(row.Completion)
	row.Prompt = RedactExploitPayloads(row.Prompt)
	return redactions
}

