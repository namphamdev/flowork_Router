// Helper: Gemini shape ↔ canonical.
package helpers

import (
	"regexp"
	"strings"
)

// MapGeminiRole maps OpenAI role → Gemini role.
func MapGeminiRole(role string) string {
	switch role {
	case "assistant":
		return "model"
	case "system":
		return "user" // gemini has no system role; prepend as user
	}
	return role
}

// MapGeminiFinishReason maps Gemini finishReason → OpenAI finish_reason.
func MapGeminiFinishReason(reason string) string {
	switch reason {
	case "STOP", "":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION":
		return "content_filter"
	}
	return "stop"
}

// reFunctionName mirrors Gemini's allowed function name pattern:
// [a-zA-Z_][a-zA-Z0-9_.:-]{0,63}.
var reFunctionName = regexp.MustCompile(`[^a-zA-Z0-9_.:\-]`)

// CleanFunctionName sanitizes a tool function name for Gemini/Antigravity.
// Replaces disallowed chars with "_", ensures leading char is alpha or "_",
// caps length at 64.
func CleanFunctionName(name string) string {
	if name == "" {
		return "_unknown"
	}
	s := reFunctionName.ReplaceAllString(name, "_")
	if s == "" {
		return "_unknown"
	}
	if !isAlphaOrUnderscore(s[0]) {
		s = "_" + s
	}
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

func isAlphaOrUnderscore(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

// CleanJSONSchemaForAntigravity strips fields the Antigravity / Cloud Code
// Assist endpoint rejects from a JSON-Schema function parameter spec:
// $schema, additionalProperties, definitions, $defs, etc. Operates in-place.
func CleanJSONSchemaForAntigravity(node any) {
	switch m := node.(type) {
	case map[string]any:
		for _, k := range []string{"$schema", "additionalProperties", "definitions", "$defs", "title", "examples"} {
			delete(m, k)
		}
		// `type` ↔ Gemini-supported set; drop "null" entries from "type": ["string","null"]
		if t, ok := m["type"].([]any); ok {
			pruned := make([]any, 0, len(t))
			for _, x := range t {
				if s, _ := x.(string); s != "" && s != "null" {
					pruned = append(pruned, s)
				}
			}
			if len(pruned) == 1 {
				m["type"] = pruned[0]
			} else if len(pruned) > 1 {
				m["type"] = pruned
			} else {
				delete(m, "type")
			}
		}
		for _, v := range m {
			CleanJSONSchemaForAntigravity(v)
		}
	case []any:
		for _, v := range m {
			CleanJSONSchemaForAntigravity(v)
		}
	}
	_ = strings.Builder{}
}
