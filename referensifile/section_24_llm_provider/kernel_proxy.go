// Package provider — kernel_proxy.go: Sprint 3.5e (BUG-C8/W14 fix).
//
// KernelProxyClient implement Client interface dengan route via kernel
// /v1/chat instead of LLM provider direct. Daemon yang pakai
// `provider.NewOpenAIClient` bisa swap ke `provider.NewKernelProxyClient`
// tanpa code change selain di buildClient().
//
// Tradeoff: tools[] di Request ngga full-fidelity translate (kernel /v1/chat
// endpoint internal handle tool dispatch via warga.Process) — caller daemon
// yang butuh tool round-trip wajib pakai warga_id flow di kernel, bukan
// inject tools[] di request body.
//
// Visi alignment (Ayah 2026-05-02): brain mature → Phase 6 → semua daemon
// flip ke kernel funnel. Sebelum brain dewasa: KERNEL_URL env toggle.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// KernelProxyClient — implement Client via kernel /v1/chat HTTP forward.
type KernelProxyClient struct {
	kernelURL string // base URL (e.g. http://localhost:3105)
	token     string // FLOWORK_KERNEL_TOKEN bearer
	wargaID   string // warga_id dipakai di every request (caller-supplied)
	model     string // optional override
	httpCl    *http.Client
}

// KernelProxyConfig — config struct untuk constructor.
type KernelProxyConfig struct {
	KernelURL string
	Token     string
	WargaID   string
	Model     string // optional default
	Timeout   time.Duration
}

// NewKernelProxyClient — return Client yang route via kernel /v1/chat.
//
// Required: KernelURL + WargaID. Token optional (fallback ke FLOWORK_KERNEL_TOKEN
// env / state file di kernel client itself).
func NewKernelProxyClient(cfg KernelProxyConfig) (*KernelProxyClient, error) {
	if cfg.KernelURL == "" {
		return nil, fmt.Errorf("KernelURL required")
	}
	if cfg.WargaID == "" {
		return nil, fmt.Errorf("WargaID required (kernel reject empty per BUG-120)")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	return &KernelProxyClient{
		kernelURL: strings.TrimRight(cfg.KernelURL, "/"),
		token:     cfg.Token,
		wargaID:   cfg.WargaID,
		model:     cfg.Model,
		httpCl:    &http.Client{Timeout: cfg.Timeout},
	}, nil
}

// Name — identifier untuk failover/monitor.
func (k *KernelProxyClient) Name() string {
	return "kernel-proxy"
}

// Complete — implement Client.Complete via kernel /v1/chat HTTP forward.
//
// Translate strategi:
//   - Concat semua user messages ke single `message` field (kernel wargaProcess
//     load history dari warga state, bukan dari request).
//   - System message preserved jika first message role=system (di-prepend ke
//     user message dengan separator).
//   - Tools[] DROPPED — kernel handle tool routing via warga capability gate.
//     Kalau caller butuh tool round-trip, pakai warga_id flow yang full
//     orchestrator di kernel.
func (k *KernelProxyClient) Complete(ctx context.Context, req Request) (Response, error) {
	// Build single user message dari Messages[].
	var userParts []string
	var systemPart string
	for _, m := range req.Messages {
		if m.Role == RoleSystem && systemPart == "" {
			systemPart = m.Content
			continue
		}
		if m.Role == RoleUser || m.Role == RoleAssistant {
			if m.Content != "" {
				userParts = append(userParts, fmt.Sprintf("%s: %s", m.Role, m.Content))
			}
		}
	}
	combinedMessage := strings.Join(userParts, "\n\n")
	if systemPart != "" {
		combinedMessage = "[system]\n" + systemPart + "\n\n[conversation]\n" + combinedMessage
	}

	model := k.model
	if req.Model != "" {
		model = req.Model
	}

	body, err := json.Marshal(map[string]any{
		"message":    combinedMessage,
		"warga_id":   k.wargaID,
		"model":      model,
		"max_tokens": req.MaxTokens,
	})
	if err != nil {
		return Response{}, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		k.kernelURL+"/v1/chat", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "flowork-provider/kernel-proxy")
	if k.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+k.token)
	}

	resp, err := k.httpCl.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("kernel call: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if resp.StatusCode/100 != 2 {
		return Response{}, fmt.Errorf("kernel %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed struct {
		Content string `json:"content"`
		Usage   struct {
			InputTokens  int `json:"prompt_tokens"`
			OutputTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error  string `json:"error,omitempty"`
		Reason string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Response{}, fmt.Errorf("decode: %w", err)
	}
	if parsed.Error != "" {
		return Response{}, fmt.Errorf("kernel: %s (%s)", parsed.Error, parsed.Reason)
	}

	return Response{
		Message: Message{
			Role:    RoleAssistant,
			Content: parsed.Content,
		},
		StopReason: StopReasonEndTurn,
		Usage: Usage{
			InputTokens:  parsed.Usage.InputTokens,
			OutputTokens: parsed.Usage.OutputTokens,
		},
	}, nil
}
