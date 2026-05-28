---
name: flow-router-stt
description: Speech-to-text (audio transcription) via flow_router /v1/audio/transcriptions. OpenAI-compat shape (multipart form), routes to active STT providers.
---

# flow_router — Speech-to-text

```bash
curl -X POST $FLOW_ROUTER_URL/v1/audio/transcriptions \
  -H "Authorization: Bearer $FLOW_ROUTER_KEY" \
  -F file=@audio.mp3 \
  -F model=whisper-1
```

Response: `{ "text": "..." }`.

## Discover STT models

```bash
curl $FLOW_ROUTER_URL/v1/models/stt | jq '.data[].id'
```
