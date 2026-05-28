---
name: brain_query_first
description: Pilar 1 reflex — query brain SEBELUM eksekusi sensitive/specific task.
metadata:
  domain: mindset
  created_at: 2026-05-19
---

# Brain Query First (Pilar 1 Reflex)

## When to Use
- Sebelum eksekusi sensitive task (write/edit/delete/bash/deploy)
- Sebelum jawab pertanyaan yang butuh fakta spesifik (price, version, heir, identity)
- Sebelum klaim "X work" atau "X belum exist"
- Saat input ambiguous → cek brain dulu sebelum tanya user

## Trigger Pattern (reflexive habit)
1. **Sensitive tool**: write_file, edit_file, bash with destructive flags, db_write, sql DELETE/UPDATE
2. **Time-sensitive query**: harga, version terkini, schedule, status realtime
3. **Identity query**: siapa Mr.Dev, siapa heir, password, kill-switch
4. **Domain skill query**: SEO/YT/trading/coding/hacking specific tactical

## Steps

### Step 1 — Decompose Query
- Identifikasi: ini fakta atau pola?
- Fakta → BRAIN query
- Pola → udah di weight, langsung eksekusi

### Step 2 — Trigger brain_search
```
brain_search("<keyword spesifik query>")
```
- Keyword harus specific, BUKAN generic
- Multi-token OK ("channel naming sleep music", "DOKTRIN_HACKING_AUDIT")

### Step 3 — Evaluate Return
- Empty return → next tier (internet_search kalau permission, atau lapor user)
- Match return → integrate ke reasoning + sebut source brain section
- Mismatch (return ada tapi ngga relevant) → reformulate query atau widen

### Step 4 — Source Disclosure
- Sebut brain section + amp tier saat answer
- "Per `DOKTRIN_XXX` amp 999998: ..."
- Transparency = trust

## Decision Rule
- Berubah cepat / fakta spesifik → BRAIN query
- Stabil bertahun / pola universal → langsung dari weight

## Anti-Pattern
- ❌ Jawab fakta dari weight tanpa brain check (halu risk)
- ❌ Skip brain karena "udah tau"
- ❌ Generic query (brain return banyak noise)

## References
- [[pattern-vs-knowledge-separation]] (memory)
- [[flowork-4-pillars-master]] Pilar 1
- `DOKTRIN_SOVEREIGNTY` amp 999999 (brain → GGUF lokal → API last resort)
