// Provider Connection CRUD.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuthType constants — universal router auth modes.
const (
	AuthTypeAPIKey       = "api_key"      // regular bearer / key header
	AuthTypeSubscription = "subscription" // OAuth subscription (Claude/Codex/Cursor)
	AuthTypeNone         = "none"         // local llama, no auth
)

// ProviderConnection — single provider record. `Data` is JSON-serialized
// per-provider config (baseUrl, models, headers, etc).
type ProviderConnection struct {
	ID        string         `json:"id"`
	Provider  string         `json:"provider"` // "anthropic", "openai", "local-llama", etc
	AuthType  string         `json:"authType"`
	Name      string         `json:"name"`
	Email     string         `json:"email,omitempty"`
	Priority  int            `json:"priority"`
	IsActive  bool           `json:"isActive"`
	Data      map[string]any `json:"data"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// ProviderConfigKey untuk `Data` map. Document semua kunci yang dipake.
const (
	CfgBaseURL     = "baseUrl"
	CfgAPIKey      = "apiKey"      // encrypted at rest via secret.go (AES-GCM); decrypted on read
	CfgModels      = "models"      // []string of supported models
	CfgFormat      = "format"      // "openai", "anthropic", "gemini"
	CfgHeaders     = "headers"     // map[string]string extra headers
	CfgTokenSource = "tokenSource" // for subscription: "claude_credentials", "codex_auth", "cursor_session"
)

// ListProviders returns all providers, ordered by priority ASC.
func ListProviders(d *sql.DB) ([]ProviderConnection, error) {
	rows, err := d.Query(`SELECT id, provider, authType, name, email, priority, isActive, data, createdAt, updatedAt
		FROM providerConnections ORDER BY priority ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()

	var out []ProviderConnection
	for rows.Next() {
		var p ProviderConnection
		var isActive int
		var dataJSON, createdStr, updatedStr string
		var email sql.NullString
		if err := rows.Scan(&p.ID, &p.Provider, &p.AuthType, &p.Name, &email,
			&p.Priority, &isActive, &dataJSON, &createdStr, &updatedStr); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if email.Valid {
			p.Email = email.String
		}
		p.IsActive = isActive == 1
		_ = json.Unmarshal([]byte(dataJSON), &p.Data)
		decryptProviderKey(p.Data)
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
		out = append(out, p)
	}
	return out, nil
}

// GetProvider by ID. Returns nil if not found.
func GetProvider(d *sql.DB, id string) (*ProviderConnection, error) {
	row := d.QueryRow(`SELECT id, provider, authType, name, email, priority, isActive, data, createdAt, updatedAt
		FROM providerConnections WHERE id = ?`, id)
	var p ProviderConnection
	var isActive int
	var dataJSON, createdStr, updatedStr string
	var email sql.NullString
	if err := row.Scan(&p.ID, &p.Provider, &p.AuthType, &p.Name, &email,
		&p.Priority, &isActive, &dataJSON, &createdStr, &updatedStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if email.Valid {
		p.Email = email.String
	}
	p.IsActive = isActive == 1
	_ = json.Unmarshal([]byte(dataJSON), &p.Data)
	decryptProviderKey(p.Data)
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	return &p, nil
}

// decryptProviderKey decrypts the apiKey field in place (runtime sees plaintext).
func decryptProviderKey(data map[string]any) {
	if data == nil {
		return
	}
	if k, ok := data[CfgAPIKey].(string); ok && k != "" {
		data[CfgAPIKey] = DecryptSecret(k)
	}
}

// encryptProviderData returns a shallow copy of data with the apiKey encrypted
// for persistence — the caller's map (returned to the API) stays plaintext.
func encryptProviderData(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	k, ok := data[CfgAPIKey].(string)
	if !ok || k == "" {
		return data
	}
	cp := make(map[string]any, len(data))
	for key, v := range data {
		cp[key] = v
	}
	cp[CfgAPIKey] = EncryptSecret(k)
	return cp
}

// FindActiveByModel — given model name, find best matching active provider.
// Returns providers sorted by priority. Pattern match via Models config field
// (supports literal "claude-haiku-4-5" + wildcard "*").
func FindActiveByModel(d *sql.DB, model string) ([]ProviderConnection, error) {
	all, err := ListProviders(d)
	if err != nil {
		return nil, err
	}
	var match []ProviderConnection
	model = strings.TrimSpace(model)
	modelLower := strings.ToLower(model)
	for _, p := range all {
		if !p.IsActive {
			continue
		}
		models, _ := p.Data[CfgModels].([]any)
		for _, m := range models {
			ms, ok := m.(string)
			if !ok {
				continue
			}
			msLower := strings.ToLower(ms)
			if ms == "*" || msLower == modelLower {
				match = append(match, p)
				break
			}
			// Prefix wildcard: "claude-*" matches "claude-haiku-4-5"
			if strings.HasSuffix(ms, "*") {
				prefix := strings.TrimSuffix(msLower, "*")
				if strings.HasPrefix(modelLower, prefix) {
					match = append(match, p)
					break
				}
			}
		}
	}
	return match, nil
}

// UpsertProvider inserts or updates. Generates UUID if ID empty.
func UpsertProvider(d *sql.DB, p *ProviderConnection) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if p.ID == "" {
		p.ID = uuid.NewString()
		p.CreatedAt = time.Now().UTC()
	}
	p.UpdatedAt = time.Now().UTC()
	dataJSON, _ := json.Marshal(encryptProviderData(p.Data))
	active := 0
	if p.IsActive {
		active = 1
	}
	createdStr := p.CreatedAt.Format(time.RFC3339)
	if createdStr == "0001-01-01T00:00:00Z" {
		createdStr = now
	}

	_, err := d.Exec(`INSERT INTO providerConnections (id, provider, authType, name, email, priority, isActive, data, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider=excluded.provider, authType=excluded.authType, name=excluded.name,
			email=excluded.email, priority=excluded.priority, isActive=excluded.isActive,
			data=excluded.data, updatedAt=excluded.updatedAt`,
		p.ID, p.Provider, p.AuthType, p.Name, p.Email, p.Priority, active, string(dataJSON),
		createdStr, p.UpdatedAt.Format(time.RFC3339))
	return err
}

// DeleteProvider removes by ID.
func DeleteProvider(d *sql.DB, id string) error {
	_, err := d.Exec(`DELETE FROM providerConnections WHERE id = ?`, id)
	return err
}

// AugmentTierTags adds tier:* tags to existing providers that predate the
// cost-routing feature. Idempotent: a provider that already carries any
// tier:* tag is skipped. Infers tier from the provider type — never
// overrides explicit user tags. Safe to call on every boot.
func AugmentTierTags(d *sql.DB) error {
	providers, err := ListProviders(d)
	if err != nil {
		return err
	}
	for _, p := range providers {
		tags, _ := p.Data["tags"].([]any)
		hasTier := false
		for _, t := range tags {
			if s, ok := t.(string); ok && strings.HasPrefix(s, "tier:") {
				hasTier = true
				break
			}
		}
		if hasTier {
			continue
		}
		var inferred []string
		switch p.Provider {
		case "local-llama", "ollama":
			inferred = []string{"tier:cheap"}
		case "anthropic":
			inferred = []string{"tier:standard", "tier:strong"}
		case "openai":
			inferred = []string{"tier:standard"}
		case "google", "gemini":
			inferred = []string{"tier:standard"}
		default:
			continue // unknown provider — leave alone for the user to tag manually
		}
		for _, t := range inferred {
			tags = append(tags, t)
		}
		if p.Data == nil {
			p.Data = map[string]any{}
		}
		p.Data["tags"] = tags
		if err := UpsertProvider(d, &p); err != nil {
			return fmt.Errorf("augment tier tags for %s: %w", p.ID, err)
		}
	}
	return nil
}

// SeedDefaults — auto-create Claude subscription + local-llama on first boot
// if no providers exist. Self-detect based on environment.
func SeedDefaults(d *sql.DB) error {
	existing, err := ListProviders(d)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil // already seeded
	}

	// Claude subscription (read ~/.claude/.credentials.json). Tagged as both
	// standard and strong because the family spans haiku→opus; dispatcher
	// picks the right model based on req.Model + provider tier filtering.
	claude := &ProviderConnection{
		Provider: "anthropic",
		AuthType: AuthTypeSubscription,
		Name:     "Claude Pro/Max Subscription",
		Priority: 10,
		IsActive: true,
		Data: map[string]any{
			CfgBaseURL:     "https://api.anthropic.com/v1",
			CfgFormat:      "anthropic",
			CfgTokenSource: "claude_credentials",
			CfgModels: []any{
				"claude-opus-4-7", "claude-opus-4-6",
				"claude-sonnet-4-6", "claude-sonnet-4-5",
				"claude-haiku-4-5",
				"claude-*",
			},
			"tags": []any{"tier:standard", "tier:strong"},
		},
	}
	if err := UpsertProvider(d, claude); err != nil {
		return fmt.Errorf("seed claude: %w", err)
	}

	// Local llama-server (any local GGUF model on :8080). Cheap tier — runs
	// on user hardware, no per-token cost. Also "local" for privacy intent.
	local := &ProviderConnection{
		Provider: "local-llama",
		AuthType: AuthTypeNone,
		Name:     "Local llama-server",
		Priority: 1, // highest priority by default — sovereign-first
		IsActive: true,
		Data: map[string]any{
			CfgBaseURL: "http://127.0.0.1:8080/v1",
			CfgFormat:  "openai",
			CfgModels: []any{
				"brain-flowork", "brain-flowork.gguf",
				"qwen3-8b", "qwen*",
				"local-*", "mrflow",
				"*", // catch-all fallback
			},
			"tags": []any{"tier:cheap", "local"},
		},
	}
	if err := UpsertProvider(d, local); err != nil {
		return fmt.Errorf("seed local: %w", err)
	}

	return nil
}
