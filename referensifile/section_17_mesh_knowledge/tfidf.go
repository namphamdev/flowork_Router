// Package mesh — tfidf.go: lightweight TF-IDF + cosine engine (M4.5).
//
// Anti-bypass L7 cosine validate (W-3 fix per AMENDMENTS-V1). Tanpa
// embedding neural (BRAIN_ARCHITECTURE §15.3 status pending), L7 stub
// return 0.5 = effectively bypass. Solusi interim:
//
//   - Tokenize incoming packet payload (Indonesian + English stopword)
//   - TF-IDF vectorize via vocabulary built from local brain knowledge
//   - Cosine vs centroid (top-N most-cited memory entries)
//
// Threshold di L7:
//
//	cosine < 0.2 = flag_suspicious (raise consensus threshold)
//	cosine 0.2-0.7 = neutral
//	cosine > 0.7 = relevance_boost (lower consensus threshold)
//
// Future-compatible: kalau embedding neural ready, swap CosineEngine
// implementation. Filter pipeline (M4) interface tetap.
//
// Reuse pattern: ported dari floworkos-go/internal/skills/skills.go
// (Voyager TF-IDF Phase 1 fallback). Per EXISTING-CODE-AUDIT.

package mesh

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ---------- Tokenizer ----------

// stopwords minimal (Indonesia + English) — cukup untuk noise reduction.
//
// Bahasa Indonesia umum: artikel, partikel, pronoun, preposisi.
// English umum: articles, common verbs, conjunctions.
//
// Tambah pattern Indo casual (lo, gw, lu, gue) — sesuai gaya komunikasi
// project per PRINSIP P-9.
var meshStopwords = map[string]bool{
	// Indonesia
	"yang": true, "dengan": true, "untuk": true, "dari": true, "pada": true,
	"ini": true, "itu": true, "ada": true, "akan": true, "atau": true,
	"jika": true, "saja": true, "dan": true, "atas": true, "bawah": true,
	"dalam": true, "luar": true, "telah": true, "sudah": true, "belum": true,
	"tidak": true, "bukan": true, "juga": true, "lebih": true, "kurang": true,
	"adalah": true, "saya": true, "kamu": true, "kami": true, "kita": true,
	"mereka": true, "dia": true, "boleh": true, "harus": true, "perlu": true,
	"bisa": true, "sangat": true, "namun": true, "tapi": true, "tetapi": true,
	"karena": true, "sebab": true, "oleh": true, "supaya": true, "agar": true,
	"hingga": true, "sampai": true, "ketika": true, "saat": true, "setelah": true,
	"sebelum": true, "selama": true, "antara": true, "sebagai": true, "secara": true,
	// Casual gw/lo
	"gw": true, "lo": true, "lu": true, "gue": true, "elo": true,

	// English
	"the": true, "and": true, "for": true, "with": true, "this": true,
	"that": true, "are": true, "was": true, "has": true, "have": true,
	"can": true, "will": true, "but": true, "not": true, "you": true,
	"your": true, "from": true, "they": true, "them": true, "their": true,
	"what": true, "which": true, "when": true, "where": true, "how": true,
}

// tfTokenize lowercase + strip punctuation + filter stopwords + min len 3.
//
// Suffix Indo light strip: -nya, -lah, -kah (sastrawi-style minimal).
func tfTokenize(s string) []string {
	s = strings.ToLower(s)
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			tok := stripIndoSuffix(cur.String())
			if len(tok) >= 3 && !meshStopwords[tok] {
				out = append(out, tok)
			}
			cur.Reset()
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

// stripIndoSuffix strip suffix umum Indonesia kalau tetap >= 3 chars.
func stripIndoSuffix(s string) string {
	for _, suf := range []string{"nya", "lah", "kah", "tah", "pun"} {
		if strings.HasSuffix(s, suf) && len(s)-len(suf) >= 3 {
			return s[:len(s)-len(suf)]
		}
	}
	return s
}

// ---------- Vocabulary ----------

// Vocabulary holds IDF scores for top-N tokens from local corpus.
type Vocabulary struct {
	Tokens   map[string]int     `json:"tokens"`    // token → vocab index (untuk debug)
	IDF      map[string]float64 `json:"idf"`       // token → IDF score
	DocCount int                `json:"doc_count"` // total docs analyzed
	TopN     int                `json:"top_n"`
	BuiltAt  time.Time          `json:"built_at"`
}

// BuildVocabulary analyze drawer texts → top-N tokens by IDF descending.
//
// IDF formula: log(N / df) where df = doc frequency (jumlah doc yang
// mengandung term). Higher IDF = rarer term = more discriminative.
//
// topN cap default 10000. Empty corpus return vocab kosong (cosine 0).
func BuildVocabulary(drawerTexts []string, topN int) *Vocabulary {
	if topN <= 0 {
		topN = 10000
	}
	docFreq := make(map[string]int)
	for _, doc := range drawerTexts {
		seen := make(map[string]bool)
		for _, tok := range tfTokenize(doc) {
			if !seen[tok] {
				docFreq[tok]++
				seen[tok] = true
			}
		}
	}
	n := float64(len(drawerTexts))
	if n == 0 {
		return &Vocabulary{
			Tokens: map[string]int{}, IDF: map[string]float64{},
			DocCount: 0, TopN: topN, BuiltAt: time.Now().UTC(),
		}
	}
	type pair struct {
		tok string
		v   float64
	}
	var pairs []pair
	for tok, df := range docFreq {
		idf := math.Log(n / float64(df))
		if idf < 0 {
			idf = 0
		}
		pairs = append(pairs, pair{tok, idf})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].tok < pairs[j].tok // stable sort
	})
	if len(pairs) > topN {
		pairs = pairs[:topN]
	}
	tokens := make(map[string]int, len(pairs))
	idf := make(map[string]float64, len(pairs))
	for i, p := range pairs {
		tokens[p.tok] = i
		idf[p.tok] = p.v
	}
	return &Vocabulary{
		Tokens: tokens, IDF: idf,
		DocCount: len(drawerTexts), TopN: topN, BuiltAt: time.Now().UTC(),
	}
}

// SaveVocabulary persist atomic via temp + rename.
func SaveVocabulary(v *Vocabulary, path string) error {
	if v == nil {
		return errors.New("nil vocabulary")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadVocabulary read vocab file.
func LoadVocabulary(path string) (*Vocabulary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v Vocabulary
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	if v.Tokens == nil {
		v.Tokens = map[string]int{}
	}
	if v.IDF == nil {
		v.IDF = map[string]float64{}
	}
	return &v, nil
}

// ---------- Vectorize + Cosine ----------

// Vectorize text → sparse TF-IDF vector (map[token]weight).
//
// TF normalisasi by total token count (avoid bias panjang text).
// Token yang ngga ada di vocab di-drop (out-of-vocabulary).
func Vectorize(text string, vocab *Vocabulary) map[string]float64 {
	if vocab == nil {
		return nil
	}
	tokens := tfTokenize(text)
	if len(tokens) == 0 {
		return nil
	}
	tf := make(map[string]float64, len(tokens))
	for _, tok := range tokens {
		tf[tok]++
	}
	totalTokens := float64(len(tokens))
	vec := make(map[string]float64, len(tf))
	for tok, freq := range tf {
		idf, ok := vocab.IDF[tok]
		if !ok {
			continue // OOV
		}
		// Normalize TF + apply IDF
		vec[tok] = (freq / totalTokens) * idf
	}
	return vec
}

// CosineSimilarity standard cosine: dot(a,b) / (|a|*|b|).
//
// Empty vector → 0 (no similarity).
func CosineSimilarity(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var dot, na, nb float64
	for tok, av := range a {
		na += av * av
		if bv, ok := b[tok]; ok {
			dot += av * bv
		}
	}
	for _, bv := range b {
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// ---------- Centroid ----------

// ComputeCentroid average TF-IDF vector dari beberapa drawer texts.
// Dipakai untuk represent "what local knowledge looks like."
//
// Empty input → empty centroid.
func ComputeCentroid(drawerTexts []string, vocab *Vocabulary) map[string]float64 {
	if len(drawerTexts) == 0 || vocab == nil {
		return nil
	}
	centroid := make(map[string]float64)
	count := 0
	for _, text := range drawerTexts {
		vec := Vectorize(text, vocab)
		if vec == nil {
			continue
		}
		for tok, v := range vec {
			centroid[tok] += v
		}
		count++
	}
	if count == 0 {
		return nil
	}
	div := float64(count)
	for tok := range centroid {
		centroid[tok] /= div
	}
	return centroid
}

// ---------- CosineEngine ----------

// CosineEngine wrap vocabulary + centroid untuk filter L7 integration.
//
// Thread-safe: mutable state (vocab + centroid) di-protect mu.
type CosineEngine struct {
	mu       sync.RWMutex
	vocab    *Vocabulary
	centroid map[string]float64
}

// NewCosineEngine create with initial vocab + centroid.
func NewCosineEngine(vocab *Vocabulary, centroid map[string]float64) *CosineEngine {
	return &CosineEngine{vocab: vocab, centroid: centroid}
}

// Update swap vocab + centroid (called by cron rebuild).
func (c *CosineEngine) Update(vocab *Vocabulary, centroid map[string]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.vocab = vocab
	c.centroid = centroid
}

// CosineForText compute cosine similarity raw text vs local centroid.
// Filter L7 entry point.
//
// Empty centroid → return 0.5 (neutral, ngga reject ngga boost) — graceful
// degrade kalau corpus kosong di awal mesh start.
func (c *CosineEngine) CosineForText(text string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.vocab == nil || len(c.centroid) == 0 {
		return 0.5 // neutral default kalau corpus kosong
	}
	vec := Vectorize(text, c.vocab)
	if len(vec) == 0 {
		return 0
	}
	return CosineSimilarity(vec, c.centroid)
}

// CosineForBytes wrap CosineForText — accept []byte payload langsung.
func (c *CosineEngine) CosineForBytes(payload []byte) float64 {
	return c.CosineForText(string(payload))
}

// VocabSize return jumlah token di vocab (debug/stats).
func (c *CosineEngine) VocabSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.vocab == nil {
		return 0
	}
	return len(c.vocab.Tokens)
}

// CentroidSize return jumlah token non-zero di centroid.
func (c *CosineEngine) CentroidSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.centroid)
}

// ---------- Threshold helpers untuk filter L7 ----------

// CosineSuspiciousThreshold cosine < ini = suspicious (consensus threshold raise).
const CosineSuspiciousThreshold = 0.2

// CosineRelevanceBoostThreshold cosine > ini = highly relevant (consensus threshold lower).
const CosineRelevanceBoostThreshold = 0.7

// CosineVerdict classify cosine score ke 3 zona untuk filter pipeline M4 L7.
type CosineVerdict string

const (
	CosineVerdictSuspicious CosineVerdict = "suspicious"
	CosineVerdictNeutral    CosineVerdict = "neutral"
	CosineVerdictBoost      CosineVerdict = "boost"
)

// ClassifyCosine return verdict berdasar threshold.
func ClassifyCosine(score float64) CosineVerdict {
	switch {
	case score < CosineSuspiciousThreshold:
		return CosineVerdictSuspicious
	case score > CosineRelevanceBoostThreshold:
		return CosineVerdictBoost
	default:
		return CosineVerdictNeutral
	}
}
