---
name: loop
description: Autonomous iteration loop - re-invoke task with self-correction sampai goal tercapai atau cap iter hit
version: 1.0.0
tags: [autonomous, iteration, self-correction]
triggers: [loop, iterate, retry until, keep trying, autonomous loop]
---

# Loop — Autonomous Iteration

## When to Use

- Task ambiguous outcome (success criteria fuzzy, need iterate)
- Multi-step research / debugging (output dari step N inform step N+1)
- Long-running monitoring (cek tiap N menit sampai condition met)
- Self-correction (output salah → revise → retry)

## Procedure

1. **Define goal**: bullet point eksplisit "selesai = X happens"
2. **Iter loop max N** (default 10, override via `--max`)
3. Per iter:
   - Execute task
   - Self-evaluate: goal achieved?
   - YA → exit loop with success
   - TIDAK → identify gap, revise approach, next iter
4. Cap hit before goal → exit + report apa yang udah dilakukan + gap remaining

## Pitfalls

- **Infinite loop**: WAJIB cap iter (max 10), bukan retry forever
- **Same error**: kalau iter N+1 sama dengan iter N (no progress) → break + escalate
- **Token cost**: tiap iter consume context. Compact /summary tiap 3-5 iter
- **Side-effect duplicate**: kalau tool punya side-effect (write file, send msg), check udah berhasil sebelum re-execute

## Verification

- Goal explicit di plan? ✅/❌
- Cap iter set? ✅/❌
- Progress check tiap iter? ✅/❌
- Escalate path saat stuck? ✅/❌
