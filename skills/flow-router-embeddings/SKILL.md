---
name: flow-router-embeddings
description: Embeddings via flow_router /v1/embeddings. OpenAI-compat shape, routes to active embedding providers (OpenAI, Gemini, Cohere, etc.).
---

# flow_router — Embeddings

```bash
curl -X POST $FLOW_ROUTER_URL/v1/embeddings \
  -H "Authorization: Bearer $FLOW_ROUTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-3-small",
    "input": ["sentence one", "sentence two"]
  }'
```

Response: `{ "data": [{ "embedding": [...], "index": 0 }, ...] }`.

## Discover

```bash
curl $FLOW_ROUTER_URL/v1/models/embedding | jq '.data[].id'
```
