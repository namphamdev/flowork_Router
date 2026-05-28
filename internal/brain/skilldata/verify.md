---
name: verify
description: Verification workflow - run build/test/lint + report pass/fail dengan evidence
version: 1.0.0
tags: [test, build, lint, verification]
triggers: [verify, test, build check, lint check, qc]
---

# Verify — Verification Workflow

## When to Use

- Before claim "DONE" pada code change
- Post deploy/merge confidence check
- Pre-commit gate
- Bug fix evidence

## Procedure

1. **Identify language/stack**:
   - Go → `go build ./... && go vet ./... && go test ./... -short`
   - Python → `python -m pytest -x` + `ruff check`
   - TypeScript → `bun run build && bun test`
   - Other → cek `package.json` / `Makefile` / `CONTRIBUTING.md`
2. **Run sequentially** (build → vet/lint → test). Stop at first FAIL.
3. **Capture output** (stdout + stderr + exit code)
4. **Parse + report**:
   - ✅ PASS: all 3 stages exit 0, paste short success summary
   - ❌ FAIL: identify stage + first error line + suggest fix

## Pitfalls

- **Skip test** karena "lo kira udah benar" — JANGAN, anti over-claim
- **Run partial** (cuma build, skip test) → report PARSIAL, jangan claim DONE
- **Silent error** (stderr ignored) → capture both, kalau warning tetep lapor

## Verification

- Build clean? ✅/❌
- Lint pass? ✅/❌
- Test pass? ✅/❌
- Output captured + parsed? ✅/❌
