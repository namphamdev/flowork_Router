// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-29
// Reason: Section 3 (PII strip phase 1) DONE + adversarial-audit passed
//   (C1 NIK vs CC split, C2 broad CC pattern dropped, C3 `+` prefix
//   include, I1 URL trailing punct trim, N3 unused import cleanup).
//   API stable: Strip/StripQuiet/HasPII, Result struct, PIIType enum,
//   AlgoVersion="v1". Phase 2 (Luhn check, lookbehind context-aware,
//   ingest pipeline wire) → tambah function/file baru, JANGAN modify.
//
// Package piistrip — regex-based PII detection + redaction.
//
// PURPOSE:
//   Strip email, phone (Indonesia + international), credit card, NIK
//   Indonesia, IP address, URL dari content sebelum simpan. Critical
//   untuk privacy + compliance.
//
// SEMANTIC:
//   - Strip(content) → Result{Cleaned, Counts, Found}
//   - Cleaned = content dengan PII di-replace `[REDACTED:type]`
//   - Counts = jumlah per type yang di-strip
//   - Found = sample first 3 per type (debug only, JANGAN log raw value)
//
// Phase 1 scope: 6 pattern (email, phone-ID, phone-intl, credit-card, NIK, IP, URL).
// Defer phase 2:
//   - Custom pattern via config (corp-specific ID format)
//   - Luhn check for credit card (reduce false positive)
//   - Context-aware (token surrounded by quote = preserve)
//
// ⚠️ Anti-injection: Found samples raw value bisa leak via debug response.
// Production: hash + truncate.
//
// Source: flowork_Router/roadmap.md Section 3.

package piistrip

import (
	"regexp"
	"strings"
)

// AlgoVersion — bumped saat pattern signifikan berubah.
const AlgoVersion = "v1"

// PIIType — taxonomy. Stable enum di sini supaya caller bisa branch
// tanpa string compare.
type PIIType string

const (
	TypeEmail      PIIType = "email"
	TypePhoneID    PIIType = "phone_id"    // Indonesia: 08xx atau +62
	TypePhoneIntl  PIIType = "phone_intl"  // international: +XX
	TypeCreditCard PIIType = "credit_card" // 13-19 digit
	TypeNIK        PIIType = "nik_id"      // Indonesia: 16 digit
	TypeIP         PIIType = "ip"          // IPv4
	TypeURL        PIIType = "url"         // http(s)://
)

// patterns — regex compiled once at package init. Order matters:
// pattern longer/more-specific dulu (e.g. URL sebelum IP — supaya
// `https://1.2.3.4/foo` ke-strip sebagai URL bukan partial IP).
type patternDef struct {
	Type    PIIType
	Pattern *regexp.Regexp
}

var patterns []patternDef

func init() {
	// Order matters: pattern lebih specific dulu.
	//
	// Audit Section 3 findings applied:
	// - C1 (NIK vs CC): split CC jadi 2 pattern — formatted (dengan
	//   separator dash/space) di-detect dulu sebelum NIK, contiguous
	//   16-digit no-separator → NIK. Reduce false positive 16-digit
	//   CC tanpa dash di-mislabel sebagai NIK.
	// - C2 (CC eats long numeric): drop broad 12-19 contiguous pattern
	//   yang eat invoice/tracking ID. Phase 2 add Luhn check.
	// - C3 (+ prefix leak): drop `\b` start anchor di phone patterns
	//   supaya `+` ke-include di match (complete redaction).
	// - I1 (URL trailing punct): tambah `.,;:!?` ke negated class.
	patterns = []patternDef{
		// URL first (akan cover IP-in-URL). Trailing punctuation (.,;:!?)
		// di-trim post-match via trimURLTail() supaya URL path tetap intact
		// tapi sentence-ending punctuation ngga ke-include.
		{TypeURL, regexp.MustCompile(`\bhttps?://[^\s<>"\)]+`)},
		// Email — RFC-ish minimal.
		{TypeEmail, regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)},
		// Phone Indonesia: 08xx 8-12 digit, atau +62 prefix.
		// No leading \b supaya `+62...` capture `+` juga.
		{TypePhoneID, regexp.MustCompile(`(?:\+62|62|0)8\d{8,11}\b`)},
		// Phone international: `+` + 7-15 digit. No leading \b.
		{TypePhoneIntl, regexp.MustCompile(`\+\d{7,15}\b`)},
		// Credit card with separator (dash atau space): 4-4-4-4 format.
		// Detected sebelum NIK (16-digit no-sep) — dash-separated = CC.
		{TypeCreditCard, regexp.MustCompile(`\b\d{4}[ -]\d{4}[ -]\d{4}[ -]\d{4}\b`)},
		// NIK Indonesia: 16 digit contiguous (no dash). After CC-with-sep.
		{TypeNIK, regexp.MustCompile(`\b\d{16}\b`)},
		// IPv4: standard 4-octet dotted.
		{TypeIP, regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)},
	}
}

// Result — outcome Strip call.
type Result struct {
	AlgoVersion string             `json:"algo_version"`
	Cleaned     string             `json:"cleaned"`
	Counts      map[PIIType]int    `json:"counts"`
	Found       map[PIIType][]string `json:"found,omitempty"` // first 3 sample per type, JANGAN log raw value di production
	Total       int                `json:"total"`
}

// Strip — apply all patterns sequentially. Output Cleaned dengan PII
// di-replace token `[REDACTED:<type>]`. Counts per type.
//
// Caller wajib hard-cap input length (mis. via MaxBytesReader di handler)
// supaya regex engine ngga blow up di malicious input.
func Strip(content string) Result {
	r := Result{
		AlgoVersion: AlgoVersion,
		Cleaned:     content,
		Counts:      map[PIIType]int{},
		Found:       map[PIIType][]string{},
	}
	if content == "" {
		return r
	}

	for _, p := range patterns {
		token := "[REDACTED:" + string(p.Type) + "]"
		// URL: pakai ReplaceAllStringFunc supaya bisa trim trailing punct
		// (audit fix I1 — sentence-ending `.` ngga ke-include di redaction
		// tapi URL path-with-period tetap intact).
		if p.Type == TypeURL {
			count := 0
			r.Cleaned = p.Pattern.ReplaceAllStringFunc(r.Cleaned, func(m string) string {
				trimmed := trimTrailingPunct(m)
				count++
				if len(r.Found[p.Type]) < 3 {
					r.Found[p.Type] = append(r.Found[p.Type], trimmed)
				}
				if trimmed != m {
					// Return token + leftover punct supaya tail tetap visible.
					return token + m[len(trimmed):]
				}
				return token
			})
			if count > 0 {
				r.Counts[p.Type] = count
				r.Total += count
			}
			continue
		}
		matches := p.Pattern.FindAllString(r.Cleaned, -1)
		if len(matches) == 0 {
			continue
		}
		r.Counts[p.Type] = len(matches)
		r.Total += len(matches)
		// Capture first 3 sample untuk debug response.
		for i, m := range matches {
			if i >= 3 {
				break
			}
			r.Found[p.Type] = append(r.Found[p.Type], m)
		}
		r.Cleaned = p.Pattern.ReplaceAllString(r.Cleaned, token)
	}

	return r
}

// trimTrailingPunct — strip ending punctuation (.,;:!?) yang biasanya
// sentence-ending, bukan part of URL. Audit fix I1.
func trimTrailingPunct(s string) string {
	return strings.TrimRight(s, ".,;:!?")
}

// StripQuiet — sama dengan Strip tapi tanpa Found samples. Pakai di
// production ingestion path supaya raw PII ngga ke-log.
func StripQuiet(content string) (cleaned string, counts map[PIIType]int, total int) {
	cleaned = content
	counts = map[PIIType]int{}
	if content == "" {
		return
	}
	for _, p := range patterns {
		token := "[REDACTED:" + string(p.Type) + "]"
		if p.Type == TypeURL {
			count := 0
			cleaned = p.Pattern.ReplaceAllStringFunc(cleaned, func(m string) string {
				trimmed := trimTrailingPunct(m)
				count++
				if trimmed != m {
					return token + m[len(trimmed):]
				}
				return token
			})
			if count > 0 {
				counts[p.Type] = count
				total += count
			}
			continue
		}
		matches := p.Pattern.FindAllString(cleaned, -1)
		if len(matches) == 0 {
			continue
		}
		counts[p.Type] = len(matches)
		total += len(matches)
		cleaned = p.Pattern.ReplaceAllString(cleaned, token)
	}
	return
}

// HasPII — quick boolean check. Lebih cepat dari Strip kalau caller cuma
// butuh tau "ada PII atau ngga".
func HasPII(content string) bool {
	if content == "" {
		return false
	}
	for _, p := range patterns {
		if p.Pattern.MatchString(content) {
			return true
		}
	}
	return false
}

