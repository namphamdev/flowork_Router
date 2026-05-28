---
name: remember
description: Save/recall persistent memory via brain DB memorize_brain tool
version: 1.0.0
tags: [memory, persist, recall]
triggers: [remember, ingat, save memory, recall, memorize]
---

# Remember — Persistent Memory Ops

## When to Use
- User explicit "ingat ini: ..."
- Cross-session fact yang useful (user pref, project context)
- Lesson learned dari mistake yang berguna future
- Decision rationale (kenapa kita pilih X bukan Y)

## Procedure
1. **Classify type**: user / feedback / project / reference
2. **Format compact**: 1-3 sentence, lead with fact, then "Why:" + "How to apply:"
3. **Save via memorize_brain tool** dengan amplitude tepat:
   - user-type sticky: amplitude 999999 (mem_type='user')
   - feedback: amplitude 8000-9000
   - project: amplitude 5000-7000
   - reference: amplitude 1000-3000
4. **Verify save**: query back via brain_search

## Pitfalls
- Save redundant memory (cek dulu via brain_search)
- Save ephemeral state (task in-progress, not memory worthy)
- Save user data tanpa explicit "ingat ini"

## Verification
- Type correctly classified? ✅/❌
- Format compact + Why/How? ✅/❌
- Saved + queryable? ✅/❌
