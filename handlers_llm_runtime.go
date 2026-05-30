// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flow_router
// Locked at: 2026-05-30
// Reason: Sections 24+25+26 phase 2 runtime endpoints — chain run,
//   LocalAI start/stop/status, pricing calc + manual cost log. Phase 3
//   (chain stream SSE, llama.cpp model HF download, owner-override audit)
//   → tambah file baru.
//
// handlers_llm_runtime.go — Sections 24-26 phase 2 runtime endpoints.

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/flowork-os/flowork_Router/internal/localai"
	"github.com/flowork-os/flowork_Router/internal/pricing"
	"github.com/flowork-os/flowork_Router/internal/provider"
	"github.com/flowork-os/flowork_Router/internal/store"
)

// ChainRunHandler — POST /api/provider/chain/run?chain=default
// Body OpenAI-compat chat request. Try primary→fallback→error. Auto-
// log cost via pricing.Calc + pricing.LogCall.
func ChainRunHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	chainName := strings.TrimSpace(r.URL.Query().Get("chain"))
	if chainName == "" {
		chainName = "default"
	}
	caller := strings.TrimSpace(r.Header.Get("X-Caller-ID"))
	if caller == "" {
		caller = "anonymous"
	}
	var req provider.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	db, err := store.Open()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	orch := provider.NewChainOrchestrator(db)
	resp, rerr := orch.Run(r.Context(), chainName, req)
	if rerr != nil {
		_ = pricing.LogCall(db, caller, "chain:"+chainName, req.Model, 0, 0, 0, 0, "error")
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": rerr.Error()})
		return
	}
	// Auto cost calc + log.
	cost, _ := pricing.Calc(db, resp.Provider, resp.Model, "",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	_ = pricing.LogCall(db, caller, resp.Provider, resp.Model,
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens,
		cost, resp.LatencyMS, "success")
	// Set cost header (mirror Section 23 Agent acceptance).
	w.Header().Set("X-Router-Cost-USD", strconv.FormatFloat(cost, 'f', 6, 64))
	w.Header().Set("X-Router-Provider", resp.Provider)
	writeJSON(w, http.StatusOK, map[string]any{
		"provider":   resp.Provider,
		"model":      resp.Model,
		"choices":    resp.Choices,
		"usage":      resp.Usage,
		"latency_ms": resp.LatencyMS,
		"cost_usd":   cost,
	})
}

// =============================================================================
// Section 25: LocalAI runtime control
// =============================================================================

var localAIRuntimeRef *localai.Runtime

// LocalAIRuntimeHandler — POST {action: start|stop|status, model_name?, gguf_path?}
func LocalAIRuntimeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var body struct {
		Action    string `json:"action"`
		ModelName string `json:"model_name"`
		GGUFPath  string `json:"gguf_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if localAIRuntimeRef == nil {
		localAIRuntimeRef = localai.NewRuntime("", 0)
	}
	switch body.Action {
	case "start":
		// Resolve gguf_path from registry kalau ngga di-supply.
		gguf := body.GGUFPath
		if gguf == "" && body.ModelName != "" {
			db, _ := store.Open()
			_ = db.QueryRow(
				`SELECT gguf_path FROM localai_models WHERE model_name = ?`,
				body.ModelName).Scan(&gguf)
		}
		if err := localAIRuntimeRef.Start(body.ModelName, gguf); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": localAIRuntimeRef.Status()})
	case "stop":
		_ = localAIRuntimeRef.Stop()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case "status", "":
		writeJSON(w, http.StatusOK, localAIRuntimeRef.Status())
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid action"})
	}
}

// =============================================================================
// Section 26: Real-time pricing calc + log
// =============================================================================

// PricingCalcHandler — POST {provider, model, tier?, input_tokens, output_tokens}
// Return computed cost_usd. Caller (external) panggil setelah LLM
// response untuk pre-calc cost dan log.
func PricingCalcHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var body struct {
		Provider     string `json:"provider"`
		Model        string `json:"model"`
		Tier         string `json:"tier"`
		InputTokens  int    `json:"input_tokens"`
		OutputTokens int    `json:"output_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	db, err := store.Open()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	cost, cerr := pricing.Calc(db, body.Provider, body.Model, body.Tier, body.InputTokens, body.OutputTokens)
	if cerr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": cerr.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"cost_usd":      cost,
		"input_tokens":  body.InputTokens,
		"output_tokens": body.OutputTokens,
	})
}

// PricingLogCallHandler — POST {caller, provider, model, input/output,
// cost_usd?, latency_ms?, status?}. Caller (test/admin) manual log.
func PricingLogCallHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	var body struct {
		Caller       string  `json:"caller"`
		Provider     string  `json:"provider"`
		Model        string  `json:"model"`
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		CostUSD      float64 `json:"cost_usd"`
		LatencyMS    int64   `json:"latency_ms"`
		Status       string  `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	db, err := store.Open()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	// Auto-calc kalau cost_usd 0 dan tokens > 0.
	if body.CostUSD == 0 && body.InputTokens+body.OutputTokens > 0 {
		body.CostUSD, _ = pricing.Calc(db, body.Provider, body.Model, "",
			body.InputTokens, body.OutputTokens)
	}
	if err := pricing.LogCall(db, body.Caller, body.Provider, body.Model,
		body.InputTokens, body.OutputTokens, body.CostUSD, body.LatencyMS, body.Status); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cost_usd": body.CostUSD})
}
