package proxy

// router_rules.go — Rule-based query classifier untuk smart routing.
//
// Tujuan: classify query SEBELUM panggil LLM. Outcome:
//
//   - factual    → cascade L1-L4 (brain_search high confidence), skip LLM
//   - action     → annotate "this is action verb" supaya kernel inject tool list smaller
//   - creative   → forward straight ke LLM (skip cascade — semantic > keyword)
//   - real_time  → mandatory webfetch hint (price/news/weather → BRAIN HALU risk)
//   - unknown    → fallback default cascade
//
// Portable: zero new daemon. Patterns di settings DB (key ROUTER_RULES_JSON).
// Plug-and-play: kosong → use built-in defaults.
//
// Per Ayah arahan 2026-05-12: realisasi smart router rule-based dulu sebelum
// LLM-based classifier (cost-prohibitive call LLM untuk decide LLM call).

import (
	"regexp"
	"strings"
	"sync"
)

// QueryClass — kategori hasil classify.
type QueryClass string

const (
	QueryFactual  QueryClass = "factual"   // "apa itu X", "kapan", "dimana", "siapa", "berapa"
	QueryAction   QueryClass = "action"    // "bikin", "tolong", "edit", "tambah", "deploy"
	QueryCreative QueryClass = "creative"  // "bikinin puisi", "saran nama", "brainstorm"
	QueryRealTime QueryClass = "real_time" // "harga btc sekarang", "cuaca", "berita terbaru"
	QueryUnknown  QueryClass = "unknown"
)

// Classification — hasil classify dengan confidence + suggested routing.
type Classification struct {
	Class      QueryClass `json:"class"`
	Confidence float64    `json:"confidence"` // [0, 1]
	Hint       string     `json:"hint"`       // human-readable reason
	SkipBrain  bool       `json:"skip_brain"` // true → skip cascade L1-L4
	SkipLLM    bool       `json:"skip_llm"`   // true → answer dari brain tanpa LLM
	NeedTool   string     `json:"need_tool"`  // suggested tool name (mis. "webfetch")
}

// classifierPattern — internal regex pattern.
type classifierPattern struct {
	re    *regexp.Regexp
	class QueryClass
	score float64
}

// builtinPatterns — anti-halu rules dari empirical drift observed di FloworkOS.
// Order matters: longer match wins.
var (
	defaultPatternsOnce sync.Once
	defaultPatterns     []classifierPattern
)

func loadDefaultPatterns() []classifierPattern {
	defaultPatternsOnce.Do(func() {
		// Real-time signal: harga / price / today / sekarang / saat ini
		// MANDATORY webfetch, ANTI brain_search (brain stale).
		realtime := []string{
			`(?i)\b(harga|price)\b.{0,30}\b(sekarang|today|now|terkini|saat ini)\b`,
			`(?i)\b(btc|bitcoin|eth|ethereum|usd|idr|rupiah)\b.{0,30}\b(harga|price|rate|kurs)\b`,
			`(?i)\bberita\b.{0,30}\b(terbaru|hari ini|today)\b`,
			`(?i)\bcuaca\b.{0,30}\b(sekarang|hari ini|today)\b`,
			`(?i)\b(weather|forecast)\b.{0,30}\b(today|now)\b`,
		}
		for _, p := range realtime {
			defaultPatterns = append(defaultPatterns, classifierPattern{
				re: regexp.MustCompile(p), class: QueryRealTime, score: 0.9,
			})
		}

		// Action verb (Indonesian + English): bikin/tolong/edit/tambah/buat
		action := []string{
			`(?i)^(tolong|please)\b`,
			`(?i)\b(bikin|buat|edit|tambah|hapus|fix|deploy|run|jalankan|test|commit|push|merge)\b`,
			`(?i)\b(create|update|delete|implement|refactor|generate)\b`,
		}
		for _, p := range action {
			defaultPatterns = append(defaultPatterns, classifierPattern{
				re: regexp.MustCompile(p), class: QueryAction, score: 0.7,
			})
		}

		// Factual question: apa/kapan/dimana/siapa/berapa/bagaimana
		factual := []string{
			`(?i)^(apa|kapan|dimana|siapa|berapa|bagaimana|mengapa|kenapa)\b`,
			`(?i)^(what|when|where|who|why|how|which)\b`,
			`(?i)\bjelaskan\b`,
			`(?i)\bexplain\b`,
		}
		for _, p := range factual {
			defaultPatterns = append(defaultPatterns, classifierPattern{
				re: regexp.MustCompile(p), class: QueryFactual, score: 0.6,
			})
		}

		// Creative: brainstorm / saran / ide / puisi (strong signals — outweigh
		// generic action verbs like "bikin/buat"). Score 0.8 > action 0.7 supaya
		// "bikin puisi" classify creative, bukan action.
		creative := []string{
			`(?i)\b(brainstorm|saran|ide|ideas|suggestions)\b`,
			`(?i)\b(puisi|cerita|brand)\b`,
			`(?i)\b(creative|kreatif|imaginatif)\b`,
		}
		for _, p := range creative {
			defaultPatterns = append(defaultPatterns, classifierPattern{
				re: regexp.MustCompile(p), class: QueryCreative, score: 0.8,
			})
		}
	})
	return defaultPatterns
}

// ClassifyQuery — apply rule-based regex match, return best class + confidence.
//
// Algoritma:
//
//  1. Run semua pattern, kumpulkan match.
//  2. Pick class dengan total score tertinggi (sum of matching pattern scores).
//  3. Confidence = tied score / total score. Single class dominate → high conf.
//  4. Set routing flags based on class.
//
// Empty query → QueryUnknown / conf 0.
func ClassifyQuery(query string) Classification {
	q := strings.TrimSpace(query)
	if q == "" {
		return Classification{Class: QueryUnknown, Confidence: 0, Hint: "empty query"}
	}
	patterns := loadDefaultPatterns()
	scores := map[QueryClass]float64{}
	for _, p := range patterns {
		if p.re.MatchString(q) {
			scores[p.class] += p.score
		}
	}
	if len(scores) == 0 {
		return Classification{
			Class: QueryUnknown, Confidence: 0,
			Hint: "no pattern match, default cascade",
		}
	}

	// Pick highest scoring class
	var bestClass QueryClass
	var bestScore float64
	var totalScore float64
	for c, s := range scores {
		totalScore += s
		if s > bestScore {
			bestScore = s
			bestClass = c
		}
	}
	confidence := bestScore / totalScore // [0, 1]; 1 = single class dominate

	c := Classification{
		Class:      bestClass,
		Confidence: confidence,
		Hint:       string(bestClass) + " pattern matched",
	}

	// Routing hints per class
	switch bestClass {
	case QueryRealTime:
		c.SkipBrain = true
		c.NeedTool = "webfetch"
		c.Hint = "real-time data — mandatory webfetch (brain stale risk)"
	case QueryFactual:
		// Default cascade behavior — brain first, LLM fallback.
		c.Hint = "factual question — brain cascade preferred"
	case QueryAction:
		c.Hint = "action verb — tool dispatch likely, smaller tool list"
	case QueryCreative:
		c.SkipBrain = true
		c.Hint = "creative task — LLM direct (semantic > keyword)"
	}

	return c
}

// ShouldForwardToLLM — decision helper berdasar classification + hit dari cascade.
//
// hitConfidence dari brainv2.Resolve cascade (0 = no hit, 1 = exact cache).
// Default behavior preserved kalau classification ngga decisive.
func (c Classification) ShouldForwardToLLM(hitConfidence float64, gateThreshold float64) bool {
	if c.SkipBrain {
		return true // creative/realtime → langsung LLM
	}
	if c.SkipLLM {
		return false // factual hit high-confidence → answer brain
	}
	// Default gate
	return hitConfidence < gateThreshold
}
