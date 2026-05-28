// Executor: kiro — Kiro AWS CodeWhisperer wrapper (conversationState shape).
package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/store"
)

func init() { Register(&kiroExecutor{}) }

type kiroExecutor struct{}

func (k *kiroExecutor) Name() string { return "kiro" }

func (k *kiroExecutor) endpoint(p *store.ProviderConnection) string {
	base := ProviderString(p, store.CfgBaseURL)
	if base == "" {
		base = "https://codewhisperer.us-east-1.amazonaws.com"
	}
	return trimRightSlash(base) + "/generateAssistantResponse"
}

func (k *kiroExecutor) headers(p *store.ProviderConnection) map[string]string {
	h := map[string]string{
		"Accept":            "text/event-stream",
		"amz-sdk-invocation-id": "flow-router",
	}
	if tok, ok := p.Data[store.CfgAPIKey].(string); ok && tok != "" {
		h["Authorization"] = "Bearer " + tok
	}
	if pid, ok := p.Data["profileArn"].(string); ok && pid != "" {
		h["x-amzn-codewhisperer-profile-arn"] = pid
	}
	return h
}

// kiroBody translates OpenAI messages to Kiro's conversationState shape: the
// last user message goes into currentMessage, the rest into history.
func kiroBody(r Request) []byte {
	var history []map[string]any
	var current map[string]any
	for i, m := range r.Messages {
		entry := map[string]any{"role": m.Role, "content": []map[string]any{{"type": "text", "text": m.Content}}}
		if i == len(r.Messages)-1 && m.Role == "user" {
			current = entry
			continue
		}
		history = append(history, entry)
	}
	body, _ := json.Marshal(map[string]any{
		"conversationState": map[string]any{
			"chatTriggerType": "MANUAL",
			"history":         history,
			"currentMessage":  current,
			"modelId":         r.Model,
		},
	})
	return body
}

func (k *kiroExecutor) Stream(ctx context.Context, p *store.ProviderConnection, req Request, w http.ResponseWriter, flusher http.Flusher) (Usage, int, error) {
	httpReq, err := BuildRequest(ctx, http.MethodPost, k.endpoint(p), kiroBody(req), k.headers(p))
	if err != nil {
		return Usage{}, 0, fmt.Errorf("build req: %w", err)
	}
	return DoStream(httpReq, w, flusher)
}

func (k *kiroExecutor) NonStream(ctx context.Context, p *store.ProviderConnection, req Request) ([]byte, Usage, int, error) {
	req.Stream = false
	httpReq, err := BuildRequest(ctx, http.MethodPost, k.endpoint(p), kiroBody(req), k.headers(p))
	if err != nil {
		return nil, Usage{}, 0, fmt.Errorf("build req: %w", err)
	}
	return DoNonStream(httpReq)
}
