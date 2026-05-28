---
name: suno_music_production
description: Workflow Suno generate → Demucs separate → Reaper remix → export untuk lolos AI detection Tunecore/Spotify.
metadata:
  domain: music
  created_at: 2026-05-19
  trigger_count: 0
---

# Suno Music Production Workflow

## When to Use
- Generate music untuk YouTube channel
- Mass produce track untuk 200 channel scaling
- Track untuk Tunecore distribution (Spotify/Apple Music)

## Trigger Pattern
- User request: "generate music", "Suno produce", "track untuk channel"
- Volume: 1-30 track per batch

## Steps

1. **Suno generate** prompt sub-niche spesifik:
   ```
   "soft piano for sleep, no drums, no vocals, ambient pad, slow tempo 60 BPM"
   ```
   - Generate 3-5 variant
   - Pilih terbaik
   - Export WAV (BUKAN MP3 — MP3 leak signature)

2. **Demucs stem separation:**
   ```bash
   demucs --model htdemucs_6s song.wav
   # Output: drums.wav, bass.wav, vocals.wav, other.wav
   ```

3. **REPLACE stem (anti AI detection):**
   - Replace min 2 stem (drum + bass) dengan sample independent
   - Source: Cymatics free, Looperman, LANDR free tier
   - Layer real instrument kalau ada

4. **Re-mix di Reaper:**
   - EQ surgical cut 5-8 kHz (Suno signature)
   - Re-compress dynamic 12-18 dB
   - Pan + panning automation subtle
   - Reverb plate/hall berbeda dari Suno default
   - Foley: vinyl crackle + tape hiss + room noise -50 dB

5. **Master + export:**
   - Tape emulation (Klanghelm IVGI free)
   - Tube saturation (TDR SlickEQ)
   - Limiter -14 LUFS YouTube / -16 LUFS Spotify
   - Export WAV 24-bit clean metadata

6. **Disclose** "AI-assisted" di Tunecore upload form

## Detection Rate Target
- Suno raw: 60-80%
- Stem-only remix: 30-50%
- Replace 2 stem + heavy edit: 10-20%
- Replace 3 stem + Foley + tape: **<5%** ← TARGET

## Tools FREE
- Demucs (Python pip)
- Reaper (unlimited trial)
- Klanghelm IVGI, Airwindows (tape)
- TDR Nova/Kotelnikov/SlickEQ/Limiter (FREE)
- TAL Reverb 4 (FREE)

## References
- `DOKTRIN_MUSIC_PRODUCTION` (brain amp 999998)
- `mixing.md` workflow detail full
