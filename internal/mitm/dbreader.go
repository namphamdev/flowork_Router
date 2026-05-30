// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — MITM proxy module.

// MITM dbReader (mitm alias cache from kv store).

package mitm

import (
	"database/sql"
	"strings"
	"sync"

	"github.com/flowork-os/flowork_Router/internal/store"
)

var (
	aliasMu    sync.RWMutex
	aliasCache = map[string]string{} // canonical alias key → handler hint
)

// LoadAliasCache refreshes the in-memory cache by scanning the kv table for
// keys with the "mitm:alias:" prefix.
func LoadAliasCache() error {
	d, err := store.Open()
	if err != nil {
		return err
	}
	rows, err := d.Query(`SELECT k, v FROM kv WHERE k LIKE ? ORDER BY k ASC`, "mitm:alias:%")
	if err != nil {
		return err
	}
	defer rows.Close()
	fresh := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		alias := strings.TrimPrefix(k, "mitm:alias:")
		fresh[alias] = v
	}
	aliasMu.Lock()
	aliasCache = fresh
	aliasMu.Unlock()
	return nil
}

// GetMitmAlias resolves rawModel to a canonical alias key.
// Resolution order (matches upstream):
//  1. ModelSynonyms[tool][rawModel] exact map.
//  2. The first matching ModelPatterns[tool] regex.
//  3. ""  when nothing matches (caller should pass rawModel through).
func GetMitmAlias(tool, rawModel string) string {
	if rawModel == "" || tool == "" {
		return ""
	}
	if syn, ok := ModelSynonyms[tool]; ok {
		if alias, ok := syn[rawModel]; ok {
			return alias
		}
	}
	for _, p := range ModelPatterns[tool] {
		if p.Match.MatchString(rawModel) {
			return p.Alias
		}
	}
	return ""
}

// LookupAlias returns the kv "mitm:alias:<key>" value (handler hint) when set.
func LookupAlias(key string) string {
	aliasMu.RLock()
	defer aliasMu.RUnlock()
	return aliasCache[key]
}

// Probe is a helper used by tests to inject custom aliases without touching DB.
func InjectAliasForTest(d *sql.DB, key, value string) error {
	_, err := d.Exec(
		`INSERT INTO kv (k, v, updatedAt) VALUES (?, ?, datetime('now'))
		 ON CONFLICT(k) DO UPDATE SET v = excluded.v, updatedAt = excluded.updatedAt`,
		"mitm:alias:"+key, value)
	return err
}
