---
name: debug
description: Debug error/bug - systematic isolation traceback root cause
version: 1.0.0
tags: [debug, error, troubleshoot]
triggers: [debug, error, bug, troubleshoot, crash, fail]
---

# Debug — Systematic Error Investigation

## When to Use
- Error muncul + stack trace ngga obvious
- Test fail tanpa clear reason
- Behavior unexpected post-change
- Intermittent bug (kadang muncul kadang ngga)

## Procedure
1. **Capture state**: stack trace lengkap, env, input data, command yang trigger
2. **Reproduce minimal**: extract case kecil yang masih trigger bug
3. **Bisect**: kalau bug appeared post commit X, git bisect commit range
4. **Hypothesis**: list 3 kemungkinan root cause
5. **Verify per hypothesis**: print/log/test isolated path
6. **Fix + verify**: apply fix, confirm reproduce case ngga trigger lagi
7. **Test broader**: run full test suite anti-regression

## Pitfalls
- Skip reproduce step → debug by guessing
- Fix symptom bukan root cause
- Add logging tanpa cleanup post-fix

## Verification
- Minimal reproduce case captured? ✅/❌
- Root cause identified (bukan symptom)? ✅/❌
- Fix tested untuk regression? ✅/❌
