// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — CLI command/menu.

// CLI Tools Registry.

package clitools

import (
	"os"
	"path/filepath"
)

// ConfigFormat — how the tool stores its settings on disk.
type ConfigFormat string

const (
	FormatJSON   ConfigFormat = "json"
	FormatTOML   ConfigFormat = "toml"
	FormatEnv    ConfigFormat = "env"    // .env file (KEY=VALUE)
	FormatYAML   ConfigFormat = "yaml"   // YAML config (custom writer)
	FormatCustom ConfigFormat = "custom" // bespoke multi-file writer
	FormatProxy  ConfigFormat = "proxy"  // no on-disk config, MITM proxy only
)

// Tool — one CLI integration.
type Tool struct {
	ID            string       `json:"id"`
	DisplayName   string       `json:"displayName"`
	BinaryName    string       `json:"binaryName"`              // for PATH lookup
	BinaryAliases []string     `json:"binaryAliases,omitempty"` // alternate executables
	SettingsPath  string       `json:"settingsPath"`            // expanded path
	Format        ConfigFormat `json:"format"`
	BaseURLKey    string       `json:"baseUrlKey,omitempty"` // env var or key path
	TokenKey      string       `json:"tokenKey,omitempty"`
	EnvKeys       []string     `json:"envKeys"` // keys this tool understands (for reset)
	Notes         string       `json:"notes,omitempty"`
}

// All — registry of supported tools. Paths resolved at runtime via home().
func All() []Tool {
	home, _ := os.UserHomeDir()
	join := func(parts ...string) string {
		return filepath.Join(append([]string{home}, parts...)...)
	}
	return []Tool{
		{
			ID:           "claude",
			DisplayName:  "Claude Code (Anthropic CLI)",
			BinaryName:   "claude",
			SettingsPath: join(".claude", "settings.json"),
			Format:       FormatJSON,
			BaseURLKey:   "ANTHROPIC_BASE_URL",
			TokenKey:     "ANTHROPIC_AUTH_TOKEN",
			EnvKeys: []string{
				"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN",
				"ANTHROPIC_DEFAULT_OPUS_MODEL",
				"ANTHROPIC_DEFAULT_SONNET_MODEL",
				"ANTHROPIC_DEFAULT_HAIKU_MODEL",
				"API_TIMEOUT_MS",
			},
			Notes: "Settings nested under `env` key; setter also sets hasCompletedOnboarding:true",
		},
		{
			ID:           "codex",
			DisplayName:  "OpenAI Codex CLI",
			BinaryName:   "codex",
			SettingsPath: join(".codex", "config.toml"),
			Format:       FormatCustom,
			BaseURLKey:   "model_providers.flow_router.base_url",
			TokenKey:     "OPENAI_API_KEY",
			EnvKeys:      []string{"model", "model_provider", "model_providers.flow_router.base_url", "model_providers.flow_router.name", "model_providers.flow_router.wire_api"},
			Notes:        "Custom: config.toml model_provider=flow_router block + auth.json OPENAI_API_KEY.",
		},
		{
			ID:           "cline",
			DisplayName:  "Cline (VS Code extension)",
			BinaryName:   "code",
			SettingsPath: join(".cline", "globalState.json"),
			Format:       FormatJSON,
			BaseURLKey:   "baseUrl",
			TokenKey:     "apiKey",
			EnvKeys:      []string{"baseUrl", "apiKey"},
			Notes:        "VS Code extension globalState.json (+ secrets.json holds apiKey separately).",
		},
		{
			ID:            "copilot",
			DisplayName:   "GitHub Copilot CLI",
			BinaryName:    "gh",
			BinaryAliases: []string{"copilot"},
			SettingsPath:  join(".config", "github-copilot", "chatLanguageModels.json"),
			Format:        FormatJSON,
			EnvKeys:       []string{"customEndpoint", "customApiKey"},
			Notes:         "GitHub managed; custom endpoint via copilot-language-models patch.",
		},
		{
			ID:           "cowork",
			DisplayName:  "Cowork CLI",
			BinaryName:   "cowork",
			SettingsPath: join(".cowork", "config.json"),
			Format:       FormatJSON,
			BaseURLKey:   "baseUrl",
			TokenKey:     "apiKey",
			EnvKeys:      []string{"baseUrl", "apiKey", "model", "provider"},
		},
		{
			ID:           "deepseek-tui",
			DisplayName:  "DeepSeek TUI",
			BinaryName:   "deepseek",
			SettingsPath: join(".deepseek", "config.toml"),
			Format:       FormatTOML,
			BaseURLKey:   "api_base",
			TokenKey:     "api_key",
			EnvKeys:      []string{"api_base", "api_key", "model"},
		},
		{
			ID:           "droid",
			DisplayName:  "Factory.ai Droid",
			BinaryName:   "droid",
			SettingsPath: join(".factory", "settings.json"),
			Format:       FormatJSON,
			BaseURLKey:   "baseUrl",
			TokenKey:     "apiKey",
			EnvKeys:      []string{"baseUrl", "apiKey", "model"},
		},
		{
			ID:           "hermes",
			DisplayName:  "Hermes CLI",
			BinaryName:   "hermes",
			SettingsPath: join(".hermes", "config.yaml"),
			Format:       FormatYAML,
			BaseURLKey:   "model.base_url",
			TokenKey:     "OPENAI_API_KEY",
			EnvKeys:      []string{"model.base_url", "model.provider", "OPENAI_API_KEY"},
			Notes:        "Custom writer: config.yaml model-block (provider=custom, base_url) + .env OPENAI_API_KEY.",
		},
		{
			ID:           "jcode",
			DisplayName:  "JCode CLI",
			BinaryName:   "jcode",
			SettingsPath: join(".jcode", "config.toml"),
			Format:       FormatTOML,
			BaseURLKey:   "api.endpoint",
			TokenKey:     "API_KEY",
			EnvKeys:      []string{"api.endpoint", "API_KEY", "api.model"},
		},
		{
			ID:           "kilo",
			DisplayName:  "Kilo Code (VS Code extension)",
			BinaryName:   "code",
			SettingsPath: join(".config", "kilo", "settings.json"),
			Format:       FormatCustom,
			BaseURLKey:   "kilocode.customProvider.baseURL",
			TokenKey:     "apiKey",
			EnvKeys:      []string{"kilocode.customProvider"},
			Notes:        "Custom: settings.json kilocode.customProvider + auth.json.",
		},
		{
			ID:           "openclaw",
			DisplayName:  "OpenClaw CLI",
			BinaryName:   "openclaw",
			SettingsPath: join(".openclaw", "openclaw.json"),
			Format:       FormatCustom,
			BaseURLKey:   "baseUrl",
			TokenKey:     "apiKey",
			EnvKeys:      []string{"baseUrl", "apiKey", "model"},
			Notes:        "Custom writer: openclaw.json (agents.defaults.model.primary=flow_router/<model>) + models.json providers map.",
		},
		{
			ID:           "opencode",
			DisplayName:  "OpenCode CLI",
			BinaryName:   "opencode",
			SettingsPath: join(".config", "opencode", "opencode.json"),
			Format:       FormatJSON,
			BaseURLKey:   "baseUrl",
			TokenKey:     "apiKey",
			EnvKeys:      []string{"baseUrl", "apiKey", "model"},
		},
		{
			ID:           "antigravity-mitm",
			DisplayName:  "Antigravity (MITM mode)",
			BinaryName:   "",
			SettingsPath: join(".flow_router", "antigravity-mitm.json"),
			Format:       FormatProxy,
			EnvKeys:      []string{"mitmUrl", "interceptHosts"},
			Notes:        "MITM only; no on-disk tool config — flow_router proxies traffic for Antigravity inline.",
		},
	}
}

// Get returns Tool by ID, nil if unknown.
func Get(id string) *Tool {
	for _, t := range All() {
		if t.ID == id {
			tt := t
			return &tt
		}
	}
	return nil
}
