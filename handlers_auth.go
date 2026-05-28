// Auth HTTP Handlers (login/logout/status/OIDC stub).

package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// authStatusHandler — GET /api/auth/status — return session info if cookie
// or Bearer provided, else { authenticated: false, requireLogin: bool }.
func authStatusHandler(w http.ResponseWriter, r *http.Request) {
	d, _ := store.Open()
	settings, _ := store.LoadSettings(d)
	requireLogin := false
	authMode := "none"
	if settings != nil {
		requireLogin = settings.RequireLogin
		authMode = settings.AuthMode
	}
	token := extractAuthToken(r)
	if token == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
			"requireLogin":  requireLogin,
			"authMode":      authMode,
		})
		return
	}
	s, err := store.GetSessionByToken(d, token)
	if err != nil || s == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
			"requireLogin":  requireLogin,
			"authMode":      authMode,
		})
		return
	}
	_ = store.TouchSession(d, s.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"requireLogin":  requireLogin,
		"authMode":      authMode,
		"session": map[string]any{
			"userId":    s.UserID,
			"createdAt": s.CreatedAt.Format(time.RFC3339),
			"expiresAt": s.ExpiresAt.Format(time.RFC3339),
		},
	})
}

// authLoginHandler — POST { password } (password mode). Returns
// { token, expiresAt } on success. Sets `flow_router_session` cookie.
// Protected by an in-memory progressive lockout (see login_limiter.go).
func authLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ip := clientIPForLock(r)
	if locked, retry := loginCheckLock(ip); locked {
		w.Header().Set("Retry-After", strconvItoa(retry))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":      "too many failed attempts",
			"retryAfter": retry,
		})
		return
	}
	d, _ := store.Open()
	settings, err := store.LoadSettings(d)
	if err != nil {
		http.Error(w, "settings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if settings.AuthMode != "password" {
		http.Error(w, "auth mode != password", http.StatusBadRequest)
		return
	}
	var body struct {
		Password string `json:"password"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !verifyPassword(settings.Password, body.Password) {
		locked, retry := loginRecordFail(ip)
		if locked {
			w.Header().Set("Retry-After", strconvItoa(retry))
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":      "too many failed attempts",
				"retryAfter": retry,
			})
			return
		}
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	loginRecordSuccess(ip)
	userID := body.Username
	if userID == "" {
		userID = "admin"
	}
	s, err := store.CreateSession(d, userID, r.RemoteAddr, r.UserAgent())
	if err != nil {
		http.Error(w, "session: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  s.ExpiresAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"token":     s.Token,
		"expiresAt": s.ExpiresAt.Format(time.RFC3339),
	})
}

// authLogoutHandler — POST clear cookie + delete session row.
func authLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := extractAuthToken(r)
	if token != "" {
		d, _ := store.Open()
		_ = store.DeleteSession(d, token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	writeJSON(w, http.StatusOK, map[string]any{"loggedOut": true})
}

// authOIDCHandler — Phase 1 stub. Returns settings + redirect URL when
// OIDC config present. The code-flow itself lives at /api/auth/oidc/init →
// (browser) → /api/auth/oidc/callback; this endpoint just reports status.
func authOIDCHandler(w http.ResponseWriter, r *http.Request) {
	d, _ := store.Open()
	settings, _ := store.LoadSettings(d)
	if settings == nil || settings.AuthMode != "oidc" {
		http.Error(w, "OIDC not configured (set authMode=oidc + oidcConfig)", http.StatusBadRequest)
		return
	}
	issuer, clientID, _, redirectURI, scopes := oidcConfigFromSettings(settings)
	writeJSON(w, http.StatusOK, map[string]any{
		"authMode":    "oidc",
		"issuer":      issuer,
		"clientId":    clientID,
		"redirectUri": redirectURI,
		"scopes":      scopes,
		"configured":  issuer != "" && clientID != "",
		"startUrl":    "/api/auth/oidc/init",
	})
}

// ── helpers ────────────────────────────────────────────────────────────

const sessionCookieName = "flow_router_session"

func extractAuthToken(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

// hashPassword — argon2id with a per-password random salt, encoded in the
// standard self-describing PHC string ($argon2id$v=19$m=,t=,p=$salt$hash).
// Pure-Go (golang.org/x/crypto/argon2), no CGO.
func hashPassword(plain string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	const (
		t      = 1
		m      = 64 * 1024
		p      = 4
		keyLen = 32
	)
	h := argon2.IDKey([]byte(plain), salt, t, m, p, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, m, t, p,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(h))
}

// hashPasswordSHA — LEGACY SHA256 hex (pre-argon2 installs). Kept only so
// verifyPassword can still authenticate passwords set before the upgrade.
func hashPasswordSHA(plain string) string {
	salt := "flow_router_local_v1"
	h := sha256.Sum256([]byte(salt + ":" + plain))
	return hex.EncodeToString(h[:])
}

// verifyPassword accepts both the argon2id PHC format and the legacy SHA256 hex.
func verifyPassword(stored, plain string) bool {
	if stored == "" || plain == "" {
		return false
	}
	if strings.HasPrefix(stored, "$argon2id$") {
		return verifyArgon2(stored, plain)
	}
	return stored == hashPasswordSHA(plain) // legacy
}

func verifyArgon2(stored, plain string) bool {
	parts := strings.Split(stored, "$") // ["", "argon2id", "v=19", "m=,t=,p=", salt, hash]
	if len(parts) != 6 {
		return false
	}
	var version, m, t, p int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(plain), salt, uint32(t), uint32(m), uint8(p), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
