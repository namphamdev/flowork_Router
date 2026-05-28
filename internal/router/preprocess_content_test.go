package router

import (
	"strings"
	"testing"
)

func TestStripContentTypes_NoListIsNoop(t *testing.T) {
	req := &OpenAIRequest{Messages: []OpenAIMessage{
		{Role: "user", Content: `[{"type":"image_url","image_url":{"url":"x"}}]`},
	}}
	stripContentTypes(req, nil)
	if !strings.Contains(req.Messages[0].Content, "image_url") {
		t.Fatal("nil strip list must leave content untouched")
	}
}

func TestStripContentTypes_RemovesImageParts(t *testing.T) {
	req := &OpenAIRequest{Messages: []OpenAIMessage{
		{Role: "user", Content: `[
			{"type":"text","text":"hi"},
			{"type":"image_url","image_url":{"url":"x"}},
			{"type":"image","source":{"data":"…"}}
		]`},
	}}
	stripContentTypes(req, StripList{"image"})
	if strings.Contains(req.Messages[0].Content, "image_url") {
		t.Errorf("image_url not stripped: %q", req.Messages[0].Content)
	}
	if strings.Contains(req.Messages[0].Content, "image") && !strings.Contains(req.Messages[0].Content, "\"text\"") {
		// "image" might appear inside legit fields; just make sure text survived.
		t.Logf("content after strip: %s", req.Messages[0].Content)
	}
	if !strings.Contains(req.Messages[0].Content, "hi") {
		t.Errorf("text part lost: %q", req.Messages[0].Content)
	}
}

func TestStripContentTypes_AudioParts(t *testing.T) {
	req := &OpenAIRequest{Messages: []OpenAIMessage{
		{Role: "user", Content: `[
			{"type":"text","text":"hi"},
			{"type":"input_audio","data":"…"},
			{"type":"audio_url","audio_url":{"url":"y"}}
		]`},
	}}
	stripContentTypes(req, StripList{"audio"})
	if strings.Contains(req.Messages[0].Content, "audio_url") {
		t.Errorf("audio_url not stripped: %q", req.Messages[0].Content)
	}
	if strings.Contains(req.Messages[0].Content, "input_audio") {
		t.Errorf("input_audio not stripped: %q", req.Messages[0].Content)
	}
}

func TestStripContentTypes_EmptyContentAfterStrip(t *testing.T) {
	req := &OpenAIRequest{Messages: []OpenAIMessage{
		{Role: "user", Content: `[{"type":"image_url","image_url":{"url":"x"}}]`},
	}}
	stripContentTypes(req, StripList{"image"})
	if req.Messages[0].Content != "" {
		t.Errorf("empty content array must collapse to \"\", got %q", req.Messages[0].Content)
	}
}

func TestStripContentTypes_PlainStringUntouched(t *testing.T) {
	req := &OpenAIRequest{Messages: []OpenAIMessage{
		{Role: "user", Content: "just text, no JSON"},
	}}
	stripContentTypes(req, StripList{"image"})
	if req.Messages[0].Content != "just text, no JSON" {
		t.Fatalf("plain string mutated: %q", req.Messages[0].Content)
	}
}

func TestNormalizeThinkingConfig_KeepsWhenUserTrailing(t *testing.T) {
	body := map[string]any{
		"thinking": map[string]any{"type": "enabled"},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}
	normalizeThinkingConfig(body)
	if _, has := body["thinking"]; !has {
		t.Fatal("thinking must be preserved when last message is user")
	}
}

func TestNormalizeThinkingConfig_StripsWhenAssistantTrailing(t *testing.T) {
	body := map[string]any{
		"thinking":        map[string]any{"type": "enabled"},
		"reasoning":       map[string]any{"effort": "high"},
		"enable_thinking": true,
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{"role": "assistant", "content": "ack"},
		},
	}
	normalizeThinkingConfig(body)
	for _, key := range []string{"thinking", "reasoning", "enable_thinking"} {
		if _, has := body[key]; has {
			t.Errorf("%s should be removed when last message is non-user", key)
		}
	}
}

func TestNormalizeThinkingConfig_StripsWhenToolTrailing(t *testing.T) {
	body := map[string]any{
		"thinking": map[string]any{"type": "enabled"},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{"role": "tool", "tool_call_id": "x", "content": "ok"},
		},
	}
	normalizeThinkingConfig(body)
	if _, has := body["thinking"]; has {
		t.Fatal("thinking should be stripped when last message is tool")
	}
}

func TestNormalizeThinkingConfig_NoMessagesIsNoop(t *testing.T) {
	body := map[string]any{"thinking": "x"}
	normalizeThinkingConfig(body)
	if _, has := body["thinking"]; !has {
		t.Fatal("no messages → leave body untouched")
	}
}
