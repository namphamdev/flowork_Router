# Brain Endpoint — Autonomous Session Report (2026-05-28)

Worked autonomously overnight while you slept. Everything below is on branch
`brain-endpoint`, pushed to the **private** remote (`personal`) only — public
`origin` stays  (verified: public has only `main`, no brain code).

## ✅ Done + tested

**Phase 0–3** (earlier): brain bridge (read-only FTS5 RAG over the 32 GB Memory
Palace, AND-first → ~337 ms), 40 embedded skills + selector, enrichment
(augment/brain mode) hooked into both dispatchers, GUI 🧬 Brain tab, compounding
queue (per-agent contributions), all unit + e2e tested.

**Phase 4 — real Ollama** ✅
- Installed Ollama **user-local, no sudo** → `~/.local/ollama` (v0.24.0).
- `ollama serve` running, detected your **RTX 4060** (CUDA). Note: only ~1.3 GiB
  VRAM was free (desktop using the rest), so the 4.7 GB brain model runs
  CPU-split = slow first load, ~2.6 s/response once warm.
- Imported `models/brain-flowork.gguf` into Ollama as **`flowork-brain`** (5 GB,
  non-destructive — your gguf untouched).
- **LIVE e2e PASS**: agent → flow_router (brain on) → enriched RAG → real Ollama
  model → response → contribution recorded. Path 100% proven.
- ⚠️ The brain gguf output itself is low quality/repetitive ("This is not a
  sentence…") — that's the fine-tune model (likely needs a proper chat template
  / more training), NOT flow_router. flow_router's job works perfectly.

**Phase 5b — brain dashboards in flow_router** ✅ (additive, safe)
- New read-only endpoints: `/api/brain/explore` (counts: 859k active drawers,
  46 constitution, 1 agent, 40 skills, category + source breakdowns),
  `/api/brain/constitution` (sacred rules).
- GUI 🧬 Brain tab restructured into sub-tabs: **Overview · Search · Constitution
  · Config** — backed by real brain-DB data. Verified live.
- 🔒 Locked stable files: `internal/brain/explore.go`, `handlers_brain.go`
  (plus the earlier brain.go/retrieve.go/skills.go/brainenrich.go).

## ⏸️ Deliberately DEFERRED (needs you / flowork stopped) — my judgment call

You said decide autonomously; my decision was **not to damage your main project
unsupervised**. These need flowork **stopped** and your eyes on it:

- **Phase 5a — move the 32 GB brain DB into `~/.flow_router/brain/`.** ABORTED at
  runtime: I re-checked and **flowork daemons were live with the DB open for
  WRITE** (flowork-worker/gui/telegram/brain-maintenance). Moving a 32 GB DB
  being written by 6 processes risks corruption. Safe to do only when flowork is
  stopped. Same disk → the move itself is instant (rename).
- **Phase 5c — migrate the rest of flowork's brain GUI menus → flow_router, then
  delete them from flowork + cut flowork's `BrainProvider`/db layer over.** This
  is large (Memory Palace / Sovereignty / Brain V3 cascade / FQ-Brain Explorer
  with ~20 sub-tabs, Identity & Doktrin, Tasking) and most need their backend
  data layer ported too. Tearing down working flowork features half-finished +
  unsupervised would leave your main project broken, so I built the flow_router
  side first and stopped before the teardown.

## ▶️ Next steps for you (supervised)

1. Stop flowork daemons → `mv` brain DB to `~/.flow_router/brain/flowork-brain.sqlite`
   (flow_router's zero-config default path), then start flowork pointed at the
   new location (update `brain/db/schema.go` + `shared.go` — currently hardcode
   `<workspace>/brain/...`; only `proxy.go` honors `FQBRAIN_DB_PATH`).
2. Enable Brain in flow_router (GUI 🧬 Brain → Config → toggle on). Add an Ollama
   provider serving `flowork-brain` (preset exists), or alias it to a cloud model.
3. Decide scope for the full GUI-menu migration (Phase 5c) and we do it together.

— left the machine as you asked (shutting down after this commit).

## Thin / Pi mode (the tail — flowork reads via flow_router)

flow_router exposes `GET /api/brain/search-drawers?query=X&k=N` (flowork-kernel
compatible shape). flowork's kernel honors `FLOWORK_BRAIN_REMOTE`:

- **Full caretaker (default, unset):** flowork reads its local brain (worker GUI)
  and runs the rich ingestor/training. Unchanged.
- **Thin / Pi body (set `FLOWORK_BRAIN_REMOTE=http://<flow_router-host>:2402`):**
  the kernel fetches RAG from flow_router's shared brain — **no local 32GB DB
  needed**. Run only the body daemons; do NOT run the ingestor/mining on the
  thin body (those stay on the caretaker/server). Verified: kernel builds + the
  remote-routing unit test passes (mock flow_router).
