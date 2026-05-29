// manifest_sign.go — rc154 opus-2. Ed25519 signature verify untuk
// internal/localai/manifest.json supaya supply chain tidak bisa di-swap
// dengan URL/SHA256 mutated diff (per Gemini rc120 spec + EXTBUG-012
// PKI tie-in).
//
// Reuse BFT keygen infra dari internal/bft (rc140) — tiap agent punya
// Ed25519 keypair di state/bft/<agent>.key. Manifest signing pakai key
// yang sama, sehingga PKI baseline bisa share dengan BFT quorum gate.
//
// Wire pattern:
//
//	cmd/flowork-bin sign-manifest    -> sign manifest.json + write .sig file
//	flowork-bin pull <name>          -> verify .sig + manifest match before pull
//	flowork-bin verify-manifest      -> standalone verify
//
// Sig file format: hex-encoded Ed25519 sig over canonical(manifest.json bytes)
// stored di internal/localai/manifest.json.sig (next to manifest).
package localai

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// SignManifest signs manifestPath bytes with the given Ed25519 private key
// and writes hex-encoded signature to manifestPath + ".sig".
//
// Caller responsibility: pass the agent's BFT private key (from
// bft.LoadOrCreateKey) — we don't import internal/bft here to avoid cycle.
func SignManifest(manifestPath string, privKey ed25519.PrivateKey) error {
	if len(privKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("localai sign: invalid private key size %d", len(privKey))
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("localai sign: read manifest: %w", err)
	}
	sig := ed25519.Sign(privKey, data)
	sigHex := hex.EncodeToString(sig)
	sigPath := manifestPath + ".sig"
	tmp := sigPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(sigHex), 0o644); err != nil {
		return fmt.Errorf("localai sign: write sig: %w", err)
	}
	return os.Rename(tmp, sigPath)
}

// ErrManifestSigInvalid — verification failure (bad sig OR mismatch).
var ErrManifestSigInvalid = errors.New("localai: manifest signature invalid")

// ErrManifestSigMissing — sig file not found. Caller decides whether to
// reject (strict mode) or proceed with warning (legacy compat).
var ErrManifestSigMissing = errors.New("localai: manifest signature file missing")

// VerifyManifest checks manifestPath bytes against manifestPath+".sig"
// using pubKey. Returns ErrManifestSigMissing if .sig absent (caller
// decides policy), ErrManifestSigInvalid on bad signature.
func VerifyManifest(manifestPath string, pubKey ed25519.PublicKey) error {
	if len(pubKey) != ed25519.PublicKeySize {
		return fmt.Errorf("localai verify: invalid pubkey size %d", len(pubKey))
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("localai verify: read manifest: %w", err)
	}
	sigHex, err := os.ReadFile(manifestPath + ".sig")
	if err != nil {
		if os.IsNotExist(err) {
			return ErrManifestSigMissing
		}
		return fmt.Errorf("localai verify: read sig: %w", err)
	}
	sig, err := hex.DecodeString(string(sigHex))
	if err != nil {
		return fmt.Errorf("localai verify: decode sig hex: %w", err)
	}
	if !ed25519.Verify(pubKey, data, sig) {
		return ErrManifestSigInvalid
	}
	return nil
}
