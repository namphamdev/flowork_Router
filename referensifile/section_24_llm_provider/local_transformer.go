// Local Native Go Transformer provider — Brain V4 Phase 5.
//
// Implementasi provider.Client menggunakan transformer pure Go yang di-train
// via Phase 4 Dream Daemon. Compatible dengan fallback chain existing.
//
// Cara pake (per-warga di DB agents.model):
//
//	model = "v4:micro-router-v1"   // prefix "v4:" → routed sini
//
// Atau di config provider.type = "local-transformer".
//
// Filosofi: lazy-init runtime + checkpoint + tokenizer per process. Single
// shared inference machine karena transformer thread-safe (Gorgonia copy
// values per Run).
//
// Phase 5 minimum viable:
//   - Load checkpoint dari ~/.flowork/v4/<name>/ (model.bin + tokenizer.json)
//   - Forward inference: encode prompt → forward → softmax sample → decode
//   - Tools field di-ignore (transformer micro-router gak punya function-call)
//   - Greedy + temperature sampling — Phase 6 stretch: top-k/top-p
package provider

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/teetah2402/flowork/internal/bpe"
	"github.com/teetah2402/flowork/internal/transformer"
)

// LocalTransformerConfig parameter init.
type LocalTransformerConfig struct {
	// Name — folder name di ~/.flowork/v4/<name>/. Folder berisi
	// model.bin (checkpoint) + tokenizer.json (BPE vocab+merges).
	Name string

	// MaxTokens cap output (default 256).
	MaxTokens int

	// Temperature 0..2. 0 = greedy. Default 0.7.
	Temperature float64
}

// LocalTransformerClient implements provider.Client.
//
// Lazy-init: load checkpoint + tokenizer di first Complete call. Kalau folder
// belum ada (Phase 4 training belum selesai), return error rapih supaya
// fallback chain bisa skip.
type LocalTransformerClient struct {
	cfg LocalTransformerConfig

	once    sync.Once
	initErr error

	modelDir string
	loaded   bool

	// Phase 5b runtime state (rc159 — Opus 1 unblock):
	// Loaded sekali di init(), reused across Complete calls. Forward
	// rebuild graph per inference step (lihat inference.go).
	model     *transformer.Model
	tokenizer *bpe.Tokenizer
	rng       *rand.Rand

	// Cached embedding weights — extract sekali dari model, pakai per step
	// untuk LookupEmbedding + AddPositional (host-side).
	tokenEmbedData []float32
	posEmbedData   []float32
}

// NewLocalTransformerClient bikin client baru. Tidak load model — itu lazy.
func NewLocalTransformerClient(cfg LocalTransformerConfig) (*LocalTransformerClient, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, errors.New("provider.local-transformer: Name required")
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 256
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	return &LocalTransformerClient{cfg: cfg}, nil
}

// Name identifier.
func (c *LocalTransformerClient) Name() string {
	return "local-transformer:" + c.cfg.Name
}

// Complete — Phase 5b (rc159): actual greedy/temperature decode loop.
//
// Steps per call:
//  1. Concat req.Messages → prompt string
//  2. tokenizer.Encode(prompt) → token IDs
//  3. Generate loop sampai MaxTokens / EOS / ctx done:
//     a. Truncate context kalau > MaxSeq (sliding window)
//     b. LookupEmbedding + AddPositional (host-side)
//     c. RunInferenceStep → logits flat [seq*vocab]
//     d. Take last-position logits → SampleNextToken
//     e. Append + continue
//  4. tokenizer.Decode(generated[len(prompt):]) → text
//  5. Return Response{Message: assistant text, Usage}
func (c *LocalTransformerClient) Complete(ctx context.Context, req Request) (Response, error) {
	c.once.Do(c.init)
	if c.initErr != nil {
		return Response{}, fmt.Errorf("local-transformer: init: %w", c.initErr)
	}
	if !c.loaded {
		return Response{}, fmt.Errorf("local-transformer: model %s not yet trained (Phase 4 in progress, dir empty: %s)", c.cfg.Name, c.modelDir)
	}

	// 1. Concat messages → prompt.
	prompt := buildPromptFromMessages(req.Messages)
	if strings.TrimSpace(prompt) == "" {
		return Response{}, errors.New("local-transformer: empty prompt (no messages)")
	}

	// 2. Tokenize.
	tokenIDs := c.tokenizer.Encode(prompt)
	if len(tokenIDs) == 0 {
		return Response{}, errors.New("local-transformer: tokenize produced 0 tokens")
	}

	// 3. Resolve generation params.
	maxNew := req.MaxTokens
	if maxNew <= 0 {
		maxNew = c.cfg.MaxTokens
	}
	temperature := req.Temperature
	if temperature == 0 {
		temperature = c.cfg.Temperature
	}

	cfg := c.model.Cfg
	embedDim := cfg.EmbedDim
	vocabSize := cfg.VocabSize
	maxSeq := cfg.MaxSeq

	eosID, hasEOS := c.tokenizer.Vocab.SpecialID("eos")

	// 4. Generation loop.
	generated := append([]int(nil), tokenIDs...)
	stopReason := StopReason("max_tokens") // default kalau loop habis tanpa EOS
	for step := 0; step < maxNew; step++ {
		if err := ctx.Err(); err != nil {
			stopReason = StopReason("ctx_cancelled")
			break
		}

		// Sliding window: ambil last MaxSeq token (truncate left kalau over).
		ctxTokens := generated
		if len(ctxTokens) > maxSeq {
			ctxTokens = ctxTokens[len(ctxTokens)-maxSeq:]
		}
		seqLen := len(ctxTokens)

		// Embed lookup + positional (host-side).
		embedded, err := transformer.LookupEmbedding(ctxTokens, c.tokenEmbedData, embedDim)
		if err != nil {
			return Response{}, fmt.Errorf("local-transformer: lookup embed step %d: %w", step, err)
		}
		if err := transformer.AddPositional(embedded, c.posEmbedData, seqLen, embedDim); err != nil {
			return Response{}, fmt.Errorf("local-transformer: add pos step %d: %w", step, err)
		}

		// Forward via fresh graph (rebuild per step — slow tapi sound).
		logitsFlat, err := transformer.RunInferenceStep(c.model, embedded, seqLen)
		if err != nil {
			return Response{}, fmt.Errorf("local-transformer: forward step %d: %w", step, err)
		}

		// Last position logits.
		startIdx := (seqLen - 1) * vocabSize
		endIdx := startIdx + vocabSize
		if endIdx > len(logitsFlat) {
			return Response{}, fmt.Errorf("local-transformer: logits len %d < expected %d", len(logitsFlat), endIdx)
		}
		lastLogits := logitsFlat[startIdx:endIdx]

		// Sample.
		nextID := SampleNextToken(lastLogits, temperature, c.rng)

		// EOS stop.
		if hasEOS && nextID == eosID {
			stopReason = StopReasonEndTurn
			break
		}

		generated = append(generated, nextID)
	}

	// 5. Decode output (prompt prefix excluded).
	newTokens := generated[len(tokenIDs):]
	text := c.tokenizer.Decode(newTokens)

	return Response{
		Message: Message{
			Role:    RoleAssistant,
			Content: text,
		},
		StopReason: stopReason,
		Usage: Usage{
			InputTokens:  len(tokenIDs),
			OutputTokens: len(newTokens),
		},
	}, nil
}

// buildPromptFromMessages concat messages dengan separator role: content.
// Format simple — micro model gak butuh chat template kompleks.
func buildPromptFromMessages(msgs []Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		role := strings.TrimSpace(string(m.Role))
		if role == "" {
			role = "user"
		}
		sb.WriteString(role)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	sb.WriteString("assistant: ")
	return sb.String()
}

// Close release resources. No-op kalau belum loaded.
func (c *LocalTransformerClient) Close() error {
	return nil
}

func (c *LocalTransformerClient) init() {
	home, err := os.UserHomeDir()
	if err != nil {
		c.initErr = fmt.Errorf("home dir: %w", err)
		return
	}
	// rc159b path canonical (match cmd/v4-bootstrap + cmd/v4-train output):
	//   ~/.flowork/models/v4/<name>/ — kalau Name set
	//   ~/.flowork/models/v4/        — kalau Name empty (default single-model)
	// Filename canonical: checkpoint.fqb4 + tokenizer.json
	baseDir := filepath.Join(home, ".flowork", "models", "v4")
	if strings.TrimSpace(c.cfg.Name) != "" && c.cfg.Name != "default" {
		c.modelDir = filepath.Join(baseDir, c.cfg.Name)
	} else {
		c.modelDir = baseDir
	}

	checkpointPath := filepath.Join(c.modelDir, "checkpoint.fqb4")
	tokenizerPath := filepath.Join(c.modelDir, "tokenizer.json")

	if _, err := os.Stat(checkpointPath); err != nil {
		c.loaded = false
		return // not an error — just not yet trained
	}
	if _, err := os.Stat(tokenizerPath); err != nil {
		c.loaded = false
		return
	}

	// Phase 5b actual load (rc159):
	// 1. Load tokenizer (HF JSON format).
	tok, err := bpe.LoadHF(tokenizerPath)
	if err != nil {
		c.initErr = fmt.Errorf("load tokenizer %s: %w", tokenizerPath, err)
		return
	}
	if tok.Vocab == nil || tok.Vocab.Size() == 0 {
		c.initErr = errors.New("tokenizer vocab empty")
		return
	}
	c.tokenizer = tok

	// 2. Build model dengan MicroConfig sesuai vocab size, lalu load checkpoint.
	cfg := transformer.MicroConfig(tok.Vocab.Size())
	g := transformer.NewGraph()
	model, err := transformer.NewModel(g, cfg)
	if err != nil {
		c.initErr = fmt.Errorf("new model: %w", err)
		return
	}
	if err := transformer.LoadCheckpoint(model, checkpointPath); err != nil {
		c.initErr = fmt.Errorf("load checkpoint %s: %w", checkpointPath, err)
		return
	}
	c.model = model

	// 3. Cache embedding weights (extracted sekali di sini, reused per step).
	tokenEmbed, err := transformer.ExtractWeight(model.TokenEmbed)
	if err != nil {
		c.initErr = fmt.Errorf("extract token embed: %w", err)
		return
	}
	posEmbed, err := transformer.ExtractWeight(model.PosEmbed)
	if err != nil {
		c.initErr = fmt.Errorf("extract pos embed: %w", err)
		return
	}
	c.tokenEmbedData = tokenEmbed
	c.posEmbedData = posEmbed

	// 4. RNG seeded dari time — kalau Temperature=0 ga dipake (greedy).
	c.rng = rand.New(rand.NewSource(time.Now().UnixNano()))

	c.loaded = true
}

// ParseLocalTransformerModel detect prefix "v4:" di model name. Return
// (cleaned_name, true) kalau match.
//
// Contoh:
//
//	"v4:micro-router-v1"   → ("micro-router-v1", true)
//	"v4:phi3-distilled-1m" → ("phi3-distilled-1m", true)
//	"local:phi-3.gguf"     → ("", false)  // V3 prefix, beda
func ParseLocalTransformerModel(model string) (string, bool) {
	m := strings.TrimSpace(model)
	if strings.HasPrefix(strings.ToLower(m), "v4:") {
		return strings.TrimSpace(m[len("v4:"):]), true
	}
	return "", false
}

// ─── Sampling helpers (Phase 5b ready) ────────────────────────────────

// SampleNextToken — pilih token id dari logits via temperature sampling.
// Greedy kalau temperature == 0.
//
// Public buat Phase 5b — ngebantu integration test sebelum full inference loop.
func SampleNextToken(logits []float32, temperature float64, rng *rand.Rand) int {
	if len(logits) == 0 {
		return 0
	}
	if temperature <= 0 {
		// Greedy: argmax.
		bestIdx := 0
		bestVal := logits[0]
		for i, v := range logits {
			if v > bestVal {
				bestVal = v
				bestIdx = i
			}
		}
		return bestIdx
	}

	// Temperature softmax + categorical sample.
	scaled := make([]float64, len(logits))
	maxLogit := float64(logits[0])
	for _, v := range logits {
		if float64(v) > maxLogit {
			maxLogit = float64(v)
		}
	}
	var sumExp float64
	for i, v := range logits {
		scaled[i] = math.Exp((float64(v) - maxLogit) / temperature)
		sumExp += scaled[i]
	}
	// Sample.
	if rng == nil {
		rng = rand.New(rand.NewSource(42))
	}
	r := rng.Float64() * sumExp
	cum := 0.0
	for i, p := range scaled {
		cum += p
		if r <= cum {
			return i
		}
	}
	return len(logits) - 1 // fallback
}
