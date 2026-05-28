---
name: flow-router-web-search
description: Web search via flow_router /v1/search. Routes to active search providers (Tavily, Brave, SerpAPI, etc.) and normalizes results.
---

# flow_router — Web search

```bash
curl -X POST $FLOW_ROUTER_URL/v1/search \
  -H "Authorization: Bearer $FLOW_ROUTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "latest Go 1.25 features",
    "max_results": 5
  }'
```

Response: `{ "results": [{ "title": "...", "url": "...", "snippet": "..." }] }`.

## Discover providers

```bash
curl $FLOW_ROUTER_URL/v1/models/web | jq '.data[].id'
```
