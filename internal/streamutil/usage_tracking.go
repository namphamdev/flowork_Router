// Usage-shape helpers: normalize between OpenAI / Claude / Gemini, estimate
// when the upstream omits usage, and add a small token buffer to avoid
// context-window edge cases.

package streamutil

import (
	"encoding/json"
	"math"
)

// BufferTokens is the small fixed pad added to estimated usage so the next
// turn's context-window calculation has slack. Matches the reference.
const BufferTokens = 2000

// charsPerToken is the rough rule-of-thumb (1 token ≈ 4 chars) used by
// EstimateInputTokens / EstimateOutputTokens. Good enough for fallback
// reporting when the upstream didn't include real usage.
const charsPerToken = 4

// AddBufferToUsage returns a copy of usage with BufferTokens added to every
// "input"/prompt counter and total. nil/empty input passes through.
func AddBufferToUsage(usage map[string]any) map[string]any {
	if usage == nil {
		return usage
	}
	out := make(map[string]any, len(usage))
	for k, v := range usage {
		out[k] = v
	}
	// Claude shape.
	if v, ok := toFiniteFloat(out["input_tokens"]); ok {
		out["input_tokens"] = v + BufferTokens
	}
	// OpenAI shape.
	if v, ok := toFiniteFloat(out["prompt_tokens"]); ok {
		out["prompt_tokens"] = v + BufferTokens
	}
	// Total — bump if present, derive if absent and both halves known.
	if v, ok := toFiniteFloat(out["total_tokens"]); ok {
		out["total_tokens"] = v + BufferTokens
	} else {
		p, hasP := toFiniteFloat(out["prompt_tokens"])
		c, hasC := toFiniteFloat(out["completion_tokens"])
		if hasP && hasC {
			out["total_tokens"] = p + c
		}
	}
	return out
}

// NormalizeUsage extracts the numeric usage fields into a single canonical
// map: OpenAI keys for the counters, plus prompt/completion details when
// the source carried them. Returns nil for nil/non-map input.
func NormalizeUsage(usage map[string]any) map[string]any {
	if usage == nil {
		return nil
	}
	out := map[string]any{}
	for _, k := range []string{
		"prompt_tokens", "completion_tokens", "total_tokens",
		"cache_read_input_tokens", "cache_creation_input_tokens",
		"cached_tokens", "reasoning_tokens",
	} {
		if v, ok := toFiniteFloat(usage[k]); ok {
			out[k] = v
		}
	}
	if d, ok := usage["prompt_tokens_details"].(map[string]any); ok {
		out["prompt_tokens_details"] = d
	}
	if d, ok := usage["completion_tokens_details"].(map[string]any); ok {
		out["completion_tokens_details"] = d
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// FilterUsageForFormat keeps only the keys the target format expects on the
// wire. Stops accidental leakage of OpenAI-only fields into Claude/Gemini
// responses (and vice versa). Unknown formats pass through unchanged.
func FilterUsageForFormat(usage map[string]any, targetFormat string) map[string]any {
	if usage == nil {
		return nil
	}
	allowed, ok := allowedUsageFields[targetFormat]
	if !ok {
		return usage // unknown format → leave intact
	}
	out := map[string]any{}
	for _, k := range allowed {
		if v, present := usage[k]; present {
			out[k] = v
		}
	}
	return out
}

// allowedUsageFields lists the wire-shape per target format. Keep in sync
// with translator/response writers so we don't filter out fields the
// downstream encoder needs.
var allowedUsageFields = map[string][]string{
	FormatOpenAI: {
		"prompt_tokens", "completion_tokens", "total_tokens",
		"prompt_tokens_details", "completion_tokens_details",
		"estimated",
	},
	FormatResponses: {
		"input_tokens", "output_tokens", "total_tokens",
		"input_tokens_details", "output_tokens_details",
		"estimated",
	},
	FormatClaude: {
		"input_tokens", "output_tokens",
		"cache_read_input_tokens", "cache_creation_input_tokens",
		"estimated",
	},
	FormatGemini: {
		"promptTokenCount", "candidatesTokenCount", "totalTokenCount",
		"cachedContentTokenCount", "thoughtsTokenCount",
		"estimated",
	},
}

// EstimateInputTokens approximates the prompt token count from the request
// body's JSON size — a rough chars/4 estimate suitable for fallback when
// the provider doesn't include usage in its response.
func EstimateInputTokens(body any) int {
	if body == nil {
		return 0
	}
	raw, err := json.Marshal(body)
	if err != nil || len(raw) == 0 {
		return 0
	}
	return int(math.Ceil(float64(len(raw)) / float64(charsPerToken)))
}

// EstimateOutputTokens approximates the completion token count from the
// assistant response's character length. Same rough chars/4 mapping.
func EstimateOutputTokens(contentLength int) int {
	if contentLength <= 0 {
		return 0
	}
	if contentLength < charsPerToken {
		return 1
	}
	return contentLength / charsPerToken
}

// EstimateUsage returns a fallback usage map for targetFormat. Marks the
// result with estimated=true so callers / analytics can tell the difference
// from real upstream counts.
func EstimateUsage(body any, contentLength int, targetFormat string) map[string]any {
	in := EstimateInputTokens(body)
	out := EstimateOutputTokens(contentLength)
	return formatEstimatedUsage(in, out, targetFormat)
}

// formatEstimatedUsage emits the estimate in the right wire shape and adds
// the BufferTokens pad so downstream "did we run out of context?" math is
// conservative.
func formatEstimatedUsage(input, output int, targetFormat string) map[string]any {
	switch targetFormat {
	case FormatClaude:
		return AddBufferToUsage(map[string]any{
			"input_tokens":  float64(input),
			"output_tokens": float64(output),
			"estimated":     true,
		})
	case FormatGemini:
		total := input + output
		return AddBufferToUsage(map[string]any{
			"promptTokenCount":     float64(input),
			"candidatesTokenCount": float64(output),
			"totalTokenCount":      float64(total),
			"estimated":            true,
		})
	default: // OpenAI
		total := input + output
		return AddBufferToUsage(map[string]any{
			"prompt_tokens":     float64(input),
			"completion_tokens": float64(output),
			"total_tokens":      float64(total),
			"estimated":         true,
		})
	}
}

// HasValidUsage reports whether usage carries at least one positive token
// counter. Used by streaming dispatch to decide whether to fall back to
// EstimateUsage.
func HasValidUsage(usage map[string]any) bool {
	if usage == nil {
		return false
	}
	for _, k := range []string{
		"prompt_tokens", "completion_tokens", "total_tokens",
		"input_tokens", "output_tokens",
		"promptTokenCount", "candidatesTokenCount", "totalTokenCount",
	} {
		if v, ok := toFiniteFloat(usage[k]); ok && v > 0 {
			return true
		}
	}
	return false
}

// toFiniteFloat coerces any numeric type to float64 and reports whether the
// value was actually a finite number (filters NaN / Inf and non-numeric).
func toFiniteFloat(v any) (float64, bool) {
	var f float64
	switch t := v.(type) {
	case float64:
		f = t
	case float32:
		f = float64(t)
	case int:
		f = float64(t)
	case int32:
		f = float64(t)
	case int64:
		f = float64(t)
	case uint:
		f = float64(t)
	case uint32:
		f = float64(t)
	case uint64:
		f = float64(t)
	default:
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}
