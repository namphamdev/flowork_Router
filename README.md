<div align="center">

# 🛣️ Flow Router

### Self-hosted AI gateway & LLM proxy — one OpenAI-compatible endpoint for every provider

**Route Claude, GPT, Gemini, DeepSeek, Ollama, vLLM and 40+ providers through a single fast endpoint.**
Bring your own subscription or API key, plug it into Claude Code, Cursor, Codex, Cline and any OpenAI-compatible tool.

A lightweight, **self-hosted [LiteLLM](https://github.com/BerriAI/litellm) / [OpenRouter](https://openrouter.ai) alternative** — one Go binary, no Docker, no Python, no database server.

[![License: MIT](https://img.shields.io/badge/License-MIT-8b5cf6.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-informational)](#-quick-start)
[![Single Binary](https://img.shields.io/badge/deploy-single%20binary-success)](#-deployment)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#-contributing)

[Features](#-features) · [Quick Start](#-quick-start) · [Supported Tools](#-supported-cli-tools) · [Providers](#-supported-providers) · [API](#-api-reference) · [Deploy](#-deployment)

</div>

> ### 🤖 The optimal pairing — [Flowork AI Agent](https://github.com/flowork-os/flowork-ai-agent)
>
> Flow Router welcomes **any OpenAI-compatible agent** (Claude Code, Cursor, Cline, Codex, Continue, Aider, custom apps…). For the **deepest, most-integrated experience** — autonomous multi-agent body, native [`FLOWORK_BRAIN_REMOTE`](#-brain-endpoint--shared-portable-knowledge-brain) thin-mode flag, full caretaker pipeline (ingestor, training, dashboards), 6 purpose-built subagent types — install **Flowork** alongside.
>
> **One brain (this router) + many bodies (Flowork or any agent) = your full sovereign AI stack.**
>
> 👉 **[github.com/flowork-os/flowork-ai-agent](https://github.com/flowork-os/flowork-ai-agent)**

---

## Why Flow Router?

Modern AI workflows are fragmented. Every CLI, IDE and agent speaks a slightly different API — OpenAI, Anthropic, or Gemini — and every provider bills differently. Switching models means editing config in a dozen places, and your paid subscriptions sit idle while you burn API credits.

**Flow Router fixes that with one local endpoint:**

- 🔌 **One endpoint, every model.** Point any OpenAI-compatible tool at `http://127.0.0.1:2402/v1` and reach Claude, GPT, Gemini, DeepSeek, Groq, local models — anything.
- 🔑 **Use what you already pay for.** Drive Claude Code / Cursor through your existing Claude Pro/Max subscription — no extra API key required.
- 🔁 **Automatic fallback.** Define a priority chain (subscription → cheap → free) so requests never fail when one provider is rate-limited.
- 🖥️ **Single Go binary.** No runtime, no database server. Portable across Linux, macOS and Windows. Built-in web dashboard.
- 🛡️ **Private by default.** Everything runs on your machine. Your keys and traffic never leave localhost unless you choose to tunnel out.

---

## How It Works

```
   Flowork ⭐  ·  Claude Code  ·  Cursor  ·  Cline  ·  OpenClaw  ·  Hermes  ·  custom
                                       │
                                       │  OpenAI / Anthropic / Gemini dialects
                                       │  base_url = http://router:2402/v1
                                       ▼
   ┌──────────────────────────────────────────────────────────────────────┐
   │                       FLOW  ROUTER   :2402                            │
   │                                                                        │
   │   Gateway   format translation · priority / RR / random fallback       │
   │             combos · load-balance · usage + quota                      │
   │             OAuth import · MCP registry · tunnels · edge proxy         │
   │                                                                        │
   │   ┌──────── 🧬 SHARED KNOWLEDGE BRAIN  (Option C) ────────────┐        │
   │   │                                                              │        │
   │   │   RAG cascade    FTS5 BM25 over Memory Palace                │        │
   │   │   skill inject   40 behavioral skills (embedded in binary)   │        │
   │   │   modes          augment ▸ keep caller │ brain ▸ persona      │        │
   │   │   admin CRUD     constitution editor · dashboard              │        │
   │   │   compounding    interactions ─▶ ingest ─▶ FTS-indexed drawers│        │
   │   │   thin/Pi mode   FLOWORK_BRAIN_REMOTE — body needs no local DB │        │
   │   │                                                              │        │
   │   │   ◀── all connected agents make the brain smarter together ──┘        │
   │   └──────────────────────────────────────────────────────────────┘        │
   │                                                                        │
   └─────────────────────────────────┬──────────────────────────────────────┘
                                     │
         ┌───────────┬───────────────┼───────────────┬───────────────┐
         ▼           ▼               ▼               ▼               ▼
    Claude (sub)  OpenAI / GPT   Gemini       DeepSeek / Groq   Local backends:
    Anthropic     Mistral        (Google)     Together · Qwen    Ollama · vLLM
                                                                 llama.cpp · LM Studio
```

A request from any agent body arrives (OpenAI / Anthropic / Gemini dialect). Flow Router routes it:

- **Plain gateway request** → format-translated, fallback-chained, forwarded to the matching provider; the response is translated back.
- **Brain request** (`model: "flowork-brain"`) → enriched server-side first: cascade RAG over the Memory Palace + relevant skills injected + (optional) persona, then forwarded.
- **Every interaction** can be queued and ingested back into the brain (opt-in) — so the shared knowledge **compounds across every connected agent**, no matter who hits the endpoint.

Usage, cost and latency are logged for every call. One brain, many bodies, learning together.

---

## ✨ Features

| Feature | What it does |
|---|---|
| 🧬 **Shared knowledge brain** | Server-side **RAG enrichment** + skill injection + persona — any OpenAI-compat agent gets the same intelligence. See [Brain Endpoint](#-brain-endpoint--shared-portable-knowledge-brain). |
| 🪞 **Compounding ingest** | Every interaction can be queued and ingested back as FTS-indexed knowledge — all connected agents make the brain smarter together |
| 🛣️ **Thin / Pi body mode** | `FLOWORK_BRAIN_REMOTE` flag lets a Flowork-style agent body run with **no local 32 GB brain DB** — RAG via the router |
| 🔌 **Universal endpoint** | OpenAI `/v1/chat/completions` + streaming, Anthropic `/v1/messages`, OpenAI `/v1/responses`, Gemini `/v1beta/models` — all served at once |
| 🔄 **Format translation** | Transparent OpenAI ⇄ Anthropic ⇄ Gemini conversion, including streaming SSE |
| 🛠️ **Tool calling** | Full `tool_calls` ⇄ `tool_use` conversion so agentic tools work across providers |
| 🔁 **Smart fallback** | Priority-ordered providers; auto-retry the next one on error or rate limit |
| 🧩 **Combos** | Group models into one alias with `priority` / `round-robin` / `random` / `cost-optimal` strategies |
| 💸 **Cost-tier routing** | Heuristic classifier scores each request as cheap / standard / strong (char count + code blocks + tool_use + multi-turn) and filters providers by `tier:*` tag — simple queries auto-route to small/local models. Honors explicit model choices. |
| 🔑 **Subscription auth** | Drive workloads through your existing **Claude Pro/Max** subscription, **Codeium Plus**, **Windsurf Cascade**, **JetBrains AI Pro**, **Zed AI**, **Kiro**, **GitHub Copilot**, **Cursor Pro**, **Google Antigravity** — no API key required |
| 🪪 **OAuth & key import** | Connect Codex, Cursor, GitLab, iFlow, Kiro, Claude — or paste a token directly |
| ⌨️ **CLI auto-config** | Detect and configure 13 popular AI CLIs/extensions in one click |
| 📊 **Usage analytics** | Per-day charts, per-provider breakdown, live request stream, cost estimates |
| ⏱️ **Quota tracking** | Track subscription/API limits per provider, daily/weekly/monthly |
| 🛡️ **MITM inspector** | Capture, inspect and replay full request/response bodies for debugging |
| 🚇 **Tunnels** | Expose your router securely via Cloudflare Tunnel or Tailscale |
| 🌐 **Edge proxy deploy** | Generate ready-to-ship proxy workers for Cloudflare, Deno Deploy or Vercel |
| 🎬 **Media providers** | Route embeddings, text-to-image, TTS, STT and web search to dedicated backends |
| 🎙️ **STT stack** | `/v1/audio/transcriptions` backed by Deepgram, AssemblyAI, Gemini multimodal, and OpenAI Whisper (OpenAI-compat multipart). Plug-and-play registry — drop a vendor file in `internal/providers/stt/`. |
| 🕸️ **Web-fetch stack** | `/v1/web/fetch` backed by Jina Reader (markdown extraction), Firecrawl (cleaned scrape), and a zero-config raw HTTP fallback. |
| 📡 **Live quota fetcher** | `/api/quota-tracker/live?provider=claude\|copilot` — pulls Anthropic OAuth windows (5h + 7d + per-model) and GitHub Copilot entitlement snapshots directly from upstream, not just the cached DB row. |
| 🧭 **Smart RTK auto-detect** | Explicit-priority filter chain (git-diff → git-status → build-output → grep → find → tree → ls → search-list → read-numbered → dedup-log → smart-truncate) with extra heuristics (porcelain ratio gate, grep-line shape, find path-likeness) — replaces non-deterministic first-Register-wins. |
| 🆎 **Kiro live model discovery** | `/api/kiro/models?token=…` hits AWS CodeWhisperer's ListAvailableModels with IDE-style UA + per-(token,region) 5-min cache. Auto-expands each base model into `{id}` / `-thinking` / `-agentic` / `-thinking-agentic` synthetic variants. |
| 🪨 **Caveman style modifier** | Output-token saver: appends a "respond tersely" instruction (`lite` / `full` / `ultra`) to every system message before dispatch. Code blocks, file paths, commands, URLs and security warnings stay exact regardless of level. |
| 🧬 **Cursor ConnectRPC executor** | Experimental `cursor-proto` executor speaks the real api2.cursor.sh wire format (ConnectRPC + hand-rolled protobuf for StreamUnifiedChat) so real Cursor subscriptions work without an OpenAI-compat shim. Codec is fully unit-tested. |
| ⚡ **Claude CLI bypass** | Detects 5 known no-op patterns from Claude Code (Warmup / count / title-extraction / isNewTopic / custom skip patterns) and answers locally with a 2-token stub. Pure local round-trip — zero upstream cost. |
| 🛡️ **Google CloudCode projectId resolver** | Antigravity / gemini-cli `useRealProjectId=true` resolves a real account-bound project id via `/v1internal:loadCodeAssist` (with onboardUser fallback), cached 1h per connection. Avoids the random-id anti-abuse flag. |
| 🔁 **Per-model combo fallback** | When all providers for a combo's picked model 5xx, the dispatcher walks the remaining combo.Models in order instead of giving up. 4xx-class errors break out so operator blocks aren't masked. |
| 🧩 **Provider compat prefixes** | A provider named `openai-compatible-<x>` or `anthropic-compatible-<x>` auto-resolves format + base URL without explicit fields. Suffix `-responses` switches to the OpenAI Responses API path. Explicit fields always win. |
| 🔧 **Tool-call hygiene** | Pre-dispatch sanitises every `tool_use` / `tool_call_id` to the strict `[a-zA-Z0-9_-]+` pattern Anthropic requires + auto-inserts empty `tool_result` stubs when an assistant message lists tool_calls without follow-up. Prevents the most common Claude API 400. |
| 📐 **Prompt-cache reporting** | Translator extracts `cache_read_input_tokens` + `cache_creation_input_tokens` from Anthropic responses and emits `usage.prompt_tokens_details.cached_tokens` so prompt-cache savings actually show up in logs. |
| 📜 **Always-on doctrine** | Constitution (sacred rules), skills, and brain knowledge inject into every chat — not just requests that explicitly target `flowork-brain`. Set `Brain.AlwaysOn=false` to revert to the old model-gated behaviour. |
| 🪃 **Fallback rules** | 17-rule cooldown table covering rate-limit / quota / capacity / overloaded text variants + status 401/402/403/404/429/5xx. Text rules win over status rules, so "capacity reached" inside a 500 escalates to exponential backoff rather than a generic 15s nap. |
| ⏱️ **Per-provider token-refresh leads** | Lead times tuned per vendor (codex/openai 5d, claude/anthropic 4h, iflow 24h, qwen 20m, antigravity/gemini-cli 5m). New providers fall through to the package-level default — no per-vendor config required to get started. |
| 🎛️ **Full optional-params passthrough** | Dispatcher preserves the 22-param OpenAI spec surface (temperature, top_p, top_k, max_completion_tokens, thinking, reasoning, presence/frequency_penalty, seed, stop, response_format, prediction, store, metadata, n, logprobs, top_logprobs, logit_bias, user, parallel_tool_calls, tools, tool_choice). |
| 🎞️ **Forced-stream collapser** | `ParseSSEToOpenAIResponse` aggregates a streaming-only provider's chunks into a single `chat.completion` when the client asked for non-streaming — content + reasoning + tool_calls + usage all merge correctly. |
| 🎬 **Responses-API streamer** | `/v1/responses` emits the full Codex event sequence — response.created → output_item.added → content_part.added → output_text.delta* → tool / reasoning items → completed — with monotonic sequence numbers and idempotent close. |
| 📡 **13 live quota fetchers** | `/api/quota-tracker/live?provider=…` covers claude, copilot, codex, gemini-cli, antigravity, kiro, glm, glm-cn, minimax, minimax-cn, qwen, iflow, ollama. Each vendor's actual upstream endpoint + response shape parsed into a unified Window struct. |
| 📝 **Codex default instructions** | `/backend-api/codex/responses` requests get the upstream Codex CLI's harness/git/editing/frontend rules injected as the `instructions` field when the caller didn't supply one. `store=false` is force-set (Codex API requirement). |
| 📐 **Smart max_tokens auto-bump** | `AdjustMaxTokens` lifts the cap to 32k when tools are present (prevents truncated `arguments` JSON) and to `budget_tokens + 1024` when thinking-mode is on (Anthropic enforces `max_tokens > budget_tokens`). |
| 🔄 **Responses-API converter** | `ConvertResponsesAPIFormat` handles the full `/v1/responses` shape: `input_text` / `output_text` → text, `input_image` → image_url, `function_call` grouped into assistant tool_calls, `function_call_output` → role=tool, reasoning items dropped. |
| 🆎 **Kiro synthetic-suffix routing** | Users pick `<model>-thinking` / `-agentic` / `-thinking-agentic` from the GUI; executor strips the suffix before sending upstream + prepends the chunked-write protocol system prompt for agentic variants (avoids Kiro's 2-3 min timeout on large writes). |
| 🎞️ **Responses SSE collapser** | `ParseResponsesSSEToJSON` aggregates a Responses-API event stream into a single JSON envelope when the client wanted non-streaming but the provider forces stream=true. Gap-fills missing item indexes with empty assistant placeholders. |
| 🔐 **Cursor checksum auto-gen** | Generates the full `x-cursor-checksum` / `x-session-id` / `x-client-key` / `x-amzn-trace-id` header bundle (Jyh-cipher + RFC 4122 UUIDv5) so real Cursor subscriptions work without manually scraping headers from the IDE. |
| 🛡️ **MCP spawn allowlist** | `internal/mcpsecurity` gates every MCP child process behind a known-interpreter whitelist (npx/node/uvx/python/bunx/bun/deno/pnpm/yarn). Path-traversal rejected; Windows extensions normalised. Operators extend via `Allow()`. |
| 🧠 **Reasoning-content injector** | DeepSeek + Kimi thinking-mode require non-empty `reasoning_content` on assistant turns — `InjectReasoningContent` adds a placeholder so OpenAI-format clients work upstream. DeepSeek `v4-pro-{max,none}` aliases auto-rewrite to base model + thinking knobs. |
| ✂️ **Tool deduper** | When Exa/Tavily/BrowserMCP servers are wired up, `DedupeTools` strips Anthropic's built-in `WebSearch`/`WebFetch` (or the duplicate Claude_in_Chrome connector) before dispatch — cuts token bloat from overlapping tool defs. |
| 📦 **One-click MCP catalog** | `/api/mcp/catalog` exposes a curated registry (Exa HTTP, Tavily HTTP+OAuth, BrowserMCP stdio) the dashboard can render as register-with-one-click cards. Operators extend via `mcpcatalog.Register`. |
| 🪶 **Ollama NDJSON stream** | `TransformOpenAISSEToOllamaNDJSON` converts OpenAI chat-completion SSE into the native `ollama` JSON-lines shape (content rows + final `done:true` with aggregated tool_calls), so the `ollama` CLI and `ollama-python` work against any backend the router exposes. |
| 🧠 **MCP registry** | Register Model Context Protocol servers and list their tools live |
| 🏷️ **Tags & pricing** | Organize providers with tags; maintain per-model rate cards |
| 🔐 **Optional login** | Password or OIDC (OpenID Connect) with opt-in session enforcement |
| 🔒 **Login rate limiter** | Per-IP progressive lockout for `/api/auth/login` (5 fails → 30s/2m/10m/30m) — auto-reset after 1h idle |
| 💾 **DB backup + migrations** | Versioned `VACUUM INTO` snapshots (keep-N rolling) + sequential idempotent schema migrations with pre-migrate auto-snapshot |
| ✂️ **Advanced RTK token-saver** | 11 auto-detected tool-output compressors (git-diff, git-status, build-output, grep, find, tree, ls, search-list, read-numbered, dedup-log, smart-truncate) — typical 40–80% reduction in agent loops |
| 📦 **Drop-in SKILL.md packages** | 7 ready-to-paste skills (`flow-router-chat / -image / -tts / -stt / -embeddings / -web-search / -web-fetch`) any Claude/Cursor/ChatGPT can ingest |
| 🧪 **18 vendor executors + 2 aliases** | Plug-and-play backend per vendor: antigravity · azure · codex · commandcode · cursor (+ `cu`) · default · gemini-cli · github · grok-web · iflow (HMAC-signed) · kiro · ollama-local · opencode · opencode-go · perplexity-web · qoder · qwen · vertex (+ `vertex-partner`) |
| 🕵️ **MITM TLS interception** | Local HTTPS proxy with per-SNI cert minting (RSA root CA + cached leaves), DNS hijack (hosts-file), per-IDE rewriters (Antigravity / Copilot / Cursor / Kiro) → traffic flows through the full dispatcher |
| 🩺 **Tunnel watchdog** | Background 60s health-probe of the active Cloudflare/Tailscale tunnel; flips dashboard state when it goes dark |
| 🛠️ **`flow-cli` companion binary** | Stand-alone control binary — `status / providers / keys / settings / ui (interactive TUI) / tray / autostart` — no extra deps |
| 📌 **Cross-OS autostart** | One-command login-time autostart: Linux `.desktop` · macOS `LaunchAgent` plist · Windows HKCU Run |

---

## 🧬 Brain Endpoint — shared portable knowledge brain

Turn Flow Router into a **server-side shared brain** that any OpenAI-compatible
agent (OpenClaw, Hermes, Cursor, Claude Code, flowork…) can plug into. One brain,
many bodies — every agent shares the same knowledge + skills + persona, and the
brain compounds as they use it.

**How it works.** When a request hits `/v1/chat/completions` with `model: "flowork-brain"`,
Flow Router enriches it server-side **before forwarding**:
- **L2 FTS5 cascade** retrieves the top-K knowledge chunks from a Memory-Palace
  SQLite (drawers table, BM25, AND-first → ~300 ms over a 5 M-chunk DB)
- **Embedded skill library** (40 markdown behavioral skills) injects relevant
  working methods
- Two modes — **augment** (caller's system prompt kept, knowledge appended for
  max compatibility) or **brain** (full flowork persona + constitution + skills
  prepended)
- **Compounding** (opt-in): every interaction can be recorded and later ingested
  back into the brain (`drawers + memory_fts`) so all agents make it smarter

**Inference backend** = Ollama by default (cross-OS, plug-and-play) serving a
fine-tuned GGUF; falls back through the standard Flow Router provider chain
(cloud, API key) when local is unavailable.

**API surface:**

| Endpoint | Purpose |
|---|---|
| `GET  /api/brain/status` | DB availability, size, wing breakdown |
| `GET/PUT /api/brain/config` | Toggle, mode (augment / brain), DB path, top-K |
| `POST /api/brain/test` | Preview retrieval + selected skills for a query |
| `GET  /api/brain/explore` | Content overview (drawers, constitution, agents, categories) |
| `GET/POST/PUT/DELETE /api/brain/constitution` | Sacred-rule CRUD (soft-delete) |
| `GET  /api/brain/by-type` | Typed memory (drawers by category) |
| `GET  /api/brain/personas` | Subagent prompt-library |
| `GET  /api/brain/contributions` · `POST /api/brain/ingest/run` | Compounding loop |
| `GET  /api/brain/search-drawers` | flowork-kernel-compatible RAG (thin-body endpoint) |
| `POST /api/brain/init` | Bootstrap an empty Memory-Palace DB (idempotent — fresh-install button) |
| `POST /api/brain/drawer` | Bring-your-own-knowledge: add a drawer manually (auto-deduped) |

**Dashboard.** The 🧬 Brain tab in the Flow Router UI exposes everything (Overview,
Search, Constitution editor, Typed Memory, Personas, Config) wired to real DB data.

**Thin / Raspberry Pi body.** Any flowork-style agent can run as a thin body —
set `FLOWORK_BRAIN_REMOTE=http://<router>:2402` and the kernel routes RAG reads
through Flow Router; no local 32 GB brain DB needed on the body.

**Storage layout.** The brain assets live in the router project root, gitignored:
`flow_router/brain/flowork-brain.sqlite` + `flow_router/models/*` (GGUF + training
intermediates). Flow Router resolves them zero-config via `~/.flow_router/brain/`
(symlink to project) or `$FLOW_ROUTER_BRAIN_DB`.

Default state is **off** (plug-and-play). Enable in the dashboard or via settings.

### 🌱 Start empty — bring your own knowledge

The brain ships with **no DB included** (it would be gigabytes; we keep the binary
lean). You can either point Flow Router at an existing Memory-Palace SQLite (set
`brainDBPath` / `$FLOW_ROUTER_BRAIN_DB`) — or bootstrap an empty one and grow it
yourself. Three ways to ingest:

**1. Dashboard (no terminal needed)** — open `http://127.0.0.1:2402` → 🧬 Brain.

- If no DB exists yet, the Overview shows a **🆕 Initialize empty brain** button.
  One click creates a fresh Memory-Palace at the configured path (idempotent — safe
  to re-press).
- Switch to the **✍️ Add Knowledge** sub-tab. Paste text, optionally tag wing /
  room / memType, hit **+ Add to brain**. Duplicates are silently deduped by
  content hash. The new drawer is immediately searchable from the 🔍 Search tab.

**2. CLI / curl** — perfect for scripts or remote bodies:

```bash
# Bootstrap an empty brain (safe on existing DBs — it's idempotent)
curl -X POST http://127.0.0.1:2402/api/brain/init

# Add a single drawer (the "bring-your-own-knowledge" call)
curl -X POST http://127.0.0.1:2402/api/brain/drawer \
  -H 'Content-Type: application/json' \
  -d '{
    "content": "Flow Router is a self-hosted LLM gateway with a shared knowledge brain.",
    "wing":    "general",
    "room":    "docs",
    "memType": "knowledge"
  }'

# Verify it's searchable
curl "http://127.0.0.1:2402/api/brain/search-drawers?query=self-hosted+gateway&k=3"
```

**3. Bulk ingest from a file** — one knowledge chunk per line:

```bash
# my_knowledge.jsonl — one self-contained chunk per line, plain text
while IFS= read -r line; do
  curl -s -X POST http://127.0.0.1:2402/api/brain/drawer \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg c "$line" '{content:$c,wing:"general",memType:"knowledge"}')"
done < my_knowledge.jsonl
```

**4. Auto-compounding (recommended long-term)** — turn on `Compounding` in the
Brain → Config tab (or set `record: true` via `/api/brain/config`). Every
interaction that flows through `model: "flowork-brain"` is queued; press
**⚙️ Run ingest now** (or `POST /api/brain/ingest/run`) and the queue becomes
knowledge drawers. The brain learns from every body that uses it.

> 💡 Aim for **self-contained chunks** per drawer (one idea / paragraph / snippet).
> Retrieval is FTS5 BM25 — small, focused chunks score better than monoliths.
> Typical sweet spot: 200–800 characters. The dashboard form happily takes more,
> but the retriever returns top-K *per chunk*, so split where it makes sense.

### 🤝 Recommended companion agent

Flow Router works with **any OpenAI-compatible agent** (Claude Code, Cursor, Cline,
Codex, OpenClaw, Hermes, Continue, Aider, custom apps, …). They all just need
`base_url = http://<router>:2402/v1` and they get the shared brain.

But for the **most-integrated experience** — thin-body remote-brain flag, full
caretaker mode (ingestor + training + DB management), kernel-level cascade
routing, the whole "1 brain, many bodies" loop — use the purpose-built companion:

> 👉 **[`flowork-os/flowork-ai-agent`](https://github.com/flowork-os/flowork-ai-agent)**

It honors `FLOWORK_BRAIN_REMOTE` natively (kernel's RAG path routes through
Flow Router with zero config), ships the matching ingestor/training/dashboards
on the caretaker side, and is the reference implementation of the "thin/Pi body"
mode. Good for any agent → optimal with `flowork-ai-agent`.

---

## 🚀 Quick Start

### Build from source

```bash
git clone https://github.com/flowork-os/flowork_Router.git
cd flowork_Router
go build -o flow-router ./...
./flow-router            # listens on http://127.0.0.1:2402
```

> Requires Go 1.25+. The result is a single self-contained binary — copy it anywhere.

### Point your tool at it

Set your tool's base URL to:

```
http://127.0.0.1:2402/v1
```

Open the dashboard at **http://127.0.0.1:2402** to add providers, create combos, and watch usage in real time.

### Connect a provider in 10 seconds

1. Open the dashboard → **Providers** → add a provider (API key or subscription).
2. (Optional) Create a **Combo** to alias several models behind one name.
3. Send a request:

```bash
curl http://127.0.0.1:2402/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-haiku-4-5",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

---

## ⌨️ Supported CLI Tools

Auto-detect and one-click configure these AI CLIs and editor extensions:

`Claude Code` · `Codex` · `Cline` · `Copilot` · `Cowork` · `DeepSeek TUI` · `Droid` · `Hermes` · `JCode` · `Kilo` · `OpenClaw` · `OpenCode` · `Antigravity`

Each integration writes the correct config (JSON / TOML / `.env`) pointing at Flow Router, and can be reset just as easily.

---

## 🧪 Vendor Executors — pluggable per-provider backends

Flow Router ships a **registry of 18 vendor executors** that handle the per-provider quirks (URL templates, auth headers, request shape, signature schemes) so the dispatcher stays format-neutral. Set `data.format = "<name>"` on a provider connection and the matching executor fires automatically.

| Name | Vendor | Notable quirks handled |
|---|---|---|
| `antigravity` | Google Cloud Code Assist Antigravity | `{project, model, request:{contents:[…]}}` wrap + IDE-version override |
| `azure` | Azure OpenAI | `/openai/deployments/<name>/chat/completions?api-version=…` template, `api-key` header |
| `codex` | ChatGPT Codex backend | Session token + `chatgpt-account-id` + `openai-project` rotation |
| `commandcode` | api.commandcode.ai | Bearer `user_xxx` + per-request `x-session-id` |
| `cursor` (+ `cu`) | api2.cursor.sh | Cursor session + `x-cursor-checksum` + `x-cursor-session-id` |
| `default` | OpenAI-compatible fallback | `<baseURL>/chat/completions` with Bearer |
| `gemini-cli` | Cloud Code Assist (Gemini CLI) | `{project, model, request}` wrap + `X-Goog-Api-Client` |
| `github` | GitHub Copilot Chat | Copilot integration headers + `editor-version` / `x-github-api-version` |
| `grok-web` | x.com Grok web | Full MODEL_MAP (grok-3/4/4.1/4.2 + thinking variants) |
| `iflow` | apis.iflow.cn | HMAC-SHA256(userAgent + sessionID + tsMs, apiKey) signature |
| `kiro` | AWS CodeWhisperer Kiro | OpenAI → `conversationState.history + currentMessage` translator |
| `ollama-local` | localhost:11434 | Direct OpenAI-compat passthrough |
| `opencode` / `opencode-go` | OpenCode | Bearer + OpenAI-compat |
| `perplexity-web` | perplexity.ai | Cookie-auth + `{query, sources}` shape |
| `qoder` | qoder.com | Bearer + OpenAI-compat |
| `qwen` | chat.qwen.ai | Bearer + `source: web` header |
| `vertex` (+ `vertex-partner`) | Google Vertex AI | `/v1/projects/<pid>/locations/<loc>/publishers/google/models/<m>` template |

Adding a new vendor = drop a file under [`internal/executors/`](internal/executors/) implementing `Executor.Stream()` + `NonStream()`. `init() { Register(…) }` and the dispatcher picks it up automatically — true plug-and-play.

---

## 🕵️ MITM Proxy — let IDE traffic flow through the router

The MITM module turns Flow Router into a **transparent local TLS proxy** for AI-coding IDEs (Antigravity, GitHub Copilot, Cursor, Kiro). Each IDE thinks it's talking to the upstream vendor; in reality the request is intercepted, normalized, and passed through the full dispatcher chain (combos, fallback, RTK token-saver, usage tracking).

**What you get**

- **Per-domain root CA + leaf certs** — auto-generated 4096-bit RSA root in `<dataDir>/mitm/rootCA.pem`; per-SNI leaves signed on demand and cached. Install the root once in your OS trust store and intercepted IDEs see valid certs.
- **DNS hijack** — marker-wrapped block appended to the OS hosts file (`/etc/hosts` on Unix, `…\drivers\etc\hosts` on Windows with atomic rename + backup). Idempotent: re-applying is byte-identical.
- **Per-IDE rewriters** under [`internal/mitm/handlers/`](internal/mitm/handlers/) — `antigravity / copilot / cursor / kiro` each strips IDE-specific headers (cursor-checksum, codewhisperer profile-arn, copilot-integration-id) and re-routes to `127.0.0.1:2402/v1/chat/completions`.
- **Lifecycle manager** — pidfile under `<MITMDir>/.mitm.pid`, `Start/Stop` couples DNS hijack with the TLS listener so cleanup is symmetric.
- **Privilege awareness** — `IsAdmin()` works on Windows (whoami /groups) and Unix (euid). Hosts-file write tries direct first, then passwordless `sudo`. On Windows the `RunElevatedPowerShell` helper triggers UAC.

**Intercepted hosts** (TARGET_HOSTS): `daily-cloudcode-pa.googleapis.com` · `cloudcode-pa.googleapis.com` · `api.individual.githubcopilot.com` · `q.us-east-1.amazonaws.com` · `api2.cursor.sh`.

> **Safety:** the TLS listener binds `127.0.0.1` only — it cannot accidentally expose to the network. The OS root CA install is a deliberate user-driven step (the binary writes the PEM but does not auto-install — blind auto-install across OS variants is too risky).

---

## 🛠️ `flow-cli` — stand-alone control binary

Drives a running Flow Router via its HTTP API. No external Go deps; one binary per OS.

```bash
go build -o flow-cli ./cmd/flow-cli
./flow-cli status                      # version + uptime + auth
./flow-cli providers                   # list connections
./flow-cli keys new dev-laptop         # create + clipboard-copy a key
./flow-cli settings                    # pretty-print full settings
./flow-cli ui                          # interactive menu (Providers / Keys / Combos / Settings / CLI Tools)
./flow-cli tray status                 # per-OS desktop notification
./flow-cli autostart enable            # register login-time autostart
```

**Tray** is per-OS:

- **Windows** — real native tray via `scripts/tray-win.ps1` (Forms.NotifyIcon: Open dashboard / Check status / Quit)
- **Linux / macOS** — CGO-free control surface via `scripts/tray-{linux,mac}.sh` (notify-send / osascript + xdg-open / open). A real persistent menu-bar icon needs CGO (AppIndicator / Cocoa) and is intentionally left as a sub-binary build option.

**Autostart** is per-OS too — `flow-cli autostart enable` writes the matching entry: Linux `~/.config/autostart/flow_router.desktop`, macOS `~/Library/LaunchAgents/com.flow_router.plist` (auto-loaded via `launchctl load`), Windows `HKCU\…\Run\flow_router` via PowerShell.

---

## 🔌 Supported Providers

Flow Router speaks three API dialects, so it works with essentially any modern LLM provider:

- **Subscription / OAuth** — Claude (Pro/Max), Codex, Cursor, GitLab, iFlow, Kiro
- **API-key cloud** — OpenAI, Anthropic, Google Gemini, DeepSeek, Groq, Together, Mistral, OpenRouter and any OpenAI-compatible endpoint
- **Local** — llama.cpp / `llama-server`, Ollama, LM Studio, vLLM, or any OpenAI-compatible local server

Add models freely — there is no hardcoded allow-list. If a provider exposes a `/models` endpoint, Flow Router can discover and validate it.

---

## 🔗 API Reference

Flow Router exposes a multi-dialect surface so any client just works:

| Endpoint | Dialect |
|---|---|
| `POST /v1/chat/completions` | OpenAI (streaming supported) |
| `POST /v1/messages` | Anthropic Messages |
| `POST /v1/responses` | OpenAI Responses |
| `GET  /v1/models` | OpenAI model list |
| `GET  /v1beta/models` · `POST /v1beta/models/{model}:generateContent` | Gemini |
| `POST /v1/embeddings` · `/v1/images` · `/v1/audio` · `/v1/search` | Media routing |

**Brain API (shared knowledge brain — see [Brain Endpoint](#-brain-endpoint--shared-portable-knowledge-brain)):**

| Endpoint | Purpose |
|---|---|
| `GET /api/brain/status` · `GET/PUT /api/brain/config` | Brain availability, size, wing stats · Toggle, mode, DB path, top-K |
| `POST /api/brain/test` | Preview RAG retrieval + skills for a query (what enrichment would inject) |
| `GET /api/brain/explore` · `/api/brain/by-type` · `/api/brain/personas` | Content overview · Typed memory · Subagent personas |
| `GET/POST/PUT/DELETE /api/brain/constitution` | Sacred-rule CRUD (soft-delete, FQP-12 honouring) |
| `GET /api/brain/contributions` · `POST /api/brain/ingest/run` | Compounding queue · Run ingest (contributions → drawers + FTS) |
| `GET /api/brain/search-drawers` | flowork-kernel-compatible RAG (`{query,hits[],count}`) — what thin bodies hit |

**Streaming example:**

```bash
curl -N http://127.0.0.1:2402/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-haiku-4-5","stream":true,
       "messages":[{"role":"user","content":"Count to 3"}]}'
```

Management APIs live under `/api/*` (providers, combos, usage, tunnel, oauth, mcp, tags, pricing, cli-tools) and power the dashboard.

---

## 🚢 Deployment

Flow Router is a single binary, so deployment is trivial:

```bash
# Run on a VPS, bind to all interfaces behind your own TLS/reverse proxy
./flow-router -addr 0.0.0.0:2402
```

For remote access without opening ports, use the built-in **Tunnel** panel:

- **Cloudflare Tunnel** — instant public `*.trycloudflare.com` URL
- **Tailscale** — private mesh access over your tailnet

Need an edge proxy? The **Proxy Pools** panel generates ready-to-deploy worker scripts for **Cloudflare Workers**, **Deno Deploy** and **Vercel Edge**.

---

## 🧱 Tech Stack

- **Language:** Go 1.25 (single static binary, no CGO required for core)
- **Storage:** embedded SQLite (`~/.flow_router/db/data.sqlite`)
- **Brain (optional):** Memory Palace SQLite with FTS5 BM25 cascade (zero-config at `~/.flow_router/brain/flowork-brain.sqlite`); supports drawer collections up to several million chunks with sub-second retrieval
- **Skills:** 40 behavioural skills **embedded at compile time** (`go:embed`) — travel with the binary, no extra files
- **UI:** embedded single-page dashboard (no build step required to run)
- **Footprint:** small single binary, low memory — runs comfortably on a Raspberry Pi or mini PC (use thin-mode `FLOWORK_BRAIN_REMOTE` to keep the brain on a server while the body stays light)

---

## 🗺️ Roadmap

- [x] Multi-dialect endpoint (OpenAI / Anthropic / Gemini) + streaming
- [x] Tool calling translation
- [x] CLI auto-config, OAuth flows, tunnels, MITM, usage analytics
- [x] MCP registry with live tool discovery
- [x] Streaming tool-use rounds
- [x] Per-intent multiplexing (local model for private prompts, cloud for the rest)
- [x] Cross-device sync (pull-based config sync between instances)
- [x] **Shared knowledge brain** — Memory Palace FTS5 cascade + embedded skill library + auto-injection
- [x] **Brain admin dashboard** — Overview · Search · Constitution editor · Typed Memory · Personas · Compounding
- [x] **Sole-owner brain** — read + write + admin + auto-learn (Option C)
- [x] **Compounding ingest loop** — interactions → drawers + FTS, quality-gated
- [x] **Thin / Pi body mode** — `FLOWORK_BRAIN_REMOTE` flag for any Flowork-style agent
- [x] **flowork-kernel-compatible RAG endpoint** — `/api/brain/search-drawers`
- [x] **Login rate limiter** — progressive lockout per-IP
- [x] **DB backup + migrations** — `VACUUM INTO` snapshots, versioned migrations
- [x] **Advanced RTK token-saver** — 11 tool-output filters with auto-detect
- [x] **18 vendor executors** — antigravity, azure, codex, cursor, github, grok-web, iflow, kiro, qwen, vertex, …
- [x] **MITM TLS proxy** — cert manager + DNS hijack + per-IDE rewriters
- [x] **`flow-cli` companion binary + interactive TUI + tray + autostart**
- [x] **Drop-in SKILL.md packages** — 7 skills any Claude/Cursor/ChatGPT can ingest
- [ ] **GUI tabs for new features** — Backups · MITM control panel · vendor executor selector dropdown · tunnel-watchdog status indicator (planned next dashboard pass)

---

## ❓ FAQ

**Is Flow Router a LiteLLM or OpenRouter alternative?**
Yes. It's a self-hosted, open-source LLM gateway that gives you one OpenAI-compatible endpoint for every provider — like LiteLLM or OpenRouter, but as a single Go binary (no Python, no Docker, no managed service) with a built-in dashboard.

**How do I self-host an OpenAI-compatible API proxy?**
Download the binary, run `./flow-router-bin`, add a provider key in the dashboard, and point any tool at `http://127.0.0.1:2402/v1`. That's the whole setup — see [Quick Start](#-quick-start).

**Which LLM providers does it support?**
OpenAI (GPT), Anthropic (Claude), Google Gemini, DeepSeek, Groq, OpenRouter, Mistral, Qwen, and any OpenAI-compatible endpoint — plus local models via Ollama, vLLM and llama.cpp.

**Does it work with Claude Code, Cursor and Codex?**
Yes — auto-config for Claude Code, Cursor, Codex, Cline, OpenCode and more. Drive them through your existing Claude Pro/Max subscription instead of paying per-token API.

**Does it support streaming, tool calling and MCP?**
Yes — streaming SSE, OpenAI ⇄ Anthropic ⇄ Gemini tool-call translation (incl. streaming tool-use rounds), and an MCP registry with live tool discovery.

**Can different agents (OpenClaw, Hermes, Cursor, Claude Code, Flowork) share the same brain?**
Yes — that's the core idea. The shared brain is served via the OpenAI-compatible endpoint, so any agent that points `base_url` at Flow Router and selects the brain model gets the same retrieved knowledge + skills + persona, server-side, with **zero changes to the agent itself**. One brain, many bodies.

**Which agent is the most-integrated companion?**
[**Flowork**](https://github.com/flowork-os/flowork-ai-agent) — it ships native support for `FLOWORK_BRAIN_REMOTE`, the full caretaker pipeline (ingestor, training, dashboards), and the matching multi-agent runtime. Flow Router works great solo or with any compatible agent; pair it with Flowork for the most-integrated stack.

**How does the brain learn?**
Opt-in compounding. Every brain-tagged interaction can be queued as a contribution; `POST /api/brain/ingest/run` (or the dashboard button) turns the queue into FTS-indexed knowledge chunks the brain serves next time. All connected agents benefit.

**Can I run a thin body (Raspberry Pi) without a local brain DB?**
Yes. Set `FLOWORK_BRAIN_REMOTE=http://<router>:2402` on a Flowork-style body — the kernel's RAG path routes through Flow Router and the body needs no local 32 GB brain DB nor an embed server.

**Is my data private?**
Everything runs on `localhost`. Keys are encrypted at rest; traffic never leaves your machine unless you opt into a tunnel.

---

## 🤝 Contributing

Contributions are welcome! Open an issue to discuss a feature or bug, or submit a pull request. Please keep changes focused and include a clear description.

---

## 📄 License

Released under the [MIT License](LICENSE). Free to use, modify and self-host.

---

<div align="center">

**Flow Router** — your AI traffic, your rules, your machine.

Pairs perfectly with **[Flowork AI Agent](https://github.com/flowork-os/flowork-ai-agent)** — the purpose-built body for the "1 brain, many bodies" stack.

⭐ Star this repo if it saves you time or money.

<sub>Keywords: AI gateway · LLM gateway · LLM proxy · LLM router · OpenAI-compatible API · self-hosted · LiteLLM alternative · OpenRouter alternative · multi-provider · OpenAI · Anthropic Claude · Google Gemini · DeepSeek · Ollama · vLLM · Claude Code · Cursor · MCP · Go single binary · AI agent · autonomous agent · agentic AI · multi-agent · shared brain · Memory Palace · RAG · brain-as-a-service · Flowork · 1 brain many bodies</sub>

</div>
