// KV-backed Misc Stores.

package store

import (
	"database/sql"
	"encoding/json"
	"time"
)

// ── OAuth Tokens ───────────────────────────────────────────────────────

type OAuthTokenRecord struct {
	Provider     string `json:"provider"` // codex|cursor|gitlab|iflow|kiro|claude|...
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	IDToken      string `json:"idToken,omitempty"`
	TokenType    string `json:"tokenType"`
	Scope        string `json:"scope,omitempty"`
	ExpiresAt    string `json:"expiresAt,omitempty"` // RFC3339
	Extra        any    `json:"extra,omitempty"`
	UpdatedAt    string `json:"updatedAt"`
}

const oauthKVPrefix = "oauth:"

func ListOAuthTokens(d *sql.DB) ([]OAuthTokenRecord, error) {
	return kvList[OAuthTokenRecord](d, oauthKVPrefix)
}

func GetOAuthToken(d *sql.DB, provider string) (*OAuthTokenRecord, error) {
	return kvGetByKey[OAuthTokenRecord](d, oauthKVPrefix+provider)
}

func UpsertOAuthToken(d *sql.DB, t *OAuthTokenRecord) error {
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return kvUpsert(d, oauthKVPrefix+t.Provider, t)
}

func DeleteOAuthToken(d *sql.DB, provider string) error {
	return kvDelete(d, oauthKVPrefix+provider)
}

// ── MCP Servers ────────────────────────────────────────────────────────

type MCPServer struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // stdio|sse|http
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Enabled   bool              `json:"enabled"`
	UpdatedAt string            `json:"updatedAt"`
}

const mcpKVPrefix = "mcp:"

func ListMCPServers(d *sql.DB) ([]MCPServer, error) {
	return kvList[MCPServer](d, mcpKVPrefix)
}

func GetMCPServer(d *sql.DB, id string) (*MCPServer, error) {
	return kvGetByKey[MCPServer](d, mcpKVPrefix+id)
}

func UpsertMCPServer(d *sql.DB, m *MCPServer) error {
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return kvUpsert(d, mcpKVPrefix+m.ID, m)
}

func DeleteMCPServer(d *sql.DB, id string) error {
	return kvDelete(d, mcpKVPrefix+id)
}

// ── Tunnel State (singleton) ───────────────────────────────────────────

type TunnelState struct {
	CloudflareEnabled    bool   `json:"cloudflareEnabled"`
	CloudflareURL        string `json:"cloudflareUrl,omitempty"`
	CloudflareToken      string `json:"cloudflareToken,omitempty"`
	CloudflarePID        int    `json:"cloudflarePid,omitempty"`
	TailscaleInstalled   bool   `json:"tailscaleInstalled"`
	TailscaleEnabled     bool   `json:"tailscaleEnabled"`
	TailscaleURL         string `json:"tailscaleUrl,omitempty"`
	DashboardAccess      bool   `json:"dashboardAccess"`
	UpdatedAt            string `json:"updatedAt"`
}

const tunnelKey = "tunnel:state"

func LoadTunnelState(d *sql.DB) (*TunnelState, error) {
	t, err := kvGetByKey[TunnelState](d, tunnelKey)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return &TunnelState{}, nil
	}
	return t, nil
}

func SaveTunnelState(d *sql.DB, t *TunnelState) error {
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return kvUpsert(d, tunnelKey, t)
}

// ── Locale Pref (singleton) ────────────────────────────────────────────

type LocalePref struct {
	Locale    string `json:"locale"`   // en|id|zh|...
	Timezone  string `json:"timezone"` // Asia/Jakarta default
	Theme     string `json:"theme"`    // dark|light|auto
	UpdatedAt string `json:"updatedAt"`
}

const localeKey = "locale:pref"

func LoadLocalePref(d *sql.DB) (*LocalePref, error) {
	p, err := kvGetByKey[LocalePref](d, localeKey)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return &LocalePref{Locale: "id", Timezone: "Asia/Jakarta", Theme: "dark"}, nil
	}
	return p, nil
}

func SaveLocalePref(d *sql.DB, p *LocalePref) error {
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return kvUpsert(d, localeKey, p)
}

// ── CLI Tool State (per-toolId detection cache) ────────────────────────

type CLIToolState struct {
	ToolID          string            `json:"toolId"`     // claude|codex|cursor|cline|...
	Installed       bool              `json:"installed"`
	HasCredentials  bool              `json:"hasCredentials"`
	BinaryPath      string            `json:"binaryPath,omitempty"`
	CredentialsPath string            `json:"credentialsPath,omitempty"`
	Version         string            `json:"version,omitempty"`
	Settings        map[string]any    `json:"settings,omitempty"`
	Status          string            `json:"status"` // ok|missing|stale|error
	Notes           string            `json:"notes,omitempty"`
	UpdatedAt       string            `json:"updatedAt"`
}

const cliToolKVPrefix = "clitool:"

func ListCLIToolState(d *sql.DB) ([]CLIToolState, error) {
	return kvList[CLIToolState](d, cliToolKVPrefix)
}

func GetCLIToolState(d *sql.DB, toolID string) (*CLIToolState, error) {
	return kvGetByKey[CLIToolState](d, cliToolKVPrefix+toolID)
}

func UpsertCLIToolState(d *sql.DB, s *CLIToolState) error {
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return kvUpsert(d, cliToolKVPrefix+s.ToolID, s)
}

// ── Generic KV helpers ─────────────────────────────────────────────────

func kvList[T any](d *sql.DB, prefix string) ([]T, error) {
	rows, err := d.Query(`SELECT k, v FROM kv WHERE k LIKE ? ORDER BY k ASC`, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		var t T
		if err := json.Unmarshal([]byte(v), &t); err != nil {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func kvGetByKey[T any](d *sql.DB, key string) (*T, error) {
	row := d.QueryRow(`SELECT v FROM kv WHERE k = ?`, key)
	var v string
	err := row.Scan(&v)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var t T
	if err := json.Unmarshal([]byte(v), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func kvUpsert(d *sql.DB, key string, val any) error {
	v, err := json.Marshal(val)
	if err != nil {
		return err
	}
	_, err = d.Exec(`INSERT INTO kv (k, v, updatedAt) VALUES (?, ?, ?)
		ON CONFLICT(k) DO UPDATE SET v=excluded.v, updatedAt=excluded.updatedAt`,
		key, string(v), time.Now().UTC().Format(time.RFC3339))
	return err
}

func kvDelete(d *sql.DB, key string) error {
	_, err := d.Exec(`DELETE FROM kv WHERE k = ?`, key)
	return err
}
