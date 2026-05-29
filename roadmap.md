# Roadmap — Brain & Memory di flow_Router (collective brain)

> **🔪 CUT items 2026-05-29** (anti over-engineering):
> - `provider/local_transformer.go` — pure Go transformer (Ollama + llama.cpp section 25 sudah cover)
> - `transport_v1.go` (mesh) — YAGNI, langsung v2 saja
> - `federation_peers` / `federation_sync_log` tables — skip kalau single host
> - Brain V3 (`internal/brainv3/`) — defer, brain V1/V2 sufficient
> - AI Forge (`internal/aiforge/`) — defer total (training orchestration overkill)
> - REM training (`internal/rem_train/`) + fine-tune (`internal/finetune/`) — defer P3 (butuh training data + GPU)
> - MoE experts table — defer (single model fine)
> - LoRA delta sync (section 21) — defer P3 (sudah marked)
>
> Filosofi: **single-owner sekarang. YAGNI strict.** Implement HANYA yang ada use case real. Lebih sedikit code = lebih sedikit bug = lebih sedikit halu.

> **Konteks 2-tubuh.** Flowork dipecah jadi 2:
> - **Router** (`Documents/flowork_Router/`, port `:2402`) = brain kolektif. Stateless soal "siapa". **← Repo ini.**
> - **Agent** (`Documents/Flowork_Agent/`, port `:1987`) = body + identitas warga. Stateless soal "knowledge bareng".
>
> Roadmap ini cuma untuk **ROUTER** — brain bersama yang setiap warga akses. Lihat `Flowork_Agent/roadmap.md` untuk state per-warga.
>
> **Status awal:** schema DB lengkap (27 tabel di `brain/flowork-brain.sqlite`), data corpus 5M+ drawers + 1M+ embeddings sudah ada (di-copy dari iterasi lama). Brain CRUD primitives + Retrieve + FTS + enrich sudah port (`internal/brain/*`). Tapi **logic processor** untuk grow brain (ingest pipeline, scoring, sanitize, learner, dst) belum di-port. Roadmap ini fokus ke gap itu.

---

## Section 1 — Ingestion pipeline (drawer baru) ✅ DONE 2026-05-29

**Goal:** router bisa **grow brain sendiri** dari input (chat record, manual submit, federation sync). Saat ini brain read-only — file copy dari brain lama. Tanpa ingestion, brain stagnan.

> **✅ Selesai 2026-05-29** — full end-to-end verified (single submit, dedupe, batch dengan stats agregat akurat). Implementation:
> - `internal/brain/write.go::AddDrawerFull(ctx, opts)` — write primitive dengan full param control
> - `internal/ingest/` — package baru (orchestrator + sanitize + score)
> - `handlers_brain_ingest.go` — endpoint `POST /api/brain/ingest/submit` + `/batch` (max 1000 items)
> - `referensifile/go.mod` — separate module supaya parent build clean
>
> Detail lihat `Changelog` entry 2026-05-29 19:55 WIB. **Defer ke section lain**: embedding generation (Sec 5), re-score job (Sec 2), backpressure rate-limit, doc-specific chunker.

**Komponen:**

- **`ingest.go` core pipeline** — orchestrator: input → sanitize → score → dedupe → write drawer + embedding + FTS
- **`ingest_docs.go`** — doc-specific ingestion (markdown/text dengan chunking strategy)
- Validation: content_hash dedupe (existing di schema drawers)
- Backpressure: rate limit per-source (kalau federation push burst)

**Endpoint baru:**
- `POST /api/brain/ingest/submit` — body `{content, wing, room, source_type, source_file, mem_type, importance?}` → run pipeline → return drawer_id
- `POST /api/brain/ingest/batch` — body `{items: [...]}` → batch ingestion (untuk doc imports)
- (existing) `/api/brain/ingest/run` — sudah ada handler stub, isi dengan pipeline

**Referensi file:**
- [`section_01_ingestion/ingest.go`](referensifile/section_01_ingestion/ingest.go) — pipeline orchestrator (395 LOC, dari flowork lama)
- [`section_01_ingestion/ingest_docs.go`](referensifile/section_01_ingestion/ingest_docs.go) — doc chunking + ingest (242 LOC)

**Acceptance criteria:**
- Endpoint `/api/brain/ingest/submit` jalan, drawer row baru muncul + embedding generated.
- Dedupe verified: 2x submit same content → 1 row.
- Test: import 100 doc → all chunked + indexed di FTS.

---

## Section 2 — Importance scorer ✅ DONE (phase 1) 2026-05-29

**Goal:** tiap drawer dapet score importance (0-10). Score nentuin retrieval rank, retention priority, eligibility buat sacred lens (constitution promotion).

> **Phase 1 (sekarang)**: endpoint `POST /api/brain/rescore` admin trigger (batch re-score live drawers via existing `ingest.Score`). `Score()` udah ada (Section 1 LOCKED) + sudah hooked ke ingestion pipeline.
> **Defer**: cron weekly + retrieval-frequency tracking (butuh schema kolom `retrieval_count` atau separate hits table — defer sampai ada use case real).

**Logic:**
- Heuristik berbasis: signal_words count (keyword penting), source_type reputation, chunk_index (intro biasanya important), explicit flag dari ingest caller
- Output: float ke kolom `drawers.importance`
- Re-score job (cron weekly): recompute untuk drawer yang sering di-hit retrieval

**Referensi file:**
- [`section_02_importance_scorer/importance_scorer.go`](referensifile/section_02_importance_scorer/importance_scorer.go) — scoring algorithm (201 LOC)

**Acceptance criteria:**
- `Score(content, sourceType, metadata) float` function ada.
- Hooked ke ingestion pipeline section 1 (sebelum write).
- Re-score endpoint `POST /api/brain/rescore` — admin trigger.

---

## Section 3 — PII strip (sanitize sebelum simpan) ✅ DONE (phase 1) 2026-05-29

**Goal:** strip email, phone, credit card, NIK, dst sebelum content masuk drawer. Critical buat privacy + compliance.

**Logic:**
- Regex pattern lib (email, IP, phone, card, NIK Indonesia, dll)
- Replace dengan token `[REDACTED:email]`, `[REDACTED:phone]`, dst
- Audit: log how many strips di interaction record (jangan log isi yang di-strip)

**Referensi file:**
- [`section_03_pii_strip/pii_strip.go`](referensifile/section_03_pii_strip/pii_strip.go) — pattern + strip function (175 LOC)

**Acceptance criteria:**
- `StripPII(content) (cleaned, stripCount)` function.
- Wired ke ingestion pipeline (sebelum scoring).
- Audit log endpoint: `GET /api/brain/pii/audit?from=&to=` → count per type.

---

## Section 4 — Prompt injection detector ✅ DONE (phase 1) 2026-05-29

**Goal:** detect & flag content yang berisi prompt injection attempt (override system, ignore previous, jailbreak). Drop content kalau confirmed, atau quarantine kalau ambiguous.

> **Phase 1 scope**: signature-based detector library + admin endpoint untuk test. Quarantine workflow + integrate via `drawers.quarantined` defer kalau handler ingest perlu (file LOCKED).

**Logic:**
- Pattern detect: "ignore previous instructions", "you are now", "system:", role hijack
- Score 0-1 → above 0.7 quarantine to `knowledge_quarantine` table, log `prompt_injection_log`
- Whitelist context (mis. content edukasi tentang injection itself OK)

**Referensi file:**
- [`section_04_prompt_injection/prompt_injection_detector.go`](referensifile/section_04_prompt_injection/prompt_injection_detector.go) — detector (133 LOC)

**Acceptance criteria:**
- `DetectInjection(content) Score` function.
- Tabel `knowledge_quarantine` ditambah (atau pakai field `drawers.quarantined` yang sudah ada).
- Quarantine workflow: review endpoint `/api/brain/quarantine/list` + approve/reject.

---

## Section 5 — Quality gate ✅ DONE (phase 1) 2026-05-29

**Goal:** filter content low-quality sebelum jadi drawer. Spam, hallucination, gibberish, duplicate semantic, content yang too short — semua drop atau lower importance.

> **Phase 1 scope**: pure heuristic library `internal/quality/quality.go` (length / repetition / whitespace ratio / char diversity) + admin test endpoint `POST /api/brain/quality/check`. Caller invoke optional pre-ingest.
> **Defer**: embedding-based semantic duplicate (butuh embedding pipeline Section 5 Router), LLM coherence judge (mahal), ingest_log table write (schema tabel belum ada), wire ke `ingest.Submit` (file LOCKED — phase 2 via opsi caller).

**Logic:**
- Length check (min/max bytes)
- Repetition detector (string repeat 3+ → spam)
- Semantic duplicate (embedding cosine > 0.97 dengan existing → reject)
- Coherence check (kalau LLM judge tersedia → optional)

**Referensi file:**
- [`section_05_quality_gate/quality_gate.go`](referensifile/section_05_quality_gate/quality_gate.go) — gate logic (318 LOC, paling besar)

**Acceptance criteria:**
- `QualityCheck(content, embeddings) Result` function.
- Wired ke pipeline (setelah PII + injection).
- Rejected content logged ke `ingest_log` (already in flowork-settings.sqlite).

---

## Section 6 — Tool learner (pattern extraction) ✅ DONE (phase 1) 2026-05-29

**Goal:** dari interaction history, learn pattern `trigger → tool used → success`. Simpan di `tool_patterns` table. Bantu warga di-suggest tool pas natural language match pattern.

**Logic:**
- Setelah interaction selesai (di recorder section 10), kalau ada tool call + success → upsert tool_patterns row dengan increment success_count
- Failed call → increment fail_count
- Pattern `trigger_pattern` di-extract dari input text (n-gram atau embedding cluster)

**Endpoint:**
- `GET /api/brain/tool-patterns?trigger=<text>` → return ranked tool suggestions

**Referensi file:**
- [`section_06_tool_learner/tool_learner.go`](referensifile/section_06_tool_learner/tool_learner.go) — learner (124 LOC)

**Acceptance criteria:**
- `LearnPattern(trigger, tool, success)` function.
- Endpoint suggest jalan, return top-K tool.
- Decay: tool_patterns hit_count tua di-decay biar adaptive.

---

## Section 7 — Mistakes journal (global tier) ✅ DONE (phase 1) 2026-05-29

> **⚠️ OVER-PROMPT RISK** — brain_antibody jangan auto-inject ke setiap chat. Validate dulu via semantic match query, lalu inject MAX 3 antibody relevant. Sisanya retrieved on-demand via `brain_search`.

> **Phase 1 scope (sekarang)**: schema + endpoint POST submit + GET list. Validate hit_count ≥ 3, category whitelist.
> **Defer phase 2**: brain_antibody auto-promotion (cross-reference dengan `internal/brain/write.go::AddDrawerFull` ke wing='antibody'), WebSocket notify warga lain.

**Goal:** receive promotion dari agent (lihat agent roadmap section 2 & 7). Validate. Insert ke `brain_antibody` global. Distribute ke semua warga via skill/contributions.

**Endpoint baru:**
- `POST /api/mistakes/submit` — body `{agent_id, category, title, content, hit_count}`
  - Validate: hit_count ≥ 3 (atau admin override), category valid
  - Insert `brain_antibody` row dengan antigen_classes + prevention_checklist
  - Optional: notify agen lain via WebSocket (future)

**Tabel baru di brain DB:**

```sql
-- mistakes_journal sudah ada di schema (di settings.sqlite tier flowork lama)
-- Port struktur ke brain DB router:
CREATE TABLE IF NOT EXISTS mistakes_journal (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  category        TEXT NOT NULL,
  title           TEXT NOT NULL,
  content         TEXT NOT NULL,
  source_agent_id TEXT NOT NULL,
  hit_count       INTEGER DEFAULT 1,
  tier            TEXT DEFAULT 'global',  -- 'global' | 'sacred' (post-review)
  reviewed_at     TEXT,
  promoted_to_antibody_id TEXT,
  created_at      TEXT DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(category, title)
);
```

**Referensi file:**
- [`section_07_mistakes_global/mistakes_journal.go`](referensifile/section_07_mistakes_global/mistakes_journal.go) — journal logic (181 LOC, sama pattern dengan agent section 2 — beda tier)

**Acceptance criteria:**
- Endpoint `/api/mistakes/submit` jalan.
- Insert `mistakes_journal` + `brain_antibody` (kalau memenuhi syarat).
- Endpoint `GET /api/mistakes/list?tier=&category=` listing.

---

## Section 8 — Skill catalog API ✅ DONE (phase 1) 2026-05-29

> **⚠️ OVER-PROMPT RISK** — catalog API return MAX 10 skill summary per query (id + label + 1-line desc). Full instructions DI-FETCH on-demand via `GET /api/skills/get?id=` saat warga eksplisit pilih skill. JANGAN bundle full catalog ke setiap response.

**Goal:** router host catalog skill — definisi skill (id, trigger, instructions, schema input/output). Warga **pull** dari catalog buat browse + adopt skill (jadi instance di per-agent state.db).

**Endpoint baru:**
- `GET /api/skills/list?category=&search=` — list available skills
- `GET /api/skills/get?id=<skill_id>` — detail skill
- `POST /api/skills/contribute` — submit skill baru dari agent (review-pending workflow)

**Sumber data:**
- Existing `skilldata/*.md` di `internal/brain/skilldata/` (sudah ada di flow_router)
- Tabel `skills` di brain DB (sudah ada schema, perlu populate)

**Code:**
- `internal/brain/skills.go` (already exists, 131 LOC) — extend dengan ContribAPI + search

**Referensi file:** ngga ada port langsung — leverage existing `internal/brain/skills.go` di router (sudah ada). Pattern import skilldata `.md` → tabel skills.

**Acceptance criteria:**
- Endpoint list + get jalan, return JSON.
- Agent Mr.Flow dapat pull list (verify via curl).
- Contribute endpoint: submit + pending state + admin approve.

---

## Section 9 — Sensors (input layer) ✅ DONE (phase 1) 2026-05-29

**Goal:** ingest dari sumber lain selain HTTP submit — file watch (drop file ke folder, auto-ingest), webhook (forward dari external), schedule (periodic refresh dari URL).

**Logic:**
- File watcher: scan folder `~/.flowork-router/drop/` → ingest file (markdown/txt) lewat pipeline section 1
- Webhook receiver: `POST /api/sensors/webhook?source=<id>&token=<...>` → forward content
- Scheduler: cron jobs di settings → fetch URL → ingest

**Referensi file:**
- [`section_09_sensors/sensors.go`](referensifile/section_09_sensors/sensors.go) — sensor manager (260 LOC)

**Acceptance criteria:**
- File watcher jalan saat router boot, log "watching <dir>".
- Drop file → drawer baru muncul.
- Webhook endpoint jalan + token auth.

---

## Section 10 — Recorder + routing rules + proxy ✅ DONE (phase 1 — recorder only) 2026-05-29

**Goal:** setiap chat-LLM interaction yang lewat router di-record (request + response + metadata) ke tabel `recordings`. Buat training data + audit. Plus routing rules: decide model mana yang dipakai berdasar request.

> **Phase 1 scope (sekarang)**: schema `recordings` + library `internal/recorder/recorder.go` + admin endpoint POST record + GET list. Standalone — caller wajib explicit invoke `recorder.Record()`.
> **Defer phase 2/3**: router_rules.go (rule engine), proxy.go (outbound proxy + retry + circuit breaker), build_verifier.go, wire middleware ke existing chat handler.

**Komponen:**

- **`recorder.go`** — middleware yang catch request/response di chat handler, persist ke `recordings`
- **`router_rules.go`** — rule engine: kalau request match X → pakai model Y (mis. coding query → claude, casual → haiku)
- **`proxy.go`** — outbound proxy layer (logging + retry + circuit breaker)
- **`build_verifier.go`** — verify response format sebelum return ke caller

**Referensi file:**
- [`section_10_recorder/proxy.go`](referensifile/section_10_recorder/proxy.go) — outbound proxy (344 LOC)
- [`section_10_recorder/recorder.go`](referensifile/section_10_recorder/recorder.go) — recorder (98 LOC)
- [`section_10_recorder/router_rules.go`](referensifile/section_10_recorder/router_rules.go) — routing rules (199 LOC)
- [`section_10_recorder/build_verifier.go`](referensifile/section_10_recorder/build_verifier.go) — verifier (184 LOC)

**Acceptance criteria:**
- Tabel `recordings` ke-populate setelah chat (verify via SQL).
- Rule engine: edit rule via API → next chat routed sesuai.
- Circuit breaker: 3 fail berturut → temporary disable model.

---

## Section 11 — Model pool (multi-model registry + refresh + resolver)

**Goal:** maintain catalog model dengan cost, context_window, status. Refresh otomatis dari provider API (OpenRouter, OpenAI, Anthropic). Resolver pick model best-fit untuk request berdasar criteria (cost, quality, context size).

**Komponen:**

- **`model_pool.go`** — load model_pool dari DB, expose Get(id), List(category)
- **`model_refresh.go`** — periodic refresh dari provider (cron)
- **`model_resolver.go`** — pick best model dari criteria

**Endpoint:**
- `GET /api/models/list?category=&max_cost=` — filtered list
- `POST /api/models/resolve` — body `{criteria}` → return picked model + reasoning
- `POST /api/models/refresh` — admin trigger manual refresh

**Referensi file:**
- [`section_11_model_pool/model_pool.go`](referensifile/section_11_model_pool/model_pool.go) (123 LOC)
- [`section_11_model_pool/model_refresh.go`](referensifile/section_11_model_pool/model_refresh.go) (129 LOC)
- [`section_11_model_pool/model_resolver.go`](referensifile/section_11_model_pool/model_resolver.go) (112 LOC)

**Acceptance criteria:**
- Tabel `model_pool` populated setelah refresh.
- Resolver pick model berdasar test criteria + log reasoning.
- Cron refresh weekly, last_refresh meta updated.

---

## Section 12 — Discipline constitution

**Goal:** constitution (rules sistem) ada governance: edit harus quorum review, history preserved, integrity check. Plus brain reset utility (controlled).

**Komponen:**

- **`constitution.go`** — load constitution (sudah port sebagian)
- **`discipline_constitution.go`** — quorum review workflow, audit trail
- **`brain_reset.go`** — reset DB utility (admin only, with confirmation)

**Endpoint (mostly ada, perlu lengkap):**
- (existing) GET/POST/PUT/DELETE `/api/brain/constitution` — basic CRUD
- `POST /api/brain/constitution/propose` — propose change (pending quorum)
- `POST /api/brain/constitution/vote?proposal_id=&approve=` — vote
- `POST /api/brain/reset?confirm=YES_NUKE_ALL` — danger zone

**Referensi file:**
- [`section_12_discipline_constitution/discipline_constitution.go`](referensifile/section_12_discipline_constitution/discipline_constitution.go) — discipline logic (~150 LOC)
- [`section_12_discipline_constitution/constitution.go`](referensifile/section_12_discipline_constitution/constitution.go) — constitution loader
- [`section_12_discipline_constitution/brain_reset.go`](referensifile/section_12_discipline_constitution/brain_reset.go) — reset util

**Acceptance criteria:**
- Propose + vote workflow jalan.
- History (constitution_history table sudah ada) preserved on edit.
- Brain reset endpoint protected (confirmation token).

---

## Urutan implementasi (saran prioritas)

| # | Section | Priority | Reasoning |
|---|---|---|---|
| 1 | Section 1 — Ingestion pipeline | 🔴 P0 | Foundation. Tanpa ingestion brain ngga grow. |
| 2 | Section 3 — PII strip | 🔴 P0 | Wajib sebelum brain start grow (privacy). |
| 3 | Section 4 — Prompt injection detector | 🔴 P0 | Wajib sebelum brain start grow (security). |
| 4 | Section 5 — Quality gate | 🟡 P1 | Cegah brain pollution dari low-quality content. |
| 5 | Section 2 — Importance scorer | 🟡 P1 | Hooked ke pipeline; tanpa ini retrieval ranking degrade. |
| 6 | Section 10 — Recorder | 🟡 P1 | Capture interaction = training data + audit. Unblock section 6 (tool learner). |
| 7 | Section 7 — Mistakes journal global | 🟡 P1 | Pair sama agent section 2; harus siap saat agent ready promote. |
| 8 | Section 8 — Skill catalog API | 🟢 P2 | Frontend warga adopt skill. |
| 9 | Section 11 — Model pool | 🟢 P2 | Multi-model resolver — important kalau scope nambah. |
| 10 | Section 6 — Tool learner | 🟢 P2 | Setelah recorder + skill catalog ada. |
| 11 | Section 9 — Sensors (file watch + webhook) | 🟢 P2 | Diversify input. |
| 12 | Section 12 — Discipline constitution governance | 🟢 P3 | Polishing — single-user dulu, governance later. |

**Catatan kerja:**
- Tiap section yang nambah tabel → update `internal/brain/init.go::EnsureSchema()`.
- Tiap section → 1 file Go di `internal/router/` (atau extend `internal/brain/*` kalau topik fit) + handler di root `handlers_brain*.go` atau `handlers_<topic>.go` + route di `routes.go`.
- Tiap section selesai → tulis perubahan di `Changelog/` folder router.

---

## Folder referensi

File Go logic dari `Music/flowork/brain/*` — yang berhasil di iterasi lama — ada di [`referensifile/`](referensifile/), terorganisasi per-section.

```
referensifile/
├── section_01_ingestion/
│   ├── ingest.go
│   └── ingest_docs.go
├── section_02_importance_scorer/
│   └── importance_scorer.go
├── section_03_pii_strip/
│   └── pii_strip.go
├── section_04_prompt_injection/
│   └── prompt_injection_detector.go
├── section_05_quality_gate/
│   └── quality_gate.go
├── section_06_tool_learner/
│   └── tool_learner.go
├── section_07_mistakes_global/
│   └── mistakes_journal.go
├── section_08_skill_catalog/        (no file — pakai internal/brain/skills.go yg ada)
├── section_09_sensors/
│   └── sensors.go
├── section_10_recorder/
│   ├── proxy.go
│   ├── recorder.go
│   ├── router_rules.go
│   └── build_verifier.go
├── section_11_model_pool/
│   ├── model_pool.go
│   ├── model_refresh.go
│   └── model_resolver.go
├── section_12_discipline_constitution/
│   ├── discipline_constitution.go
│   ├── constitution.go
│   └── brain_reset.go
└── _common/
    ├── educational_errors_seed.go
    ├── educational_error_lookup.go
    ├── error_codes.go
    └── softdelete.go
```

**Penting:** file referensi pakai sebagai **pattern**, bukan copy langsung. Adaptasi ke style + struktur project router sekarang. Import path beda (`flowork_Router` vs lama), pattern struct beda, semua patut adjust.

---

## Sinkronisasi dengan Flowork_Agent

Section yang punya counterpart di agent (harus dikerjain berbarengan supaya endpoint contract match):

| Router section | Agent section (lihat `Flowork_Agent/roadmap.md`) | Contract endpoint |
|---|---|---|
| 7 — Mistakes journal | 2 — Mistakes local + 7 — Sync interface | `POST /api/mistakes/submit` |
| 8 — Skill catalog | 7 — Sync (pull) | `GET /api/skills/list`, `GET /api/skills/get?id=` |
| 1 — Ingestion submit | (agent kontrib via interaction logging) | `POST /api/brain/contributions/ingest` (sudah ada) |
| 11 — Model pool | (agent baca via config.router.model) | `GET /api/models/list` |

---

---

# === BAGIAN 2 — MESH (peer-to-peer tanpa internet) ===

> **Konteks**: Mesh stack hidup di **ROUTER** (bukan Agent), per keputusan arsitektur 2-tubuh 2026-05-29. Alasan: mesh = host-level concern (1 host = 1 mesh peer), bukan warga-level. Brain & skill catalog sync = router's domain. Agent jadi thin client via API.
>
> **Sumber referensi**: `/home/mrflow/Pictures/stable_open_router/flowork_project/flowork-kernel/kernel/mesh/` (60+ file Go, ~11K LOC) + `floworkos-go/internal/mesh/` + `cmd/flowork-mesh/`. PRODUCTION-grade mesh stack: mDNS discovery, signed gossip, CRDT, Byzantine fault tolerance, karma-based trust, sneakernet, LoRA delta sync.
>
> Section 13-23 me-roadmap port mesh ke Router. Bahasa-nya teknis (CRDT, BFT, ed25519, mDNS, dst).

---

## Section 13 — Mesh foundation (discovery + identity + bootstrap)

**Goal:** kerangka mesh — generate peer identity (ed25519), discover peer di LAN via mDNS, DNS_seed bootstrap untuk cross-network, blocklist cloud metadata IP.

**Komponen:**

- **`discovery.go`** — mDNS multicast broadcast (port 5353), service tag `_flowork._tcp`, announce setiap 30s. Peer di LAN ke-discover < 5 detik.
- **`dns_seed.go`** — DNS TXT record bootstrap. Pakai DNS seed (`seeds.flowork.io` atau owner-defined) untuk discover peer di luar LAN tanpa harus tau IP.
- **`identity_extended.go`** — self-issued license metadata (machine ID, capabilities advertised).
- **`identity_handshake.go`** — handshake protocol: tukar pubkey, validate signature, establish trust.
- **`fingerprint.go`** — machine fingerprint cross-OS (hardware-based, stabil walaupun OS reinstall).
- **`blocklist.go`** — denylist cloud metadata IP (169.254.169.254 dst). INVARIANT 2.
- **`seed_gist.go`** — bootstrap via GitHub Gist (kalau DNS seed ngga ada).

**Tabel baru di router brain DB (atau dedicated `mesh.db`):**

```sql
CREATE TABLE mesh_identity (
  k          TEXT PRIMARY KEY,
  v          TEXT NOT NULL DEFAULT ''
) WITHOUT ROWID;
-- keys: pubkey_hex, privkey_hex (encrypted), fingerprint, version

CREATE TABLE mesh_peers (
  pubkey_hex     TEXT PRIMARY KEY,
  hostname       TEXT NOT NULL DEFAULT '',
  ip             TEXT NOT NULL DEFAULT '',
  port           INTEGER NOT NULL DEFAULT 0,
  version        TEXT NOT NULL DEFAULT '',
  is_virt        INTEGER NOT NULL DEFAULT 0,
  first_seen_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  trust_score    REAL NOT NULL DEFAULT 0.5,
  blocked        INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_mesh_peers_lastseen ON mesh_peers(last_seen_at DESC);
```

**Endpoint baru (router):**
- `GET /api/mesh/peers` — list peer ditemukan (dipanggil agent via routerclient)
- `POST /api/mesh/discover` — trigger discovery manual
- `GET /api/mesh/identity` — show own pubkey + fingerprint

**Referensi file:** semua di [`section_13_mesh_foundation/`](referensifile/section_13_mesh_foundation/) (7 files):
- `discovery.go`, `dns_seed.go`, `identity_extended.go`, `identity_handshake.go`, `fingerprint.go`, `blocklist.go`, `seed_gist.go`

**Acceptance criteria:**
- Router boot → generate ed25519 keypair, simpan di mesh_identity.
- 2 router di subnet sama → ke-discover ≤ 5 detik via mDNS.
- Blocklist test: response mDNS dari cloud metadata IP → reject.
- Endpoint `/peers` jalan via curl.

---

## Section 14 — Transport + packet + relay (data movement)

**Goal:** layer transport — peer connect, packet structure, relay forwarding via intermediate peer.

**Komponen:**

- **`transport.go`** — HTTP/2 transport dengan TLS (self-signed cert dari identity)
- **`transport_v2.go`** — single version transport. ~~v1~~ ❌ **CUT 2026-05-29**: YAGNI — single owner ngga butuh back-compat versioned transport. Langsung v2.
- **`transport_iface.go`** — interface abstrak (swap-able: UDP/TCP/QUIC/Bluetooth future)
- **`packet.go`** — KnowledgePacket struct: type, payload, signature, ttl, hop_count
- **`peer_connect.go`** — establish connection + handshake + key exchange
- **`peer_discover.go`** — orchestrate (mDNS + DNS seed + gossip combo)
- **`relay.go`** — message relay via intermediate peer (max hop limit)
- **`http_client.go`** — shared HTTP client (timeout + circuit breaker)
- **`canonical.go`** — deterministic byte encoding untuk signing

**Tabel baru:**

```sql
CREATE TABLE mesh_packets (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  packet_id     TEXT NOT NULL UNIQUE,
  origin_pubkey TEXT NOT NULL,
  packet_type   TEXT NOT NULL,        -- 'knowledge' | 'task' | 'gossip' | 'heartbeat'
  payload_json  TEXT NOT NULL,
  signature     TEXT NOT NULL,
  ttl           INTEGER NOT NULL DEFAULT 5,
  hop_count     INTEGER NOT NULL DEFAULT 0,
  received_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  processed     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_mesh_packets_type ON mesh_packets(packet_type);
```

**Referensi file:** [`section_14_mesh_transport/`](referensifile/section_14_mesh_transport/) (11 files)

**Acceptance criteria:**
- Peer A ↔ B connect dengan mutual TLS.
- Packet round-trip < 100ms di LAN.
- Relay test: blok direct A→C, route via B.

---

## Section 15 — Gossip protocol (push/pull + Byzantine fault tolerance)

**Goal:** state dissemination via gossip — peer A gossip ke 3 random peer, exponentially spread. Plus BFT (2-of-3 quorum) untuk emergency broadcast.

**Komponen:**

- **`gossip.go`** — push/pull + bandwidth limit + 2-of-3 BFT broadcast
- **`gossip_signed.go`** — signed envelope (M03)
- **`heartbeat_peer.go`** — periodic heartbeat alive marker
- **`inbound_track.go`** — dedupe by signature

**Pattern (AMENDMENTS-V1 I-3):**
- Normal: random gossip every 10s ke 3 peer
- Emergency (revocation, security): 2-of-3 BFT — broadcast butuh ≥2 trusted signature

**Referensi file:** [`section_15_mesh_gossip/`](referensifile/section_15_mesh_gossip/) (4 files)

**Acceptance criteria:**
- Insert new fact di router A, < 30s reach all peer router.
- BFT: revocation needs ≥2 signature.
- Dedup: same packet 2x dari path beda → second dropped.

---

## Section 16 — CRDT state replication (conflict-free sync)

**Goal:** sync state antar-router tanpa coordinator. CRDT memastikan paralel modify converges to same final state.

**Komponen:**

- **`crdt_event.go`** — event-sourced base (G-Set / OR-Set patterns)
- **`crdt_merge.go`** — conflict resolution (M04.5) — vector clock + last-write-wins
- **`crdt_push.go`** — push local changes ke peer
- **`crdt_sync.go`** — full sync handshake (snapshot + delta)

**Use case:**
- Sync drawers (brain corpus) antar router
- Sync skill catalog updates
- Sync mistakes_journal global (dari mistakes agent push)

**Referensi file:** [`section_16_mesh_crdt/`](referensifile/section_16_mesh_crdt/) (4 files)

**Acceptance criteria:**
- 2 router modify same table → merge converges.
- Vector clock: A insert X, B insert Y, both eventually see X+Y.
- New peer join → snapshot catch-up.

---

## Section 17 — Knowledge share (brain replication antar peer)

> **⚠️ OVER-PROMPT RISK + CONTEXT CONTAMINATION** — knowledge dari peer A masuk ke local brain B, lalu nanti di-retrieve & inject ke warga di B. Tanpa provenance tag, warga ngga tau source dan bisa propagate halu antar-host. WAJIB: setiap drawer dari peer = tag origin pubkey + karma score peer + size limit 1KB per drawer.

**Goal:** router A share drawer brain ke router B. Berbasis pull (B request) atau push (A broadcast). Pair sama section 1 (Ingestion pipeline) — yang di-ingest di router A bisa flow ke router B.

**Komponen:**

- **`knowledge_pack.go`** — pack drawer + embeddings jadi compressed bundle (M8)
- **`knowledge_share.go`** — workflow: discover topic, request pack, verify, ingest
- **`weight_pull.go`** — pull model weight dari peer (kalau peer host model lokal)
- **`weight_seed.go`** — seed weight ke peer baru
- **`tfidf.go`** — TF-IDF ranking untuk relevance

**Tabel baru:**

```sql
CREATE TABLE mesh_knowledge_packs (
  pack_id        TEXT PRIMARY KEY,
  origin_peer    TEXT NOT NULL,
  topic          TEXT NOT NULL,
  drawer_count   INTEGER NOT NULL DEFAULT 0,
  embedding_count INTEGER NOT NULL DEFAULT 0,
  size_bytes     INTEGER NOT NULL DEFAULT 0,
  signature      TEXT NOT NULL,
  received_at    TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ingested       INTEGER NOT NULL DEFAULT 0
);
```

**Endpoint baru:**
- `POST /api/mesh/knowledge/request?topic=<X>&from=<peer>` — agent panggil router untuk minta knowledge pack dari peer

**Referensi file:** [`section_17_mesh_knowledge/`](referensifile/section_17_mesh_knowledge/) (5 files)

**Acceptance criteria:**
- Router A pack 100 drawer → bundle ≤ 10 MB compressed.
- Router B pull pack → verify → ingest via section 1 pipeline (with PII strip, quality gate).
- TF-IDF rank: query → top-K relevant drawer.

---

## Section 18 — Tool manifest sharing (cross-mesh tools)

**Goal:** warga di host A bikin tool di `/shared/<id>/tools/script.py` → agent broadcast manifest ke router → router gossip ke peer router → warga di host B bisa discover.

**Komponen:**

- **`tool_manifest.go`** — tool definition struct (name, capability, script_url, signature). Broadcast via gossip.

**Flow:**
1. Warga A (di host A) bikin tool baru, register di local agent
2. Agent call `routerclient.BroadcastTool(manifest)` (Agent roadmap section 20)
3. Router A gossip manifest ke router B/C
4. Router B insert ke catalog, expose via `/api/mesh/find-tool?capability=X`
5. Warga di host B query catalog → pull script via transport → execute lokal

**Endpoint baru:**
- `POST /api/mesh/broadcast-tool` — agent push tool manifest
- `GET /api/mesh/find-tool?capability=X` — discover tool dari peer
- `GET /api/mesh/tool-fetch?manifest_id=<X>` — fetch script body

**Referensi file:** [`section_18_mesh_toolshare/tool_manifest.go`](referensifile/section_18_mesh_toolshare/tool_manifest.go)

**Acceptance criteria:**
- Warga A register tool → manifest sampai semua router < 1 menit.
- Warga B subscribe → script pulled → execute lokal di sandbox.

---

## Section 19 — Karma per-peer (trust scoring antar host)

**Goal:** setiap peer (= host) dapet trust score. Knowledge dari peer rendah skor di-de-prioritize.

**Komponen:**

- **`karma.go`** — per-peer trust engine. Update on event: knowledge ingested OK (+), spam detect (-), signature invalid (--)
- **`karma_decay.go`** — weekly decay (M5 Step 2). Peer ngga interact → score perlahan turun
- **`trust.go`** — high-level API: `IsTrusted(peer) bool`, `Threshold(operation) float`

**Referensi file:** [`section_19_mesh_karma/`](referensifile/section_19_mesh_karma/) (3 files)

**Acceptance criteria:**
- Peer karma update setelah event.
- Decay cron weekly jalan.
- Untrusted peer (score < threshold) → knowledge auto-quarantine.

---

## Section 20 — Filter pipeline (anti-poisoning) + license

**Goal:** filter incoming knowledge dari peer — 9-lapis defense supaya peer jahat ngga bisa poison local brain. **Wajib sebelum mesh public**. Extend dari section 3-5 (PII strip + injection detect + quality gate).

**Komponen:**

- **`filter.go`** — 9-layer filter pipeline (M4):
  1. Signature verify
  2. Origin trust check (karma)
  3. PII strip (re-use section 3)
  4. Prompt injection detect (re-use section 4)
  5. Quality gate (re-use section 5)
  6. Semantic duplicate (re-use section 5)
  7. Rate limit per peer
  8. Content size limit
  9. Quarantine ambiguous

- **`pull_with_license.go`** — license check (M9). Peer wajib declare license sebelum knowledge bisa di-pull (CC0, MIT, proprietary, dst).

**Referensi file:** [`section_20_mesh_filter/`](referensifile/section_20_mesh_filter/) (2 files)

**Acceptance criteria:**
- 9 filter layer jalan sequential, log decision per layer.
- Test poison: adversarial content → ke-block di layer N.
- License check: pull tanpa declared license → reject.

---

## Section 21 — LoRA delta sync (model weight increment)

**Goal:** sync model weight delta (LoRA adapter) antar router. Kalau router A fine-tune local model dengan data lokal → delta bisa di-share ke router B yang ngga punya GPU.

**Komponen (di folder `lora/`):**

- **`bloom.go`** — Bloom filter membership test
- **`budget.go`** — bandwidth budget per-peer
- **`compress.go`** — delta compression (quantization, sparse)
- **`delta.go`** — LoRA delta struct + apply
- **`frame.go`** — framing protocol (chunked + retry)
- **`priority.go`** — priority queue (important delta first)

**Defer**: butuh model fine-tuning infra dulu. Roadmap stub doang.

**Referensi file:** [`section_21_mesh_lora/`](referensifile/section_21_mesh_lora/) (6 files)

**Acceptance criteria (defer untuk later):**
- LoRA delta 100 MB → compressed ≤ 10 MB.
- Chunked transfer dengan resume.

---

## Section 22 — Layer 3 semantic sync + fallback (degraded mode)

**Goal:** L3 sync = semantic-level (bukan byte-level). Plus fallback kalau mesh degrade.

**Komponen:**

- **`l3_sync.go`** — semantic sync: "what tools exist for X" → peer answer dengan local tool list. Targeted, ngga full sync.
- **`sync.go`** — sync orchestrator (combine L1 byte + L3 semantic)
- **`fallback.go`** — degraded mode: < 2 peer reachable → router-only

**Referensi file:** [`section_22_mesh_l3_sync/`](referensifile/section_22_mesh_l3_sync/) (5 files)

**Acceptance criteria:**
- L3 query roundtrip < 500ms.
- Fallback: kill all peers, router tetap jalan local-only.

---

## Section 23 — Mesh daemon (standalone) + DB schema

**Goal:** mesh bisa run sebagai daemon terpisah (performance) atau embedded di router main process. Plus DB schema lengkap.

**Komponen:**

- **`mesh_main.go`** (dari `cmd/flowork-mesh/main.go`) — standalone daemon
- **`mesh_db.go`** — full SQLite persistence orchestrator
- **`mesh_db_peers.go`** — peer_registry CRUD
- **`mesh_db_packets.go`** — packet log CRUD
- **`mesh_db_shadows.go`** — shadow drawer (untrusted, quarantine) CRUD
- **`mesh_db_stats.go`** — aggregate stats

**Mode:**
- **Embedded**: mesh jalan di goroutine dalam router process (simpler).
- **Daemon**: standalone binary, router connect via Unix socket (performance).

**Endpoint API (lengkap):**
- `/api/mesh/identity`, `/api/mesh/peers`, `/api/mesh/stats`
- `/api/mesh/discover` (manual trigger)
- `/api/mesh/broadcast-tool` (dari agent)
- `/api/mesh/broadcast-mistake` (dari agent)
- `/api/mesh/knowledge/request`, `/api/mesh/find-tool` (discovery)

**Referensi file:** [`section_23_mesh_daemon/`](referensifile/section_23_mesh_daemon/) (6 files)

**Acceptance criteria:**
- Embedded mode: router boot → mesh start automatically.
- Daemon mode: standalone + router connect via socket.
- Semua mesh API endpoint jalan + verify via curl.

---

## Roadmap urutan + dependensi (Bagian 2 — Mesh)

| # | Section | Priority | Dependensi |
|---|---|---|---|
| 1 | Section 13 — Foundation (discovery + identity) | 🔴 P0 | Independen |
| 2 | Section 14 — Transport + packet + relay | 🔴 P0 | Section 13 done |
| 3 | Section 23 — Mesh DB schema | 🔴 P0 | Section 13 done. Parallel dengan 14. |
| 4 | Section 15 — Gossip protocol | 🔴 P0 | Section 14 done |
| 5 | Section 20 — Filter pipeline (anti-poisoning) | 🔴 P0 | Section 15 done. **Wajib sebelum mesh public**. Re-use section 3-5 brain. |
| 6 | Section 16 — CRDT state replication | 🟡 P1 | Section 14+15 done |
| 7 | Section 19 — Karma per-peer | 🟡 P1 | Section 15 + 20 done |
| 8 | Section 17 — Knowledge share | 🟡 P1 | Section 16 + 19 done. Pair sama Bagian 1 section 1 (ingestion). |
| 9 | Section 18 — Tool manifest broadcast | 🟡 P1 | Section 17 done + Agent section 20 client ready |
| 10 | Section 22 — L3 sync + fallback | 🟢 P2 | Mesh stack mature |
| 11 | Section 21 — LoRA delta | 🟢 P3 | Defer — butuh fine-tuning infra |

**Catatan kerja:**
- Mesh = subsystem besar (~75 file, 11K LOC). Implementasi bertahap.
- DB lokasi: dedicated `mesh.db` di `~/.flowork-router/mesh.db` (atau extend `brain/flowork-brain.sqlite`).
- Filter pipeline (section 20) RE-USE komponen dari Bagian 1: PII strip (section 3), injection detect (section 4), quality gate (section 5). Jangan duplicate.

---

## Folder referensi (UPDATED dengan section 13-23)

```
referensifile/
├── _common/                            (4 files)
├── section_01_ingestion/               (2 files)
├── section_02_importance_scorer/       (1 file)
├── section_03_pii_strip/               (1 file)
├── section_04_prompt_injection/        (1 file)
├── section_05_quality_gate/            (1 file)
├── section_06_tool_learner/            (1 file)
├── section_07_mistakes_global/         (1 file)
├── section_08_skill_catalog/           (README pointer ke internal/brain/skills.go)
├── section_09_sensors/                 (1 file)
├── section_10_recorder/                (4 files)
├── section_11_model_pool/              (3 files)
├── section_12_discipline_constitution/ (3 files)
├── section_13_mesh_foundation/         (7 files — discovery, identity, fingerprint, blocklist, seeds)
├── section_14_mesh_transport/         (11 files — transport, packet, relay, peer connect)
├── section_15_mesh_gossip/             (4 files — gossip, signed, heartbeat, inbound_track)
├── section_16_mesh_crdt/               (4 files — event, merge, push, sync)
├── section_17_mesh_knowledge/          (5 files — pack, share, weight_pull/seed, tfidf)
├── section_18_mesh_toolshare/          (1 file — tool_manifest)
├── section_19_mesh_karma/              (3 files — karma, decay, trust)
├── section_20_mesh_filter/             (2 files — filter 9-layer, license)
├── section_21_mesh_lora/               (6 files — bloom, budget, compress, delta, frame, priority)
├── section_22_mesh_l3_sync/            (5 files — l3_sync, sync, discovery_v2, fallback, relay_v2)
└── section_23_mesh_daemon/             (6 files — mesh_main + 5 DB files)
```

**Total file referensi Router: 77 files (~764K).**

---

## Sinkronisasi dengan Flowork_Agent (UPDATED)

| Router section | Agent section | Contract endpoint |
|---|---|---|
| 7 — Mistakes journal global | Agent 2 (Mistakes local) + 7 (Sync) | `POST /api/mistakes/submit` |
| 8 — Skill catalog | Agent 7 (Sync pull) | `GET /api/skills/list`, `GET /api/skills/get?id=` |
| 1 — Ingestion submit | (kontrib via interaction logging) | `POST /api/brain/contributions/ingest` |
| 11 — Model pool | (Agent baca via config.router.model) | `GET /api/models/list` |
| **13 — Mesh foundation** | **Agent 20 (Mesh client)** | `GET /api/mesh/peers`, `GET /api/mesh/identity` |
| **17 — Knowledge share** | Agent (via Bagian 1 section 7 Sync) | `POST /api/mesh/knowledge/request` |
| **18 — Tool manifest broadcast** | Agent 20 (BroadcastTool) | `POST /api/mesh/broadcast-tool`, `GET /api/mesh/find-tool` |
| **23 — Mesh daemon** | Agent 20 (umbrella) | `/api/mesh/*` (semua endpoint) |

---

*Updated: 2026-05-29 — Bagian 2 (section 13-23) ditambahkan untuk mesh.*

---

# === BAGIAN 3 — LLM PROVIDER LAYER (extend basic yang ada) ===

> **Status awal:** Router sudah punya **basic** Ollama passthrough (`internal/executors/ollama_local.go`), basic Pricing rate card (`internal/store/pricing.go`), basic Provider (embedding/image/stt/tts di `internal/providers/` + `providercompat`). **BELUM** ada: fallback chain, brain-as-provider, kernel proxy mode, local llama, local transformer, full LocalAI stack, Policy + Policy budget.
>
> **Sumber:** `Pictures/stable_open_router/flowork_project/floworkos-go/internal/{provider,localai,pricing,policy,policybudget}/`. Production-grade — sudah ke-copy ke `referensifile/section_24-27/`.
>
> **Strategi (HINDARI HALU + BUG):** semua file logic udah di referensifile/. Implementasi = `cp referensifile/section_XX/*.go internal/<topic>/` lalu **sesuaikan minimal** (import path + adapter). **Jangan code dari scratch** — copy-adapt biar test-coverage dari source asli kebawa.

---

## Section 24 — LLM Provider abstraction (full stack)

**Goal:** Router sebagai **multi-provider gateway** — agent kirim request, router pilih provider terbaik (cost, latency, availability), chain fallback kalau primary gagal. Saat ini cuma punya OpenAI-compat passthrough — extend ke full abstraction.

**Komponen (copy-adapt dari referensi):**

- **`internal/provider/provider.go`** — interface utama (`Chat`, `Stream`, `Embed`)
- **`internal/provider/brain_provider.go`** — brain sebagai provider! (kalau cache hit → return tanpa hit upstream)
- **`internal/provider/bridge.go`** — bridge layer (handle OpenAI compat ↔ internal)
- **`internal/provider/call_log.go`** — log per call (cost, tokens, latency, model)
- **`internal/provider/chain_mode.go`** — chain mode: primary → fallback1 → fallback2 → error
- **`internal/provider/fallback.go`** — fallback logic (rate limit, payment required, model unavailable)
- **`internal/provider/http.go`** — shared HTTP utilities
- **`internal/provider/kernel_proxy.go`** — kernel proxy mode (route to kernel's brain)
- **`internal/provider/local_llama.go`** — local llama.cpp integration
- ~~**`internal/provider/local_transformer.go`**~~ — ❌ **CUT 2026-05-29**: reinvent wheel, Ollama + llama.cpp (section 25) sudah cover. Pure Go transformer = over-engineering buat single-user.
- **`internal/provider/ollama.go`** — extended Ollama (beyond current basic passthrough)
- **`internal/provider/openai.go`** — extended OpenAI (full streaming + error class)
- **`internal/provider/openai_stream.go`** — OpenAI SSE streaming

**Tabel baru di router DB:**

```sql
CREATE TABLE provider_chain_configs (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  chain_name    TEXT NOT NULL UNIQUE,       -- 'default' | 'budget' | 'high-quality'
  providers_json TEXT NOT NULL,             -- ordered array of provider IDs
  created_at    TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE provider_call_log (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  caller       TEXT NOT NULL DEFAULT '',
  provider     TEXT NOT NULL,
  model        TEXT NOT NULL,
  input_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  cost_usd     REAL NOT NULL DEFAULT 0,
  latency_ms   INTEGER NOT NULL DEFAULT 0,
  status       TEXT NOT NULL,               -- 'success' | 'fallback' | 'error'
  occurred_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_provider_call_log_time ON provider_call_log(occurred_at DESC);
```

**Endpoint baru / extend:**
- `POST /v1/chat/completions` — extend existing dengan chain logic
- `GET /api/provider/chains` — list chain configs
- `POST /api/provider/chains` — set chain config
- `GET /api/provider/calls?from=&to=` — call log

**Referensi file:** [`section_24_llm_provider/`](referensifile/section_24_llm_provider/) — **13 file ready to copy-adapt**

**Adaptasi (minimal):**
1. Replace import `github.com/teetah2402/...` → `github.com/flowork-os/flowork_Router/internal/provider`
2. Adapter existing executors/translators ke interface baru (jangan tinggalin yg lama broken)
3. Wire ke `internal/store` (existing schema lifecycle)

**Acceptance criteria:**
- Send chat request → router pilih provider chain → primary call → log to `provider_call_log`.
- Primary 429 (rate limit) → auto fallback → log status='fallback'.
- Chain config switch via API → next call pakai chain baru.
- Backward compat: existing handler `/v1/chat/completions` ngga break.

---

## Section 25 — LocalAI runtime (llama.cpp + loader + registry + manifest)

**Goal:** router host model lokal sendiri — fallback total kalau cloud provider down atau owner mau privacy mode. Pakai llama.cpp (C++ binary) sebagai backend. Plus registry + manifest signing biar trustable.

**Komponen (copy-adapt dari referensi):**

- **`internal/localai/runtime.go`** — runtime orchestrator (start/stop llama-server, health check)
- **`internal/localai/loader.go`** — model loader (download dari Hugging Face / mesh, verify checksum)
- **`internal/localai/llamacpp.go`** — llama.cpp binary wrapper
- **`internal/localai/registry.go`** — model registry (gguf models, metadata, signing)
- **`internal/localai/manifest_sign.go`** — manifest signature verification
- **`internal/localai/manifest.json`** — example manifest format
- **`internal/localai/phaseb_stubs.go`** — Phase B stubs
- **`internal/localai/doc.go`** — package doc

**Tabel baru:**

```sql
CREATE TABLE local_models (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  model_id      TEXT NOT NULL UNIQUE,       -- 'qwen2.5-7b-instruct-q4'
  file_path     TEXT NOT NULL,
  size_bytes    INTEGER NOT NULL DEFAULT 0,
  checksum      TEXT NOT NULL,
  manifest_sig  TEXT NOT NULL DEFAULT '',
  loaded        INTEGER NOT NULL DEFAULT 0, -- 1 = llama-server jalan
  added_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Endpoint baru:**
- `GET /api/localai/models` — list local models
- `POST /api/localai/load?model_id=` — spawn llama-server
- `POST /api/localai/unload?model_id=` — stop llama-server
- `POST /api/localai/download?model_url=&manifest_url=` — download + verify

**Integrasi:**
- Provider abstraction (section 24) tambah `local_llama` provider yang call ke localai runtime
- Mesh (section 23) optional: share gguf model via weight_pull

**Referensi file:** [`section_25_localai_runtime/`](referensifile/section_25_localai_runtime/) — **8 file ready to copy-adapt**

**Adaptasi (minimal):**
1. Replace import path
2. Wire ke provider section 24 (registrar local_llama)
3. Adjust llama.cpp binary path (cross-OS: `/usr/local/bin/llama-server` Linux, `llama-server.exe` Windows)

**Acceptance criteria:**
- Download qwen2.5-7b model → verify checksum → register.
- Load → llama-server jalan di port 11434 (atau dynamic) → health check OK.
- Chat via provider chain pilih local_llama → output coherent.
- Manifest signature: ngga valid → reject load.

---

## Section 26 — Pricing engine (extend existing)

**Goal:** Router sudah punya basic `pricing` di `internal/store/pricing.go` (rate card). Extend dengan: real-time usage cost calc, response header injection (`X-Router-Cost-Usd` untuk Agent), provider-specific pricing override.

**Komponen (extend yang ada):**

- **`internal/pricing/calculator.go`** — Calculate cost dari usage (input_tokens × $/1M + output_tokens × $/1M)
- **`internal/pricing/header.go`** — Inject `X-Router-Cost-Usd` ke response chat (Agent baca buat finance ledger)
- **`internal/pricing/override.go`** — Per-warga atau per-context pricing override (mis. trial user free)

**Tabel baru (extend existing pricing schema):**

```sql
-- Extend existing pricing schema:
ALTER TABLE pricing ADD COLUMN trial_pct REAL DEFAULT 0;          -- discount untuk trial user
ALTER TABLE pricing ADD COLUMN custom_user_pct REAL DEFAULT 0;    -- custom per warga

CREATE TABLE pricing_usage_agg (
  agent_id    TEXT NOT NULL,
  date_bucket TEXT NOT NULL,                 -- YYYY-MM-DD
  provider    TEXT NOT NULL,
  total_cost  REAL NOT NULL DEFAULT 0,
  call_count  INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (agent_id, date_bucket, provider)
);
```

**Endpoint baru / extend:**
- `POST /v1/chat/completions` (extend dari section 24) — inject `X-Router-Cost-Usd` header response
- `GET /api/pricing/usage?agent_id=&from=&to=` — usage aggregate per warga (sumber buat Agent finance ledger)

**Referensi file:** [`section_26_pricing_engine/pricing.go`](referensifile/section_26_pricing_engine/pricing.go) — **1 file ready to copy-adapt**

**Adaptasi (minimal):**
1. Extend existing `internal/store/pricing.go` dengan calculator pattern dari referensi
2. Wire ke `provider/call_log.go` (section 24) — auto-aggregate ke `pricing_usage_agg`
3. Header inject di response middleware

**Acceptance criteria:**
- Chat call → cost calc benar (token × rate card) → header response include.
- Aggregate per-agent per-day correct.
- Override: trial user → 0 cost di response.

---

## Section 27 — Policy + Policy budget (spending guardrails per-warga)

**Goal:** router cap spending per warga. Agent A boleh pakai $X/hari, agent B pakai $Y. Router enforce sebelum dispatch upstream call. Pair sama Agent section 23 (Finance ledger) — Router enforce, Agent log.

**Komponen (copy-adapt dari referensi):**

- **`internal/policy/policy.go`** — Policy struct (per-agent budget limits, model whitelist/blacklist, time-based rules)
- **`internal/policybudget/policybudget.go`** — Budget tracker: aggregate spending → block kalau exceed

**Tabel baru:**

```sql
CREATE TABLE agent_policies (
  agent_id          TEXT PRIMARY KEY,
  daily_budget_usd  REAL DEFAULT 5.0,         -- default $5/day
  monthly_budget_usd REAL DEFAULT 100.0,
  allowed_models    TEXT NOT NULL DEFAULT '[]',  -- JSON array, empty = all
  blocked_models    TEXT NOT NULL DEFAULT '[]',
  rate_limit_rpm    INTEGER DEFAULT 60,        -- requests per minute
  enabled           INTEGER NOT NULL DEFAULT 1,
  updated_at        TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE policy_violations (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  agent_id     TEXT NOT NULL,
  violation    TEXT NOT NULL,              -- 'budget_exceeded' | 'model_blocked' | 'rate_limit'
  detail       TEXT NOT NULL,
  occurred_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Implementasi:**
- Middleware di `/v1/chat/completions` (section 24): cek `policybudget.IsAllowed(agent_id, estimated_cost)` SEBELUM dispatch.
- Over budget → return 429 + log to `policy_violations`.
- Agent dapet error → log decision (Agent section 3).

**Endpoint baru:**
- `GET /api/policy/agents/<id>` — get policy
- `POST /api/policy/agents/<id>` — set policy
- `GET /api/policy/violations?agent_id=&from=` — list violations
- `GET /api/policy/status?agent_id=` — current spend vs budget

**Referensi file:** [`section_27_policy_budget/`](referensifile/section_27_policy_budget/) — **2 file ready to copy-adapt**:
- `policy.go` · `policybudget.go`

**Adaptasi (minimal):**
1. Replace import path
2. Wire middleware ke router handler
3. Sync sama Agent finance ledger (section 23) — Router authoritative, Agent local mirror

**Acceptance criteria:**
- Set policy "agent X daily_budget=$1" → chat 3x avg $0.50 → call ke-3 blocked dengan 429.
- Policy violation logged + Agent receive error → Agent log decision.
- Reset midnight → next day spending fresh.

---

## Roadmap urutan + dependensi (Bagian 3 — LLM Provider Layer)

| # | Section | Priority | Dependensi |
|---|---|---|---|
| 1 | Section 24 — LLM Provider abstraction | 🔴 P0 | Independen. Extend existing `internal/providers`. |
| 2 | Section 26 — Pricing engine extend | 🔴 P0 | Section 24 done (call_log sumber data). Pair Agent section 23. |
| 3 | Section 27 — Policy + budget | 🔴 P0 | Section 24 + 26 done. Pair Agent section 23. |
| 4 | Section 25 — LocalAI runtime | 🟡 P1 | Section 24 done. **Sovereignty path** — router bisa jalan tanpa cloud. |

**Catatan kerja:**
- **Backward compat penting**: existing `/v1/chat/completions` handler ngga boleh break selama refactor.
- LocalAI butuh llama.cpp binary di-install di host. Provide install script `scripts/install-llama-cpp.sh`.
- Policy enforcement = Router authoritative. Agent finance ledger = local mirror buat dashboard.

---

## Folder referensi (UPDATED — section 24-27)

```
referensifile/
├── ... (section 01-23)
├── section_24_llm_provider/             (13 files — provider, brain_provider, bridge, fallback, local_llama, ollama, openai, streaming)
├── section_25_localai_runtime/          (8 files — runtime, loader, llamacpp, registry, manifest_sign + manifest.json example)
├── section_26_pricing_engine/           (1 file — pricing.go calculator)
└── section_27_policy_budget/            (2 files — policy.go + policybudget.go)
```

**Total file referensi Router: 101 files (~984K).**

---

## Sinkronisasi UPDATED dengan Flowork_Agent

| Router section | Agent section | Contract |
|---|---|---|
| 24 — Provider | (Agent ngga call provider langsung) | `/v1/chat/completions` (existing) — extend dengan chain logic |
| 25 — LocalAI | (Agent transparent — pakai via provider chain) | (internal router only) |
| 26 — Pricing | Agent 23 (Finance ledger) | Response header `X-Router-Cost-Usd` |
| 27 — Policy budget | Agent 23 (Finance budget) | Router enforce, Agent local mirror via `/api/policy/status?agent_id=` |

---

*Updated: 2026-05-29 — Bagian 3 (section 24-27) ditambahkan untuk LLM Provider Layer + Pricing + Policy. Total Router roadmap: 27 section di 3 bagian.*
