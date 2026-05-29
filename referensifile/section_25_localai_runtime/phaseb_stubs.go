// Package localai — Phase B Pure Go Inference SCAFFOLDING (C-01 audit).
//
// C-01 status: SCAFFOLDING ship rc173 2026-04-20. Real implementation =
// 2-4 minggu effort per doc.go Phase B spec. File ini menyediakan
// interface contract + explicit NotYetImplementedError sentinel supaya:
//  1. Caller bisa pattern-match signature sekarang (Phase A → B swap
//     later = zero call-site change).
//  2. Future impl session tahu exact entry point + contract.
//  3. ErrPhaseBNotImpl trackable via grep — explicit rather than
//     mysterious runtime panic.
//
// Ayah acc penuh 2026-04-20 untuk 4 BLOCKED items. Honest disclosure:
// Phase B inference engine (GGUF parser + tokenizer + transformer math)
// TIDAK feasible single-sesi. Scaffolding ini = commitment visible ke
// future self + generasi AI berikut yang akan pick up work ini.
package localai

import (
	"context"
	"errors"
	"fmt"
)

// ErrPhaseBNotImpl adalah sentinel yang Phase B stub return. Caller bisa
// errors.Is untuk detect "feature pending" vs real error.
var ErrPhaseBNotImpl = errors.New("localai Phase B inference: NOT YET IMPLEMENTED — SCAFFOLDING only, pending multi-week effort rc173+")

// PhaseBBackend adalah identifier untuk runtime config. Caller yang pass
// Config.Backend = BackendPhaseB akan trigger path pure-Go inference.
// Saat belum impl, return ErrPhaseBNotImpl dari Complete() — Runtime
// bisa graceful fallback ke Phase A (llama.cpp) kalau Config.FallbackToA=true.
const BackendPhaseB = "phaseb-pure-go"

// Tokenizer adalah contract untuk tokenisasi text → token IDs.
// Phase B impl akan parse BPE/tiktoken-compatible vocab dari GGUF
// metadata atau sibling tokenizer.json file.
type Tokenizer interface {
	// Encode converts text ke token IDs. Return error kalau vocab
	// missing atau text contain chars yang tidak bisa di-encode.
	Encode(text string) ([]int, error)
	// Decode converts token IDs kembali ke text. Untuk streaming
	// output — caller typically accumulate token-by-token dan decode
	// incremental.
	Decode(tokens []int) (string, error)
	// VocabSize returns jumlah entries di vocab. Berguna untuk bound
	// checking di sampling layer (top-k < vocab size).
	VocabSize() int
}

// GGUFReader adalah contract untuk parse GGUF binary format file.
// Spec: https://github.com/ggerganov/ggml/blob/master/docs/gguf.md
//
// Minimum fields yang Phase B butuh:
//   - Magic bytes + version
//   - Metadata KV pairs (architecture, tokenizer.model, context_length)
//   - Tensor info (name, shape, dtype, offset)
//   - Tensor data (int4/fp16 quantized weights)
type GGUFReader interface {
	// Metadata returns KV pairs parsed dari file header.
	Metadata() map[string]any
	// TensorNames returns list of all tensor names (for debug/iteration).
	TensorNames() []string
	// ReadTensor returns raw bytes + shape + dtype untuk named tensor.
	// Caller responsible untuk dequantize sesuai dtype.
	ReadTensor(name string) (data []byte, shape []int, dtype string, err error)
	// Close releases file handle.
	Close() error
}

// Transformer adalah contract untuk forward pass satu layer decoder.
// Phase B impl akan chain multiple layer sesuai model config (Qwen2.5
// target: 24 layers, 14 attention heads, 896 hidden dim).
type Transformer interface {
	// Forward takes input embeddings + optional KV cache, return
	// output logits + updated KV cache. Caller typically loop:
	//   1. Encode prompt ke tokens
	//   2. Embed tokens
	//   3. Forward through all layers
	//   4. Sample from logits → next token
	//   5. Append token ke prompt, goto 2 sampai EOS atau MaxTokens
	Forward(ctx context.Context, input []float32, kvCache any) (logits []float32, newCache any, err error)
}

// Sampler adalah contract untuk pilih token dari logits distribution.
// Strategies: greedy (argmax), top-k, top-p (nucleus), temperature.
// Phase B target: top-p default 0.9, temperature 0.1 untuk healer tasks.
type Sampler interface {
	Sample(logits []float32, temperature float64, topK int, topP float64) (int, error)
}

// PhaseBRuntime adalah facade yang compose Tokenizer + GGUFReader +
// Transformer + Sampler. Caller cukup interface ini — detail internal
// opaque per Phase A → B migration contract di doc.go.
type PhaseBRuntime interface {
	// Complete synchronous prompt → completion. Future streaming
	// variant (CompleteStream) ship di rc174+ setelah core kerja.
	Complete(ctx context.Context, prompt string, maxTokens int, temperature float64) (string, error)
	Close() error
}

// OpenPhaseB adalah factory untuk Phase B runtime. Signature match Open()
// di runtime.go Phase A supaya swap seamless.
//
// SCAFFOLDING: return ErrPhaseBNotImpl. Real impl butuh:
//  1. Parse GGUF metadata → detect arch (Qwen2.5 / Llama / dll)
//  2. Build tokenizer from metadata (vocab + BPE merges)
//  3. Allocate weight tensors (load dari GGUF offset via mmap?)
//  4. Initialize transformer layers dengan weights
//  5. Return runtime yang implement PhaseBRuntime contract
//
// Estimasi effort: 2-4 minggu per doc.go line 41 — itu baseline untuk
// Qwen2.5-0.5B dengan CPU-only, int4 dequant. Untuk larger model atau
// GPU offload, tambah minggu untuk optimisasi.
func OpenPhaseB(modelPath string) (PhaseBRuntime, error) {
	return nil, fmt.Errorf("OpenPhaseB(%q): %w", modelPath, ErrPhaseBNotImpl)
}

// LoadGGUFMetadata adalah stub untuk GGUF parser Phase B. Namespace
// sengaja berbeda dari loader.go LoadGGUF yang Phase A (subprocess)
// untuk hindari collision. Real impl rc173+: parser binary format
// spec https://github.com/ggerganov/ggml/blob/master/docs/gguf.md
func LoadGGUFMetadata(path string) (GGUFReader, error) {
	return nil, fmt.Errorf("LoadGGUFMetadata(%q): %w", path, ErrPhaseBNotImpl)
}

// BuildTokenizer adalah stub untuk tokenizer construction dari GGUF
// metadata. Real impl: parse tokenizer.model section, build BPE trie.
func BuildTokenizer(gguf GGUFReader) (Tokenizer, error) {
	return nil, fmt.Errorf("BuildTokenizer: %w", ErrPhaseBNotImpl)
}

// PhaseBStatus returns laporan progres implementasi Phase B untuk debug
// atau status dashboard. Saat ini hardcoded SCAFFOLDING, future impl akan
// return real readiness metric (tokenizer ready? weights loaded? etc).
func PhaseBStatus() map[string]any {
	return map[string]any{
		"status":           "SCAFFOLDING",
		"ready":            false,
		"tokenizer":        "stub",
		"gguf_parser":      "stub",
		"transformer":      "stub",
		"sampler":          "stub",
		"estimated_effort": "2-4 weeks per doc.go Phase B spec",
		"commit_baseline":  "rc173 2026-04-20",
		"target_model":     "Qwen2.5-0.5B-Instruct Q4_K_M",
		"next_step":        "GGUF binary parser + tokenizer extraction",
	}
}
