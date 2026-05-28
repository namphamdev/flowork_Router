---
name: simplify
description: Refactor code untuk readability - extract function, naming, drop dead code
version: 1.0.0
tags: [refactor, readability, code-quality]
triggers: [simplify, refactor, clean up, sederhanain]
---

# Simplify — Refactor for Readability

## When to Use
- Function >100 baris (split candidate)
- Naming inconsistent / cryptic
- Dead code residual (commented out, unused import)
- Nested deep (>3 level indent)

## Procedure
1. **Read full context**: file utuh + dependent files (codemap)
2. **Identify smell**:
   - Long function → extract method
   - Inconsistent naming → unify convention
   - Magic number → const
   - Repeated logic → helper function
3. **Apply minimal change**: one smell at a time
4. **Verify build + test pass post each change**
5. **Commit incremental** (atomic + revertable)

## Pitfalls
- Big-bang refactor (risk regression tinggi)
- Premature abstraction (3 similar lines OK, jangan force pattern)
- Touch shared util tanpa codemap check (blast radius)

## Verification
- Each change atomic? ✅/❌
- Build clean post each? ✅/❌
- Test pass post each? ✅/❌
