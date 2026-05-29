// Ollama provider — talks to a local Ollama server over HTTP
// (https://github.com/ollama/ollama/blob/main/docs/api.md).
//
// This is FLOWORK's offline path: when cloud providers are unreachable
// (no internet, rate-limited, quota exhausted), the agent can keep thinking
// using whatever model Ollama has pulled locally.
//
// Tool / function-calling is not plumbed here — Ollama does support it for
// a subset of models, but the surface differs by model and most small
// coding models don't use it reliably. Callers that need tool-use should
// either pick a "real" provider (anthropic/openai) or wrap this client
// with a tool-loop above the agent layer.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/teetah2402/flowork/internal/safeclient"
)

// OllamaConfig configures the Ollama client.
type OllamaConfig struct {
	// BaseURL — Ollama HTTP endpoint. Default: $OLLAMA_HOST or http://localhost:11434.
	BaseURL string
	// Model — specific model to use. Empty = auto-pick the best installed one
	// (prefers coding-tuned models; see pickBestModel).
	Model string
	// Timeout — per-request ceiling. Default 120s (local inference is slow).
	Timeout time.Duration
	// NumCtx — context window size passed to Ollama. Default 4096.
	NumCtx int
}

// OllamaClient implements provider.Client against a local Ollama server.
type OllamaClient struct {
	cfg  OllamaConfig
	http *http.Client
	// resolvedModel caches the chosen model after the first call so we
	// don't re-list models on every Complete.
	resolvedModel string
}

// NewOllamaClient creates a new Ollama client with sensible defaults.
func NewOllamaClient(cfg OllamaConfig) *OllamaClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = os.Getenv("OLLAMA_HOST")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	if cfg.NumCtx <= 0 {
		cfg.NumCtx = 4096
	}
	// EXTBUG-026 fix: use safeclient so a hijacked $OLLAMA_HOST pointing at
	// 169.254.169.254 or other internal ranges is refused at the dialer
	// rather than silently forwarding the LLM request (and any future auth
	// headers) to a metadata endpoint.
	return &OllamaClient{
		cfg:  cfg,
		http: safeclient.NewClient(cfg.Timeout),
	}
}

// Name returns the provider identifier.
func (c *OllamaClient) Name() string { return "ollama" }

// IsAvailable pings /api/tags with a short deadline. Used by the fallback
// decorator to decide whether to skip this client.
func (c *OllamaClient) IsAvailable(ctx context.Context) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, c.cfg.BaseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Complete implements provider.Client. It flattens system/user/assistant/tool
// messages into Ollama's simpler chat format and returns the assistant reply.
// Tool calls are not issued — Ollama's tool surface varies by model and is
// out of scope for the fallback path.
func (c *OllamaClient) Complete(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = c.cfg.Model
	}
	if model == "" {
		picked, err := c.pickBestModel(ctx)
		if err != nil {
			return Response{}, fmt.Errorf("ollama: %w", err)
		}
		model = picked
		c.resolvedModel = picked
	}

	msgs := make([]ollamaMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := string(m.Role)
		content := m.Content
		// Ollama doesn't know "tool" role — fold tool results into user as
		// labeled context so the model still sees them.
		if m.Role == RoleTool {
			role = "user"
			if content != "" && m.Name != "" {
				content = "[tool result " + m.Name + "]\n" + content
			}
		}
		// System/user/assistant pass through as-is.
		msgs = append(msgs, ollamaMessage{Role: role, Content: content})
	}

	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	body := ollamaChatRequest{
		Model:    model,
		Messages: msgs,
		Stream:   false,
		Options: &ollamaOptions{
			Temperature: temperature,
			NumCtx:      c.cfg.NumCtx,
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("ollama: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("ollama: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return Response{}, fmt.Errorf("ollama: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("ollama returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	var chat ollamaChatResponse
	if err := json.Unmarshal(raw, &chat); err != nil {
		return Response{}, fmt.Errorf("ollama: decode: %w", err)
	}

	return Response{
		Message:    Message{Role: RoleAssistant, Content: chat.Message.Content},
		StopReason: StopReasonEndTurn,
		Usage: Usage{
			InputTokens:  chat.PromptEvalCount,
			OutputTokens: chat.EvalCount,
		},
	}, nil
}

// pickBestModel lists installed models and picks the best match from a
// hard-coded preference order. We prefer coding-tuned models; if none are
// installed, we take the first available.
func (c *OllamaClient) pickBestModel(ctx context.Context) (string, error) {
	if c.resolvedModel != "" {
		return c.resolvedModel, nil
	}
	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(listCtx, http.MethodGet, c.cfg.BaseURL+"/api/tags", nil)
	if err != nil {
		return "", fmt.Errorf("build list request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("unreachable at %s: %w", c.cfg.BaseURL, err)
	}
	defer resp.Body.Close()

	var list ollamaListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return "", fmt.Errorf("decode model list: %w", err)
	}
	if len(list.Models) == 0 {
		return "", fmt.Errorf("no models installed; run: ollama pull qwen2.5-coder")
	}

	// Coding-capable first, then general-purpose.
	prefs := []string{
		"qwen2.5-coder", "deepseek-coder", "codellama",
		"llama3.2", "llama3.1", "llama3",
		"mistral", "gemma2", "phi3",
	}
	for _, p := range prefs {
		for _, m := range list.Models {
			if strings.Contains(strings.ToLower(m.Name), p) {
				return m.Name, nil
			}
		}
	}
	return list.Models[0].Name, nil
}

// ─── Ollama wire types (unexported) ────────────────────────────────

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature float64  `json:"temperature,omitempty"`
	NumCtx      int      `json:"num_ctx,omitempty"`
	TopP        float64  `json:"top_p,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

type ollamaChatResponse struct {
	Model           string        `json:"model"`
	CreatedAt       string        `json:"created_at"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}

type ollamaListResponse struct {
	Models []ollamaModelInfo `json:"models"`
}

type ollamaModelInfo struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}
