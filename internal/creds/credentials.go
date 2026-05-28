// Claude Subscription Credential Reader.

package creds

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CredentialsFile shape — minimal projection. Kita cuma butuh accessToken
// + refreshToken + expiresAt. organizationUuid optional (mungkin diperlukan
// di future untuk Anthropic header).
type CredentialsFile struct {
	ClaudeAiOauth struct {
		AccessToken      string   `json:"accessToken"`
		RefreshToken     string   `json:"refreshToken"`
		ExpiresAt        int64    `json:"expiresAt"`
		Scopes           []string `json:"scopes"`
		SubscriptionType string   `json:"subscriptionType"`
		RateLimitTier    string   `json:"rateLimitTier"`
	} `json:"claudeAiOauth"`
	OrganizationUUID string `json:"organizationUuid"`
}

var (
	cachedMu       sync.Mutex
	cachedCreds    *CredentialsFile
	cachedLoadedAt time.Time
	cacheValidity  = 30 * time.Second // re-read disk every 30s max
)

// credentialsPath returns the canonical path. Override via FLOW_CREDS_PATH env
// untuk testing.
func credentialsPath() string {
	if p := os.Getenv("FLOW_CREDS_PATH"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", ".credentials.json")
}

// Load reads credentials from disk dengan in-process cache 30s.
// Returns error kalau file ngga ada atau parse gagal.
// Anti-leak: ngga log token value, cuma masked prefix.
func Load() (*CredentialsFile, error) {
	cachedMu.Lock()
	defer cachedMu.Unlock()

	if cachedCreds != nil && time.Since(cachedLoadedAt) < cacheValidity {
		return cachedCreds, nil
	}

	p := credentialsPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("creds: read %s: %w", p, err)
	}

	var c CredentialsFile
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("creds: parse %s: %w", p, err)
	}

	if c.ClaudeAiOauth.AccessToken == "" {
		return nil, fmt.Errorf("creds: accessToken empty di %s — login Claude Code dulu", p)
	}

	cachedCreds = &c
	cachedLoadedAt = time.Now()
	return cachedCreds, nil
}

// IsExpired check apakah token udah expired berdasarkan expiresAt (unix ms).
// Buffer 60s biar refresh duluan sebelum benar-benar expired.
func (c *CredentialsFile) IsExpired() bool {
	if c.ClaudeAiOauth.ExpiresAt == 0 {
		return false // unknown, assume valid
	}
	expiry := time.UnixMilli(c.ClaudeAiOauth.ExpiresAt)
	return time.Now().Add(60 * time.Second).After(expiry)
}

// MaskedAccessToken returns token dengan most chars masked — buat log.
// Format: "sk-ant-oat...XXXX...[masked, total Y chars]"
func (c *CredentialsFile) MaskedAccessToken() string {
	t := c.ClaudeAiOauth.AccessToken
	if len(t) < 20 {
		return "[masked]"
	}
	return t[:10] + "...[masked total " + fmt.Sprintf("%d", len(t)) + " chars]"
}

// InvalidateCache forces next Load() to re-read disk.
// Useful setelah refresh token (future).
func InvalidateCache() {
	cachedMu.Lock()
	defer cachedMu.Unlock()
	cachedCreds = nil
	cachedLoadedAt = time.Time{}
}
