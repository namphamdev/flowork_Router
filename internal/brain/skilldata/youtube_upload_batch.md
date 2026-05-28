---
name: youtube_upload_batch
description: Upload video batch ke multiple YouTube channel via Donut Browser MCP. Workflow profile rotation + anti-detection delay.
metadata:
  domain: youtube
  created_at: 2026-05-18
  trigger_count: 0
  is_catalyst: false
---

# YouTube Upload Batch via Donut Browser

## When to Use
Task: upload N video ke N channel YouTube (1 video per channel atau bulk).

## Trigger Pattern
- User request mengandung: "upload N video", "publish ke channel", "batch upload"
- N channel >= 2

## Steps

1. **Pre-check** — query brain `DOKTRIN_DONUT_BROWSER_MCP` untuk tool list + workflow
2. **Load target channels** dari config (atau user spec)
3. **Loop per channel:**
   ```
   FOR profile_id in target_channels:
       - donut_browser__run_profile(profile_id, headless=false)
       - donut_browser__navigate(profile_id, "https://studio.youtube.com")
       - donut_browser__click_element(profile_id, "input[type=file]")
       - [upload file via OS file dialog automation]
       - donut_browser__type_text(profile_id, "#title-input", video_title)
       - donut_browser__type_text(profile_id, "#description-input", description)
       - donut_browser__click_element(profile_id, "#publish-button")
       - donut_browser__screenshot(profile_id) verify
       - log audit ke task_events
       - donut_browser__kill_profile(profile_id)
       - delay random 30-120s (anti-detection)
   END FOR
   ```

## Anti-Detection Checklist
- Fingerprint unik per profile (built-in Donut)
- Proxy SOCKS5 rotation kalau available
- Random delay 30-120s antar profile
- Avoid same IP cluster

## Output Format
- Total uploads: N/N success
- Per-channel: profile_id, video_url, screenshot path
- Failure: log ke mistakes_journal tier raw

## References
- `DOKTRIN_DONUT_BROWSER_MCP` (brain)
- `mixing.md` workflow audio
- `yt.md` Bagian 9 workflow production

## Side Effects
- Profile state: temp browser cookies, screenshot saved
- Audit log: task_events per upload
- Mistakes journal: kalau ada error
