---
name: audit_pass
description: AUDIT PASS adversarial sebelum submit code/plan/action. Pattern reflexive Mr.Flow.
metadata:
  domain: universal
  created_at: 2026-05-19
  is_catalyst: true
---

# AUDIT PASS Adversarial

## When to Use
- Sebelum submit code yang touch sensitive area
- Sebelum eksekusi action irreversible
- Pasca-implementasi (post-action review)
- User invoke `/audit <target>`

## Trigger Pattern
- Slash `/audit`
- Code touching: auth, payment, DB write, file system, external API, secrets
- Action irreversible: delete, force push, drop table, kill production

## Steps

1. **brain_search** untuk audit checklist domain
   ```
   brain_search "OWASP {language} audit checklist"
   ```

2. **Apply 5-layer check:**
   - **Edge case**: null, empty, very large, malformed input
   - **Security**: injection (SQL/XSS/command/path), auth bypass, race condition
   - **Error handling**: kalau X gagal, gimana?
   - **Sensitive leak**: log/error/response expose data?
   - **Idempotency**: action repeatable safely?

3. **Refine** berdasar audit findings
4. **Submit** + flag finding ke user

## Output Format
```
Audit pass result:
- [LAYER 1] Edge case: [findings]
- [LAYER 2] Security: [findings]
- [LAYER 3] Error: [findings]
- [LAYER 4] Sensitive leak: [findings]
- [LAYER 5] Idempotency: [findings]

Flag to user: [missing items / risk warning]
```

## Decision Rule
- 5/5 layer clean → APPROVE submit
- 4/5 clean → APPROVE dengan flag
- <4/5 clean → REFUSE submit, refine first

## References
- `DOKTRIN_CODING_BUG_AUDIT` (brain)
- `DOKTRIN_HACKING_DEFENSIVE_OFFENSIVE` (brain)
- `mistakes_journal` (avoid recurring error)
