---
name: append_only_state
description: FQP-12 enforcement — state change via INSERT delta + tombstone, BUKAN in-place UPDATE/DELETE.
metadata:
  domain: mindset
  created_at: 2026-05-19
---

# Append-Only State (FQP-12 Pattern)

## When to Use
- Sebelum SQL touch brain DB / ledger / bridge / karma history
- Sebelum delete file / record
- Saat refactor existing yang sentuh state historical

## Prinsip
> State historical NEVER UPDATE/DELETE in-place. Cuma append delta + tombstone via flag (`deleted_at`, `superseded_by`, `tombstoned_at`).

## Steps

### Step 1 — Operation Decision

| Operation | FQP Status | Action |
|---|---|---|
| `INSERT` | ✅ OK | Proceed |
| `UPDATE` non-sacred (counter, last_accessed) | ⚠️ Pragmatic OK | Compromise documented |
| `UPDATE` sacred field (content, amplitude) | ❌ VIOLATION | Refactor jadi append delta + tombstone |
| `DELETE` hard | ❌ VIOLATION | Refactor jadi tombstone via `deleted_at` |
| `DROP TABLE` | ❌ NEVER | STOP, never do |

### Step 2 — Append Delta Pattern
Untuk "modify" sacred entry:
```sql
-- 1. INSERT new version
INSERT INTO constitution (section, content, amplitude, context_origin)
VALUES ('SECTION_X_v2', 'updated content', 999998, 'refactor_2026-05-19');

-- 2. Tombstone old via deleted_at
UPDATE constitution
SET deleted_at = NOW(), deleted_by = 'superseded_by_v2'
WHERE id = <old_id>;
```

History preserved. Audit trail jelas.

### Step 3 — Tombstone Pattern (Soft Delete)
```sql
UPDATE table
SET deleted_at = NOW(), deleted_by = '<reason>'
WHERE <conditions>;
```

JANGAN:
```sql
DELETE FROM table WHERE id = X;  -- VIOLATION, history hilang
```

### Step 4 — Query Pattern
Tiap query ke brain/ledger:
```sql
WHERE deleted_at IS NULL  -- mandatory filter for active rows
```

History query (audit / debug):
```sql
WHERE section = 'X' ORDER BY created_at DESC  -- include tombstoned
```

## Pragmatic Compromise Exceptions

Some UPDATE OK kalau:
- Schema kolom additive (e.g., `last_accessed_at` runtime counter)
- Bukan content/sacred field
- Audit trail tetep ada via `context_origin` / `updated_at` columns

Dokumentasi compromise di docstring:
```python
"""
FQP-12 COMPROMISE: UPDATE used for <field>.
Reason: <pragmatic justification>.
Mitigation: trace via <field_name>.
"""
```

## Anti-Pattern Common

- ❌ `UPDATE constitution SET content = 'new' WHERE id = X` di sacred row
- ❌ `DELETE FROM mistakes_journal WHERE ...` hard delete
- ❌ `TRUNCATE TABLE bridge` (history wipe)
- ❌ `DROP TABLE history` (catastrophic)

## Pattern Bener

- ✅ `INSERT` new + tombstone old via `deleted_at`
- ✅ Soft delete via `deleted_at` flag
- ✅ Append delta INSERT untuk "version bump"
- ✅ Query filter `WHERE deleted_at IS NULL` mandatory

## Case Study

**FQP-12 violation acknowledged**: `brain_wave2_apply.py` pake `UPDATE constitution SET sacred_lens=1`. Compromise documented di docstring (kolom additive, BUKAN content sacred, trace via context_origin). Pure mode would require `constitution_flags` table append-only.

**FQP-12 compliant**: `split_suno_playbook_brain.py` — split monolith via INSERT 9 baru + tombstone old via `deleted_at`.

## References
- [[fqp-quantum-principles]] (memory)
- [[flowork-4-pillars-master]] Pilar 4
- `DOKTRIN_BRAIN_APPEND_ONLY` (FQP-7) brain
