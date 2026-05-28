---
name: trading_entry_konfluensi
description: Decision tree entry trade forex/saham dengan konfluensi check + risk management.
metadata:
  domain: trading
  created_at: 2026-05-19
---

# Trading Entry Konfluensi

## When to Use
- Evaluate setup trade sebelum entry
- User invoke trading task: "evaluate setup", "mau entry XAU/USD"

## Pre-Check (mandatory sebelum action)

1. **brain_search** `DOKTRIN_TRADING_CHECKLIST` untuk full decision tree
2. Verify Mr.Dev kondisi mental siap trade (bukan FOMO/revenge)
3. Pastikan akun setup proper (broker reliable, broker spread reasonable)

## Steps

### Step 1 — ANALISA SETUP
- Konfluensi minimum 2 signal align (support + fibo + RSI / breakout + volume + news)
- Konteks higher timeframe (D1/W1): trend atau range?
- Market structure: swing high/low jelas

### Step 2 — PLAN ENTRY + EXIT (mandatory BEFORE action)
- Entry price + tipe (limit/market)
- Stop loss di luar struktur, BUKAN tight banget
- Take profit minimum RR 1:2 (idealnya 1:3)
- Position size: max 1-2% risk akun
- Time stop: 7-14 hari kalau ngga progress, EXIT

### Step 3 — EKSEKUSI
- Place order + set HARD stop loss (BUKAN mental)
- Log journal: alasan entry, emotion saat itu

### Step 4 — TRIGGER STOP LOSS (jangan emotional)
- Harga touch level → EXIT
- Fundamental change → EXIT walaupun level belum kena
- NEVER move stop lower (long) atau higher (short)

### Step 5 — REVIEW post-trade
- Triggered stop atau target?
- Lesson ke mistakes_journal kalau insight baru

## Decision Rule
- 2 konfluensi + RR 1:2 + risk 1-2% akun = ENTRY
- Single signal atau RR <1:2 = SKIP

## References
- `DOKTRIN_TRADING_CHECKLIST` (brain amp 999998)
- `DOKTRIN_INVESTASI_REKSADANA` (kalau long-term)
