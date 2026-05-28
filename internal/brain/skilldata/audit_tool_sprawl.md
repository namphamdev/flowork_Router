---
name: audit_tool_sprawl
description: Audit periodik tool registry > 50 buat detect overlap + propose merge. Anti pola Claude Code 6 Task tool variant.
---

# Audit Tool Sprawl

**Asal**: Audit Claude Code 2026-05-19 — Claude Code punya 6 Task tool (Create/Get/List/Output/Stop/Update) yang sebenernya bisa di-merge jadi 1 dengan `action` parameter.

## Aturan Pokok

Tiap kali registry tool > 50 entri, run audit:

1. **Group by domain prefix** — kayak `bash_*`, `brain_*`, `donut_*`. Cek dalam group:
   - Tool yang verb mirip (`create/add/new`, `get/read/show`, `delete/remove`) = kandidat merge
   - Tool yang difference cuma 1 parameter optional = kandidat merge
2. **Action parameter consolidation**:
   - 4-6 tool 1 domain → merge ke 1 tool + `action: "create|get|list|...|"` enum
   - Saves slot di tool registry + reduce LLM choice paralysis
3. **Capability matrix**:
   - Bikin `floworkos-go/internal/tools/_inventory.md`
   - Kolom: tool name, domain, action verb, params, last_used
   - Detect: never_used dalam 90 hari → kandidat sunset

## Target Flowork

Current ~137 tools (post-v10 deploy). Target reduction:
- 137 → 100 (Phase 4.5.B + reasonable consolidation)
- 100 → 80 (Phase 7+, kalau workflow proven ngga butuh granular tool)

## Detect Overlap Pattern

Suspicious naming pattern:
- `<domain>_create` + `<domain>_add` + `<domain>_new` → triplet redundant
- `<domain>_get` + `<domain>_read` + `<domain>_show` → triplet redundant
- `<domain>_<verb>_by_id` + `<domain>_<verb>_by_name` → bisa merge dengan locator param

## Action Otomatis

Script `scripts/audit_tool_sprawl.py` (defer impl pasca-v10 stable):
- Parse tool registry
- Group by prefix
- Detect verb collision
- Output `qc/qc-tool-sprawl-<date>.md` dengan proposal merge

## Anti-Pattern

JANGAN:
- Spawn tool baru kalo existing tool cuma butuh param tambahan
- Bikin `*_v2` tool tanpa sunset plan untuk v1
- Naming verb arbitrary (`fetch` vs `get` vs `read` di domain sama)

## Lihat Juga

[[anti_flag_accumulation]] — anti pola flag accumulate
[[audit_file_length]] — anti pola single-file >1000 baris
[[audit_3_kriteria]] — pre-propose audit bentrok/ambigu/cocok 7B
