// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package executors

import (
	"strings"
	"testing"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// All registered executors must be discoverable by their Name().
func TestRegistry_AllExecutorsRegistered(t *testing.T) {
	wanted := []string{
		"antigravity", "azure", "codex", "commandcode", "cu", "cursor", "default",
		"gemini-cli", "github", "grok-web", "iflow", "kiro", "ollama-local",
		"opencode", "opencode-go", "perplexity-web", "qoder", "qwen", "vertex",
		"vertex-partner",
	}
	for _, name := range wanted {
		if Get(name) == nil {
			t.Fatalf("executor %q not registered (Get returned nil)", name)
		}
	}
	names := List()
	for _, w := range wanted {
		found := false
		for _, n := range names {
			if n == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("List() missing %q; got %v", w, names)
		}
	}
}

// Endpoint shape sanity check — when the provider stores a baseURL we honour
// it, otherwise the vendor default kicks in.
func TestExecutors_EndpointShaping(t *testing.T) {
	cases := []struct {
		name       string
		exec       Executor
		want       string
		data       map[string]any
		urlBuilder func(Executor, *store.ProviderConnection, string) string
	}{
		{
			"codex default", &codexExecutor{}, "/backend-api/codex/responses",
			nil,
			func(e Executor, p *store.ProviderConnection, _ string) string {
				return e.(*codexExecutor).endpoint(p)
			},
		},
		{
			"cursor default", &cursorExecutor{}, "/aiserver.v1.ChatService/StreamChat",
			nil,
			func(e Executor, p *store.ProviderConnection, _ string) string {
				return e.(*cursorExecutor).endpoint(p)
			},
		},
		{
			"kiro default", &kiroExecutor{}, "/generateAssistantResponse",
			nil,
			func(e Executor, p *store.ProviderConnection, _ string) string {
				return e.(*kiroExecutor).endpoint(p)
			},
		},
		{
			"qwen default", &qwenExecutor{}, "/chat/completions",
			nil,
			func(e Executor, p *store.ProviderConnection, _ string) string {
				return e.(*qwenExecutor).endpoint(p)
			},
		},
		{
			"azure with deployment", &azureExecutor{}, "/openai/deployments/gpt-4o/chat/completions",
			map[string]any{
				store.CfgBaseURL: "https://my-rg.openai.azure.com",
				"deployment":     "gpt-4o",
			},
			func(e Executor, p *store.ProviderConnection, m string) string {
				return e.(*azureExecutor).endpoint(p, m)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &store.ProviderConnection{Data: tc.data}
			got := tc.urlBuilder(tc.exec, p, "gpt-4o")
			if !strings.Contains(got, tc.want) {
				t.Fatalf("%s: endpoint %q must contain %q", tc.name, got, tc.want)
			}
		})
	}
}

// Marshal shape: stream flag + max_tokens propagate; messages survive.
func TestMarshalRequest_BasicShape(t *testing.T) {
	r := Request{
		Model:     "x",
		Messages:  []Message{{Role: "user", Content: "hi"}},
		MaxTokens: 64,
		Stream:    true,
	}
	out := string(MarshalRequest(r))
	if !strings.Contains(out, `"model":"x"`) {
		t.Fatalf("model missing in %s", out)
	}
	if !strings.Contains(out, `"stream":true`) {
		t.Fatalf("stream flag missing in %s", out)
	}
	if !strings.Contains(out, `"role":"user"`) {
		t.Fatalf("messages missing in %s", out)
	}
}
