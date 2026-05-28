---
name: skillify
description: META skill - create new skill .md file from workflow yang berhasil
version: 1.0.0
tags: [meta, self-improving, skill-creation]
triggers: [skillify, create skill, save workflow]
---

# Skillify — Meta Skill (Self-Improving)

## When to Use
- Workflow baru berhasil 5+ tool call success
- Pattern yang kemungkinan re-usable future
- User explicit "save ini sebagai skill"
- Lesson learned bisa di-bundle jadi workflow standar

## Procedure
1. **Extract pattern dari recent successful execution**:
   - Tools yang dipakai (list)
   - Trigger context (kapan workflow apply)
   - Step sequence (procedure)
   - Pitfall observed (apa yang harus dihindari)
   - Verification criteria
2. **Generate SKILL.md file** dengan format standar:
   ```
   ---
   name: <kebab-case-name>
   description: <one-line>
   version: 1.0.0
   tags: [a, b]
   triggers: [keyword1, keyword2]
   ---
   # Skill Title
   ## When to Use
   ## Procedure
   ## Pitfalls
   ## Verification
   ```
3. **Save ke ~/.flowork/skills/<name>.md** atau workspaces/<agent>/skills/<name>.md
4. **Loader auto-pickup** next scan (TTL 5min). Force via Refresh()

## Pitfalls
- Skill yang too-specific (one-off, ngga re-usable)
- Skill yang duplicate existing
- Skill tanpa Verification step (can't validate use)

## Verification
- Format SKILL.md valid frontmatter? ✅/❌
- Trigger pattern jelas? ✅/❌
- Re-usable across session? ✅/❌
- Loader picks up after Refresh? ✅/❌
