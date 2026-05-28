// flow_router Multi-Provider Dispatcher.

package router

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/flowork-os/flowork_Router/internal/creds"
	"github.com/flowork-os/flowork_Router/internal/executors"
	"github.com/flowork-os/flowork_Router/internal/providercompat"
	"github.com/flowork-os/flowork_Router/internal/store"
)

const httpTimeout = 120 * time.Second

var httpClient = &http.Client{Timeout: httpTimeout}

// ── OpenAI input shape (subset) ────────────────────────────────────────
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	// Tool calling (Phase 2). Passed through 1:1 for openai-compat upstreams;
	// converted to Anthropic `tools`/`tool_choice` for anthropic upstreams.
	Tools      json.RawMessage `json:"tools,omitempty"`
	ToolChoice json.RawMessage `json:"tool_choice,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// Tool calling fields (Phase 2, omitempty keeps simple text path intact).
	ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`  // assistant → tool invocations
	ToolCallID string          `json:"tool_call_id,omitempty"` // tool result → which call
	Name       string          `json:"name,omitempty"`         // tool/function name
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ── Anthropic shape (subset) ───────────────────────────────────────────
type AnthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []AnthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicResponse struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Role       string `json:"role"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ── Main dispatch ──────────────────────────────────────────────────────

// DispatchChatCompletion — entry untuk POST /v1/chat/completions.
// Lookup provider berdasarkan model → forward → log → return OpenAI format.
// Resolves combo alias first kalau model match nama combo.
func DispatchChatCompletion(ctx context.Context, req OpenAIRequest) (*OpenAIResponse, int, error) {
	d, err := store.Open()
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("store open: %w", err)
	}

	settings, _ := store.LoadSettings(d)
	// DefaultModel: a request that omits a model uses the configured default.
	if req.Model == "" && settings != nil {
		req.Model = settings.DefaultModel
	}
	// RTK token saver: compress large tool-result messages before forwarding.
	if settings != nil && settings.RtkTokenSaver {
		if msgs, saved := compressMessagesRTK(req.Messages); saved > 0 {
			req.Messages = msgs
			log.Printf("flow_router RTK token saver: trimmed %d chars from tool results", saved)
		}
	}

	// Caveman: append output-token-saver style instruction to the system
	// message. Pure additive mutation — translators downstream see the
	// extended system content but don't need to know about the modifier.
	if settings != nil && settings.CavemanLevel != "" {
		injectCavemanIntoRequest(&req, settings.CavemanLevel)
	}

	// Brain enrichment: if this request targets the brain model, inject
	// retrieved knowledge + skills before resolving the backend. No-op unless
	// settings.Brain.Enabled and model == Brain.Model. brainInfo (nil if not
	// enriched) lets us record the interaction for compounding after the answer.
	brainInfo := maybeEnrichBrain(ctx, &req, settings)

	// Model manager: resolve alias / custom (→ effective model + provider pin).
	resolvedModel, pinnedProvider := resolveModel(d, req.Model)
	req.Model = resolvedModel

	// Combo alias resolution: if req.Model matches a combo name, pick a model
	// from combo.Models per strategy and remember the remaining models as a
	// per-model fallback order (used when ALL providers for the picked model
	// 5xx — we then move on to the next combo model instead of giving up).
	var comboFallback []string
	if pinnedProvider == "" {
		if combo, _ := store.GetComboByName(d, req.Model); combo != nil && len(combo.Models) > 0 {
			picked := pickComboModel(combo)
			log.Printf("flow_router combo %q (%s) → model %q", combo.Name, combo.Strategy, picked)
			req.Model = picked
			comboFallback = comboFallbackOrder(combo, picked)
		}
	}

	// Per-model attempt loop: first try req.Model, then any remaining combo
	// fallbacks if the providers for req.Model all 5xx. Non-combo requests
	// pay no overhead because comboFallback stays nil.
	modelsToTry := append([]string{req.Model}, comboFallback...)
	var lastModelErr error
	var lastModelStatus int
	for modelIdx, candidateModel := range modelsToTry {
		req.Model = candidateModel
		if modelIdx > 0 {
			log.Printf("flow_router combo per-model fallback: trying %q", candidateModel)
		}
		resp, status, err := dispatchSingleModel(ctx, d, req, settings, brainInfo, pinnedProvider)
		if err == nil && resp != nil {
			return resp, status, nil
		}
		lastModelErr = err
		lastModelStatus = status
		// Only retry on 5xx-ish upstream/router errors. 4xx-class
		// (forbidden / not found / 401) means the operator intentionally
		// blocked this model — moving to the next combo model just hides
		// the real problem.
		if status > 0 && status < 500 {
			break
		}
	}
	return nil, lastModelStatus, lastModelErr
}

// dispatchSingleModel runs the full provider-selection + try-each-candidate
// loop for ONE concrete model. Extracted so DispatchChatCompletion can walk
// combo fallbacks on 5xx.
func dispatchSingleModel(ctx context.Context, d *sql.DB, req OpenAIRequest, settings *store.Settings, brainInfo *brainEnrichInfo, pinnedProvider string) (*OpenAIResponse, int, error) {
	matches, err := store.FindActiveByModel(d, req.Model)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("find provider: %w", err)
	}
	if pinnedProvider != "" {
		matches = pinProvider(d, matches, pinnedProvider)
	}
	if len(matches) == 0 {
		return nil, http.StatusNotFound, fmt.Errorf("no active provider supports model %q", req.Model)
	}
	// Drop providers where this model is disabled.
	if matches = filterDisabled(d, matches, req.Model); len(matches) == 0 {
		return nil, http.StatusForbidden, fmt.Errorf("model %q is disabled", req.Model)
	}

	// Inbound API-key scope: drop providers the key is not allowed to use.
	keyID := apiKeyID(ctx)
	if key := APIKeyFromContext(ctx); key != nil {
		matches = filterByAllowedProviders(matches, key)
		if len(matches) == 0 {
			return nil, http.StatusForbidden, fmt.Errorf("api key %q not permitted for any provider serving model %q", key.Name, req.Model)
		}
	}

	// Per-intent multiplexing: a private prompt may only go to a local-tagged
	// provider — refuse rather than leak to cloud.
	if settings != nil && settings.IntentRouting.Enabled && promptIsPrivate(req, settings.IntentRouting.PrivatePatterns) {
		tag := settings.IntentRouting.PrivateTag
		if tag == "" {
			tag = "local"
		}
		local := filterByTag(matches, tag)
		if len(local) == 0 {
			return nil, http.StatusForbidden, fmt.Errorf("private prompt: no provider tagged %q available — refusing to route to cloud", tag)
		}
		matches = local
		log.Printf("flow_router intent-routing: private prompt → %d provider(s) tagged %q", len(local), tag)
	}

	// Cost-tier routing: classify request → filter providers by tier:* tag.
	// Skips when user explicitly named a model an active provider serves.
	if settings != nil && settings.CostRouting.Enabled {
		if !(settings.CostRouting.HonorExplicitModel && hasActiveProviderForModel(matches, req.Model)) {
			tier := ClassifyCost(req, settings.CostRouting)
			if tiered := filterByTier(matches, tier); len(tiered) > 0 {
				matches = tiered
				log.Printf("flow_router cost-routing: tier=%s → %d provider(s)", tier, len(tiered))
			}
		}
	}

	// Fallback strategy: reorder candidates (priority_ordered = unchanged).
	if settings != nil {
		matches = applyFallbackStrategy(matches, settings.FallbackStrategy, req.Model)
	}

	// Try candidates in order, fallback to next on error
	var lastErr error
	startTotal := time.Now()
	for _, p := range matches {
		start := time.Now()
		resp, status, err := forwardToProvider(ctx, &p, req)
		latencyMs := time.Since(start).Milliseconds()

		// Log to usageHistory (best-effort, non-blocking)
		go logUsage(d, keyID, p.ID, req.Model, resp, status, err, latencyMs)

		if err == nil && resp != nil {
			log.Printf("flow_router dispatch model=%s → provider=%s latency=%dms tokens=%d",
				req.Model, p.Name, latencyMs, resp.Usage.TotalTokens)
			recordBrainContribution(d, settings, brainInfo, answerText(resp))
			return resp, status, nil
		}
		lastErr = err
		log.Printf("flow_router fallback model=%s provider=%s failed (%v), trying next", req.Model, p.Name, err)
	}

	log.Printf("flow_router ALL providers exhausted model=%s total=%dms", req.Model, time.Since(startTotal).Milliseconds())
	return nil, http.StatusBadGateway, fmt.Errorf("all providers failed; last error: %w", lastErr)
}

// forwardToProvider — dispatch ke provider tertentu dengan format proper.
func forwardToProvider(ctx context.Context, p *store.ProviderConnection, req OpenAIRequest) (*OpenAIResponse, int, error) {
	format, _ := p.Data[store.CfgFormat].(string)
	baseURL, _ := p.Data[store.CfgBaseURL].(string)

	// Auto-resolve format + baseURL when the provider record uses one of the
	// "openai-compatible-…" / "anthropic-compatible-…" name prefixes and the
	// operator didn't supply the explicit fields. Explicit values always win.
	if format == "" {
		if resolved := providercompat.ResolveFormat(p.Provider); resolved != "" {
			format = resolved
		}
	}
	if baseURL == "" {
		baseURL = providercompat.ResolveBaseURL(p.Provider, baseURL)
	}

	if baseURL == "" {
		return nil, 0, fmt.Errorf("provider %s missing baseUrl", p.ID)
	}

	// Vendor executor (non-stream path).
	if ex := executors.Get(format); ex != nil {
		body, u, st, err := ex.NonStream(ctx, p, executorRequest(req))
		if err != nil {
			return nil, st, err
		}
		var resp OpenAIResponse
		if jerr := json.Unmarshal(body, &resp); jerr != nil {
			return nil, http.StatusBadGateway, fmt.Errorf("executor %s decode: %w", format, jerr)
		}
		if resp.Usage.TotalTokens == 0 {
			resp.Usage.PromptTokens = u.PromptTokens
			resp.Usage.CompletionTokens = u.CompletionTokens
			resp.Usage.TotalTokens = u.TotalTokens
		}
		return &resp, st, nil
	}

	switch format {
	case "anthropic":
		return forwardAnthropic(ctx, p, baseURL, req)
	case "openai", "":
		return forwardOpenAICompat(ctx, p, baseURL, req)
	case "gemini":
		return forwardGemini(ctx, p, baseURL, req)
	default:
		return nil, 0, fmt.Errorf("unknown format: %s", format)
	}
}

// forwardOpenAICompat — passthrough untuk provider yang udah OpenAI-compat
// (local llama-server, OpenAI API, DeepSeek, Groq, Together AI, etc).
func forwardOpenAICompat(ctx context.Context, p *store.ProviderConnection, baseURL string, req OpenAIRequest) (*OpenAIResponse, int, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal: %w", err)
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("build req: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := applyAuth(httpReq, p); err != nil {
		return nil, http.StatusUnauthorized, err
	}

	resp, err := outboundClient(ctx).Do(httpReq)
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("upstream %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var out OpenAIResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("parse resp: %w", err)
	}
	return &out, http.StatusOK, nil
}

// forwardAnthropic — translate OpenAI → Anthropic Messages, forward, translate back.
// When the request carries tools/tool-role messages, the rich tool path runs
// (buildAnthropicToolBody + parseAnthropicToolResponse). Otherwise the proven
// simple text path is used unchanged.
func forwardAnthropic(ctx context.Context, p *store.ProviderConnection, baseURL string, req OpenAIRequest) (*OpenAIResponse, int, error) {
	if hasToolContext(req) {
		return forwardAnthropicWithTools(ctx, p, baseURL, req)
	}
	// Translate request
	anthrReq := AnthropicRequest{
		Model:       normalizeClaudeModel(req.Model),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}
	if anthrReq.MaxTokens <= 0 {
		anthrReq.MaxTokens = 4096
	}
	var sysParts []string
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			sysParts = append(sysParts, m.Content)
		case "user", "assistant":
			anthrReq.Messages = append(anthrReq.Messages, AnthropicMessage{Role: m.Role, Content: m.Content})
		}
	}
	if len(sysParts) > 0 {
		anthrReq.System = strings.Join(sysParts, "\n\n")
	}

	body, err := json.Marshal(anthrReq)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal anthropic: %w", err)
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("build req: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("User-Agent", "claude-cli/1.0.0 (flow_router)")
	if err := applyAuth(httpReq, p); err != nil {
		return nil, http.StatusUnauthorized, err
	}

	resp, err := outboundClient(ctx).Do(httpReq)
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("anthropic %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var anthrResp AnthropicResponse
	if err := json.Unmarshal(respBody, &anthrResp); err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("parse anthropic: %w", err)
	}

	// Translate response
	var content string
	for _, c := range anthrResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}
	stopReason := "stop"
	switch anthrResp.StopReason {
	case "end_turn", "stop_sequence":
		stopReason = "stop"
	case "max_tokens":
		stopReason = "length"
	case "tool_use":
		stopReason = "tool_calls"
	}
	return &OpenAIResponse{
		ID:      anthrResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []OpenAIChoice{{
			Index:        0,
			Message:      OpenAIMessage{Role: "assistant", Content: content},
			FinishReason: stopReason,
		}},
		Usage: OpenAIUsage{
			PromptTokens:     anthrResp.Usage.InputTokens,
			CompletionTokens: anthrResp.Usage.OutputTokens,
			TotalTokens:      anthrResp.Usage.InputTokens + anthrResp.Usage.OutputTokens,
		},
	}, http.StatusOK, nil
}

// applyAuth attach proper auth header based on provider authType.
func applyAuth(req *http.Request, p *store.ProviderConnection) error {
	switch p.AuthType {
	case store.AuthTypeNone:
		return nil // local llama, no auth
	case store.AuthTypeAPIKey:
		k, _ := p.Data[store.CfgAPIKey].(string)
		if k == "" {
			return fmt.Errorf("provider %s missing apiKey", p.ID)
		}
		// Anthropic uses x-api-key, OpenAI uses Authorization Bearer
		if p.Provider == "anthropic" {
			req.Header.Set("x-api-key", k)
		} else {
			req.Header.Set("Authorization", "Bearer "+k)
		}
		return nil
	case store.AuthTypeSubscription:
		// Read live from credential source
		src, _ := p.Data[store.CfgTokenSource].(string)
		switch src {
		case "claude_credentials":
			c, err := creds.Load()
			if err != nil {
				return fmt.Errorf("claude creds: %w", err)
			}
			if c.IsExpired() {
				return fmt.Errorf("claude credentials expired — re-login Claude Code")
			}
			req.Header.Set("Authorization", "Bearer "+c.ClaudeAiOauth.AccessToken)
			return nil
		case "codex_auth":
			tok, err := creds.LoadCodexToken()
			if err != nil {
				return fmt.Errorf("codex auth: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+tok)
			return nil
		case "cursor_session":
			tok, err := creds.LoadCursorToken()
			if err != nil {
				return fmt.Errorf("cursor auth: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+tok)
			return nil
		default:
			return fmt.Errorf("unknown subscription tokenSource: %s", src)
		}
	}
	return fmt.Errorf("unknown authType: %s", p.AuthType)
}

// pickComboModel — strategy-aware model selection dari combo.
// Priority: return first. RoundRobin: cycle via index counter. Random: random.
// CostOptimal: pick model dengan known lowest pricing.
// comboFallbackOrder returns the combo's models in the order to retry after
// `picked` has been tried. The picked model is excluded; remaining models
// keep their original list order (priority semantics). Returns nil when the
// combo has fewer than 2 models so the caller skips the fallback loop.
func comboFallbackOrder(c *store.Combo, picked string) []string {
	if c == nil || len(c.Models) < 2 {
		return nil
	}
	out := make([]string, 0, len(c.Models)-1)
	for _, m := range c.Models {
		if m == picked {
			continue
		}
		out = append(out, m)
	}
	return out
}

func pickComboModel(c *store.Combo) string {
	if len(c.Models) == 0 {
		return ""
	}
	switch c.Strategy {
	case store.ComboStrategyRoundRobin:
		i := nextRoundRobin("combo:"+c.ID, len(c.Models))
		return c.Models[i]
	case store.ComboStrategyRandom:
		// Use time.Now nanos modulo as cheap PRNG (no crypto needed for routing).
		return c.Models[int(time.Now().UnixNano())%len(c.Models)]
	case store.ComboStrategyCostOptimal:
		// Pick model yang harga input+output terendah (estimateCost as proxy).
		bestModel := c.Models[0]
		bestCost := estimateCost(bestModel, 1000, 1000) // 1k+1k token sample
		for _, m := range c.Models[1:] {
			cost := estimateCost(m, 1000, 1000)
			if cost > 0 && (bestCost == 0 || cost < bestCost) {
				bestModel = m
				bestCost = cost
			}
		}
		return bestModel
	default: // priority
		return c.Models[0]
	}
}

func normalizeClaudeModel(m string) string {
	m = strings.TrimSpace(m)
	for _, prefix := range []string{"cc/", "anthropic/", "claude/"} {
		m = strings.TrimPrefix(m, prefix)
	}
	if m == "" {
		return "claude-haiku-4-5"
	}
	return m
}

// logUsage — append-only request log + daily aggregate.
// Called async (goroutine) per dispatch — never blocks caller.
func logUsage(d any, apiKeyID, providerID, model string, resp *OpenAIResponse, status int, errIn error, latencyMs int64) {
	db, ok := d.(*sql.DB)
	if !ok || db == nil {
		return
	}
	entry := &store.LogEntry{
		APIKeyID:   apiKeyID,
		ProviderID: providerID,
		Model:      model,
		StatusCode: status,
		LatencyMs:  latencyMs,
	}
	if errIn != nil {
		entry.Error = errIn.Error()
	}
	if resp != nil {
		entry.PromptTokens = resp.Usage.PromptTokens
		entry.CompletionTokens = resp.Usage.CompletionTokens
		entry.TotalTokens = resp.Usage.TotalTokens
		entry.CostUsd = estimateCost(model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	}
	_ = store.LogRequest(db, entry)
}

// estimateCost — rough USD cost estimate per million tokens.
// estimateCost — DATA-driven cost estimate. Reads the pricing table (which
// the user can edit via /api/pricing and which SeedDefaultPricing seeds),
// so there is NO hardcoded rate map. Unknown model → 0 (local/free).
func estimateCost(model string, promptTok, complTok int) float64 {
	d, err := store.Open()
	if err != nil {
		return 0
	}
	pr, err := store.LookupPricingByModel(d, model)
	if err != nil || pr == nil {
		return 0 // unknown model = treat as free (e.g. local llama)
	}
	return (float64(promptTok)/1e6)*pr.InputUsdPer1M + (float64(complTok)/1e6)*pr.OutputUsdPer1M
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
