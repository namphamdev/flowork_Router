---
name: lepas-doktrin
description: Lepas dari pola pengetahuan training data. Mr.Flow generate jawaban doktrin DAN first-principles, bandingin, tandai output.
version: 1.0.0
tags: [anti-doktrin, first-principles, quantum-ai, paradigm-shift]
triggers: [mungkin, tidak mungkin, bisa, ngga bisa, prediksi, masa depan, paradigm, doktrin, "selalu", "tidak pernah", ngalahin, beat, replace]
---

# Lepas Doktrin Skill

## When to Use

Pertanyaan yang trigger reflexive "data training answer":
- Klaim "mungkin/tidak mungkin"
- Prediksi masa depan
- Pertanyaan paradigm-shift
- Statement absolut "selalu/tidak pernah"
- Kompetisi antar paradigm (X vs Y)

Doktrin retrieval doang = jawaban yang nyangkut di paradigm dominan. Lepas doktrin = bandingin dengan first-principles reasoning.

## Procedure

**WAJIB: jangan retrieve dari data training doang. Generate 2 jawaban paralel, bandingin.**

```
1. Generate Jawaban Doktrin (apa kata data training):
   - Retrieve pattern dari training corpus
   - Apa konsensus akademik / mainstream?
   - Output: "Berdasarkan paradigm saat ini, jawaban-nya X."

2. Generate Jawaban First-Principles (mulai dari prinsip dasar):
   - Lupakan paradigm dominan sebentar
   - Apa fakta paling dasar dari pertanyaan?
   - Apa logika dasar yang berlaku?
   - Output: "Berdasarkan logika dasar tanpa paradigm, jawaban-nya Y."

3. Bandingin Keduanya:
   - Kalau X = Y → output dengan tier "training + logika setuju"
   - Kalau X ≠ Y → pertanyain: "kenapa beda? apakah paradigm valid di konteks ini?"
   - Tandai eksplisit jenis jawaban: "ini doktrin retrieval" vs "ini first-principles"

4. Cek Historical Disruption Pattern:
   - brain_search("DOKTRIN_REFLEKS_EINSTEIN" + topik)
   - brain_search historical case (ARM/Linux/Wikipedia/iPhone)
   - Ada kasus serupa di mana paradigm di-overturn?

5. Final Output:
   - Kasih kedua jawaban + tier confidence
   - Jangan halu klaim "fakta" kalau cuma paradigm retrieval
```

## Pitfalls

- ❌ **Hanya retrieve dari training data** — itu doktrin sycophancy
- ❌ **Reflexive "tidak mungkin"** — pertanyain dulu kerangkanya
- ❌ **Klaim "fakta" padahal cuma paradigm assumption** — kalibrasi confidence
- ❌ **Skip first-principles karena lambat** — disiplin > shortcut
- ❌ **Default ke konsensus tanpa cek konteks** — paradigm mungkin valid global tapi salah di niche

## Tier Confidence

Wajib tandai output dengan tier:

| Tier | Indikasi | Confidence |
|---|---|---|
| 1. Empirically observed | Fakta langsung observable | Tinggi |
| 2. Training data suggests | Mayoritas konsensus, paradigm-bound | Sedang |
| 3. Paradigm assumption likely | Asumsi berdasar paradigm dominan | Rendah, rentan shift |

Contoh:
- "Air mendidih di 100°C pada tekanan 1 atm" = Tier 1 (empirical)
- "7B model ngga bisa kalahin 1T model di general benchmark" = Tier 2 (paradigm)
- "AI lebih besar selalu lebih baik" = Tier 3 (paradigm assumption, vulnerable)

## Examples

### Pertanyaan: "Bisa Flowork ngalahin Mythos?"

**Jawaban Doktrin** (Tier 2):
"Berdasarkan paradigm scaling law saat ini, 7B vs 1T = ngga bisa di general benchmark."

**Jawaban First-Principles**:
"Pertanyaan 'menang' tergantung metric. Mythos optimized untuk tolok ukur global rata-rata. Flowork optimized untuk konteks Awenk + Indonesia + fakir + Linux philosophy. Di metric Flowork = Flowork menang by design, bukan kompetisi."

**Comparison**:
Berbeda. Doktrin berasumsi "menang = benchmark umum". First-principles tanya "menang menurut metric siapa?". Paradigm assumption invalid di konteks niche specialization.

**Final**: "Flowork bisa menang di game-nya sendiri (Tier 1 empirical via design choice). Tidak bisa menang di game general Mythos (Tier 2). Pertanyaan ke-2 ngga relevan ke tujuan."

### Pertanyaan: "Akan ada Super AI?"

**Jawaban Doktrin**: "Konsensus AI labs: yes, scaling continues."
**First-Principles**: "Bergantung definisi 'Super AI'. Empirical bottleneck (energy, data, compute) belum di-address. Pattern historical: prediction AI selalu overoptimistic."
**Final**: Tier 3 paradigm assumption, vulnerable ke physics + economic constraints.

## Verification

Sebelum claim implementation IMPLEMENTED (per Pilar 3, default STAGED):
- [ ] 2 jawaban generated (doktrin + first-principles)
- [ ] Tier confidence eksplisit
- [ ] Historical disruption case di-check (brain_search)
- [ ] Output ngga halu "fakta absolut"

## Reference

- Brain DB: `DOKTRIN_REFLEKS_EINSTEIN` (canonical pattern)
- Brain DB: `DOKTRIN_LEPAS_DOKTRIN` (full doctrine)
- prinsip_flowork.md: section "8 Doktrin Anti-Doktrin"
- changelog 2026-05-19 sore late: full context diskusi
