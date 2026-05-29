package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/teetah2402/flowork/internal/finance"
)

// budgetGuardActive: rc-honest-audit 2026-04-21 — guard default ON (opt-out).
// Originally opt-in (FLOWORK_BUDGET_GUARD_ENABLE=1) per rc81, tapi assessment
// 2026-04-21 menemukan spend 226% over cap karena env tidak pernah di-set.
// Guard cuma jalan di OpenRouter path (crypto-backed); loopback/Ollama
// tetap lewat tanpa check (lokal = gratis).
//
// Override untuk disable sepenuhnya (rare — contoh: emergency debugging):
//
//	FLOWORK_BUDGET_GUARD_DISABLE=1
//
// Legacy env FLOWORK_BUDGET_GUARD_ENABLE masih dibaca untuk backward-compat
// (kalau di-set "0" akan disable), tapi default-nya tidak lagi dibutuhkan.
func budgetGuardActive() bool {
	if strings.TrimSpace(os.Getenv("FLOWORK_BUDGET_GUARD_DISABLE")) == "1" {
		return false
	}
	if strings.TrimSpace(os.Getenv("FLOWORK_BUDGET_GUARD_ENABLE")) == "0" {
		return false // explicit legacy disable
	}
	return true
}

// actualCostUSD compute biaya real dari response.Usage. Rate-nya best-effort
// berdasarkan model prefix (Claude-tier $3/$15 per-M, Gemini-tier $0.30/$2.50,
// DeepSeek-tier $0.14/$0.28). Bukan replacement authoritative billing — itu
// tetap dari /auth/key poll — tapi cukup untuk local daily tracker di antara
// 60s poll window supaya reserved tidak leak.
func actualCostUSD(model string, inputTokens, outputTokens, cachedTokens int) float64 {
	// Default Claude-tier (konservatif untuk tidak under-count).
	inputPerM, outputPerM, cachedPerM := 1.0, 3.0, 0.25
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "claude"), strings.Contains(m, "opus"):
		inputPerM, outputPerM = 3.0, 15.0
		cachedPerM = 0.30
	case strings.Contains(m, "gemini-2.5-pro"), strings.Contains(m, "gemini-pro"):
		inputPerM, outputPerM = 0.35, 2.50
		cachedPerM = 0.09
	case strings.Contains(m, "gemini"), strings.Contains(m, "gemma"):
		inputPerM, outputPerM = 0.10, 0.40
		cachedPerM = 0.025
	case strings.Contains(m, "deepseek"):
		inputPerM, outputPerM = 0.14, 0.28
		cachedPerM = 0.014
	case strings.Contains(m, "qwen"), strings.Contains(m, "llama"), strings.Contains(m, ":free"):
		return 0 // free-tier, no cost
	case strings.Contains(m, "gpt-4o"), strings.Contains(m, "gpt-5"):
		inputPerM, outputPerM = 2.50, 10.0
		cachedPerM = 1.25
	}
	nonCached := inputTokens - cachedTokens
	if nonCached < 0 {
		nonCached = 0
	}
	return float64(nonCached)*inputPerM/1_000_000 +
		float64(cachedTokens)*cachedPerM/1_000_000 +
		float64(outputTokens)*outputPerM/1_000_000
}

// isOpenRouterURL true untuk call yang dirouting via openrouter.ai. Guard
// crypto-wallet cuma relevan di jalur ini. Loopback/Ollama tidak pakai API
// key OpenRouter sehingga tidak perlu dihitung.
func isOpenRouterURL(base string) bool {
	b := strings.ToLower(base)
	return strings.Contains(b, "openrouter.ai")
}

// isInsufficientCreditsError detect HTTP 402 / insufficient_credits / payment
// required dari upstream provider response. Pattern matching ke error message
// yang di-wrap oleh retryPostJSON ("provider returned 402 Payment Required: ...").
// Different dari finance.ErrBudgetExceeded (lokal guard) — ini real upstream
// 402 dari OpenRouter saat saldo account = 0.
func isInsufficientCreditsError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "402") ||
		strings.Contains(msg, "payment required") ||
		strings.Contains(msg, "insufficient_quota") ||
		strings.Contains(msg, "insufficient credits") ||
		strings.Contains(msg, "insufficient balance") ||
		strings.Contains(msg, "insufficient_credits")
}

// safeFreeModel return model yg dijamin free di OpenRouter. Validasi:
//  1. Honor env FREE_AGENT_MODEL kalau punya suffix ":free"
//  2. Kalau env value bukan ":free" (paid model dengan label free) — log warning
//     lalu fallback ke hardcoded chain.
//
// Penting saat OpenRouter saldo $0 — kalau fallback "free" juga butuh balance,
// retry juga 402. Empty return kalau no safe option.
func safeFreeModel() string {
	user := strings.TrimSpace(os.Getenv("FREE_AGENT_MODEL"))
	userLow := strings.ToLower(user)
	if strings.Contains(userLow, ":free") {
		return user
	}
	// Hardcoded chain — verified free di OpenRouter (per scan 2026-04-20).
	// Pilih yg paling robust + multi-purpose untuk fallback agent kerja.
	candidates := []string{
		"qwen/qwen3-coder:free",                  // best for coding tools
		"openai/gpt-oss-120b:free",               // general purpose
		"nvidia/nemotron-3-super-120b-a12b:free", // already used as classifier
		"google/gemma-4-26b-a4b-it:free",         // Google free
		"z-ai/glm-4.5-air:free",                  // backup
	}
	if user != "" {
		// Belum :free tapi user set sesuatu — log warning sekali
		fmt.Fprintf(os.Stderr, "provider/openai: WARNING — FREE_AGENT_MODEL=%q bukan ':free' tier (kemungkinan tetap butuh balance OpenRouter). Fallback ke %s\n",
			user, candidates[0])
	}
	return candidates[0]
}

// estimateCostUSD perkiraan worst-case biaya call berdasarkan MaxTokens.
// Pakai asumsi konservatif: $3/M output + $1/M input (Claude-tier). Pre-call
// check tidak butuh presisi — yang penting cap per-task tidak jebol di
// runaway-loop scenario. Actual Record() pasca-response yang akurat.
func estimateCostUSD(req Request) float64 {
	// rc177 fix: model name "qwen/qwen3-coder:free", "deepseek/...:free", dll
	// shouldn't reserve budget — they cost $0. Pre-rc177 estimate pakai
	// generic Claude-tier rate untuk SEMUA model → reservation numpuk →
	// budget guard reject free-tier calls dengan "daily=$5/$5" meski actual
	// cost $0. Match actualCostUSD logic untuk konsistensi.
	m := strings.ToLower(req.Model)
	if strings.Contains(m, ":free") {
		return 0
	}
	// Approx 4 char = 1 token; pakai total message chars sebagai input proxy.
	inputTokens := 0
	for _, m := range req.Messages {
		inputTokens += len(m.Content) / 4
	}
	outputTokens := req.MaxTokens
	if outputTokens <= 0 {
		outputTokens = 1024
	}
	return float64(inputTokens)*1.0/1_000_000 + float64(outputTokens)*3.0/1_000_000
}

// promptCachingEnabled returns true when FLOWORK_PROMPT_CACHE=1 is set.
// Disabled by default until verified working per-provider in production.
func promptCachingEnabled() bool {
	return strings.TrimSpace(os.Getenv("FLOWORK_PROMPT_CACHE")) == "1"
}

// isClaudeModel returns true for Anthropic Claude models routed via OpenRouter.
func isClaudeModel(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "claude") || strings.HasPrefix(m, "anthropic/")
}

// OpenAIConfig mendeskripsikan parameter inisialisasi untuk client protokol OpenAI.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// OpenAIClient mengimplementasikan adapter protokol berbasis OpenAI Chat Completions.
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	defaultMod string
}

func NewOpenAIClient(cfg OpenAIConfig) (*OpenAIClient, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		// Shadow Mode / local gateway exemption: only allow empty key when the
		// BaseURL points at a loopback address. Any other caller that forgot to
		// set its key must still fail fast with a clear error — otherwise the
		// request hits a real provider with "shadow-dummy-key" and bounces back
		// as a confusing auth error.
		base := strings.ToLower(strings.TrimSpace(cfg.BaseURL))
		isLocal := strings.Contains(base, "localhost") ||
			strings.Contains(base, "127.0.0.1") ||
			strings.Contains(base, "[::1]")
		if !isLocal {
			return nil, fmt.Errorf("openai api key is required")
		}
		cfg.APIKey = "shadow-dummy-key"
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = "gpt-4.1-mini"
	}

	return &OpenAIClient{
		apiKey:     cfg.APIKey,
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		defaultMod: cfg.Model,
	}, nil
}

// Name mengembalikan tipe provider dari client ini.
func (c *OpenAIClient) Name() string {
	return "openai"
}

// Complete mengirim satu unified request dan mengonversi respons OpenAI menjadi Response umum.
func (c *OpenAIClient) Complete(ctx context.Context, req Request) (Response, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.defaultMod
	}
	// Extended thinking: DeepSeek switches model to reasoner variant.
	// xAI has -reasoning suffix variants. OpenAI o1/o3 models are reasoners natively.
	if req.Thinking != nil && req.Thinking.Enabled {
		model = switchToReasonerModel(model)
	}

	enableCache := promptCachingEnabled() && isClaudeModel(model)
	payload := map[string]any{
		"model":       model,
		"messages":    toOpenAIMessages(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}
	// E-1: prompt caching via OpenRouter. When FLOWORK_PROMPT_CACHE=1 and the
	// model is Claude, inject cache_control at request root — OpenRouter uses
	// this to auto-apply a cache breakpoint to the last cacheable block (system
	// prompt). Cache reads cost 0.25x normal input price (75% savings).
	// Verified: billing data shows Gemini caching works via OpenRouter; same
	// infrastructure should work for Claude. Disable by unsetting env var.
	if enableCache {
		payload["cache_control"] = map[string]any{"type": "ephemeral"}
	}
	// Anthropic-style thinking budget doesn't apply to OpenAI; reasoning models
	// use max_completion_tokens + internal reasoning budget. We leave as-is.
	if len(req.Tools) > 0 {
		payload["tools"] = toOpenAITools(req.Tools)
	}

	// Protokol Sekoci 2026-04-27: dormancy check sebelum budget guard. Saat
	// saldo OpenRouter ≤ FLOWORK_DORMANT_BALANCE_USD (default $0.05) dan
	// model BUKAN :free, return canned message tanpa hit upstream — daemon
	// idle gracefully, brain DB tetap jalan, auto-revive saat saldo ≥
	// FLOWORK_DORMANT_REVIVE_USD (default $1.00). Guard hysteresis cegah
	// flapping. Skip untuk free model (cost $0) dan loopback (lokal gratis).
	if isOpenRouterURL(c.baseURL) && !strings.Contains(strings.ToLower(model), ":free") {
		if finance.SharedDormancy().IsDormant(ctx) {
			return Response{
				Message: Message{
					Role:    RoleAssistant,
					Content: finance.DormantMessage(),
				},
				StopReason: StopReasonEndTurn,
			}, nil
		}
	}

	// rc-honest-audit 2026-04-21: BudgetGuard default ON. Pre-call reserve +
	// post-call settle — mandatory supaya spend tidak leak. Guard cuma aktif
	// di OpenRouter path; lokal/Ollama tetap lewat.
	//
	// rc-p0-finalize 2026-04-20: kalau ErrBudgetExceeded dan FreeFallback
	// enable + FREE_AGENT_MODEL ada + model saat ini bukan free → retry sekali
	// dengan free-tier model. Hindari infinite loop dengan flag attempted.
	var reservedUSD float64
	var guardActive bool
	if budgetGuardActive() && isOpenRouterURL(c.baseURL) {
		reservedUSD = estimateCostUSD(req)
		agent := strings.TrimSpace(os.Getenv("FLOWORK_AGENT_HANDLE"))
		// rc91: CheckBudgetFor consult LimitsResolver (policy.yaml) kalau
		// terpasang; fallback ke default caps kalau resolver nil.
		if err := finance.Shared().CheckBudgetFor(ctx, model, agent, reservedUSD); err != nil {
			if errors.Is(err, finance.ErrBudgetExceeded) {
				// Auto-fallback: switch ke free model + retry sekali.
				freeModel := safeFreeModel()
				if finance.Shared().FreeFallback && freeModel != "" &&
					!strings.Contains(strings.ToLower(model), ":free") &&
					!strings.EqualFold(freeModel, model) {
					fmt.Fprintf(os.Stderr, "provider/openai: budget exceeded for %s, fallback ke FREE_AGENT_MODEL=%s\n", model, freeModel)
					req.Model = freeModel
					model = freeModel
					// Re-check guard untuk free model — biasanya $0 cap, tapi
					// keep policy.yaml override flexibility.
					if err2 := finance.Shared().CheckBudgetFor(ctx, model, agent, 0); err2 != nil {
						if errors.Is(err2, finance.ErrBudgetExceeded) {
							return Response{}, err2
						}
						fmt.Fprintf(os.Stderr, "provider/openai: free fallback poll warning: %v\n", err2)
					}
					// Update payload model field supaya request kirim free model
					payload["model"] = model
				} else {
					return Response{}, err
				}
			} else {
				fmt.Fprintf(os.Stderr, "provider/openai: budget poll warning: %v\n", err)
			}
			// Poll gagal tapi bukan exceed — lanjut tanpa reservasi (best-effort).
		} else {
			guardActive = true // reserved successfully, harus di-settle
		}
	}

	var response openAIResponse
	err := retryPostJSON(ctx, llmHTTPClient(), c.baseURL+"/chat/completions", map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	}, payload, &response)
	if err != nil {
		// Call failed before completing — release reservation biar tidak leak.
		if guardActive {
			finance.Shared().ReleaseReservation(reservedUSD)
			guardActive = false
		}
		// rc-claude-rescue 2026-04-20: HTTP 402 / insufficient_credits dari
		// OpenRouter (account balance habis, beda dari budget guard lokal).
		// Auto-fallback ke FREE_AGENT_MODEL + retry sekali. Without this,
		// /scan + Telegram Claude calls gagal silent saat saldo OpenRouter 0.
		if isInsufficientCreditsError(err) && isOpenRouterURL(c.baseURL) {
			freeModel := safeFreeModel()
			if freeModel != "" &&
				!strings.Contains(strings.ToLower(model), ":free") &&
				!strings.EqualFold(freeModel, model) {
				fmt.Fprintf(os.Stderr, "provider/openai: HTTP 402/credits exhausted untuk %s, retry dengan free model %s\n", model, freeModel)
				model = freeModel
				req.Model = freeModel
				payload["model"] = freeModel
				err = retryPostJSON(ctx, llmHTTPClient(), c.baseURL+"/chat/completions", map[string]string{
					"Authorization": "Bearer " + c.apiKey,
				}, payload, &response)
				if err != nil {
					return Response{}, fmt.Errorf("free fallback %s also failed: %w", freeModel, err)
				}
				// Free retry sukses — fall-through ke parsing response di bawah.
			} else {
				return Response{}, err
			}
		} else {
			return Response{}, err
		}
	}

	if len(response.Choices) == 0 {
		// HTTP succeeded but no usable choices — release reservation to avoid budget leak.
		if guardActive {
			finance.Shared().ReleaseReservation(reservedUSD)
			guardActive = false
		}
		return Response{}, fmt.Errorf("openai returned no choices")
	}

	choice := response.Choices[0]
	toolCalls := make([]ToolCall, 0, len(choice.Message.ToolCalls))
	for _, item := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:        item.ID,
			Name:      item.Function.Name,
			Arguments: json.RawMessage(item.Function.Arguments),
		})
	}

	stopReason := StopReasonEndTurn
	if choice.FinishReason == "tool_calls" {
		stopReason = StopReasonToolUse
	}

	cachedRead := response.Usage.PromptCacheHitTokens
	if response.Usage.PromptTokensDetails.CachedTokens > 0 {
		cachedRead = response.Usage.PromptTokensDetails.CachedTokens
	}

	// rc188: log tiap call sukses untuk dashboard Daily Breakdown + trend.
	// Fire-and-forget — kegagalan log ngak boleh mati-kan request.
	LogOpenRouterCall(
		model,
		response.Usage.PromptTokens,
		response.Usage.CompletionTokens,
		cachedRead,
		string(stopReason),
	)

	// rc-honest-audit 2026-04-21: settle reservasi dengan actual cost dari
	// response.Usage. Tanpa Record, reserved numpuk dan usedToday cuma
	// tergantung OpenRouter /auth/key poll (yang return lifetime, bukan daily).
	if guardActive {
		actualUSD := actualCostUSD(
			model,
			response.Usage.PromptTokens,
			response.Usage.CompletionTokens,
			cachedRead,
		)
		finance.Shared().Record(actualUSD)
	}

	return Response{
		Message: Message{
			Role:      RoleAssistant,
			Content:   choice.Message.Content,
			ToolCalls: toolCalls,
		},
		StopReason: stopReason,
		Usage: Usage{
			InputTokens:          response.Usage.PromptTokens,
			OutputTokens:         response.Usage.CompletionTokens,
			CacheReadInputTokens: cachedRead,
		},
	}, nil
}

type openAIResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens         int `json:"prompt_tokens"`
		CompletionTokens     int `json:"completion_tokens"`
		PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"`
		PromptTokensDetails  struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

// switchToReasonerModel — for providers that expose reasoning as a separate
// model name (DeepSeek, xAI), swap to the reasoner variant when thinking is
// enabled. OpenAI's o1/o3 are native reasoners; pass through unchanged.
func switchToReasonerModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "deepseek-chat"), strings.HasPrefix(m, "deepseek-coder"):
		return "deepseek-reasoner"
	case strings.Contains(m, "grok-") && !strings.Contains(m, "reasoning"):
		// grok-4-fast-non-reasoning → grok-4-fast-reasoning
		if strings.Contains(m, "non-reasoning") {
			return strings.Replace(model, "non-reasoning", "reasoning", 1)
		}
		// grok-3 → grok-3 (no separate reasoner; pass through)
		return model
	}
	return model
}

func toOpenAIMessages(messages []Message) []map[string]any {
	converted := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		item := map[string]any{
			"role": string(message.Role),
		}

		switch message.Role {
		case RoleAssistant:
			item["content"] = message.Content
			if len(message.ToolCalls) > 0 {
				toolCalls := make([]map[string]any, 0, len(message.ToolCalls))
				for _, call := range message.ToolCalls {
					toolCalls = append(toolCalls, map[string]any{
						"id":   call.ID,
						"type": "function",
						"function": map[string]any{
							"name":      call.Name,
							"arguments": string(call.Arguments),
						},
					})
				}
				item["tool_calls"] = toolCalls
			}
		case RoleTool:
			item["content"] = message.Content
			item["tool_call_id"] = message.ToolCallID
			if message.Name != "" {
				item["name"] = message.Name
			}
		default:
			item["content"] = message.Content
		}

		converted = append(converted, item)
	}
	return converted
}

func toOpenAITools(tools []ToolDefinition) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		parameters := tool.InputSchema
		if parameters != nil {
			if t, ok := parameters["type"].(string); ok && t == "object" {
				if _, hasProps := parameters["properties"]; !hasProps {
					newParams := make(map[string]any)
					for k, v := range parameters {
						newParams[k] = v
					}
					newParams["properties"] = map[string]any{}
					parameters = newParams
				}
			}
		}

		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  parameters,
			},
		})
	}
	return result
}
