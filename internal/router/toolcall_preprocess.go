// Pre-dispatch normalisation: sanitise tool-call ids and patch missing
// tool_result follow-ups. Operates on the request RIGHT before fan-out so
// every downstream translator sees a well-formed payload.

package router

import (
	"encoding/json"

	"github.com/flowork-os/flowork_Router/internal/translator/helpers"
)

// preprocessToolCalls round-trips req.Messages through the generic
// helpers.EnsureToolCallIDs + FixMissingToolResponses pipeline. Only fires
// when the request looks like it has tool plumbing — pure text requests
// skip the marshal/unmarshal pair so the hot path stays cheap.
func preprocessToolCalls(req *OpenAIRequest) {
	if !requestLooksToolful(req) {
		return
	}
	// Encode/decode just the messages array via a small wrapper map so we
	// can hand it to the format-agnostic helpers.
	body := map[string]any{}
	raw, err := json.Marshal(req)
	if err != nil {
		return
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return
	}
	helpers.EnsureToolCallIDs(body)
	helpers.FixMissingToolResponses(body)

	// Lift mutations back into the typed struct.
	patched, err := json.Marshal(body)
	if err != nil {
		return
	}
	_ = json.Unmarshal(patched, req)
}

// requestLooksToolful is a cheap gate: only requests carrying tool plumbing
// need the validation pass. Skips the round-trip for plain chat.
func requestLooksToolful(req *OpenAIRequest) bool {
	if len(req.Tools) > 2 { // more than "[]" or "{}"
		return true
	}
	for _, m := range req.Messages {
		if len(m.ToolCalls) > 2 || m.ToolCallID != "" {
			return true
		}
		// Multi-part content (Claude shape) often carries tool_use/tool_result.
		// Cheap check: Content begins with '[' after trim.
		c := m.Content
		for i := 0; i < len(c); i++ {
			if c[i] == ' ' || c[i] == '\t' || c[i] == '\n' {
				continue
			}
			if c[i] == '[' || c[i] == '{' {
				return true
			}
			break
		}
	}
	return false
}
