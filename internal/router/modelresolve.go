// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Router dispatcher.

// Model Resolution (alias / custom / disabled).

package router

import (
	"database/sql"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// resolveModel maps the requested model through aliases then custom models.
// Returns the effective model and an optional provider pin (provider ID that
// MUST serve it). Unmatched input is returned unchanged with no pin.
func resolveModel(d *sql.DB, model string) (string, string) {
	if aliases, err := store.ListModelAliases(d); err == nil {
		for _, a := range aliases {
			if a.Alias == model {
				return a.Model, a.ProviderID
			}
		}
	}
	if customs, err := store.ListCustomModels(d); err == nil {
		for _, c := range customs {
			if c.Model == model {
				return model, c.ProviderID
			}
		}
	}
	return model, ""
}

// pinProvider keeps only the provider with the given ID; if it's not among the
// model matches, the provider is fetched directly (an alias/custom target model
// need not appear in the provider's auto-models list).
func pinProvider(d *sql.DB, matches []store.ProviderConnection, providerID string) []store.ProviderConnection {
	if providerID == "" {
		return matches
	}
	for _, p := range matches {
		if p.ID == providerID {
			return []store.ProviderConnection{p}
		}
	}
	if p, _ := store.GetProvider(d, providerID); p != nil && p.IsActive {
		return []store.ProviderConnection{*p}
	}
	return nil
}

// filterDisabled drops providers for which (provider, model) is disabled. The
// disabled key may have been stored as the provider type or its ID, so both
// are checked.
func filterDisabled(d *sql.DB, matches []store.ProviderConnection, model string) []store.ProviderConnection {
	var out []store.ProviderConnection
	for _, p := range matches {
		if store.IsModelDisabled(d, p.Provider, model) || store.IsModelDisabled(d, p.ID, model) {
			continue
		}
		out = append(out, p)
	}
	return out
}
