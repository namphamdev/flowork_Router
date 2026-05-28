---
name: audit_file_length
description: Max 600 baris per file. Anti pola Claude Code QueryEngine.ts (1297 baris) + AgentTool.tsx (1398 baris) — susah maintain + susah review.
---

# Audit File Length

**Asal**: Audit Claude Code 2026-05-19 — Claude punya file 1000+ baris (QueryEngine.ts 1297, AgentTool.tsx 1398). Susah review, susah test, susah split saat butuh.

## Aturan Pokok

Max **600 baris per file**. Soft limit 500, hard limit 600.

Force split via Pilar 2 (Nano Modular). Setiap file = 1 tanggung jawab atomic.

## Threshold Action

| Baris | Status | Action |
|---|---|---|
| 0-400 | ✅ Sehat | Lanjut |
| 401-500 | ⚠️ Warning | Review next session |
| 501-600 | 🟠 Soft cap | Plan split di session ini juga |
| 601+ | ❌ Hard cap | BLOCK commit, harus split dulu |

## Detect Pattern Pelanggaran

Trigger split kapan:
- File >600 baris ✅
- File punya 2+ struct/class besar (tanggung jawab campur)
- File punya helper function private >10 (kandidat extract ke util)
- File `import` >20 dari domain berbeda (kohesi rendah)

## Strategi Split

1. **By domain**: extract function group ke `_<domain>.go` (e.g., `handler_chat.go`, `handler_auth.go`)
2. **By layer**: extract validation → `_validate.go`, transformation → `_transform.go`
3. **By struct**: tiap struct besar → file sendiri (`<struct_name>.go`)
4. **By test**: jangan campur test + production di file sama (`_test.go` suffix mandatory di Go)

## Action Otomatis

Pre-commit hook `bundled/scripts/pre_commit_file_length.sh`:

```bash
#!/bin/bash
# Block commit kalo ada file staged > 600 baris
for f in $(git diff --cached --name-only --diff-filter=AM); do
    if [[ "$f" =~ \.(go|py|ts|tsx|js|jsx|rs)$ ]]; then
        lines=$(wc -l < "$f")
        if [ "$lines" -gt 600 ]; then
            echo "BLOCK: $f = $lines baris (> 600 hard cap). Split dulu."
            exit 1
        fi
    fi
done
```

## Pengecualian Sah

- Generated code (proto, openapi schema) — tanda `// CODE GENERATED, DO NOT EDIT` di header
- Test data fixtures (`_test_fixtures.go`) — tapi prefer JSON/YAML eksternal
- Migration SQL file — bisa long karena chronological history

## Anti-Pattern

JANGAN:
- Tahan file 599 baris terus tambah comment supaya keliatan <600
- Inline helper besar di file utama "biar 1 tempat"
- Bikin `mega_handler.go` yang handle semua endpoint

## Lihat Juga

[[anti_flag_accumulation]] — anti pola flag accumulate
[[audit_tool_sprawl]] — anti pola tool sprawl
[[Pilar 2 — Nano Modular](feedback_nano_modular.md)] — Pilar dasar
