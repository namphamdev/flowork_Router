---
name: anti_flag_accumulation
description: Setiap feature flag wajib punya exit criteria — promote stable atau drop. Anti pola Claude Code accumulate flag selamanya.
---

# Anti Flag Accumulation

**Asal**: Audit Claude Code 2026-05-19 — Anthropic accumulate `experimental_*` flag tanpa exit criteria, jadi tech debt forever.

## Aturan Pokok

Tiap kali nambahin feature flag (env var, config flag, settings DB toggle), WAJIB:

1. **Exit criteria eksplisit** — kapan flag ini di-promote ke stable (default-on, hapus flag)? Kapan di-drop (rollback)?
2. **Deadline review** — max 30 hari pasca-add, audit:
   - PASS rate >80% di test/canary → promote stable
   - PASS rate <50% → drop + revert
   - 50-80% → extend 30 hari, max 2x extension
3. **Tracker entry** — log di `bundled/flags_inventory.md`:
   ```
   - <FLAG_NAME> (added <YYYY-MM-DD>, owner <Awenk/Mr.Flow>, exit by <YYYY-MM-DD>, criteria <text>)
   ```

## Detect Pattern Pelanggaran

Trigger audit ketika:
- Flag umur >60 hari tanpa decision
- Flag dipakai cuma di 1-2 callsite (kandidat hardcode constant)
- Flag default `false` tapi semua test pakai `true` (cargo cult)

## Action Otomatis

Tiap commit yang nambah `os.Getenv("FLOWORK_*")` atau `setting_get("flowork.*.enabled")`:
- Pre-commit hook check `flags_inventory.md` ada entry-nya
- Kalo ngga ada → block commit + minta exit criteria

## Anti-Pattern

JANGAN:
- Add flag "buat jaga-jaga" tanpa exit plan
- Tinggalin flag default false yang dipake testing tapi prod ngga tau
- Pakai flag sebagai pengganti dependency injection

## Lihat Juga

[[audit_tool_sprawl]] — anti pola tool accumulate
[[audit_file_length]] — anti pola single-file >1000 baris
