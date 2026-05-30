// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package translator_test

import (
	"testing"

	"github.com/flowork-os/flowork_Router/internal/translator"
	_ "github.com/flowork-os/flowork_Router/internal/translator/request"
	_ "github.com/flowork-os/flowork_Router/internal/translator/response"
)

func TestTranslator_AllPairsRegistered(t *testing.T) {
	cases := []struct {
		From, To string
		Dir      translator.Direction
	}{
		// Request pairs (11)
		{"claude", "openai", translator.DirRequest},
		{"gemini", "openai", translator.DirRequest},
		{"openai", "claude", translator.DirRequest},
		{"openai", "gemini", translator.DirRequest},
		{"openai", "cursor", translator.DirRequest},
		{"openai", "kiro", translator.DirRequest},
		{"openai", "ollama", translator.DirRequest},
		{"openai", "vertex", translator.DirRequest},
		{"openai", "commandcode", translator.DirRequest},
		{"antigravity", "openai", translator.DirRequest},
		{"openai-responses", "openai", translator.DirRequest},

		// Response pairs (9 — incl. extra openai→gemini alias)
		{"claude", "openai", translator.DirResponse},
		{"gemini", "openai", translator.DirResponse},
		{"openai", "claude", translator.DirResponse},
		{"openai", "antigravity", translator.DirResponse},
		{"openai", "gemini", translator.DirResponse},
		{"cursor", "openai", translator.DirResponse},
		{"kiro", "openai", translator.DirResponse},
		{"ollama", "openai", translator.DirResponse},
		{"commandcode", "openai", translator.DirResponse},
		{"openai", "openai-responses", translator.DirResponse},
	}
	for _, c := range cases {
		if translator.Get(c.From, c.To, c.Dir) == nil {
			t.Fatalf("missing translator %s→%s (%s)", c.From, c.To, c.Dir)
		}
	}
}

func TestClaudeToOpenAI_Request(t *testing.T) {
	fn := translator.Get("claude", "openai", translator.DirRequest)
	out := fn(map[string]any{
		"model":  "claude-haiku-4-5",
		"system": "be helpful",
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	})
	msgs := out["messages"].([]map[string]any)
	if len(msgs) != 2 || msgs[0]["role"] != "system" || msgs[1]["content"] != "hi" {
		t.Fatalf("shape: %v", out)
	}
}

func TestOpenAIToGemini_Request(t *testing.T) {
	fn := translator.Get("openai", "gemini", translator.DirRequest)
	out := fn(map[string]any{
		"messages": []any{
			map[string]any{"role": "system", "content": "rule"},
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{"role": "assistant", "content": "bye"},
		},
	})
	contents := out["contents"].([]map[string]any)
	if len(contents) != 2 {
		t.Fatalf("contents len: %d", len(contents))
	}
	if contents[1]["role"] != "model" {
		t.Fatalf("assistant→model: %v", contents[1])
	}
	sysObj, _ := out["systemInstruction"].(map[string]any)
	if sysObj == nil {
		t.Fatal("missing systemInstruction")
	}
}

func TestClaudeToOpenAI_Response(t *testing.T) {
	fn := translator.Get("claude", "openai", translator.DirResponse)
	out := fn(map[string]any{
		"id":          "msg_1",
		"model":       "claude-haiku-4-5",
		"stop_reason": "max_tokens",
		"content":     []any{map[string]any{"type": "text", "text": "hello"}},
		"usage":       map[string]any{"input_tokens": float64(10), "output_tokens": float64(5)},
	})
	choices := out["choices"].([]map[string]any)
	msg := choices[0]["message"].(map[string]any)
	if msg["content"] != "hello" || choices[0]["finish_reason"] != "length" {
		t.Fatalf("shape: %v", out)
	}
	usage := out["usage"].(map[string]any)
	if usage["total_tokens"].(int64) != 15 {
		t.Fatalf("usage: %v", usage)
	}
}
