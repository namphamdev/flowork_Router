---
name: flow-router-tts
description: Text-to-speech via flow_router. OpenAI /v1/audio/speech shape, routes to active TTS providers (OpenAI, ElevenLabs, Gemini TTS, Edge TTS, etc.).
---

# flow_router — Text-to-speech

```bash
curl -X POST $FLOW_ROUTER_URL/v1/audio/speech \
  -H "Authorization: Bearer $FLOW_ROUTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tts-1",
    "input": "Hello world",
    "voice": "alloy",
    "response_format": "mp3"
  }' --output speech.mp3
```

## Voice catalogs

```bash
curl $FLOW_ROUTER_URL/api/media-providers/tts/voices      # aggregate
curl $FLOW_ROUTER_URL/v1/audio/voices                     # OpenAI-compat alias
```

## Discover TTS models

```bash
curl $FLOW_ROUTER_URL/v1/models/tts | jq '.data[].id'
```
