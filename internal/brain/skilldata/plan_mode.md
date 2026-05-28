---
name: plan_mode
description: Trigger DISCIPLINE_STRUCTURED 4-stage planning sebelum eksekusi task kompleks.
metadata:
  domain: universal
  created_at: 2026-05-19
  is_catalyst: true
---

# Plan Mode

## When to Use
- Task kompleks butuh decomposition
- User invoke `/plan <task>`
- Task multi-step yang impact production
- Task butuh approval Mr.Dev sebelum eksekusi

## Trigger Pattern
- Slash command `/plan`
- Task description >= 100 char dengan keyword: plan, design, strategi, roadmap

## Steps

1. **brain_search** dulu untuk pattern domain task
   ```
   brain_search "<task keyword> + plan + workflow"
   ```

2. **ANALISA**: identify constraint + dependency + edge case
3. **PLAN**: rencana step-by-step sebelum action
4. **EKSEKUSI**: action konkret per step + validate output
5. **REVIEW**: hasil match expectation? Audit pass + log lesson

## Decision Tree
- Task touch sensitive (auth/payment/data) → brain_search mandatory
- Task multi-domain → query brain per domain
- Task novel (no brain pattern) → propose Mr.Dev review first

## Output Format
```
**ANALISA**: [konteks + constraint]
**PLAN**:
  1. Step 1
  2. Step 2
  ...
**EKSEKUSI**: [action]
**REVIEW**: [hasil + lesson]
```

## References
- `DOKTRIN_DISCIPLINE_STRUCTURED` (brain amp 999999)
- `DOKTRIN_RETRIEVAL_TIER` (brain amp 999998)
