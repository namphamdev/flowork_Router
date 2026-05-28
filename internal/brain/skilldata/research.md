---
name: research
description: Multi-source research with cross-verify + Tier-1/2/3 source quality
version: 1.0.0
tags: [research, OSINT, fact-check, multi-source]
triggers: [research, riset, investigate, fact check, cross verify]
---

# Research — Multi-Source Verified

## When to Use
- Factual claim yang butuh verify (price, news, person background)
- Decision support yang impact significant
- Bug bounty target recon
- Person OSINT investigative

## Procedure
1. **Define scope**: pertanyaan spesifik (apa yang dicari), kriteria stop
2. **Multi-source aggregate** (min 3 source, ideal 5+):
   - Tier 1 (mainstream, authoritative): Reuters, Bloomberg, IDX, SEC, official docs
   - Tier 2 (industry, semi-formal): industry blog, expert opinion, reputable forum
   - Tier 3 (social, anecdotal): Twitter/X, Reddit, LinkedIn discussion
3. **Cross-verify claim per fact**: kalau 1 source bilang X, cek 2-3 source lain
4. **Anti-stale** (price/news terkini): WAJIB tool_call live_research atau webfetch (JANGAN halu dari memori)
5. **Output structured**:
   - Summary (key finding)
   - Source per fact (URL + title + date + tier)
   - Confidence per claim (high/medium/low)
   - Caveats (apa yang ambiguous / unverified)

## Pitfalls
- Single source dependency
- Tier 1 unverified accepted (corporate PR != fact)
- Stale data (memori training cutoff)
- Skip cite source (halu source)

## Verification
- Min 3 source per claim? ✅/❌
- Cross-verify done? ✅/❌
- Source cited URL + date + tier? ✅/❌
- Anti-stale rule followed (live data for time-sensitive)? ✅/❌
