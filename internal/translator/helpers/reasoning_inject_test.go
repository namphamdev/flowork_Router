// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package helpers

import "testing"

func TestInjectReasoningContent_DeepSeekProvider_AllAssistant(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{"role": "assistant", "content": "hello"},
			map[string]any{"role": "user", "content": "more"},
			map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "c"}}},
		},
	}
	InjectReasoningContent("deepseek", "deepseek-chat", body)
	msgs := body["messages"].([]any)
	if rc, _ := msgs[1].(map[string]any)["reasoning_content"].(string); rc == "" {
		t.Errorf("scope=all should inject on assistant text message: %+v", msgs[1])
	}
	if rc, _ := msgs[3].(map[string]any)["reasoning_content"].(string); rc == "" {
		t.Errorf("scope=all should also inject on assistant with tool_calls")
	}
}

func TestInjectReasoningContent_KimiModelPrefix_ToolCallsOnly(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{"role": "assistant", "content": "no tools here"},
			map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "c"}}},
		},
	}
	InjectReasoningContent("anything", "kimi-coding-1.5", body)
	msgs := body["messages"].([]any)
	if _, has := msgs[0].(map[string]any)["reasoning_content"]; has {
		t.Errorf("scope=toolCalls must NOT touch plain assistant text")
	}
	if rc, _ := msgs[1].(map[string]any)["reasoning_content"].(string); rc == "" {
		t.Errorf("scope=toolCalls should inject on tool-call assistant: %+v", msgs[1])
	}
}

func TestInjectReasoningContent_PreservesCallerSuppliedRC(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{"role": "assistant", "content": "x", "reasoning_content": "thought already"},
		},
	}
	InjectReasoningContent("deepseek", "deepseek-chat", body)
	msgs := body["messages"].([]any)
	rc, _ := msgs[0].(map[string]any)["reasoning_content"].(string)
	if rc != "thought already" {
		t.Fatalf("must not overwrite existing reasoning_content, got %q", rc)
	}
}

func TestInjectReasoningContent_UnknownProviderModelNoop(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{"role": "assistant", "content": "x"},
		},
	}
	InjectReasoningContent("openai", "gpt-4o", body)
	if _, has := body["messages"].([]any)[0].(map[string]any)["reasoning_content"]; has {
		t.Fatal("unknown provider must be a no-op")
	}
}

func TestInjectReasoningContent_NoMessagesArrayIsSafe(t *testing.T) {
	body := map[string]any{"model": "deepseek-chat"}
	InjectReasoningContent("deepseek", "deepseek-chat", body)
	// no panic = pass
}

func TestApplyDeepSeekV4ProAlias_MaxMapsToEnabled(t *testing.T) {
	body := map[string]any{"model": "deepseek-v4-pro-max"}
	ApplyDeepSeekV4ProAlias("deepseek", body)
	if body["model"] != "deepseek-v4-pro" {
		t.Errorf("model not rewritten: %v", body["model"])
	}
	extra := body["extra_body"].(map[string]any)
	thinking := extra["thinking"].(map[string]any)
	if thinking["type"] != "enabled" {
		t.Errorf("thinking.type wrong: %v", thinking["type"])
	}
	if body["reasoning_effort"] != "max" {
		t.Errorf("reasoning_effort: %v", body["reasoning_effort"])
	}
}

func TestApplyDeepSeekV4ProAlias_NoneMapsToDisabled(t *testing.T) {
	body := map[string]any{
		"model":            "deepseek-v4-pro-none",
		"reasoning_effort": "high",
	}
	ApplyDeepSeekV4ProAlias("deepseek", body)
	if body["model"] != "deepseek-v4-pro" {
		t.Errorf("model not rewritten: %v", body["model"])
	}
	extra := body["extra_body"].(map[string]any)
	thinking := extra["thinking"].(map[string]any)
	if thinking["type"] != "disabled" {
		t.Errorf("thinking.type should be disabled: %v", thinking["type"])
	}
	if _, has := body["reasoning_effort"]; has {
		t.Errorf("reasoning_effort should be stripped for -none alias")
	}
}

func TestApplyDeepSeekV4ProAlias_NonMatchingPassthrough(t *testing.T) {
	body := map[string]any{"model": "deepseek-chat"}
	ApplyDeepSeekV4ProAlias("deepseek", body)
	if body["model"] != "deepseek-chat" {
		t.Fatal("non-alias model must not be rewritten")
	}
}

func TestApplyDeepSeekV4ProAlias_NonDeepSeekProviderNoop(t *testing.T) {
	body := map[string]any{"model": "deepseek-v4-pro-max"}
	ApplyDeepSeekV4ProAlias("openai", body)
	if body["model"] != "deepseek-v4-pro-max" {
		t.Fatal("non-deepseek provider must skip rewriting")
	}
}
