// flow_router Settings Store.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// Settings — full editable router config. JSON-serialized to settings.data.
type Settings struct {
	// Auth
	RequireLogin bool   `json:"requireLogin"`
	AuthMode     string `json:"authMode"`   // "password" | "oidc" | "none"
	Password     string `json:"password,omitempty"`   // hashed, never returned to client
	OidcConfig   map[string]any `json:"oidcConfig,omitempty"`

	// Inbound API-key gate for /v1. When true, every /v1 + /v1beta request must
	// carry a valid `Authorization: Bearer flr_...` (or x-api-key) key. Default
	// false → open local mode (plug-and-play). A valid key is always honoured
	// for usage attribution + per-key cap/allowedProviders enforcement even when
	// this is false; this flag only decides whether a key is MANDATORY.
	RequireApiKey bool `json:"requireApiKey"`

	// Dispatch — all consumed by internal/router (see dispatcher.go).
	DefaultModel     string `json:"defaultModel"`     // model used when a request omits one
	FallbackStrategy string `json:"fallbackStrategy"` // "priority_ordered" | "round_robin" | "random"

	// RTK Token Saver — when on, large tool-result messages are compressed
	// (head+tail kept, middle trimmed, blank-line runs collapsed) before being
	// forwarded upstream, cutting input tokens on agentic loops. Opt-in,
	// default off; consumed in dispatcher (compressMessagesRTK).
	RtkTokenSaver bool `json:"rtkTokenSaver"`

	// Per-intent multiplexing — route "private" prompts (matching any pattern)
	// to providers tagged PrivateTag (e.g. a local model) and NEVER to cloud.
	IntentRouting IntentRouting `json:"intentRouting"`

	// Cost-tier routing — classify each request as cheap / standard / strong
	// from a heuristic (char count, code blocks, tool_use, multi-turn) and
	// filter candidate providers by their tier:* tag. Saves money on simple
	// queries by routing them to small/local models. Skipped when the request
	// already names a model whose provider is explicitly active (user choice
	// always wins). Default ON with conservative thresholds.
	CostRouting CostRouting `json:"costRouting"`

	// CavemanLevel — output-token saver: appends a "respond tersely"
	// instruction to the system message before dispatch. Empty/off = no
	// modification. Recognised: "lite" | "full" | "ultra". See
	// internal/caveman/caveman.go for the prompt bodies.
	CavemanLevel string `json:"cavemanLevel,omitempty"`

	// ClaudeCLI bypass — short-circuit Warmup/count/title/isNewTopic no-op
	// requests from the Claude Code CLI with a local stub response, saving
	// the upstream tokens. Enabled by default; SkipPatterns extends the
	// match set; CcFilterNaming toggles the topic-naming probe.
	ClaudeCliBypass ClaudeCliBypass `json:"claudeCliBypass"`

	// Budget — global spend ceiling. Enforced (blocks /v1) only when Enforce is
	// true, so out-of-box stays open (plug-and-play). Mirrors the per-key cap
	// pattern. WarnUsd just emits a server-log warning when crossed.
	Budget Budget `json:"budget"`

	// Brain — server-side RAG enrichment from a Memory Palace DB. When enabled,
	// requests whose model == Brain.Model are enriched (cascade retrieval +
	// skill inject) before forwarding, turning any agent into a brain client.
	// Opt-in, default off (plug-and-play) — see internal/router/brainenrich.go.
	Brain BrainConfig `json:"brain"`

	// Misc — tunnel public URLs (status, surfaced by the init/locale dump).
	TunnelUrl    string `json:"tunnelUrl,omitempty"`
	TailscaleUrl string `json:"tailscaleUrl,omitempty"`
}

// BrainConfig configures the brain enrichment layer.
type BrainConfig struct {
	Enabled         bool     `json:"enabled"`                 // master switch (default off)
	Model           string   `json:"model"`                   // trigger model name (default "flowork-brain")
	DBPath          string   `json:"dbPath,omitempty"`        // optional DB override; else env/default
	Mode            string   `json:"mode"`                    // "augment" (default) | "brain"
	Wings           []string `json:"wings,omitempty"`         // optional wing whitelist for retrieval
	TopK            int      `json:"topK"`                    // knowledge snippets injected (default 5)
	MaxSnippetChars int      `json:"maxSnippetChars"`         // per-snippet truncation (default 600)
	Skills          bool     `json:"skills"`                  // inject relevant skills (default true)
	SkillTopK       int      `json:"skillTopK"`               // skills injected (default 2)
	Record          bool     `json:"record"`                  // queue interactions for compounding (default off)
}

// ClaudeCliBypass short-circuits no-op requests from the Claude Code CLI
// (Warmup, count, title-extraction, isNewTopic probes, configurable skip
// patterns) with a local stub response, saving upstream tokens.
type ClaudeCliBypass struct {
	Enabled         bool     `json:"enabled"`
	SkipPatterns    []string `json:"skipPatterns,omitempty"`    // extra substrings → bypass
	CcFilterNaming  bool     `json:"ccFilterNaming,omitempty"`  // enable the isNewTopic probe
}

// IntentRouting steers prompts deemed "private" to a local provider.
type IntentRouting struct {
	Enabled         bool     `json:"enabled"`
	PrivatePatterns []string `json:"privatePatterns"` // case-insensitive substrings
	PrivateTag      string   `json:"privateTag"`      // provider tag for private prompts (default "local")
}

// CostRouting classifies each request into a tier (cheap/standard/strong)
// and filters candidate providers by their tier:* tag. Thresholds are total
// input char counts across user+system messages. Anything above StandardMax
// or carrying code blocks / tool_use / many messages is treated as "strong".
// HonorExplicitModel = when true, requests that already name a model whose
// active provider exists skip tier filtering (user choice wins).
type CostRouting struct {
	Enabled            bool `json:"enabled"`
	CheapMaxChars      int  `json:"cheapMaxChars"`      // ≤ this → "cheap"
	StandardMaxChars   int  `json:"standardMaxChars"`   // ≤ this → "standard", else "strong"
	StrongOnCode       bool `json:"strongOnCode"`       // code block detected → bump to strong
	StrongOnToolUse    bool `json:"strongOnToolUse"`    // tool_use messages → bump to strong
	StrongMinMessages  int  `json:"strongMinMessages"`  // multi-turn ≥ this → bump to strong (0 = disabled)
	HonorExplicitModel bool `json:"honorExplicitModel"` // skip filtering when req.Model exactly matches an active provider
}

type Budget struct {
	Enforce       bool    `json:"enforce"`
	DailyCapUsd   float64 `json:"dailyCapUsd"`
	MonthlyCapUsd float64 `json:"monthlyCapUsd"`
	WarnUsd       float64 `json:"warnUsd"`
}

// defaultSettings — initial config kalau settings table kosong.
func defaultSettings() Settings {
	return Settings{
		RequireLogin:     false,
		AuthMode:         "none",
		DefaultModel:     "claude-haiku-4-5",
		FallbackStrategy: "priority_ordered",
		// Budget caps present but Enforce=false → unlimited out of the box.
		Budget: Budget{
			Enforce:       false,
			DailyCapUsd:   2.0,
			MonthlyCapUsd: 60.0,
			WarnUsd:       0.5,
		},
		// Brain off by default; sensible knobs ready when toggled on.
		Brain: BrainConfig{
			Enabled:         false,
			Model:           "flowork-brain",
			Mode:            "augment",
			TopK:            5,
			MaxSnippetChars: 600,
			Skills:          true,
			SkillTopK:       2,
		},
		// Claude-CLI bypass ON by default — pure-local stub responses for
		// known no-op patterns are always a token-saver and never affect
		// non-Claude-CLI clients (the detector gates on User-Agent).
		ClaudeCliBypass: ClaudeCliBypass{
			Enabled:        true,
			CcFilterNaming: false,
		},
		// Cost-tier routing ON by default with conservative thresholds. Bumps
		// to "strong" tier on code blocks, tool_use, or ≥6 messages. Always
		// honors explicit model choices so existing client behavior survives.
		CostRouting: CostRouting{
			Enabled:            true,
			CheapMaxChars:      2000,
			StandardMaxChars:   10000,
			StrongOnCode:       true,
			StrongOnToolUse:    true,
			StrongMinMessages:  6,
			HonorExplicitModel: true,
		},
	}
}

// LoadSettings — read from DB, return defaults kalau belum ada.
func LoadSettings(d *sql.DB) (*Settings, error) {
	row := d.QueryRow(`SELECT data FROM settings WHERE id = 1`)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			s := defaultSettings()
			return &s, nil
		}
		return nil, fmt.Errorf("settings scan: %w", err)
	}
	var s Settings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		// Corrupt JSON — fall back to defaults but log
		s = defaultSettings()
	}
	// Ensure sensible defaults filled in (backward compat kalau field baru ditambah)
	if s.AuthMode == "" {
		s.AuthMode = "none"
	}
	if s.DefaultModel == "" {
		s.DefaultModel = "claude-haiku-4-5"
	}
	if s.FallbackStrategy == "" {
		s.FallbackStrategy = "priority_ordered"
	}
	// CostRouting added later — fill defaults when persisted row predates it.
	// Detect "all zero" as the migration trigger (the user hasn't deliberately
	// disabled cost-routing yet because the controls didn't exist before).
	if s.CostRouting == (CostRouting{}) {
		s.CostRouting = defaultSettings().CostRouting
	}
	// ClaudeCliBypass added later — same soft-migration pattern. A struct
	// with Enabled=false, no SkipPatterns, no CcFilterNaming = the natural
	// zero value, indistinguishable from "deliberately disabled". To stay
	// safe we only restore the default when ALL fields are zero AND the
	// SkipPatterns slice is nil (any saved state, even disabled-with-
	// custom-patterns, leaves it alone).
	if !s.ClaudeCliBypass.Enabled && len(s.ClaudeCliBypass.SkipPatterns) == 0 && !s.ClaudeCliBypass.CcFilterNaming {
		s.ClaudeCliBypass = defaultSettings().ClaudeCliBypass
	}
	return &s, nil
}

// SaveSettings — persist full settings ke DB (overwrite).
// Caller bertanggung jawab pre-validate.
func SaveSettings(d *sql.DB, s *Settings) error {
	// Lockout guard (server-side, last line of defence): never persist a state
	// that requires password login without a password set — otherwise the admin
	// is locked out and /api/settings (protected) returns 401 forever. Applies
	// regardless of which UI/endpoint wrote it. OIDC mode is exempt (no password).
	if s.RequireLogin && s.AuthMode == "password" && s.Password == "" {
		s.RequireLogin = false
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	_, err = d.Exec(`INSERT INTO settings (id, data) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data`, string(raw))
	if err != nil {
		return fmt.Errorf("upsert settings: %w", err)
	}
	return nil
}

// PatchSettings — partial update via map (PATCH semantics).
// Hanya field yang ada di patch yang di-override.
func PatchSettings(d *sql.DB, patch map[string]any) (*Settings, error) {
	current, err := LoadSettings(d)
	if err != nil {
		return nil, err
	}
	// Merge via JSON roundtrip
	curJSON, _ := json.Marshal(current)
	var curMap map[string]any
	_ = json.Unmarshal(curJSON, &curMap)
	for k, v := range patch {
		if v == nil {
			continue
		}
		// Special: password never accept plaintext via PATCH (must use SetPassword).
		if k == "password" {
			continue
		}
		curMap[k] = v
	}
	mergedJSON, _ := json.Marshal(curMap)
	var merged Settings
	if err := json.Unmarshal(mergedJSON, &merged); err != nil {
		return nil, fmt.Errorf("patch merge: %w", err)
	}
	if err := SaveSettings(d, &merged); err != nil {
		return nil, err
	}
	return &merged, nil
}
