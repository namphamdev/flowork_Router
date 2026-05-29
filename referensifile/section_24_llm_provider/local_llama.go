// Local llama.cpp provider — bridges provider.Client interface ke localai.Runtime
// (managed llama-server subprocess). Sesuai roadmap_brain_v3.md Phase 1: kasih
// Flowork akses LLM yang JALAN DI MESIN AYAH, no API call, no internet.
//
// Cara pake (per-warga di DB agents.model):
//
//	model = "local:llama-3.1-8b-instruct-q4_k_m.gguf"   // prefix "local:" → routed sini
//
// Atau di config provider.type = "local-llama". Template inferred dari nama file.
//
// Filosofi: lazy-init runtime per process, single shared subprocess karena
// llama-server boleh handle concurrent /completion calls. Kalau model file
// belum ada di ~/.flowork/models/, return ErrModelMissing — fallback chain
// (FailThreshold-based) akan probe next provider, jadi gak crash boot.
package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/teetah2402/flowork/internal/localai"
)

// LocalLlamaConfig parameter inisialisasi.
type LocalLlamaConfig struct {
	// Model nama file gguf relatif ke ~/.flowork/models/. Contoh:
	// "llama-3.1-8b-instruct-q4_k_m.gguf".
	Model string

	// Template chat template name. Empty → auto-detect dari Model name.
	// Supported: "llama3", "chatml", "gemma", "raw".
	Template string

	// ContextSize override. 0 → 4096 default.
	ContextSize int

	// GPULayers offload count. 0 → CPU only.
	GPULayers int
}

// LocalLlamaClient implementasi provider.Client menggunakan llama.cpp lokal.
// Lazy-init: subprocess di-spawn di first Complete() call, bukan di constructor —
// supaya proses gak crash kalau model file belum ada.
type LocalLlamaClient struct {
	cfg LocalLlamaConfig

	once    sync.Once
	rt      localai.Runtime
	initErr error
}

// ParseLocalModel detect prefix "local:" atau "local/" di model name. Return
// (cleaned_gguf_filename, true) kalau match — false untuk model normal.
//
// Contoh:
//
//	"local:phi-3-mini-q4_k_m.gguf"  → ("phi-3-mini-q4_k_m.gguf", true)
//	"local/llama-3.1-8b.gguf"       → ("llama-3.1-8b.gguf", true)
//	"claude-haiku-4.5"              → ("", false)
func ParseLocalModel(model string) (string, bool) {
	m := strings.TrimSpace(model)
	for _, prefix := range []string{"local:", "local/"} {
		if strings.HasPrefix(strings.ToLower(m), prefix) {
			return strings.TrimSpace(m[len(prefix):]), true
		}
	}
	return "", false
}

// NewLocalLlamaClient bikin client baru. Tidak spawn subprocess — itu lazy.
func NewLocalLlamaClient(cfg LocalLlamaConfig) (*LocalLlamaClient, error) {
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("provider.local-llama: Model required (gguf filename)")
	}
	if cfg.Template == "" {
		cfg.Template = detectChatTemplate(cfg.Model)
	}
	return &LocalLlamaClient{cfg: cfg}, nil
}

// Name identifier — "local-llama:<model>".
func (c *LocalLlamaClient) Name() string {
	return "local-llama:" + c.cfg.Model
}

// Complete render Messages → prompt → call runtime → wrap response.
// Tools field di-ignore: most local 7-13B models gak punya native function-calling
// yang reliable. Caller yang butuh tool use akan dapet response tanpa tool_calls
// dan fallback chain bisa lompat ke provider yang support.
func (c *LocalLlamaClient) Complete(ctx context.Context, req Request) (Response, error) {
	c.once.Do(c.init)
	if c.initErr != nil {
		return Response{}, fmt.Errorf("local-llama: runtime init: %w", c.initErr)
	}

	prompt := renderChatPrompt(c.cfg.Template, req.Messages)
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	laReq := localai.Request{
		Prompt:      prompt,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		TopP:        0.9,
		Stop:        stopSequencesFor(c.cfg.Template),
	}

	resp, err := c.rt.Complete(ctx, laReq)
	if err != nil {
		return Response{}, err
	}

	return Response{
		Message: Message{
			Role:    RoleAssistant,
			Content: strings.TrimSpace(resp.Text),
		},
		StopReason: StopReasonEndTurn,
		Usage: Usage{
			InputTokens:  resp.PromptTokens,
			OutputTokens: resp.CompletionTokens,
		},
	}, nil
}

// Close kill subprocess kalau udah pernah init. Aman dipanggil multi-kali.
func (c *LocalLlamaClient) Close() error {
	if c.rt != nil {
		return c.rt.Close()
	}
	return nil
}

func (c *LocalLlamaClient) init() {
	cfg := localai.Config{
		Backend:     localai.BackendLlamaCpp,
		Model:       c.cfg.Model,
		ContextSize: c.cfg.ContextSize,
		GPULayers:   c.cfg.GPULayers,
	}
	rt, err := localai.Open(cfg)
	if err != nil {
		c.initErr = err
		return
	}
	c.rt = rt
}

// ─── Chat template rendering ───────────────────────────────────────────────

// detectChatTemplate infer template dari nama file gguf. Best-effort —
// pengen explicit boleh set Template manually.
func detectChatTemplate(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "llama-3"), strings.Contains(m, "llama3"):
		return "llama3"
	case strings.Contains(m, "gemma"):
		return "gemma"
	case strings.Contains(m, "qwen"), strings.Contains(m, "deepseek"),
		strings.Contains(m, "phi-3"), strings.Contains(m, "phi3"),
		strings.Contains(m, "mistral"), strings.Contains(m, "yi-"):
		return "chatml"
	default:
		// Aman: ChatML format dimengerti by most modern instruction-tuned models.
		return "chatml"
	}
}

// renderChatPrompt translate unified Messages ke raw prompt string sesuai
// template format. Template formats:
//
//	llama3:  <|start_header_id|>{role}<|end_header_id|>\n\n{content}<|eot_id|>
//	chatml:  <|im_start|>{role}\n{content}<|im_end|>\n
//	gemma:   <start_of_turn>{role}\n{content}<end_of_turn>\n
//	raw:     {role}: {content}\n   (no special tokens — fallback)
func renderChatPrompt(template string, messages []Message) string {
	var sb strings.Builder

	switch template {
	case "llama3":
		sb.WriteString("<|begin_of_text|>")
		for _, m := range messages {
			role := mapRoleToString(m.Role, "llama3")
			content := messageContent(m)
			sb.WriteString("<|start_header_id|>")
			sb.WriteString(role)
			sb.WriteString("<|end_header_id|>\n\n")
			sb.WriteString(content)
			sb.WriteString("<|eot_id|>")
		}
		// Cue model untuk generate assistant turn.
		sb.WriteString("<|start_header_id|>assistant<|end_header_id|>\n\n")

	case "gemma":
		for _, m := range messages {
			role := mapRoleToString(m.Role, "gemma")
			content := messageContent(m)
			sb.WriteString("<start_of_turn>")
			sb.WriteString(role)
			sb.WriteString("\n")
			sb.WriteString(content)
			sb.WriteString("<end_of_turn>\n")
		}
		sb.WriteString("<start_of_turn>model\n")

	case "raw":
		for _, m := range messages {
			sb.WriteString(string(m.Role))
			sb.WriteString(": ")
			sb.WriteString(messageContent(m))
			sb.WriteString("\n")
		}
		sb.WriteString("assistant: ")

	default: // chatml
		for _, m := range messages {
			role := mapRoleToString(m.Role, "chatml")
			content := messageContent(m)
			sb.WriteString("<|im_start|>")
			sb.WriteString(role)
			sb.WriteString("\n")
			sb.WriteString(content)
			sb.WriteString("<|im_end|>\n")
		}
		sb.WriteString("<|im_start|>assistant\n")
	}

	return sb.String()
}

// mapRoleToString translate provider.Role ke string yang dimengerti template.
// Tool result di-fold jadi user content (tool calling diabaikan di Phase 1).
func mapRoleToString(role Role, template string) string {
	switch role {
	case RoleSystem:
		// Gemma gak punya system role explicit — fold ke user di caller.
		// Tapi safer: render apa adanya, model modern handle.
		if template == "gemma" {
			return "user" // gemma2-it convention
		}
		return "system"
	case RoleAssistant:
		if template == "gemma" {
			return "model"
		}
		return "assistant"
	case RoleTool:
		return "user" // fold tool result jadi context
	default:
		return "user"
	}
}

// messageContent ekstrak text content. Multimodal parts di-flatten ke text-only.
// Tool calls inline di-prefix kalau ada (info debug, gak di-execute).
func messageContent(m Message) string {
	var sb strings.Builder
	if m.Role == RoleTool && m.Name != "" {
		sb.WriteString("[tool ")
		sb.WriteString(m.Name)
		sb.WriteString("]\n")
	}
	if m.Content != "" {
		sb.WriteString(m.Content)
	}
	for _, p := range m.MultimodalParts {
		if p.Type == "text" && p.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// stopSequencesFor return token-stop list per template — mencegah model
// nge-loop ke turn berikutnya.
func stopSequencesFor(template string) []string {
	switch template {
	case "llama3":
		return []string{"<|eot_id|>", "<|end_of_text|>"}
	case "gemma":
		return []string{"<end_of_turn>"}
	case "raw":
		return []string{"\nuser:", "\nsystem:"}
	default: // chatml
		return []string{"<|im_end|>"}
	}
}
