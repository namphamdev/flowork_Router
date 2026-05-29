// Package provider — chain_mode.go: 4-mode provider chain selector (rc180).
//
// Ayah's spec 2026-04-26:
//   Mode 1: Cloud (OpenRouter/Nvidia multi-key rotation)
//   Mode 2: Local Only (llama-server + auto-pick largest gguf)
//   Mode 3: Brain Only (V4 + cached_reasoning, future)
//   Mode 4: Auto-switch — [1] habis → [2] local → [3] brain (recommended default)
//
// Selector via env FLOWORK_PROVIDER_MODE (default 4).
//
// Phase 1 (rc180): Mode 4 minimum — chain OpenAI primary → LocalLlama
// auto-pick gguf → BrainProvider stub. Multi-key rotation (Mode 1
// expansion) di-defer ke phase 2.
package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProviderMode enum.
type ProviderMode int

const (
	ModeAutoSwitch  ProviderMode = 4 // default — full chain cloud → local → brain
	ModeCloudOnly   ProviderMode = 1 // OpenRouter only (multi-key future)
	ModeLocalOnly   ProviderMode = 2 // llama-server only
	ModeBrainOnly   ProviderMode = 3 // brain provider only (V4 + cache)
)

// ResolveMode read env FLOWORK_PROVIDER_MODE, return mode atau default
// ModeAutoSwitch (4).
func ResolveMode() ProviderMode {
	raw := strings.TrimSpace(os.Getenv("FLOWORK_PROVIDER_MODE"))
	if raw == "" {
		return ModeAutoSwitch
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 4 {
		fmt.Fprintf(os.Stderr, "provider/chain: invalid FLOWORK_PROVIDER_MODE=%q, fallback ke 4 (auto-switch)\n", raw)
		return ModeAutoSwitch
	}
	return ProviderMode(n)
}

// FindLargestLocalGGUF scan <project_root>/models/*.gguf, return basename
// of largest file (heuristic: bigger = better fallback teacher). Empty
// kalau ga ada gguf — caller skip LocalLlama dari chain.
//
// 2026-05-06 portable fix per RULE EMAS §1.4: pindah dari user-home
// .flowork/models/ ke project_root/models/. Convention: kalau workspace
// = floworkos-go (daemon), naik 1 level ke project root.
func FindLargestLocalGGUF(workspace string) string {
	projectRoot := workspace
	if filepath.Base(workspace) == "floworkos-go" {
		projectRoot = filepath.Dir(workspace)
	}
	dir := filepath.Join(projectRoot, "models")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var bestName string
	var bestSize int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gguf") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() > bestSize {
			bestSize = info.Size()
			bestName = e.Name()
		}
	}
	return bestName
}

// BuildChain compose FallbackClient sesuai mode + workspace.
//
// Args:
//   primary — primary cloud client (OpenRouter via openai-compatible).
//             Required untuk Mode 1, 4. Mode 2/3 ignore.
//   workspace — path workspace root untuk auto-pick gguf + brain DB access.
//
// Return *FallbackClient yang ready dipake. Empty chain kalau mode mismatch
// dengan available clients (e.g. Mode 2 tapi ga ada gguf).
func BuildChain(primary Client, workspace string) Client {
	mode := ResolveMode()
	chain := []Client{}

	switch mode {
	case ModeCloudOnly:
		// Mode 1: cuma cloud primary. (Multi-key rotation Phase 2.)
		if primary != nil {
			chain = append(chain, primary)
		}

	case ModeLocalOnly:
		// Mode 2: cuma local llama. Auto-pick largest gguf.
		if local := buildLocalClient(workspace); local != nil {
			chain = append(chain, local)
		}

	case ModeBrainOnly:
		// Mode 3: cuma brain (cached_reasoning + V4 future).
		chain = append(chain, NewBrainProvider(BrainProviderConfig{
			Workspace: workspace,
		}))

	case ModeAutoSwitch:
		// Mode 4 (DEFAULT): full chain — primary → NVIDIA NIM → local → brain.
		// Order: cheap+available first, escalate kalau habis/down.
		// rc181: NVIDIA NIM inserted between OpenRouter (primary) dan local
		// llama. NVIDIA quota independent dari OpenRouter — kalau OpenRouter
		// habis tapi NVIDIA masih ada credit, escalate ke NVIDIA dulu (cloud,
		// faster) sebelum ke local (slower, ~10-15 tok/s hybrid GPU).
		if primary != nil {
			chain = append(chain, primary)
		}
		if nv := buildNvidiaClient(); nv != nil {
			chain = append(chain, nv)
		}
		if local := buildLocalClient(workspace); local != nil {
			chain = append(chain, local)
		}
		chain = append(chain, NewBrainProvider(BrainProviderConfig{
			Workspace: workspace,
		}))
	}

	if len(chain) == 0 {
		// Empty chain — return primary single OR error wrapper.
		if primary != nil {
			return primary
		}
		// Both nil — return brain stub yang always ErrBrainNotReady.
		return NewBrainProvider(BrainProviderConfig{Workspace: workspace})
	}
	if len(chain) == 1 {
		return chain[0]
	}

	fb := NewFallbackClient(FallbackConfig{}, chain...)
	fmt.Fprintf(os.Stderr, "provider/chain: built mode=%d, %d clients: %s\n",
		mode, len(chain), describeChain(chain))
	_ = mode // silence Go's strict unused-variable in some paths
	return fb
}

// buildNvidiaClient try create NVIDIA NIM client (OpenAI-compatible) kalau
// NVIDIA_API_KEY env set. Return nil kalau key kosong → chain skip NVIDIA.
//
// rc181: NVIDIA NIM endpoint https://integrate.api.nvidia.com/v1
// Default model dari NVIDIA_DEFAULT_MODEL env (e.g. deepseek-ai/deepseek-v3.2).
// Free tier credit gede + frontier models (DeepSeek V3.2, Llama-3.3-70B,
// Qwen3-Coder-480B, Mixtral-8x22B, Nemotron-Super-49B, dll).
//
// Phase 2 future: multi-key rotation via NVIDIA_API_KEYS=k1,k2,k3 (round-robin).
func buildNvidiaClient() Client {
	key := strings.TrimSpace(os.Getenv("NVIDIA_API_KEY"))
	if key == "" {
		return nil
	}
	model := strings.TrimSpace(os.Getenv("NVIDIA_DEFAULT_MODEL"))
	if model == "" {
		model = "deepseek-ai/deepseek-v3.2"
	}
	cli, err := NewOpenAIClient(OpenAIConfig{
		APIKey:  key,
		BaseURL: "https://integrate.api.nvidia.com/v1",
		Model:   model,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider/chain: build nvidia failed: %v\n", err)
		return nil
	}
	return cli
}

// buildLocalClient try create LocalLlamaClient pakai largest gguf available.
// Return nil kalau ga ada gguf (chain skip local segment).
//
// rc183 fix: ContextSize 4096 too small. Live trace 2026-04-26 show prompt
// 26,422 tokens (system prompt persona + brain v2 memory + doktrin + chat
// history) > 4096 → llama-server return 400 "exceeds available context".
// Bump ke 32768 (32K) — Qwen3.6-27B support up to 128K. RTX 4060 KV cache
// budget 32K = ~2-3GB VRAM (manageable dengan partial offload).
func buildLocalClient(workspace string) Client {
	gguf := FindLargestLocalGGUF(workspace)
	if gguf == "" {
		return nil
	}
	cli, err := NewLocalLlamaClient(LocalLlamaConfig{
		Model:       gguf,
		ContextSize: 32768,
		// GPULayers 0 = let llama-server auto-fit (rc179 fix).
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "provider/chain: build local-llama failed: %v\n", err)
		return nil
	}
	return cli
}

// describeChain return comma-separated client names untuk log.
func describeChain(chain []Client) string {
	var names []string
	for _, c := range chain {
		names = append(names, c.Name())
	}
	return strings.Join(names, " → ")
}
