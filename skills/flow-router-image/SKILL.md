---
name: flow-router-image
description: Image generation via flow_router OpenAI-shape /v1/images endpoint. Routes to active media providers (OpenAI, Stability, BFL, Gemini, etc.).
---

# flow_router — Image generation

```bash
curl -X POST $FLOW_ROUTER_URL/v1/images \
  -H "Authorization: Bearer $FLOW_ROUTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dall-e-3",
    "prompt": "A serene mountain lake at dawn",
    "size": "1024x1024",
    "n": 1
  }'
```

Response: `{ "data": [{ "url": "..." }] }` or `{ "data": [{ "b64_json": "..." }] }` depending on provider.

## Discover

```bash
curl $FLOW_ROUTER_URL/v1/models/image | jq '.data[].id'
```

Backed by media-providers configured in Dashboard → Media Providers.
