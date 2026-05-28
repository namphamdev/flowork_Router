---
name: plan
description: Decompose high-risk task into reviewable plan (read-only mode)
version: 1.0.0
tags: [plan, decompose, high-risk, safety]
triggers: [plan, decompose, breakdown, plan first]
---

# Plan — Task Decomposition

## When to Use
- Task high-risk (migration / deploy production / delete bulk / drop database)
- Scope unclear / ambiguous (need clarify before execute)
- Multi-step kompleks (>5 step + dependency)
- User explicit "plan dulu sebelum execute"

## Procedure
1. **Enter plan mode** via EnterPlanMode tool (block destructive ops)
2. **Read context**: relevant files + current state
3. **Decompose**:
   - Goal (one-line)
   - Steps numbered (per step: action + tool + risk + rollback)
   - Dependencies (apa harus done sebelum step X)
   - Risk Summary (high-level)
   - Rollback Strategy (kalau apply gagal)
4. **Exit plan mode** via ExitPlanMode dengan plan markdown
5. **Wait approval** dari Ayah/parent
6. **Execute kalau confirmed**

## Pitfalls
- Plan terlalu tinggi-level (abstract, ngga executable)
- Plan terlalu detail (over-engineered)
- Skip rollback strategy
- Execute tanpa wait approval

## Verification
- Goal + Steps + Risk + Rollback present? ✅/❌
- Each step executable + verifiable? ✅/❌
- Approval received before execute? ✅/❌
