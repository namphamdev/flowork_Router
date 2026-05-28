# flow_router — Agent Skills

Drop-in skills for any AI agent (Claude, Cursor, ChatGPT, custom SDK). Copy the URL or path to the relevant `SKILL.md` and paste it to your AI — it will fetch the skill and use flow_router for you.

> Start with the **flow-router** entry skill — it covers setup and indexes capability skills.

## Skills

| Capability | File |
|---|---|
| **Entry / Setup** (start here) | [flow-router/SKILL.md](flow-router/SKILL.md) |
| Chat / code-gen | [flow-router-chat/SKILL.md](flow-router-chat/SKILL.md) |
| Image generation | [flow-router-image/SKILL.md](flow-router-image/SKILL.md) |
| Text-to-speech | [flow-router-tts/SKILL.md](flow-router-tts/SKILL.md) |
| Speech-to-text | [flow-router-stt/SKILL.md](flow-router-stt/SKILL.md) |
| Embeddings | [flow-router-embeddings/SKILL.md](flow-router-embeddings/SKILL.md) |
| Web search | [flow-router-web-search/SKILL.md](flow-router-web-search/SKILL.md) |
| Web fetch (URL → markdown) | [flow-router-web-fetch/SKILL.md](flow-router-web-fetch/SKILL.md) |

## Setup once

```bash
export FLOW_ROUTER_URL="http://localhost:2402"  # local default, or your VPS / tunnel URL
export FLOW_ROUTER_KEY="flr_..."                # from Dashboard → Keys (only if requireApiKey=true)
```

Verify: `curl $FLOW_ROUTER_URL/api/health` → `{"status":"ok",...}`.
