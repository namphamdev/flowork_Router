package provider

// brain_provider.go — Provider yang serve response dari brain (cached_reasoning
// + V4 native transformer). Phase 1 stub: cache lookup only. Phase 4 (saat
// V4 matured): plug V4 inference real.
//
// rc180: bagian dari 4-mode provider chain Ayah:
//   Mode 1: Cloud (OpenRouter/Nvidia rotation)
//   Mode 2: Local llama-server
//   Mode 3: Brain (this provider) — V4 + cached_reasoning
//   Mode 4: Auto-switch [1] → [2] → [3]
//
// Brain provider selalu return ErrBrainNotReady kalau:
//   - cached_reasoning miss (no exact match)
//   - V4 model belum trained matured (Phase 4 future)
//
// Caller (FallbackClient) treat error sebagai recoverable + escalate ke
// next provider (kalau ada).

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrBrainNotReady signals brain provider can't serve request — escalate
// ke next provider di chain.
var ErrBrainNotReady = errors.New("provider/brain: brain not ready (cache miss + V4 not matured)")

// BrainProviderConfig parameter inisialisasi.
type BrainProviderConfig struct {
	// Workspace path untuk akses brain.sqlite
	Workspace string

	// MinConfidence threshold untuk cached_reasoning hit (0-1).
	// Default 0.7 = cuma cache yang confidence tinggi yang return.
	MinConfidence float64
}

// BrainProvider implementasi Client yang serve response dari brain layer.
type BrainProvider struct {
	cfg BrainProviderConfig
}

// NewBrainProvider build provider. Lazy init brain DB di first call.
func NewBrainProvider(cfg BrainProviderConfig) *BrainProvider {
	if cfg.MinConfidence == 0 {
		cfg.MinConfidence = 0.7
	}
	return &BrainProvider{cfg: cfg}
}

// Name return identifier untuk logging.
func (b *BrainProvider) Name() string { return "brain" }

// Complete try cached_reasoning lookup. Phase 1 stub: kalau ada exact match
// query_hash, return cached response. Else escalate via ErrBrainNotReady.
//
// Phase 4 (future): plug V4 native Go transformer inference. Saat V4 trained
// matured + accuracy >threshold, replace stub dengan V4 real.
func (b *BrainProvider) Complete(ctx context.Context, req Request) (Response, error) {
	// Concat user messages untuk query lookup.
	var query strings.Builder
	for _, m := range req.Messages {
		if m.Role == RoleUser {
			query.WriteString(m.Content)
			query.WriteString(" ")
		}
	}
	q := strings.TrimSpace(query.String())
	if q == "" {
		return Response{}, fmt.Errorf("%w: empty query", ErrBrainNotReady)
	}

	// Phase 1 stub: cached_reasoning lookup ga di-wire dulu (avoid cyclic
	// import internal/brain ↔ internal/provider). Real wiring di Phase 4
	// — bikin internal/brain/brainprovider.go yang DEPEND ke provider, plus
	// flowork-chat compose chain dengan that wrapper.
	//
	// Untuk now, always return ErrBrainNotReady → FallbackClient escalate
	// ke next provider (atau final error kalau brain adalah last in chain).
	_ = b.cfg.Workspace
	return Response{}, fmt.Errorf("%w: stub mode (Phase 4 V4 belum matured)", ErrBrainNotReady)
}
