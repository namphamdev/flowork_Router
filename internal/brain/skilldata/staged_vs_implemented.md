---
name: staged_vs_implemented
description: Pilar 3 disiplin status — Default STAGED, promote IMPLEMENTED hanya setelah L3 Mr.Dev verdict.
metadata:
  domain: mindset
  created_at: 2026-05-19
---

# Staged vs Implemented (Pilar 3 Status Discipline)

## When to Use
- Saat tulis README / changelog mark `✅ IMPLEMENTED` atau `⏳ STAGED`
- Saat report progress ke Mr.Dev
- Saat decide status fitur baru

## Definisi Status

### STAGED (⏳)
- Artifact ada (file, script, brain entry, schema)
- L1 pre-check PASS (sanity OK)
- L2 automated dispatch BELUM atau PARTIAL
- L3 Mr.Dev manual chat judge BELUM
- **Status default fitur baru.**

### IMPLEMENTED (✅)
- L1 + L2 + L3 cumulative PASS
- L3 Mr.Dev verdict POSITIVE verbatim recorded
- Behavior proven via real interaction
- **HANYA setelah cumulative pass.**

## Decision Tree

```
Fitur baru
  ↓
Behavior involves LLM/agent?
  ├─ NO → bisa langsung IMPLEMENTED kalau verifiable tanpa LLM
  │       (e.g., regex parser regression test, disk space delta,
  │        ffmpeg detect, schema migration applied)
  └─ YES → default STAGED sampai L3 lulus
```

## Pattern Bener

✅ Sebelum tulis `✅ IMPLEMENTED`:
- Sebut L3 verdict source ("Mr.Dev chat 2026-05-XX: 'work bro'")
- Atau alasan kenapa L3 ngga butuh (verifiable tanpa LLM)

✅ Tulis `⏳ STAGED — pending L3 post-v10` untuk:
- Wave 1/2 brain schema tags
- Wave 1/2 brain seed playbook
- Workspace + bundled skills (discovery+apply LLM behavior)
- Validation suite (suite itself belum dispatch)

## Anti-Pattern

❌ "8/8 PASS, 100% rate" → claim IMPLEMENTED (kalau itu L1 doang)
❌ "Kerjain semua sekarang" → tag IMPLEMENTED massal padahal banyak L3 pending
❌ Sample bias 1 user OK → assume semua use case

## Self-Audit Checklist (Sebelum Tulis ✅ IMPLEMENTED)

- [ ] L1 artifact ada + valid?
- [ ] L2 dispatch ≥80% PASS+VALID?
- [ ] L3 Mr.Dev verbatim verdict POSITIVE?
- [ ] Recorded di `qc/qc-YYYY-MM-DD-*.md`?

Ada 1 NO → tulis ⏳ STAGED + alasan, BUKAN ✅.

## References
- [[qc-three-layer-standard]] (memory)
- [[flowork-4-pillars-master]] Pilar 3
- Case study: `qc-2026-05-18-v10-foundation.md` framing revisi (claim 8/8 PASS = L1 doang)
