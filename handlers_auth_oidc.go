// OIDC Code Flow + Session Enforce Middleware (B2).

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// ── OIDC discovery cache ────────────────────────────────────────────────

type oidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JwksURI               string `json:"jwks_uri"`
}

func discoverOIDC(ctx context.Context, issuer string) (*oidcDiscovery, error) {
	u := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := providerProbeClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery %d", resp.StatusCode)
	}
	var d oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}
	return &d, nil
}

func oidcConfigFromSettings(s *store.Settings) (issuer, clientID, clientSecret, redirectURI, scopes string) {
	if s == nil || s.OidcConfig == nil {
		return
	}
	get := func(k string) string {
		if v, ok := s.OidcConfig[k].(string); ok {
			return v
		}
		return ""
	}
	issuer = get("issuer")
	clientID = get("clientId")
	clientSecret = get("clientSecret")
	redirectURI = get("redirectUri")
	scopes = get("scopes")
	if scopes == "" {
		scopes = "openid profile email"
	}
	if redirectURI == "" {
		redirectURI = "http://127.0.0.1:2402/api/auth/oidc/callback"
	}
	return
}

// authOIDCInitHandler — GET/POST /api/auth/oidc/init → returns authorize URL.
func authOIDCInitHandler(w http.ResponseWriter, r *http.Request) {
	d, _ := store.Open()
	settings, _ := store.LoadSettings(d)
	if settings == nil || settings.AuthMode != "oidc" {
		http.Error(w, "OIDC not configured (set authMode=oidc + oidcConfig)", http.StatusBadRequest)
		return
	}
	issuer, clientID, _, redirectURI, scopes := oidcConfigFromSettings(settings)
	if issuer == "" || clientID == "" {
		http.Error(w, "oidcConfig requires issuer + clientId", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	disc, err := discoverOIDC(ctx, issuer)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "discovery: " + err.Error()})
		return
	}
	stateB := make([]byte, 16)
	_, _ = rand.Read(stateB)
	state := hex.EncodeToString(stateB)
	nonceB := make([]byte, 16)
	_, _ = rand.Read(nonceB)
	nonce := hex.EncodeToString(nonceB)
	// Stash pending state in oauthTokens kv
	_ = store.UpsertOAuthToken(d, &store.OAuthTokenRecord{
		Provider:  "oidc:pending",
		TokenType: "oidc-pending",
		Extra: map[string]any{
			"state": state, "nonce": nonce, "issuer": issuer,
			"tokenEndpoint": disc.TokenEndpoint, "userinfoEndpoint": disc.UserinfoEndpoint,
			"jwksUri":   disc.JwksURI,
			"expiresAt": time.Now().Add(10 * time.Minute).Format(time.RFC3339),
		},
	})
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", scopes)
	q.Set("state", state)
	q.Set("nonce", nonce)
	authURL := disc.AuthorizationEndpoint + "?" + q.Encode()
	writeJSON(w, http.StatusOK, map[string]any{"authUrl": authURL, "state": state})
}

// authOIDCCallbackHandler — GET /api/auth/oidc/callback?code=&state=
// Exchanges code → tokens, fetches userinfo, creates a flow_router session,
// sets cookie, then redirects to dashboard root.
func authOIDCCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	d, _ := store.Open()
	settings, _ := store.LoadSettings(d)
	issuer, clientID, clientSecret, redirectURI, _ := oidcConfigFromSettings(settings)
	_ = issuer
	pending, _ := store.GetOAuthToken(d, "oidc:pending")
	if pending == nil {
		http.Error(w, "no pending OIDC init", http.StatusBadRequest)
		return
	}
	extra, _ := pending.Extra.(map[string]any)
	storedState, _ := extra["state"].(string)
	if extra == nil || !constantTimeEqualString(storedState, state) {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	tokenEndpoint, _ := extra["tokenEndpoint"].(string)
	userinfoEndpoint, _ := extra["userinfoEndpoint"].(string)

	// Exchange code → tokens
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := providerProbeClient.Do(req)
	if err != nil {
		http.Error(w, "token exchange: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	var tok struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tok)
	if tok.AccessToken == "" {
		http.Error(w, "no access_token from IdP", http.StatusBadGateway)
		return
	}

	// Verify the id_token (RS256 signature via JWKS + iss/aud/exp/nonce). When
	// the IdP advertised a jwks_uri and returned an id_token, a failure here is
	// fatal — we never trust an unverified assertion. The verified `sub`/`email`
	// claim becomes the session subject (preferred over the userinfo call).
	jwksURI, _ := extra["jwksUri"].(string)
	expectIssuer, _ := extra["issuer"].(string)
	nonce, _ := extra["nonce"].(string)
	var verifiedSubject string
	if jwksURI != "" && tok.IDToken != "" {
		claims, verr := verifyOIDCIDToken(ctx, jwksURI, tok.IDToken, expectIssuer, clientID, nonce)
		if verr != nil {
			http.Error(w, "id_token verification failed: "+verr.Error(), http.StatusUnauthorized)
			return
		}
		if e, ok := claims["email"].(string); ok && e != "" {
			verifiedSubject = e
		} else if s, ok := claims["sub"].(string); ok {
			verifiedSubject = s
		}
	}

	// Fetch userinfo for the subject
	userID := "oidc-user"
	if verifiedSubject != "" {
		userID = verifiedSubject
	} else if userinfoEndpoint != "" {
		ureq, _ := http.NewRequestWithContext(ctx, http.MethodGet, userinfoEndpoint, nil)
		ureq.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		if uresp, err := providerProbeClient.Do(ureq); err == nil {
			defer uresp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(uresp.Body, 64*1024))
			var info map[string]any
			if json.Unmarshal(body, &info) == nil {
				if sub, ok := info["email"].(string); ok && sub != "" {
					userID = sub
				} else if sub, ok := info["sub"].(string); ok {
					userID = sub
				}
			}
		}
	}
	s, err := store.CreateSession(d, userID, r.RemoteAddr, r.UserAgent())
	if err != nil {
		http.Error(w, "session: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = store.DeleteOAuthToken(d, "oidc:pending")
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: s.Token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: s.ExpiresAt,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// ── Session enforce middleware ──────────────────────────────────────────

// authEnforceMiddleware wraps the mux. When settings.RequireLogin is true
// and authMode != none, a valid session is required for protected paths.
// Exemptions keep API + health + login reachable.
func authEnforceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !pathRequiresSession(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		d, err := store.Open()
		if err != nil {
			next.ServeHTTP(w, r) // fail-open on store error (never lock out)
			return
		}
		settings, _ := store.LoadSettings(d)
		if settings == nil || !settings.RequireLogin || settings.AuthMode == "none" {
			next.ServeHTTP(w, r) // enforcement disabled
			return
		}
		token := extractAuthToken(r)
		if token != "" {
			if s, err := store.GetSessionByToken(d, token); err == nil && s != nil {
				_ = store.TouchSession(d, s.ID)
				next.ServeHTTP(w, r)
				return
			}
		}
		// Unauthenticated on a protected path.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":        "authentication required",
				"requireLogin": true,
				"authMode":     settings.AuthMode,
			})
			return
		}
		// Dashboard HTML → redirect to login page (served by index with ?login)
		http.Redirect(w, r, "/?login=1", http.StatusFound)
	})
}

// pathRequiresSession — protected unless explicitly exempt.
func pathRequiresSession(p string) bool {
	exempt := []string{
		"/api/auth/", // login/logout/status/oidc themselves
		"/v1/",       // API-key authenticated, not session
		"/v1beta/",   // Gemini-shape API endpoints, API-key authenticated
		"/healthz",
		"/api/health",
		"/api/shutdown", // local control
		"/favicon",
		"/static/",
	}
	for _, e := range exempt {
		if strings.HasPrefix(p, e) {
			return false
		}
	}
	// Root "/" is the dashboard shell — allow (login UI lives there).
	if p == "/" {
		return false
	}
	return true
}
