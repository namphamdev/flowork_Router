---
name: nano_split_check
description: Pilar 2 enforcement — split bundled monolith jadi atomic entries / files / tools.
metadata:
  domain: mindset
  created_at: 2026-05-19
---

# Nano Split Check (Pilar 2 Enforcement)

## When to Use
- Sebelum INSERT brain entry baru (cek 1 hal atau N hal?)
- Sebelum bikin file `.md` / tool / skill baru
- Saat audit existing artifact untuk refactor potensial

## Diagnostic Question
> "1 file/entry/tool ini tanggung jawab berapa hal?"
- 1 = atomic OK
- 2+ = split jadi N items

## Steps

### Step 1 — Identify Boundaries
- Apa unit "1 hal" dalam konteks ini? (1 niche, 1 doktrin, 1 capability, 1 workflow)
- Cek konvensi existing: kalau ada `X_NICHE_A` + `X_NICHE_B` + `X_NICHE_C`, jangan bikin `Y_ALL_NICHE` monolith

### Step 2 — Split Decision
Sebelum bundle, tanya:
- User akan query/pick subset? → SPLIT (per-niche/per-aspect)
- User selalu butuh semua bersama? → bundled OK (e.g., 1 doktrin yang inherent atomic)
- Komponen butuh update terpisah? → SPLIT

### Step 3 — FQP-12 Compliant Split (kalau ada existing monolith)
1. Parse monolith content per sub-unit
2. INSERT N atomic entries baru (append-only)
3. Tombstone old monolith: `UPDATE deleted_at = NOW, deleted_by = '<reason>'`
4. JANGAN hard DELETE — history harus terjaga

### Step 4 — Master Index (optional)
- Tambah 1 master index entry dengan list pointer ke atomic entries
- Amp lebih tinggi sedikit (e.g., 99500 vs 99000) biar query master surface dulu
- Cross_refs typed ke entries detail

## Anti-Pattern Bundled Monolith
- ❌ 1 brain entry isi 50 prompt / 200 nama (akses partial susah)
- ❌ 1 tool isi 10 capability (susah audit, susah swap)
- ❌ 1 file `.md` isi 5 workflow beda (susah discovery agent)
- ❌ 1 daemon isi tanggung jawab kernel + GUI + audit + scheduler

## Pattern Bener
- ✅ 1 niche per entry (e.g., `PLAYBOOK_YT_NAMING_SLEEP_MUSIC`)
- ✅ 1 atomic capability per tool (e.g., `brain_search`, BUKAN `brain_everything`)
- ✅ 1 workflow per skill `.md`
- ✅ 1 responsibility per daemon

## Test Compliance
- [ ] "Kalau gw delete sub-unit ini, sistem tetep jalan?" → YES
- [ ] "Kalau gw tambah sub-unit baru, edit berapa file existing?" → 0
- [ ] "Naming konsisten sama sibling existing?" → YES

## References
- [[nano-modular-plug-and-play]] (memory)
- [[flowork-4-pillars-master]] Pilar 2
- Case study: `split_suno_playbook_brain.py` (Suno monolith → 9 niche)
