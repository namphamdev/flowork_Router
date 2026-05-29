// Package localai provides an internal, self-hosted local inference runtime
// untuk Flowork — NO DEPENDENCY ke Ollama atau third-party daemon.
//
// Ayah direct 2026-04-19: *"ollama kita akan hapus dan akan membuat sendiri,
// kita ngak akan tergantung sama olama dan kita akan mandiri"*.
//
// Arsitektur progressive (Phase A → B → C):
//
// ## Phase A — Managed llama.cpp Subprocess (bridge interim)
//
// Flowork bundle `llama-server` binary (llama.cpp HTTP server mode) di release
// per-OS, diletakkan di `~/.flowork/bin/llama-server[.exe]`. Saat agent butuh
// surgical-class task (Healer, dreamstate synthesis, router SurgicalClass),
// Runtime spawn subprocess dengan model GGUF dari `~/.flowork/models/*.gguf`.
//
// Keuntungan vs Ollama:
//   - No external daemon required — 100% under Flowork lifecycle.
//   - Explicit port (random ephemeral), no network registration collision.
//   - User tidak install Ollama — Flowork bundle llama-server binary.
//   - Model file user download sekali (CLI: `flowork model pull <name>`).
//
// Keterbatasan Phase A:
//   - Bundled binary dependency (meski embedded di Flowork release).
//   - Platform-specific binaries (windows/amd64, linux/amd64/arm64, darwin).
//   - Belum 100% "pure Go" per GOL intent — bridge menuju Phase B.
//
// ## Phase B — Pure Go Inference (target final, Sprint 4+)
//
// Tulis inference engine native Go:
//   - GGUF file loader (parser binary format)
//   - Minimal transformer math (attention, matmul, softmax) dengan goroutine
//   - int4/fp16 dequantization
//   - Tokenizer (BPE/tiktoken-compatible)
//   - Sampling (greedy, top-k, top-p, temperature)
//
// Target model awal: Qwen2.5-0.5B-Instruct Q4_K_M (~350MB), CPU only.
// Study reference: go-llama, gomlx, go-transformers — implement sendiri sesuai
// GOL "tidak CGO + tidak Docker + cross-compile clean".
//
// Effort: 2-4 minggu initial working prototype. Performance gap vs llama.cpp
// acceptable untuk Healer task (<2s), optimize later.
//
// ## Phase C — Self-Extending (post-Sprint-5)
//
// Pakai Phase B small model untuk bantu tulis optimisasi ke Phase B sendiri.
// Meta-learning cycle: small model generate diff → Healer verify → apply.
// Align dengan GOL FASE 1 (Self-Mutation) + FASE 11 (AI Create AI).
//
// ## Public API
//
// Runtime interface:
//
//	r, err := localai.Open(localai.Config{Model: "deepseek-coder-v2-lite-q4.gguf"})
//	if err != nil { return err }
//	defer r.Close()
//
//	resp, err := r.Complete(ctx, localai.Request{
//	    Prompt:    "fix this Go code:\n" + brokenCode,
//	    MaxTokens: 512,
//	    Temperature: 0.1,
//	})
//
// Runtime opaque — bisa Phase A (llama.cpp subprocess) atau Phase B (pure Go)
// tanpa caller aware. Migration path: ubah Config.Backend tanpa ubah caller.
//
// ## BudgetGuard Integration
//
// Local inference = $0 cost. TIDAK panggil `finance.BudgetGuard.CheckBudget`
// (justru ini tujuan Two-Caste: Surgical tier bebas budget, Evolution tier
// yang gated). Budget hanya berlaku untuk Cloud provider call.
//
// ## Healer Latency Gates (per Gemini rc120 clarification)
//
// Gemini original spec rc117 "sub-2s" WAS untuk Surgical Replace only
// (repetitive hot-path). rc120 follow-up clarified: greenfield generation
// relax ke 10s. Dua tier deadline:
//
//   - **SurgicalGate = 2.0s** — patch existing code (syntax fix, import
//     reorder, type coercion). Hot-path dipanggil repetitif di mempool.
//     Miss = ErrHealerTimeout → 1x retry queue → Fatal Decoherence rollback.
//
//   - **GreenfieldGate = 10.0s** — generate blok kode dari nol (new function,
//     new file). Expected 500+ token completion di DeepSeek-Coder-V2-Lite Q4.
//     Miss = ErrHealerTimeout tapi tidak Fatal Decoherence — fallback ke
//     Evolution-tier cloud call dengan BudgetGuard gate.
//
// Caller tentukan gate via Request.IsGreenfield flag (belum diimplement,
// pending Sprint 2 wire). Default Request (no flag) = SurgicalGate.
//
// ## Binary Integrity (Anti-RCE — Gemini WAJIB rc120)
//
// Exec binary buatan luar = target RCE paling legit. Sebelum spawn
// llama-server subprocess, WAJIB SHA256 integrity verify via manifest
// `~/.flowork/bin/SHA256SUMS` (atau sibling `<bin>.sha256` fallback).
//
// Mismatch = ErrBinaryIntegrity, kernel-panic untuk localai daemon
// (bukan start subprocess, tidak retry — assume supply-chain compromise).
//
// Dev bypass: `FLOWORK_LOCALAI_SKIP_INTEGRITY=1` dengan warning log.
// Untuk prod WAJIB unset.
//
// Manifest format (GNU coreutils sha256sum compatible):
//
//	<64-hex-hash>  <binary-name>
//	<64-hex-hash>  llama-server.exe    # Windows
//	<64-hex-hash>  llama-server         # Linux/macOS
//
// Flowork release include SHA256SUMS file yang di-sign (future: ed25519
// signature + verify against Flowork pubkey; Sprint 3 deliverable).
//
// ## References di Roadmap
//
// - `docs/plan/ROADMAP_CONSENSUS_rc117_2026-04-19.md` Sprint 2
// - Memory `project_local-ai-mandiri.md`
// - GOL FLOW.md "Don't add CGO dependencies"
// - Quantum FQEC Two-Caste Cloud/Local AI (source blueprint)
package localai
