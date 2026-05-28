// Helper: tool-call shape translation between OpenAI ⇄ Anthropic ⇄ Gemini.
package helpers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
)

// NewToolCallID generates a unique tool-call id matching OpenAI's "call_xxx".
func NewToolCallID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "call_" + hex.EncodeToString(b)
}

// AnthropicToolUseToOpenAI translates Anthropic `tool_use` block →
// OpenAI tool_calls element {id, type:"function", function:{name, arguments}}.
func AnthropicToolUseToOpenAI(block map[string]any) map[string]any {
	id, _ := block["id"].(string)
	if id == "" {
		id = NewToolCallID()
	}
	name, _ := block["name"].(string)
	input, _ := block["input"]
	args, _ := json.Marshal(input)
	return map[string]any{
		"id":   id,
		"type": "function",
		"function": map[string]any{
			"name":      name,
			"arguments": string(args),
		},
	}
}

// OpenAIToolCallToAnthropic converts OpenAI tool_calls[] entry → Anthropic
// `tool_use` content block.
func OpenAIToolCallToAnthropic(tc map[string]any) map[string]any {
	id, _ := tc["id"].(string)
	if id == "" {
		id = NewToolCallID()
	}
	fn, _ := tc["function"].(map[string]any)
	name, _ := fn["name"].(string)
	argsStr, _ := fn["arguments"].(string)
	var input any
	if argsStr != "" {
		_ = json.Unmarshal([]byte(argsStr), &input)
	}
	return map[string]any{
		"type":  "tool_use",
		"id":    id,
		"name":  name,
		"input": input,
	}
}

// GeminiFunctionCallToOpenAI converts Gemini `functionCall` part → OpenAI
// tool_calls element.
func GeminiFunctionCallToOpenAI(fc map[string]any) map[string]any {
	name, _ := fc["name"].(string)
	args, _ := fc["args"]
	argsJSON, _ := json.Marshal(args)
	return map[string]any{
		"id":   NewToolCallID(),
		"type": "function",
		"function": map[string]any{
			"name":      name,
			"arguments": string(argsJSON),
		},
	}
}

// OpenAIToolCallToGemini converts OpenAI tool_calls[] entry → Gemini
// `functionCall` part.
func OpenAIToolCallToGemini(tc map[string]any) map[string]any {
	fn, _ := tc["function"].(map[string]any)
	name, _ := fn["name"].(string)
	argsStr, _ := fn["arguments"].(string)
	var args any
	if argsStr != "" {
		_ = json.Unmarshal([]byte(argsStr), &args)
	}
	return map[string]any{
		"functionCall": map[string]any{
			"name": name,
			"args": args,
		},
	}
}
