---
name: flow-router-chat
description: Chat / code generation via flow_router using OpenAI /v1/chat/completions or Anthropic /v1/messages or Gemini /v1beta/models/...:generateContent with streaming + auto-fallback combos.
---

# flow_router — Chat

Requires `FLOW_ROUTER_URL` (and `FLOW_ROUTER_KEY` if auth enabled). See `flow-router/SKILL.md` for setup.

## Endpoints

- `POST $FLOW_ROUTER_URL/v1/chat/completions` — OpenAI format
- `POST $FLOW_ROUTER_URL/v1/messages` — Anthropic format
- `POST $FLOW_ROUTER_URL/v1beta/models/{model}:generateContent` — Gemini format
- `POST $FLOW_ROUTER_URL/v1/responses` — OpenAI Responses API

## Discover

```bash
curl $FLOW_ROUTER_URL/v1/models | jq '.data[].id'
curl "$FLOW_ROUTER_URL/v1/models/info?id=anthropic/claude-haiku-4-5"
```

Combos (e.g. `vip`, `mycodex`) auto-fallback through multiple providers.

## OpenAI format

```bash
curl -X POST $FLOW_ROUTER_URL/v1/chat/completions \
  -H "Authorization: Bearer $FLOW_ROUTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-haiku-4-5",
    "messages": [{"role":"user","content":"Say hi"}],
    "stream": false
  }'
```

Set `"stream": true` to receive SSE chunks (delta format).

## Anthropic format

```bash
curl -X POST $FLOW_ROUTER_URL/v1/messages \
  -H "Authorization: Bearer $FLOW_ROUTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-haiku-4-5",
    "max_tokens": 1024,
    "messages": [{"role":"user","content":"Say hi"}]
  }'
```

## Token saver (RTK)

When `RtkTokenSaver` is enabled in settings, tool-result messages over 4 KB are auto-compressed by format (git diff, build output, grep, tree, etc.) before being forwarded upstream — typical 40-80% reduction for agent loops.
