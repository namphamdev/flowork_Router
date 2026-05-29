// Package pricing menghitung biaya session dalam USD dari token usage (#14).
// Tabel harga per 1M tokens ringkas — di-update saat provider ubah harga.
// Untuk cache tokens, Anthropic bill cache_read di 10% harga input.
package pricing

import (
	"fmt"
	"strings"

	"github.com/teetah2402/flowork/internal/provider"
)

// PricePer1M (USD). Angka diambil dari daftar harga publik per vendor
// (April 2026). Update di sini kalau vendor merubah harga.
type Tier struct {
	InputPer1M  float64
	OutputPer1M float64
	// CacheReadPer1M default = InputPer1M * 0.1 (Anthropic pattern).
	CacheReadPer1M float64
}

var table = map[string]Tier{
	// Anthropic
	"claude-opus-4-6":   {InputPer1M: 15.00, OutputPer1M: 75.00, CacheReadPer1M: 1.50},
	"claude-sonnet-4-6": {InputPer1M: 3.00, OutputPer1M: 15.00, CacheReadPer1M: 0.30},
	"claude-haiku-4-5":  {InputPer1M: 1.00, OutputPer1M: 5.00, CacheReadPer1M: 0.10},
	// OpenAI
	"gpt-4.1-mini": {InputPer1M: 0.40, OutputPer1M: 1.60},
	"gpt-4.1":      {InputPer1M: 2.00, OutputPer1M: 8.00},
	// DeepSeek
	"deepseek-chat": {InputPer1M: 0.27, OutputPer1M: 1.10},
	// Gemini
	"gemini-2.5-flash": {InputPer1M: 0.10, OutputPer1M: 0.40},
	"gemini-2.5-pro":   {InputPer1M: 1.25, OutputPer1M: 10.00},
	// XAI
	"grok-2-latest": {InputPer1M: 2.00, OutputPer1M: 10.00},
	"grok-4":        {InputPer1M: 5.00, OutputPer1M: 15.00},
	// Ollama (offline) free
	"llama3": {},
}

// Estimate returns USD cost for a given model + usage. Returns 0 kalau model
// tidak ada di tabel — owner tahu harus tambahkan price kalau mau tracking.
func Estimate(model string, u provider.Usage) float64 {
	tier, ok := table[strings.ToLower(strings.TrimSpace(model))]
	if !ok {
		// Try prefix match untuk varian minor (mis. "claude-sonnet-4-6-20260301").
		for k, v := range table {
			if strings.HasPrefix(strings.ToLower(model), k) {
				tier = v
				ok = true
				break
			}
		}
	}
	if !ok {
		return 0
	}
	cacheRead := tier.CacheReadPer1M
	if cacheRead == 0 {
		cacheRead = tier.InputPer1M * 0.1
	}
	in := float64(u.InputTokens) / 1_000_000 * tier.InputPer1M
	out := float64(u.OutputTokens) / 1_000_000 * tier.OutputPer1M
	cr := float64(u.CacheReadInputTokens) / 1_000_000 * cacheRead
	// cache_creation charged di harga input penuh (Anthropic convention).
	cc := float64(u.CacheCreationInputTokens) / 1_000_000 * tier.InputPer1M
	return in + out + cr + cc
}

// Format helper untuk display. "$0.0042" style.
func Format(usd float64) string {
	if usd <= 0 {
		return "$0.0000"
	}
	return fmt.Sprintf("$%.4f", usd)
}
