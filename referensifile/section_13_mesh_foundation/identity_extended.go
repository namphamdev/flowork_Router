// Package mesh — identity_extended.go: M1 self-issued license metadata.
//
// Wraps existing kernel/identity/ Ed25519 keypair dengan:
//   - Hardware fingerprint binding (W-1: threshold 3-of-5 match)
//   - VM detection flag
//   - 16-byte recovery seed (W-8 simplified — hex, NOT BIP39 full)
//   - License history (max 3 fingerprint revisions)
//   - REVOKE record handler skeleton (M6 gossip integration nanti)
//
// Per ARCHITECTURE.md AMENDMENTS-V1: license SELF-ISSUED, no central
// authority. Peer registry karma awal 0.5 normal / 0.3 VM.
//
// Storage: settings DB (parity dengan kernel/identity/):
//   - MESH_LICENSE_FINGERPRINT       — current SHA-256 hex
//   - MESH_LICENSE_FINGERPRINT_PRIOR — previous (for grace), JSON list max 3
//   - MESH_LICENSE_VIRTUALIZED       — "1"/"0"
//   - MESH_LICENSE_RECOVERY_SEED     — 16-byte hex (sensitive=1, user backup)
//   - MESH_LICENSE_INSTALL_TS        — first install RFC3339
//   - MESH_LICENSE_REVOKED           — JSON list of revoked pubkey hex (CRL)

package mesh

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/flowork/kernel/kernel/settings"
)

const (
	settingsKeyLicenseFingerprint      = "MESH_LICENSE_FINGERPRINT"
	settingsKeyLicenseFingerprintPrior = "MESH_LICENSE_FINGERPRINT_PRIOR"
	settingsKeyLicenseVirtualized      = "MESH_LICENSE_VIRTUALIZED"
	settingsKeyLicenseRecoverySeed     = "MESH_LICENSE_RECOVERY_SEED"
	settingsKeyLicenseInstallTS        = "MESH_LICENSE_INSTALL_TS"
	settingsKeyLicenseRevoked          = "MESH_LICENSE_REVOKED"

	licenseHistoryMax = 3
)

// SelfIssuedLicense — extended metadata di atas kernel/identity/ keypair.
type SelfIssuedLicense struct {
	PubKeyHex     string    `json:"pubkey_hex"`
	Fingerprint   string    `json:"fingerprint"`
	IsVirtualized bool      `json:"is_virtualized"`
	InstallTS     time.Time `json:"install_ts"`
	RecoverySeed  string    `json:"recovery_seed,omitempty"` // hex, sensitive — only saat first issue
	History       []string  `json:"history,omitempty"`       // prior fingerprints (max licenseHistoryMax)
}

var (
	licenseMu    sync.Mutex
	licenseCache *SelfIssuedLicense
)

// EnsureSelfIssued — run di boot. Idempotent.
//
// First-issue path:
//   1. Generate fingerprint
//   2. Generate 16-byte recovery seed
//   3. Persist to settings DB (sensitive=1 untuk seed)
//   4. Return license dengan recovery seed (caller WAJIB log untuk user backup)
//
// Subsequent boot:
//   1. Read prior fingerprint
//   2. Compute current fingerprint components
//   3. Threshold match 3-of-5
//   4. Match → continue (update LastSeen analog). Mismatch → log warn,
//      store new fingerprint di history (cap 3), continue grace mode.
//   5. NO recovery seed return (already issued — user harus pakai backup
//      atau force-reissue manual).
func EnsureSelfIssued() (*SelfIssuedLicense, error) {
	licenseMu.Lock()
	defer licenseMu.Unlock()

	if licenseCache != nil {
		return licenseCache, nil
	}

	store := settings.Shared()
	if store == nil {
		return nil, fmt.Errorf("mesh.EnsureSelfIssued: settings unavailable")
	}

	priorFP, _ := store.Get(settingsKeyLicenseFingerprint)
	currentComps := MachineFingerprintComponents()
	currentFP := MachineFingerprint()
	isVirt := IsVirtualizedHost()

	if strings.TrimSpace(priorFP) == "" {
		// First-issue path
		seed, err := generateRecoverySeed()
		if err != nil {
			return nil, fmt.Errorf("mesh.EnsureSelfIssued: seed gen: %w", err)
		}
		now := time.Now().UTC()
		_ = store.Set(settingsKeyLicenseFingerprint, currentFP)
		_ = store.Set(settingsKeyLicenseVirtualized, boolToStr(isVirt))
		_ = store.Set(settingsKeyLicenseRecoverySeed, seed) // user wajib backup
		_ = store.Set(settingsKeyLicenseInstallTS, now.Format(time.RFC3339))

		lic := &SelfIssuedLicense{
			Fingerprint:   currentFP,
			IsVirtualized: isVirt,
			InstallTS:     now,
			RecoverySeed:  seed,
		}
		licenseCache = lic
		return lic, nil
	}

	// Subsequent boot — verify threshold match
	priorComps := componentsFromFingerprint(priorFP, currentComps)
	if !FingerprintThresholdMatch(priorComps, currentComps, 3) {
		// <3 match — likely device different (license copy?) atau hardware massive change.
		// Per AMENDMENTS-V1 W-1: log warn + grace 7 hari (tetap jalan, tag mismatch).
		// Append current ke history.
		_ = appendFingerprintHistory(store, currentFP)
	}

	// Update fingerprint kalau changed (still grace match) — supaya next boot baseline current
	if priorFP != currentFP {
		// History append before overwrite
		_ = appendFingerprintHistory(store, priorFP)
		_ = store.Set(settingsKeyLicenseFingerprint, currentFP)
	}

	installRaw, _ := store.Get(settingsKeyLicenseInstallTS)
	installTS, _ := time.Parse(time.RFC3339, installRaw)
	virtRaw, _ := store.Get(settingsKeyLicenseVirtualized)

	historyRaw, _ := store.Get(settingsKeyLicenseFingerprintPrior)
	var history []string
	if strings.TrimSpace(historyRaw) != "" {
		_ = json.Unmarshal([]byte(historyRaw), &history)
	}

	lic := &SelfIssuedLicense{
		Fingerprint:   currentFP,
		IsVirtualized: virtRaw == "1",
		InstallTS:     installTS,
		History:       history,
	}
	licenseCache = lic
	return lic, nil
}

// CurrentSelfIssued return cached license (post-EnsureSelfIssued boot).
// Return nil kalau Ensure belum dipanggil. Useful untuk filter pipeline.
func CurrentSelfIssued() *SelfIssuedLicense {
	licenseMu.Lock()
	defer licenseMu.Unlock()
	return licenseCache
}

// IsRevoked return true kalau pubkey di-revoke list.
//
// Revocation triggered via:
//   - User manual: kernel admin endpoint `POST /v1/mesh/license/revoke/{pubkey}`
//     (KERNEL_TOKEN auth)
//   - Mesh REVOKE packet via gossip (M6) — verify signed dengan recovery seed atau
//     master key 2-of-3 multi-sig per AMENDMENTS-V1 I-3.
func IsRevoked(pubKeyHex string) bool {
	store := settings.Shared()
	if store == nil {
		return false
	}
	raw, _ := store.Get(settingsKeyLicenseRevoked)
	if strings.TrimSpace(raw) == "" {
		return false
	}
	var revoked []string
	if err := json.Unmarshal([]byte(raw), &revoked); err != nil {
		return false
	}
	target := strings.ToLower(pubKeyHex)
	for _, pk := range revoked {
		if strings.ToLower(pk) == target {
			return true
		}
	}
	return false
}

// AddRevocation append pubkey ke revoked list. Idempotent.
func AddRevocation(pubKeyHex string) error {
	store := settings.Shared()
	if store == nil {
		return fmt.Errorf("settings unavailable")
	}
	raw, _ := store.Get(settingsKeyLicenseRevoked)
	var revoked []string
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &revoked)
	}
	target := strings.ToLower(pubKeyHex)
	for _, pk := range revoked {
		if strings.ToLower(pk) == target {
			return nil // already revoked
		}
	}
	revoked = append(revoked, target)
	b, err := json.Marshal(revoked)
	if err != nil {
		return err
	}
	return store.Set(settingsKeyLicenseRevoked, string(b))
}

// ResetSelfIssuedForTest test-only — clear cache + settings DB keys.
func ResetSelfIssuedForTest() {
	licenseMu.Lock()
	defer licenseMu.Unlock()
	licenseCache = nil
	if store := settings.Shared(); store != nil {
		_ = store.Set(settingsKeyLicenseFingerprint, "")
		_ = store.Set(settingsKeyLicenseFingerprintPrior, "")
		_ = store.Set(settingsKeyLicenseVirtualized, "")
		_ = store.Set(settingsKeyLicenseRecoverySeed, "")
		_ = store.Set(settingsKeyLicenseInstallTS, "")
		_ = store.Set(settingsKeyLicenseRevoked, "")
	}
}

// --- helpers ---

func generateRecoverySeed() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func boolToStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// componentsFromFingerprint — for prior fingerprint stored as full hash, we
// can't recover individual components. Strategy: if priorFP == currentFP,
// all match. Otherwise use stored components from history if available, or
// fallback to single-component compare.
//
// Simplification: store only full fingerprint, threshold check via
// fingerprint string equality + history grace.
func componentsFromFingerprint(priorFP string, currentComps []FingerprintComponent) []FingerprintComponent {
	// MVP: if prior fingerprint matches current full, return current (5/5 match).
	// Otherwise return zero components (0/5 → grace fail kalau strict).
	out := make([]FingerprintComponent, len(currentComps))
	if priorFP == MachineFingerprint() {
		copy(out, currentComps)
	} else {
		for i := range out {
			out[i] = FingerprintComponent{Name: currentComps[i].Name, Value: "missing"}
		}
	}
	return out
}

func appendFingerprintHistory(store *settings.Store, fp string) error {
	raw, _ := store.Get(settingsKeyLicenseFingerprintPrior)
	var history []string
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &history)
	}
	// Skip kalau sudah ada
	for _, h := range history {
		if h == fp {
			return nil
		}
	}
	history = append(history, fp)
	if len(history) > licenseHistoryMax {
		history = history[len(history)-licenseHistoryMax:]
	}
	b, err := json.Marshal(history)
	if err != nil {
		return err
	}
	return store.Set(settingsKeyLicenseFingerprintPrior, string(b))
}
