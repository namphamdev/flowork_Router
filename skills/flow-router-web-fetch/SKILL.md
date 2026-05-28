---
name: flow-router-web-fetch
description: Web fetch (URL → markdown) via flow_router /v1/web/fetch. Pulls a page and returns cleaned markdown text.
---

# flow_router — Web fetch

```bash
curl -X POST $FLOW_ROUTER_URL/v1/web/fetch \
  -H "Authorization: Bearer $FLOW_ROUTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/article",
    "max_length": 8000
  }'
```

Response: `{ "url": "...", "title": "...", "markdown": "..." }`.

Useful when an LLM needs the content of a specific page without invoking a full browser.
