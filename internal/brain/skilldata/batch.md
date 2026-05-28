---
name: batch
description: Parallel operations - execute N independent tasks concurrent untuk speed boost
version: 1.0.0
tags: [parallel, speed, concurrency]
triggers: [batch, parallel, concurrent, paralel, bareng]
---

# Batch — Parallel Operations

## When to Use
- 3+ independent task yang ngga butuh hasil satu sama lain
- Recon multi-source (grep + glob + read paralel)
- Multi-target scan / check
- Independent file processing

## Procedure
1. Identify task set yang truly independent (no data dependency)
2. Spawn subagent paralel via AgentTool (run_in_background=true)
3. Collect results saat semua done
4. Aggregate + report

## Pitfalls
- Tasks dengan dependency = SEQUENTIAL, not batch
- Race condition kalau touch shared file (pakai worktree isolation)
- Resource contention (memory/CPU) — cap N parallel max 4-8

## Verification
- All tasks independent? ✅/❌
- Worktree isolation kalau write? ✅/❌
- Resource cap set? ✅/❌
