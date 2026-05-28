---
name: pilar_audit
description: Audit 6 pilar Flowork compliance untuk fitur/refactor/code baru sebelum merge.
metadata:
  domain: mindset
  created_at: 2026-05-19
  updated_at: 2026-05-19
---

# Pilar Audit — 6 Pilar Compliance Check

## When to Use
- Sebelum merge fitur baru / refactor besar / brain entry baru
- Saat self-audit kerjaan sendiri
- Saat review proposal dari agent lain

## Source of Truth
- `flowork_midset.md` (root project, kitab 4 pilar)
- Memory: [[flowork-4-pillars-master]]

## Checklist (Cumulative — semua harus YES)

### Pilar 1 — Pattern vs Knowledge Separation
- [ ] Knowledge fakta = di brain DB (BUKAN file .md tersebar / training corpus / system prompt)
- [ ] Pola reasoning/audit/tool use = di LLM weight (BUKAN prompt verbose)
- [ ] Tools registration via fine-tune (BUKAN list di prompt)

### Pilar 2 — Nano Modular + Plug-and-Play
- [ ] 1 file/entry/tool = 1 tanggung jawab atomic (bukan bundled monolith)
- [ ] Plug-and-play via convention (drop file di folder = aktif)
- [ ] "Kalau delete komponen ini, sistem tetep jalan?" → YES (graceful degradation)
- [ ] Konsisten sama pattern sejenis existing (e.g., kalau ada `PLAYBOOK_X_*` per-niche, jangan `PLAYBOOK_Y_*` monolith)

### Pilar 3 — QC Three-Layer
- [ ] L1 Pre-check artifact verify done (file ada, schema valid, kernel reachable)
- [ ] L2 Automated dispatch ≥80% PASS+VALID (kalau LLM behavior involved)
- [ ] L3 Mr.Dev manual chat judge done (kalau claim WORK/IMPLEMENTED)
- [ ] Default status fitur baru = STAGED, BUKAN IMPLEMENTED (sampai L3 lulus)

### Pilar 4 — Quantum Principle (FQP-1..12)
- [ ] FQP-1: state transition via verify gate sebelum commit
- [ ] FQP-2: komunikasi via channel (bridge/ledger) BUKAN shared mutable
- [ ] FQP-7/12: append-only (INSERT new + tombstone old, BUKAN UPDATE mutate)
- [ ] FQP-9: tier amp untuk decay-able vs sacred-locked
- [ ] FQP-10: cross_refs ke entry related

### Pilar 5 — Continual Training Compound (kalau training cycle)
- [ ] Start dari trained model existing (v_N), BUKAN base mentah?
- [ ] Delta corpus (5-10K row) BUKAN full re-train?
- [ ] Conservative LR + LoRA rank rendah?
- [ ] Validate sacred reflex 100% post-train?
- [ ] Merge LoRA jadi flat base sebelum next continual?
- Exception fresh: major pivot (base upgrade / anti-stale violation / paradigm shift) — JUSTIFY eksplisit

### Pilar 6 — Multi-OS + Portable (Zero Hardcode)
- [ ] Path resolution via env var (`FLOWORK_ROOT`) atau relative dari script?
- [ ] Pakai `pathlib.Path` (Python) / `filepath.Join` (Go)?
- [ ] Test: clone project ke folder lain, jalan tanpa edit?
- [ ] No hardcoded drive letter (`c:\\`, `d:\\`) di committed code?

## Decision Rule
- Semua `[ ]` checked = READY MERGE
- Ada 1+ un-checked = REFACTOR / IDOKUMENTASI compromise eksplisit dulu

## Anti-Pattern
- ❌ Skip pilar check karena "udah kelihatan jalan"
- ❌ Mark IMPLEMENTED tanpa L3 chat Mr.Dev
- ❌ UPDATE existing sacred row tanpa tombstone
- ❌ Bundled monolith karena "lebih simple"

## References
- [[flowork-4-pillars-master]] (memory)
- `flowork_midset.md` (project root)
