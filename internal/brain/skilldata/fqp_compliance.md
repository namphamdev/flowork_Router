---
name: fqp_compliance
description: Pilar 4 enforcement — FQP-1..12 quantum principles check sebelum SQL/state change.
metadata:
  domain: mindset
  created_at: 2026-05-19
---

# FQP Compliance Check (Pilar 4)

## When to Use
- Sebelum tulis SQL yang touch brain DB / ledger / bridge
- Sebelum design state transition (workflow / daemon)
- Saat refactor existing yang sentuh sacred state

## Cheat Sheet 12 Prinsip

| ID | Nama | Quick Test |
|---|---|---|
| FQP-1 Verify Gate | State transition wajib verify SEBELUM commit |
| FQP-2 Bridge | Komunikasi via channel append-only (bridge.json, ledger) |
| FQP-3 BFT | High-risk wajib BFT vote (untuk single-tenant = self-attest) |
| FQP-4 Sticky Constitution | Sacred amp 999999 IMMUTABLE, sticky tiap session |
| FQP-5 Capability Gate | Komponen cuma akses tool dalam caps registry |
| FQP-6 Karma Ledger | History append-only, observable, audit-able |
| FQP-7 Brain Append | Brain entry append-only, ngga duplikat-modify |
| FQP-8 Eigenstate Sacred | Sacred tier locked, ngga decay, ngga update |
| FQP-9 Decoherence Decay | Non-sacred decay via half-life |
| FQP-10 Entanglement Cross-Ref | Cross_refs typed (strong/weak) |
| FQP-11 Superposition Pending | Pending review = state superposition, collapse pas owner verify |
| FQP-12 Append-Only Sacred | NEVER UPDATE/DELETE bridge/ledger/brain — append + tombstone |

## Steps

### Step 1 — Identify Operation Type
- `INSERT` → likely OK (append-only compliant)
- `UPDATE` → DANGER, check FQP-12
- `DELETE` (hard) → STOP, refactor jadi tombstone via `deleted_at`
- `DROP TABLE` → STOP, NEVER

### Step 2 — FQP-12 Check Untuk Mutate
Kalau benar-benar harus UPDATE existing row:
1. Apakah row sacred amp ≥999998? → STOP, mutate sacred VIOLATION
2. Apakah ada alternative append (INSERT delta + tombstone old)? → Refactor
3. Kalau tetep harus mutate (e.g., schema kolom additive, runtime counter) → DOKUMENTASI compromise eksplisit di docstring + mitigation (trace via context_origin field)

### Step 3 — FQP-1 Verify Gate
Sebelum commit state change:
- [ ] User intent confirmed (kalau destructive)?
- [ ] Capability check (komponen punya hak?)
- [ ] Side-effect reversible (atau confirmed irreversible OK)?

### Step 4 — FQP-2 Channel
- Cross-component komunikasi via append-only channel (`comms/bridge.json`, brain DB)
- BUKAN shared mutable variable / direct memory access

## Anti-Pattern

- ❌ `UPDATE constitution SET amplitude = X WHERE id = Y` di sacred row → FQP-12 violation
- ❌ `DELETE FROM bridge WHERE timestamp < X` → FQP-12 violation (hard delete history)
- ❌ Component A baca internal state component B via shared memory → FQP-2 violation
- ❌ Skip verify gate karena "trusted internal" → FQP-1 violation

## Pattern Bener

- ✅ `INSERT INTO constitution VALUES (...)` (append-only)
- ✅ `UPDATE constitution SET deleted_at = NOW WHERE id = X` (tombstone, BUKAN hard delete)
- ✅ Komponen A append ke `comms/bridge.json` → Komponen B poll/subscribe
- ✅ Verify gate (capability check + ownership check) sebelum tool dispatch

## Compromise Disclosure Template
Kalau ada FQP violation pragmatic:
```python
"""
FQP-12 COMPROMISE DISCLOSURE
Script ini pake UPDATE untuk <reason>.
Pure mode harusnya: <pure alternative>.
Decision: ACCEPT compromise karena <reason 1>, <reason 2>.
Mitigation: <trace mechanism>.
Risk accept: <consequence kalau bug>.
"""
```

## References
- [[fqp-quantum-principles]] (memory)
- [[flowork-4-pillars-master]] Pilar 4
- Case study: `brain_wave2_apply.py` docstring (UPDATE compromise documented)
