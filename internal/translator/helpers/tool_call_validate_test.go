package helpers

import (
	"encoding/json"
	"testing"
)

func TestToolIDValid_AcceptsAndRejects(t *testing.T) {
	cases := map[string]bool{
		"":                false,
		"call_abc-123_x":  true,
		"call_ok":         true,
		"call with space": false,
		"call!bang":       false,
		"id/with/slash":   false,
		"abc.123":         false,
		"____":            true,
		"a-b-c-d-1-2-3":   true,
	}
	for in, want := range cases {
		if got := toolIDValid(in); got != want {
			t.Errorf("toolIDValid(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSanitizeToolID(t *testing.T) {
	cases := map[string]string{
		"call_abc":    "call_abc",
		"hello world": "helloworld",
		"id/with/!@#": "idwith",
		"!!@@##":      "",
		"a-b_c-1":     "a-b_c-1",
	}
	for in, want := range cases {
		if got := sanitizeToolID(in); got != want {
			t.Errorf("sanitizeToolID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeterministicToolCallID_Stable(t *testing.T) {
	got1 := deterministicToolCallID(2, 3, "search_files")
	got2 := deterministicToolCallID(2, 3, "search_files")
	if got1 != got2 {
		t.Fatalf("expected stable output, got %q vs %q", got1, got2)
	}
	if got1 != "call_2_3_search_files" {
		t.Errorf("unexpected shape: %q", got1)
	}
}

func TestDeterministicToolCallID_HandlesNoName(t *testing.T) {
	got := deterministicToolCallID(0, 0, "")
	if got != "call_0_0_tool" {
		t.Errorf("expected call_0_0_tool, got %q", got)
	}
}

// Helper: build a JSON body and decode to map[string]any.
func decodeBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEnsureToolCallIDs_SanitizesInvalidOpenAI(t *testing.T) {
	body := decodeBody(t, `{
		"messages":[
			{"role":"assistant","tool_calls":[
				{"id":"call/bad@id","type":"function","function":{"name":"read","arguments":"{}"}}
			]}
		]
	}`)
	EnsureToolCallIDs(body)
	calls := body["messages"].([]any)[0].(map[string]any)["tool_calls"].([]any)
	id := calls[0].(map[string]any)["id"].(string)
	if id != "callbadid" {
		t.Fatalf("sanitised id mismatch: %q", id)
	}
}

func TestEnsureToolCallIDs_RegeneratesWhenEmpty(t *testing.T) {
	body := decodeBody(t, `{
		"messages":[
			{"role":"assistant","tool_calls":[
				{"id":"!!!","type":"function","function":{"name":"do_thing","arguments":"{}"}}
			]}
		]
	}`)
	EnsureToolCallIDs(body)
	calls := body["messages"].([]any)[0].(map[string]any)["tool_calls"].([]any)
	id := calls[0].(map[string]any)["id"].(string)
	if id != "call_0_0_do_thing" {
		t.Fatalf("expected regen, got %q", id)
	}
}

func TestEnsureToolCallIDs_DefaultsTypeAndStringifiesArguments(t *testing.T) {
	body := decodeBody(t, `{
		"messages":[
			{"role":"assistant","tool_calls":[
				{"id":"call_ok","function":{"name":"x","arguments":{"k":1}}}
			]}
		]
	}`)
	EnsureToolCallIDs(body)
	call := body["messages"].([]any)[0].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)
	if call["type"] != "function" {
		t.Errorf("type default missing: %v", call["type"])
	}
	args := call["function"].(map[string]any)["arguments"]
	if _, ok := args.(string); !ok {
		t.Errorf("arguments must be stringified: %T", args)
	}
}

func TestEnsureToolCallIDs_ClaudeToolUseInContent(t *testing.T) {
	body := decodeBody(t, `{
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"bad id","name":"grep","input":{}}
			]}
		]
	}`)
	EnsureToolCallIDs(body)
	block := body["messages"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)
	id := block["id"].(string)
	if id != "badid" {
		t.Fatalf("tool_use id sanitisation: %q", id)
	}
}

func TestEnsureToolCallIDs_ClaudeToolResultBlock(t *testing.T) {
	body := decodeBody(t, `{
		"messages":[
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"!bad","content":"ok"}
			]}
		]
	}`)
	EnsureToolCallIDs(body)
	block := body["messages"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)
	if id, _ := block["tool_use_id"].(string); id == "!bad" {
		t.Fatalf("tool_use_id not fixed: %q", id)
	}
}

func TestEnsureToolCallIDs_ToolMessageIDFixed(t *testing.T) {
	body := decodeBody(t, `{
		"messages":[
			{"role":"tool","tool_call_id":"bad/id","content":"result"}
		]
	}`)
	EnsureToolCallIDs(body)
	msg := body["messages"].([]any)[0].(map[string]any)
	if id, _ := msg["tool_call_id"].(string); id != "badid" {
		t.Fatalf("tool_call_id not fixed: %q", id)
	}
}

func TestFixMissingToolResponses_InsertsStubsAfterAssistant(t *testing.T) {
	body := decodeBody(t, `{
		"messages":[
			{"role":"user","content":"hi"},
			{"role":"assistant","tool_calls":[
				{"id":"call_1","type":"function","function":{"name":"a","arguments":"{}"}},
				{"id":"call_2","type":"function","function":{"name":"b","arguments":"{}"}}
			]},
			{"role":"user","content":"continue"}
		]
	}`)
	FixMissingToolResponses(body)
	msgs := body["messages"].([]any)
	// Expect: user, assistant, tool(call_1), tool(call_2), user = 5
	if len(msgs) != 5 {
		t.Fatalf("expected 5 msgs after fix, got %d", len(msgs))
	}
	if r := msgs[2].(map[string]any)["role"]; r != "tool" {
		t.Errorf("msg[2] should be tool, got %v", r)
	}
	if id := msgs[2].(map[string]any)["tool_call_id"]; id != "call_1" {
		t.Errorf("stub[0] id wrong: %v", id)
	}
	if id := msgs[3].(map[string]any)["tool_call_id"]; id != "call_2" {
		t.Errorf("stub[1] id wrong: %v", id)
	}
}

func TestFixMissingToolResponses_NoopWhenAlreadyComplete(t *testing.T) {
	body := decodeBody(t, `{
		"messages":[
			{"role":"assistant","tool_calls":[
				{"id":"call_x","type":"function","function":{"name":"x","arguments":"{}"}}
			]},
			{"role":"tool","tool_call_id":"call_x","content":"done"}
		]
	}`)
	FixMissingToolResponses(body)
	msgs := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("expected unchanged length, got %d", len(msgs))
	}
}

func TestFixMissingToolResponses_HandlesClaudeContentBlocks(t *testing.T) {
	body := decodeBody(t, `{
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"tu_1","name":"f","input":{}}
			]}
		]
	}`)
	FixMissingToolResponses(body)
	msgs := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("expected stub appended, got %d msgs", len(msgs))
	}
	if id, _ := msgs[1].(map[string]any)["tool_call_id"].(string); id != "tu_1" {
		t.Errorf("stub points to wrong id: %v", id)
	}
}
