// Pre-dispatch content-shape normalisation. Two passes:
//
//  - stripContentTypes: removes image / audio parts from multi-part message
//    content when the target provider doesn't support them. Cheap text path
//    is left untouched.
//
//  - normalizeThinkingConfig: drops `thinking` knobs when the trailing
//    message isn't user-role. Anthropic rejects thinking-mode requests that
//    end with assistant or tool messages, so we strip the config to keep
//    the request acceptable.

package router

import (
	"encoding/json"
	"strings"
)

// StripList enumerates content categories the caller wants pruned out of
// each message. Recognised tokens:
//
//	"image"  → drops image_url / image parts
//	"audio"  → drops audio_url / input_audio parts
type StripList []string

// stripContentTypes walks req.Messages and removes any content part whose
// type falls under one of the listed strip categories. Plain string content
// stays untouched. When an entire content array is emptied we leave a "" so
// the message slot survives — providers that require ≥1 content part still
// see a well-formed (if empty) entry.
func stripContentTypes(req *OpenAIRequest, list StripList) {
	if len(list) == 0 {
		return
	}
	strip := map[string]bool{}
	for _, c := range list {
		strip[strings.ToLower(strings.TrimSpace(c))] = true
	}
	for i, m := range req.Messages {
		// Content is stored as a string; the multi-part shape arrives as a
		// JSON-encoded array. Cheap gate: only round-trip when content
		// looks like an array.
		raw := strings.TrimSpace(m.Content)
		if !strings.HasPrefix(raw, "[") {
			continue
		}
		var parts []map[string]any
		if err := json.Unmarshal([]byte(raw), &parts); err != nil {
			continue
		}
		kept := parts[:0]
		for _, p := range parts {
			typ, _ := p["type"].(string)
			t := strings.ToLower(typ)
			drop := false
			switch t {
			case "image_url", "image":
				drop = strip["image"]
			case "audio_url", "input_audio":
				drop = strip["audio"]
			}
			if !drop {
				kept = append(kept, p)
			}
		}
		if len(kept) == 0 {
			req.Messages[i].Content = ""
			continue
		}
		raw2, err := json.Marshal(kept)
		if err != nil {
			continue
		}
		req.Messages[i].Content = string(raw2)
	}
}

// normalizeThinkingConfig operates on the raw decoded body so it can run in
// the same round-trip pass as the tool-call validators. When the trailing
// message isn't user-role the thinking knobs are removed — Anthropic rejects
// thinking-mode requests that end with assistant or tool messages.
func normalizeThinkingConfig(body map[string]any) {
	msgs, ok := body["messages"].([]any)
	if !ok || len(msgs) == 0 {
		return
	}
	last, _ := msgs[len(msgs)-1].(map[string]any)
	role, _ := last["role"].(string)
	if role == "user" {
		return // user-trailing requests are fine — keep thinking config
	}
	// Drop every known thinking knob.
	for _, key := range []string{"thinking", "reasoning", "enable_thinking"} {
		delete(body, key)
	}
}
