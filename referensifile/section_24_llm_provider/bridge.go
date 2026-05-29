// Bridge provider — talks to the MR.FLOW Bridge VS Code extension at
// http://127.0.0.1:3001 (configurable). The extension proxies requests to
// VS Code's vscode.lm API, which exposes Claude / GPT / Copilot models
// without requiring API keys — the user's existing IDE subscription pays.
//
// This lets FLOWORK CLI / watcher / mesh use the same models VS Code uses,
// bypassing AntiGravity entirely.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// BridgeClient implements provider.Client against the MR.FLOW Bridge HTTP API.
type BridgeClient struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewBridgeClient creates a client targeting the bridge at baseURL
// (e.g. "http://127.0.0.1:3001"). The model string is matched against
// id/name/family fields by the extension — partial matches OK.
func NewBridgeClient(baseURL, model string) *BridgeClient {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:3001"
	}
	return &BridgeClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  llmHTTPClient(),
	}
}

// Name returns the provider identifier.
func (c *BridgeClient) Name() string { return "bridge" }

// bridgeChatRequest matches the JSON body the VS Code extension expects.
type bridgeChatRequest struct {
	Model    string              `json:"model,omitempty"`
	Messages []bridgeChatMessage `json:"messages"`
	System   string              `json:"system,omitempty"`
	Stream   bool                `json:"stream"`
}

type bridgeChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type bridgeChatResponse struct {
	OK       bool   `json:"ok"`
	Model    string `json:"model"`
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// Complete sends a non-streaming request and returns the full response.
//
// The bridge currently has no native tool-call support — vscode.lm exposes
// only text in/out. Tool definitions in the request are ignored; callers
// that need tool use should fall back to a real provider (anthropic/openai).
func (c *BridgeClient) Complete(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	// Split system messages out — the bridge takes them as a separate field.
	var system string
	msgs := make([]bridgeChatMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == RoleSystem {
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
			continue
		}
		// vscode.lm only knows user/assistant. Fold tool results into user.
		role := string(m.Role)
		if role == string(RoleTool) {
			role = "user"
		}
		content := m.Content
		if m.Role == RoleTool && content != "" {
			content = "[tool result " + m.Name + "]\n" + content
		}
		msgs = append(msgs, bridgeChatMessage{Role: role, Content: content})
	}

	body := bridgeChatRequest{
		Model:    model,
		Messages: msgs,
		System:   system,
		Stream:   false,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat", bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("bridge request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return Response{}, fmt.Errorf("read bridge response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("bridge returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	var br bridgeChatResponse
	if err := json.Unmarshal(raw, &br); err != nil {
		return Response{}, fmt.Errorf("decode bridge response: %w", err)
	}
	if br.Error != "" {
		return Response{}, fmt.Errorf("bridge error: %s", br.Error)
	}

	// vscode.lm doesn't expose token counts — leave Usage zero.
	return Response{
		Message:    Message{Role: RoleAssistant, Content: br.Response},
		StopReason: StopReasonEndTurn,
	}, nil
}
