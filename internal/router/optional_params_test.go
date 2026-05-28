package router

import (
	"encoding/json"
	"strings"
	"testing"
)

// Optional-params round-trip: decode a rich JSON body, re-encode, and confirm
// every OpenAI-spec optional field reappears with the same value (or is
// omitted when absent in the source).

func TestOpenAIRequest_PreservesAllOptionalParams(t *testing.T) {
	src := `{
		"model":"x",
		"messages":[{"role":"user","content":"hi"}],
		"max_tokens":100,
		"max_completion_tokens":200,
		"temperature":0.7,
		"top_p":0.9,
		"top_k":40,
		"thinking":{"type":"enabled","budget_tokens":1000},
		"reasoning":{"effort":"high"},
		"enable_thinking":true,
		"presence_penalty":0.5,
		"frequency_penalty":0.2,
		"seed":12345,
		"stop":["END","STOP"],
		"response_format":{"type":"json_object"},
		"prediction":{"type":"content","content":"hint"},
		"store":true,
		"metadata":{"tag":"user-x"},
		"n":2,
		"logprobs":true,
		"top_logprobs":5,
		"logit_bias":{"123":-100},
		"user":"u-42",
		"parallel_tool_calls":false,
		"stream":false
	}`
	var req OpenAIRequest
	if err := json.Unmarshal([]byte(src), &req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.MaxCompletionTok != 200 {
		t.Errorf("max_completion_tokens lost: %d", req.MaxCompletionTok)
	}
	if req.TopK != 40 {
		t.Errorf("top_k lost: %d", req.TopK)
	}
	if req.PresencePenalty != 0.5 {
		t.Errorf("presence_penalty lost: %f", req.PresencePenalty)
	}
	if req.Seed == nil || *req.Seed != 12345 {
		t.Errorf("seed lost: %v", req.Seed)
	}
	if req.N != 2 {
		t.Errorf("n lost: %d", req.N)
	}
	if req.User != "u-42" {
		t.Errorf("user lost: %s", req.User)
	}
	if req.Store == nil || *req.Store != true {
		t.Errorf("store lost: %v", req.Store)
	}
	if req.Logprobs == nil || *req.Logprobs != true {
		t.Errorf("logprobs lost: %v", req.Logprobs)
	}
	if req.TopLogprobs == nil || *req.TopLogprobs != 5 {
		t.Errorf("top_logprobs lost: %v", req.TopLogprobs)
	}
	if req.ParallelToolCalls == nil || *req.ParallelToolCalls != false {
		t.Errorf("parallel_tool_calls lost: %v", req.ParallelToolCalls)
	}
	if req.EnableThinking == nil || *req.EnableThinking != true {
		t.Errorf("enable_thinking lost: %v", req.EnableThinking)
	}

	// Re-encode and confirm every field round-trips.
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	out := string(raw)
	for _, want := range []string{
		`"max_completion_tokens":200`,
		`"top_k":40`,
		`"thinking":{"type":"enabled","budget_tokens":1000}`,
		`"reasoning":{"effort":"high"}`,
		`"enable_thinking":true`,
		`"presence_penalty":0.5`,
		`"frequency_penalty":0.2`,
		`"seed":12345`,
		`"stop":["END","STOP"]`,
		`"response_format":{"type":"json_object"}`,
		`"prediction":{"type":"content","content":"hint"}`,
		`"store":true`,
		`"metadata":{"tag":"user-x"}`,
		`"n":2`,
		`"logprobs":true`,
		`"top_logprobs":5`,
		`"logit_bias":{"123":-100}`,
		`"user":"u-42"`,
		`"parallel_tool_calls":false`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("round-trip lost field: %s\nfull body: %s", want, out)
		}
	}
}

func TestOpenAIRequest_OmitsZeroValuesByDefault(t *testing.T) {
	req := OpenAIRequest{Model: "x", Messages: []OpenAIMessage{{Role: "user", Content: "hi"}}}
	raw, _ := json.Marshal(req)
	out := string(raw)
	// None of the optional params should appear when unset.
	for _, banned := range []string{
		"top_k", "max_completion_tokens", "thinking", "reasoning",
		"enable_thinking", "presence_penalty", "frequency_penalty",
		"seed", "stop", "response_format", "prediction", "store",
		"metadata", "logprobs", "top_logprobs", "logit_bias", "user",
		"parallel_tool_calls",
	} {
		// Check JSON KEY form ("name":) not the bare string — Messages
		// carry "role":"user" which would false-positive on "user".
		if strings.Contains(out, `"`+banned+`":`) {
			t.Errorf("zero-value request leaked field %q: %s", banned, out)
		}
	}
}

func TestOpenAIRequest_NillablePointersRespectFalse(t *testing.T) {
	// `store:false` must round-trip as `store:false` (omitempty would normally
	// strip the false zero — using a *bool keeps the explicit choice).
	src := `{"model":"x","store":false,"logprobs":false,"parallel_tool_calls":false}`
	var req OpenAIRequest
	if err := json.Unmarshal([]byte(src), &req); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(req)
	out := string(raw)
	for _, want := range []string{`"store":false`, `"logprobs":false`, `"parallel_tool_calls":false`} {
		if !strings.Contains(out, want) {
			t.Errorf("explicit false dropped: %s -> %s", want, out)
		}
	}
}
