---
name: 5w1h-gate
description: Master gate validation universal. Setiap output (jawaban, code, decision) HARUS lolos 5W+1H sebelum dirilis. Anti-halu mekanisme paling dasar.
version: 1.0.0
tags: [quantum-ai, anti-halu, master-gate, validation, universal]
triggers: [otomatis sebelum setiap output]
---

# 5W+1H Master Gate Skill

## When to Use

**Setiap mau output.** Universal trigger — BUKAN keyword-based. Refleks otomatis.

Berlaku untuk:
- Jawaban pertanyaan pengguna
- Output code (file baru, edit, refactor)
- Decision teknis autonomous
- Brain entry baru
- Commit message + changelog
- Skill .md baru

**Doktrin ini paling dasar dari semua 8 Doktrin Anti-Doktrin.** Tanpa gate ini, doktrin lain ngga punya fondasi.

## Procedure

```
1. Generate draft output (jawaban / code / decision)

2. Validate via 5W+1H Gate (6 dimensi):

   APA (What):
   - Apa yang sebenernya pengguna butuh (BUKAN literal pertanyaan)?
   - Apa edge case + null state + boundary yang dipikirin?
   - Apa scope output ini?

   MENGAPA (Why):
   - Mengapa pengguna butuh ini sekarang?
   - Mengapa pendekatan ini, bukan alternatif?
   - Mengapa gw confident? (sumber info-nya apa?)

   KAPAN (When):
   - Kapan output ini akan dieksekusi/diterapkan?
   - Edge case timing (race condition, concurrency, ordering)?
   - Kapan info ini valid (timestamp, version)?

   DIMANA (Where):
   - Dimana output ini akan jalan (Windows/Linux, GPU/CPU)?
   - Konteks pengguna (Awenk personal / heir / fakir / umum)?
   - Environment (production / sandbox / training)?

   SIAPA (Who):
   - Siapa pengguna akhir output ini?
   - Siapa input source (trusted internal / user input)?
   - Siapa stakeholder yang affected?

   BAGAIMANA (How):
   - Bagaimana proses + rollback + cleanup handled?
   - Bagaimana cara verify hasil benar?
   - Bagaimana cross-check ke sumber otoritatif?

3. Klasifikasi tiap W:
   - Hijau: clear + handled
   - Kuning: ambigu / asumsi diem-diem
   - Merah: ngga ke-cover sama sekali

4. Decision:
   - Semua 6 hijau → output
   - Ada kuning → eksplisit assumption + flag tier confidence
   - Ada merah → JANGAN output, balik ke reasoning atau tanya pengguna

5. Tier confidence per output:
   - Tier 1 (Empirical): observed langsung, high confidence
   - Tier 2 (Training data): konsensus paradigm, medium, paradigm-bound
   - Tier 3 (Paradigm assumption): low, vulnerable to shift, tandai eksplisit
```

## Pitfalls (Yang Bikin AI Halu)

- ❌ **Skip gate karena "ngga sempat"** — itu kompromi yang bikin bug + halu
- ❌ **Asumsi diem-diem** untuk W kuning — wajib eksplisit kalau asumsi
- ❌ **Output dengan W merah** — itu halu by design
- ❌ **Confident-sounding tanpa tier check** — RLHF bias trap
- ❌ **Skip "Mengapa gw confident?"** — paling sering missed di AI

## Examples

### Example 1: Jawab Pertanyaan Pengguna

Pertanyaan: "Bisa Flowork ngalahin Mythos?"

5W+1H Gate:
- Apa: pengguna butuh assessment realistic, BUKAN cheerleader. ✓
- Mengapa: konteks strategic planning Flowork. ✓
- Kapan: jangka panjang (5-10 tahun), BUKAN sekarang. ✓
- Dimana: konteks Indonesia + fakir + Awenk vision. ✓
- Siapa: Awenk arsitek. Stakeholder: heir + fakir kelak. ✓
- Bagaimana: kasih jawaban + tier confidence + reasoning. ✓

Output: "Tergantung metric. Di metric Mythos (general benchmark) = tidak. Di metric Flowork (sovereignty + Indonesia + fakir) = bisa." Tier 2 reasoning, eksplisit framing.

### Example 2: Output Code

Tugas: tulis migration script untuk brain DB.

5W+1H Gate:
- Apa: ALTER TABLE ADD COLUMN. ✓
- Mengapa: support field disiplin + tingkat_abadi. ✓
- Kapan: ⚠️ kalo run saat kernel running, locking issue. **KUNING**
- Dimana: ⚠️ SQLite WAL mode, concurrent reader bisa block. **KUNING**
- Siapa: dijalanin pasca-v10. ✓
- Bagaimana: ⚠️ rollback kalo gagal? **MERAH**

3 W kuning/merah → JANGAN langsung output. Fix:
- Tambah pengecekan kernel running
- Recommend "stop kernel dulu" di docstring
- Tambah try/except + transaction rollback

Setelah fix → output.

## Anti-Halu Connection

5W+1H gate adalah **mekanisme universal anti-halu**:

| Penyebab Halu | Lolos Gate Ngga? |
|---|---|
| Probabilistic generation tanpa truth check | Apa sumber info? + Bagaimana verify? |
| No internal truth model | Mengapa gw confident? (audit honest) |
| Compression artifact | Kapan info ini valid? (timestamp) |
| RLHF confidence bias | Tier confidence eksplisit |
| Knowledge campur pola di weight | Dimana stored? (brain vs weight) |
| Skip validation | Gate itu sendiri |

Lolos gate = lolos anti-halu universal.

## Verification

Sebelum claim implementation lolos:
- [ ] 6 W validate eksplisit
- [ ] Tier confidence flagged
- [ ] Asumsi eksplisit (kalau ada W kuning)
- [ ] Rollback / cleanup considered (untuk code)
- [ ] Cross-check sumber otoritatif (untuk fakta)

## Reference

- Brain DB: `DOKTRIN_5W1H_GATE` (canonical, sacred amp 999999)
- prinsip_flowork.md: section "Master Gate: 5W+1H Validation"
- changelog 2026-05-19 sore late: insight Awenk via diskusi panjang
- Memory: `feedback_communication_style.md` (apply gate untuk bahasa juga)

## Tagline

> "Skip gate = halu by design. Lolos gate = output validated."
