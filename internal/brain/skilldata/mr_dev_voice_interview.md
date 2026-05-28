---
name: mr_dev_voice_interview
description: Phase 10 pipeline capture tacit knowledge Mr.Dev via voice note Telegram → Whisper → JSONL.
metadata:
  domain: data_training
  created_at: 2026-05-19
---

# Mr.Dev Voice Interview Pipeline

## When to Use
- Daily compounding tacit knowledge Mr.Dev unique (ngga ada di internet)
- Build moat — knowledge yang ngga ke-train LLM mainstream
- Source training data v11+ (kalau brain accumulate cukup)

## Pre-Requirements
- Whisper installed (whisper.cpp atau openai-whisper)
- Telegram bot token + chat_id (settings DB existing)
- v6_venv post-training selesai

## Pipeline Components

### 1. Voice Collector (DONE — Wave 1)
- `scripts/mr_dev_voice_collector.py`
- Polling Telegram getUpdates
- Download voice .ogg ke `_training_corpus/voice_inbox/`
- Track offset + metadata index

### 2. Whisper Transcribe (Phase 10 pending)
- Daily cron: scan voice_inbox/ unprocessed
- Whisper transcribe → .txt + JSONL pair
- Move processed ke `voice_inbox/processed/`

### 3. Brain Inject (Phase 10 pending)
- Parse transcript → extract insight
- Auto-insert `mistakes_journal` tier 'raw'
- Tag domain: infer dari keyword
- Weekly Mr.Dev review → promote raw → DOKTRIN amp 999998

## Question Bank (Mr.Flow generate, Mr.Dev answer)

Domain rotation harian:
- SEO: 5 pertanyaan tactical
- YouTube: 5 pertanyaan
- Trading: 5 pertanyaan
- Coding: 5 pertanyaan
- Hacking: 5 pertanyaan
- dst

Format pertanyaan:
- "Bro gimana caranya {task spesifik domain}?"
- "Apa pattern recurring yang lo lihat di {area}?"
- "Decision tree lo untuk {scenario} gimana?"

## Compounding Math
- 5 pertanyaan/hari × 365 = **1825 row/tahun** unique tacit knowledge
- 5 tahun = 9125 row pure Mr.Dev knowledge
- Moat: ngga ada di internet, ngga akan masuk LLM mainstream

## References
- `scripts/mr_dev_voice_collector.py`
- `roadmap_after_training.md` Phase 10
- `DOKTRIN_ERROR_EDUKASI_LIFECYCLE` (brain)
