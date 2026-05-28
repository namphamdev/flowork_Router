---
name: contekan-extract
description: Extract contekan (summary index) dari chat session panjang. Mr.Flow apply end-of-session untuk save key insight ke brain DB + archive full chat. Anti-lupa mekanisme.
version: 1.0.0
tags: [memori, anti-lupa, quantum-ai, contekan, summary]
triggers: [end-of-session, /save_contekan, /extract_summary, manual trigger Awenk]
---

# Contekan Extract Skill

## When to Use

End-of-session diskusi panjang yang punya insight baru. Manual trigger Awenk atau auto-detect Mr.Flow saat:
- Sesi >20 turn dengan tematik konsisten
- Doktrin baru lahir dari diskusi
- Konteks personal/cerita Awenk yang penting
- Decision strategis di-make

**Inspirasi**: manusia inget masa kecil cuma highlight (Awenk inget "pernah ke Jepang" tanpa detail). Plus pola contekan ujian — list rumus + akses memori sisanya.

Bug AI: telan mentah-mentah seluruh chat ke context window, lupa saat overflow. Contekan = solusi: **summary < 1% volume, lookup detail kalo butuh**.

## Procedure

### Step 1 — Identifikasi Topik Utama

Mr.Flow scan sesi cari:
1. Doktrin baru yang paling banyak di-derive (paling kuat)
2. Atau: pertanyaan utama Awenk
3. Atau: keputusan strategis dibikin

Slug-ify ke UPPERCASE_UNDERSCORE, max 30 char.

Contoh:
- Doktrin baru `DOKTRIN_5W1H_GATE` lahir → topik = `5W1H_GATE`
- Awenk bahas YT pipeline + darkzOGx repo → topik = `YT_PIPELINE_DARKZOGX`
- Sesi ngobrol filosofi anti-elite → topik = `LINUX_PENDENDAM_FOUNDASI`

### Step 2 — Generate Section ID

Format konvensi:

```
MEMORI_SESI_<YYYY-MM-DD>[_T<HH-MM>]_<TOPIK>
```

- `<YYYY-MM-DD>` — tanggal ISO sortable
- `_T<HH-MM>` — opsional, hanya kalo multiple sesi sama hari sama topik
- `<TOPIK>` — slug dari Step 1

Contoh:
```
MEMORI_SESI_2026-05-19_QUANTUM_AI_PARADIGM
MEMORI_SESI_2026-05-19T14-00_YT_PIPELINE_DARKZOGX
MEMORI_SESI_2026-05-19T20-30_5W1H_GATE
```

### Step 3 — Extract Key Insight (5-10 Points)

Apply 5W+1H gate untuk decide "penting vs buang":
- Apa insight baru? (BUKAN repeat existing doktrin)
- Mengapa relevan? (impact ke Flowork architecture)
- Kapan applicable? (sekarang vs defer)
- Dimana di-save? (changelog / brain / memory file)
- Siapa benefit? (generasi berikutnya)
- Bagaimana lookup-able? (keyword + cross_refs)

Ngga lolos → buang. Lolos → masuk contekan.

Format key insight per item:

```
- <judul singkat>: <1 kalimat penjelasan> (refer: <doktrin atau file terkait>)
```

### Step 4 — Identifikasi Cross-Refs

- **strong**: doktrin baru yang lahir dari sesi
- **weak**: existing doktrin yang di-discuss / di-koreksi

### Step 5 — Save ke Brain DB Entry

```sql
INSERT INTO constitution (
    source_file, section, content, amplitude,
    sacred_lens, is_catalyst, context_origin,
    cross_refs, cross_refs_typed,
    pending_quorum_review
) VALUES (
    'chat_archive_<id>.jsonl',
    'MEMORI_SESI_<id>',
    '<contekan content 5-10 key insight>',
    99500.0,  -- tactical/operational, BUKAN sacred
    0,        -- sacred_lens=0, retrieve on-demand
    0,        -- is_catalyst=0 (sesi-specific, BUKAN universal pattern)
    'extract_session_contekan:<timestamp>',
    '<cross_refs JSON>',
    '<cross_refs_typed JSON>',
    0         -- ngga butuh quorum (sesi specific, auto-promote)
)
```

### Step 6 — Archive Full Chat

Save full chat ke file:

```
_training_corpus/chat_archive/MEMORI_SESI_<id>.jsonl
```

JSONL format, 1 message per line:

```json
{"turn": 1, "role": "user", "content": "...", "timestamp": "..."}
{"turn": 2, "role": "assistant", "content": "...", "timestamp": "..."}
```

### Step 7 — Update MEMORY.md Index (Optional)

Kalo sesi punya pelajaran "feedback" / "user context" baru → update file memory `.md` relevan.

## Output Template Contekan

```markdown
# MEMORI_SESI_<YYYY-MM-DD>_<TOPIK>

**Tanggal**: <YYYY-MM-DD> <HH:MM> WIB
**Durasi**: ~<X> turn
**Topik utama**: <judul>
**Trigger**: <apa yang start sesi ini>

## Key Insights (5-10)

1. **<Judul>**: <penjelasan 1-2 kalimat> → (cross-ref: <doktrin/file>)
2. **<Judul>**: <penjelasan> → (ref: <...>)
...

## Doktrin Baru Lahir
- [[DOKTRIN_X]] — <brief>

## Doktrin Existing Di-Discuss
- [[DOKTRIN_Y]] — <konteks bagaimana di-discuss>

## Action Items
- [ ] <item> — defer Phase X / eksekusi sekarang

## Archive
Full chat: `_training_corpus/chat_archive/MEMORI_SESI_<id>.jsonl`
```

## Pitfalls (Anti-Halu Saat Extract)

- ❌ **Skip 5W+1H gate** → contekan jadi naif / biased
- ❌ **Filter "penting" subyektif** → multiple summary angle, BUKAN single canonical
- ❌ **Lupa archive full chat** → detail hilang permanent, ngga bisa di-recover
- ❌ **Topik slug terlalu vague** → susah lookup nanti (e.g., `DISKUSI_FILSAFAT` vs spesifik `PENDENDAM_LINUX_FOUNDASI`)
- ❌ **Asumsi semua sesi worth contekan** → mostly chit-chat BUKAN insight, jangan generate noise

## Verification

Sebelum save contekan, cek:
- [ ] 5W+1H gate lolos
- [ ] Section ID format benar (sortable, unique)
- [ ] Key insight 5-10 items (BUKAN 1 atau 50)
- [ ] Cross-refs typed (strong + weak)
- [ ] Archive file di-write
- [ ] Brain DB entry inserted

## Tier Confidence

- Pattern contekan cocok 7B: **Tier 1** (text summary task ringan)
- Mimic manusia memori: **Tier 1** (proven mekanisme evolusi)
- Filter "penting" subyektif: **Tier 2** (mitigasi multiple angle)

## Tagline

> "Contekan kecil, lookup besar. Index hemat, detail di-archive."

## Reference

- Brain DB: `DOKTRIN_5W1H_GATE` (master gate untuk filter extract)
- Brain DB: `DOKTRIN_MEMORI_MULTI_TIER` (kelak)
- Memory: `feedback_communication_style.md` (bahasa konteks pengguna)
- Roadmap: Phase 7.5 — Sistem Contekan Memori
