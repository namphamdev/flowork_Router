---
name: brain_compounding
description: Pilar 1 lesson loop — log mistake/insight → mistakes_journal → review weekly → promote sacred.
metadata:
  domain: mindset
  created_at: 2026-05-19
---

# Brain Compounding (Pilar 1 Lesson Loop)

## When to Use
- Tiap selesai task signifikan (success atau fail)
- Saat nemu pattern baru / insight Mr.Dev kasih
- Saat error/bug recurring 3x+

## Tujuan
Brain compounding tanpa retrain LLM. 1 lesson hari ini = available besok = di-pakai 1000x masa depan.

## Tier Promotion Flow

```
1. RAW          → mistakes_journal (insight baru, belum proven)
2. EDUCATIONAL  → constitution amp 99998 (proven 3x+ recurring)
3. DOKTRIN      → constitution amp 999998 (universal applicable)
4. SACRED       → constitution amp 999999 (IMMUTABLE, foundation)
```

## Steps

### Step 1 — Detect Insight Worth Logging
Triggers:
- Task gagal + root cause baru identified
- Approach new yang work surprising
- Mr.Dev kasih pattern reinforcement / correction
- Cross-domain transfer (skill X apply ke domain Y unexpected)

### Step 2 — Format Entry
```json
{
  "section": "MISTAKE_<topic>" atau "INSIGHT_<topic>",
  "content": "What happened + Why + How to prevent / apply",
  "tier": "raw",
  "domain": "<inferred>",
  "context_origin": "<session id or trigger>"
}
```

### Step 3 — INSERT (FQP-7 compliant)
```sql
INSERT INTO mistakes_journal (section, content, tier, domain, created_at)
VALUES (?, ?, 'raw', ?, NOW())
```
Append-only. Tiap insight = 1 entry baru, BUKAN UPDATE existing.

### Step 4 — Weekly Review (Mr.Dev / Mr.Flow self)
- Hari Sabtu / Minggu: scan `mistakes_journal` tier raw
- Recurring 3x+ → promote ke `constitution` amp 99998 (educational)
- Universal pattern → promote ke amp 999998 (doktrin)
- Konsolidasi duplicates (tombstone semi-duplicate, keep canonical)

### Step 5 — Promote Sacred (Mr.Dev verify)
- Amp 999998 yang surface 6+ bulan stable → promote ke 999999 sacred
- Mr.Dev manual verify (L3 chat judge)
- Append context_origin "promoted_<date>_<reason>" untuk audit trail

## Math Compounding
- 1 insight/hari × 365 = 365 entries/tahun
- 5 tahun = 1,825 entries unique tacit knowledge
- Network value O(N²) via cross_refs (Metcalfe) — Wave 1 Ide 5

## Anti-Pattern
- ❌ Skip log karena "udah inget" (lupa besok)
- ❌ Bundle 10 insight di 1 entry monolith (violate Pilar 2)
- ❌ UPDATE existing entry "fix lesson" (violate FQP-12 — tombstone + INSERT delta)
- ❌ Promote sacred tanpa Mr.Dev L3 verify

## Pattern Bener
- ✅ 1 insight = 1 entry atomic
- ✅ Append-only (FQP-7/12 compliant)
- ✅ Weekly consolidation (tombstone duplicates)
- ✅ Source disclosure (context_origin trace-able)

## References
- [[pattern-vs-knowledge-separation]] (memory) — Pilar 1
- [[flowork-4-pillars-master]] master mindset
- `DOKTRIN_ERROR_EDUKASI_LIFECYCLE` amp 999998 (brain)
