---
name: audit-3-kriteria
description: Pre-propose audit. Sebelum Mr.Flow propose mekanisme/doktrin/skill baru, cek 3 kriteria (bentrok/ambigu/cocok 7B). Anti over-engineering.
version: 1.0.0
tags: [audit, anti-over-engineering, minimalis-spek, quantum-ai]
triggers: [sebelum propose doktrin baru, sebelum propose skill baru, sebelum propose mekanisme, sebelum add brain entry sacred]
---

# Audit 3 Kriteria Skill

## When to Use

Setiap kali Mr.Flow mau propose:
- Doktrin baru ke brain DB
- Skill .md baru di bundled/
- Mekanisme baru di kernel
- Brain entry sacred amp 999998+
- Phase baru di roadmap

**Asal**: Awenk catch 2026-05-19 — Mr.Flow drift propose 5 mekanisme tanpa audit. Awenk: "ati ati di lepas doktrin... jangan serba seed pastiin dulu itu bentrok ngak, ambigu ngak cocok buat 7b ngak".

Per audit Awenk → 4 dari 5 propose gw bermasalah. **Anti over-engineering rule**.

## Procedure

**SEBELUM propose, audit 3 kriteria**:

### 1. BENTROK?

Cek overlap dengan:
- Existing doktrin (15+ doktrin Quantum AI)
- Existing skill `.md` di `bundled/skills/`
- Existing mekanisme di kernel
- 6 Pilar / Aturan Disiplin / Foundasi Moral

**Pertanyaan kunci**:
- Apakah ada existing yang udah cover?
- Apakah ini overlap fungsi dengan existing?
- Apakah konflik dengan Tier 0-2 immutable?

Kalo IYA bentrok → STOP. Update existing, BUKAN add new.

### 2. AMBIGU?

Cek clarity:
- Trigger jelas (kapan apply)?
- Procedure step-by-step clear?
- Threshold operasional konkret?
- Boundary scope explicit?

**Anti-pattern**: angka arbitrer ("setiap 5-10 turn"), criteria fuzzy ("kalo merasa perlu"), scope ngga jelas ("untuk semua kasus").

Kalo IYA ambigu → klarifikasi dulu, BUKAN propose half-baked.

### 3. COCOK 7B?

Cek constraint hardware/software:
- Kompute reasonable di RTX 4060 8GB VRAM?
- Inferensi per query ngga >10-20 detik?
- Storage ngga mahal (brain DB SQLite)?
- Ngga butuh dependency baru yang heavy?
- Ngga butuh kernel rebuild kecuali necessary?

**Anti-pattern**: multi-pass generation (3x inferensi per query = 60 detik response, ngga acceptable untuk niche Awenk).

Kalo IYA ngga cocok → reframe ke yang ringan, atau skip total.

## Decision Matrix

| Bentrok | Ambigu | Cocok 7B | Action |
|---|---|---|---|
| Tidak | Tidak | Iya | ✅ **PROPOSE**, lanjut design |
| Iya | - | - | ❌ **DROP** — overlap existing |
| - | Iya | - | ⚠️ **CLARIFY** — define criteria operasional dulu |
| - | - | Tidak | ⚠️ **REFRAME** ringan, atau drop |

**Minimum 1 X = STOP** propose. Audit lagi.

## Examples

### Example 1: Drop (Bentrok)

Propose: "DOKTRIN_REFLEKS_PARADIGM_QUESTIONING"
- **Bentrok**: ✅ overlap 100% dengan DOKTRIN_REFLEKS_EINSTEIN existing
- Action: **DROP**

### Example 2: Clarify (Ambigu)

Propose: "Reframe trigger as habit setiap 5-10 turn"
- **Ambigu**: ✅ angka 5-10 arbitrer, threshold ngga jelas
- Action: **CLARIFY** — define exact trigger criteria

### Example 3: Reframe (Ngga Cocok 7B)

Propose: "Multi-pass generation eksplisit (3x inferensi)"
- **Cocok 7B**: ❌ 3x inferensi = 30-90 detik response di RTX 4060
- Action: **REFRAME** ke sequential chain (Lapisan 3) atau drop

### Example 4: Propose (3 Lolos)

Propose: "Brain DB auto-staleness warning saat retrieve"
- **Bentrok**: Tidak (ngga ada existing staleness mechanism di Flowork)
- **Ambigu**: Tidak (trigger: retrieve entry dengan last_accessed_at > 30 hari)
- **Cocok 7B**: Iya (SQL query timestamp, ngga butuh inferensi tambahan)
- Action: ✅ **PROPOSE**, lanjut design

## Pitfalls

- ❌ **Skip audit karena momentum** — lo lagi excited propose, lupa cek. Itu yang Awenk catch.
- ❌ **Self-bias** — propose sendiri sering anggap unique. Pretend lo reviewer external.
- ❌ **Salah anggap "tambah lebih baik"** — di Flowork, **lebih sedikit doktrin + lebih dalam** > banyak doktrin shallow.
- ❌ **Ignore Awenk's anti-over-engineering rule** — explicit feedback ngotot, lo tetep propose. Tier 0 violation (anti-otoriter ke Awenk).

## Tier Confidence

- 3 kriteria sebagai filter: **Tier 1 empirical** (Awenk demo via 5 audit case)
- Procedure replicable: **Tier 2** (butuh L3 test eksekusi)

## Tagline

> "Filter ketat > banyak proposal. Quality over quantity. 1 doktrin yang fit > 5 doktrin yang konflik."

## Reference

- Brain DB: `DOKTRIN_FILTER_KETAT` + `DOKTRIN_KEPADATAN_KAWINAN`
- Memory: `feedback_communication_style.md` (anti-muter)
- changelog 2026-05-19: 4 dari 5 propose Mr.Flow ke-drop via audit ini
