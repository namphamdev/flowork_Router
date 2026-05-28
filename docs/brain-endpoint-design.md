# Flow Router — Brain Endpoint Design

> **Status:** design (Phase 0). Hidden project — lives on branch `brain-endpoint`, pushed to the **private** remote only (`personal`), never to public `origin`.

## 1. Goal

Turn Flow Router into a **shared, portable brain**. Today the "brain" (a fine-tuned model + a 32 GB knowledge DB + cascade logic) lives inside one monolithic agent (flowork, ~247 GB) and cannot fit on a Raspberry Pi. We split it:

- **Body** (light, runs on Pi or anywhere): orchestration, tools, Telegram, skills loop — calls out over HTTP.
- **Brain** (heavy, runs on a server): served by Flow Router behind one OpenAI-compatible endpoint.

The bigger win: the brain stops being flowork-only. **Any** OpenAI-compatible agent — OpenClaw, Hermes, Cursor, Claude Code, flowork — points its `base_url` at Flow Router and selects a brain model, and gets the *same* intelligence + skills. Every call can also feed the brain so it compounds across all agents.

## 2. Architecture

```
 OpenClaw ┐
 Hermes   ┤
 Cursor   ┼─► Flow Router  POST /v1/chat/completions   model=flowork-brain
 flowork  ┤        │
 Claude Code┘      ▼
        ┌──────────────────────── enrichment (server-side) ─────────────────┐
        │ 1. Cascade L0→L5 over brain DB  → retrieve relevant knowledge      │
        │ 2. Inject skills (bundled/*.md) relevant to the query              │
        │ 3. (brain mode) inject persona/constitution                        │
        │ 4. Inference: Ollama (local fine-tune) → cloud fallback chain      │
        │ 5. Record interaction → ingest back into brain (compounding)       │
        └────────────────────────────────────────────────────────────────────┘
```

The enrichment is inserted in `internal/router/dispatcher.go::DispatchChatCompletion`, **right after `resolveModel` and before provider forward**, so the rest of the pipeline (combos, fallback strategy, key scope, logging) is reused untouched.

## 3. Request data flow

1. Request arrives at any `/v1` entry (chat/completions, messages, responses, v1beta) — all already funnel through `DispatchChatCompletion(Stream)`.
2. `resolveModel` runs. If the effective model is a **brain model** (configured alias, default `flowork-brain`) OR brain enrichment is force-enabled in settings, run `brain.Enrich(ctx, &req, mode)`:
   - Take the latest user message as the **query**.
   - `CascadeQuery` over the brain DB:
     - **L0** constitution (sacred rules, confidence 1.0) — keyword match.
     - **L1** cached_reasoning exact-hash (absent in current DB → skipped gracefully).
     - **L2** FTS5 drawer search, BM25-ranked — **the workhorse** (5.03 M drawers). Returns top-N chunks.
     - **L3** skill registry (DB skills table empty → skills sourced from `bundled/skills/*.md` instead).
     - **L4** mesh peer (optional, off by default).
     - **L5** upstream LLM = the normal Flow Router forward (so cascade's L5 *is* the existing dispatch).
   - Build an injected `system` message: retrieved knowledge (cited by drawer id/wing) + selected skills + (brain mode) persona/constitution.
3. Continue the normal pipeline → forward to the resolved provider (Ollama serving the fine-tune, or cloud).
4. On response, optionally record `{query, retrieved, answer, model, latency}` for the compounding ingest.

## 4. Two modes

| Mode | System prompt | Use case |
|---|---|---|
| **augment** (default) | Caller's own system prompt kept; brain knowledge + skills *appended* as additional context (RAG style). | Agents with their own identity (OpenClaw, Cursor, Claude Code). Max compatibility, no conflict. |
| **brain** | Flow Router owns the full persona + constitution + skills. | "Empty" agents that want to *be* the flowork brain. |

Mode selected per request via header (`X-Flow-Brain-Mode`) or per-key/global setting; default `augment`.

## 5. Inference backend — Ollama

Decision: **standardize on Ollama** for local model serving (universal, cross-OS, plug-and-play) instead of the heavy llama.cpp/transformers paths. Ollama exposes an OpenAI-compatible API at `:11434/v1` and is already a Flow Router provider preset. The fine-tune is served as an Ollama model (e.g. `flowork-brain`).

Users who don't run a local model still work: cascade L0–L2 (knowledge + skills) enrich the prompt, and the configured **cloud fallback chain** answers. Local model is an optimization, not a requirement.

## 6. Brain DB access

- `flowork-brain.sqlite` is **32 GB** (`drawers` 5.03 M rows + `drawer_embeddings` 1.08 M bge-m3 vectors dim 1024 + `memory_fts` FTS5). It stays on the **server**, never shipped to the Pi.
- Flow Router opens it **read-only** via a configurable path (env `FLOW_ROUTER_BRAIN_DB`, default `~/.flow_router/brain/flowork-brain.sqlite`), using the existing pure-Go `modernc.org/sqlite` driver. No new dependency, no CGO.
- v1 retrieval = **FTS5 keyword (L2)** — pure SQL, fast, no embedding model needed. Semantic retrieval via the bge-m3 embeddings is a later enhancement (needs an embedding endpoint, e.g. Ollama `nomic`/`bge` embeddings).

## 7. What we port vs reuse

| Need | Source | Plan |
|---|---|---|
| Cascade orchestrator | flowork `worker/internal/brain/cascade.go` `CascadeQuery` | Port into `internal/brain/` (self-contained, takes `*sql.DB`). |
| FTS5 drawer / cross-wing search | flowork `worker/internal/brain/cross_domain.go` | Port the SQL (BM25 ranking + scoring). |
| Skills | flowork `bundled/skills/*.md` (40) | Copy into Flow Router skill store + a relevance selector. |
| Inference + fallback | **Flow Router already has it** (dispatch, combos, fallback strategy) | Reuse — cascade L5 == existing forward. |
| Streaming, tool-calls, translation | **Flow Router already has it** | Reuse. |

## 8. Identity & compounding (later phases)

- Per inbound API key = per agent. Memory/recordings scoped by key so each agent has its own conversational state while sharing the global knowledge.
- Every interaction can be recorded and ingested back into the brain (quality-gated) so all agents' usage makes the shared brain smarter.

## 9. Phase plan

- **P0** grounding + this doc ✅
- **P1** `internal/brain` bridge (open DB, port cascade L0–L3) · Ollama brain backend wiring · skill library + selector
- **P2** enrichment hook in dispatcher (augment + brain mode) · per-key identity
- **P3** compounding record + ingest
- **P4** test as external agents (curl as OpenClaw/Hermes) + live test with real Ollama + brain DB
- **P5** point flowork-body's `BrainProvider` at the remote Flow Router endpoint
