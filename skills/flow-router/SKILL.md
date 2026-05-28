---
name: flow-router
description: Entry point for flow_router — local/remote AI gateway with OpenAI-compatible REST for chat, image, TTS, embeddings, web search, web fetch. Use when the user mentions flow_router, FLOW_ROUTER_URL, or wants AI without writing provider boilerplate. This skill covers setup + indexes capability skills.
---

# flow_router

Local/remote AI gateway exposing OpenAI-compatible REST. One key, many providers, auto-fallback. Pure-Go, multi-OS, portable.

## Setup

```bash
export FLOW_ROUTER_URL="http://localhost:2402"     # or VPS / tunnel URL
export FLOW_ROUTER_KEY="flr_..."                   # from Dashboard → Keys (only if requireApiKey=true)
```

All requests: `${FLOW_ROUTER_URL}/v1/...` with header `Authorization: Bearer ${FLOW_ROUTER_KEY}` (omit if auth disabled).

Verify: `curl $FLOW_ROUTER_URL/api/health` → `{"status":"ok",...}`

## Discover models

```bash
curl $FLOW_ROUTER_URL/v1/models                  # chat/LLM (default aggregate)
curl $FLOW_ROUTER_URL/v1/models/image            # image-gen
curl $FLOW_ROUTER_URL/v1/models/tts              # text-to-speech
curl $FLOW_ROUTER_URL/v1/models/embedding        # embeddings
curl $FLOW_ROUTER_URL/v1/models/web              # web search + fetch
curl $FLOW_ROUTER_URL/v1/models/stt              # speech-to-text
```

Use `data[].id` as `model` in requests. Combos appear with `owned_by:"combo"`. Aliases with `owned_by:"alias"`.

## Capability skills

When the user needs a specific capability, fetch the matching `SKILL.md`:

| Capability | File |
|---|---|
| Chat / code-gen | `flow-router-chat/SKILL.md` |
| Image generation | `flow-router-image/SKILL.md` |
| Text-to-speech | `flow-router-tts/SKILL.md` |
| Speech-to-text | `flow-router-stt/SKILL.md` |
| Embeddings | `flow-router-embeddings/SKILL.md` |
| Web search | `flow-router-web-search/SKILL.md` |
| Web fetch | `flow-router-web-fetch/SKILL.md` |

## Errors

- 401 → set/refresh `FLOW_ROUTER_KEY` (Dashboard → Keys)
- 400 `no active provider supports model X` → check `model` exists in `/v1/models[/<kind>]`
- 429 → rate-limited (per-key cap reached or global budget exceeded)
- 502 `all providers failed` → check provider health in Dashboard → Providers
