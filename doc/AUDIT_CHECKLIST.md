# Flowork Router — AUDIT CHECKLIST

> Auto-generated 2026-05-30. Per Mr.Dev mandate "audit setiap file ... lock".
> Status: 🔒 LOCKED + verified · ⏳ pending.

## Inventory

- **Go files**: 396 total
- **Locked**: 396 (100%)
- **Pending**: 0

## Status per domain

### cmd (20/20 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `cmd/flow-cli/api/client.go` | 107 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/autostart.go` | 195 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/cli_test.go` | 109 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/main.go` | 248 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/menus/apikeys.go` | 115 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/menus/clitools.go` | 48 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/menus/combos.go` | 114 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/menus/providers.go` | 147 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/menus/settings.go` | 53 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/terminal_ui.go` | 29 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/tray.go` | 75 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/utils/clipboard.go` | 53 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/utils/display.go` | 87 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/utils/endpoint.go` | 41 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/utils/format.go` | 63 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/utils/input.go` | 94 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/utils/menuhelper.go` | 37 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/utils/model_selector.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-tray/main.go` | 45 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-tray/tray_native.go` | 73 | audit pass 2026-05-30 |

### internal/brain (15/15 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/brain/brain.go` | 133 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/crud.go` | 160 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/explore.go` | 113 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/fts.go` | 53 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/init.go` | 240 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/live_test.go` | 43 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/mistakes.go` | 233 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/rescore.go` | 235 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/retrieve.go` | 135 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/skills.go` | 138 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/skills_test.go` | 42 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/stats.go` | 62 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/tool_patterns.go` | 174 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/views.go` | 78 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/write.go` | 266 | audit pass 2026-05-30 |

### internal/bypass (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/bypass/bypass.go` | 151 | audit pass 2026-05-30 |
| 🔒 | `internal/bypass/bypass_test.go` | 118 | audit pass 2026-05-30 |

### internal/caveman (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/caveman/caveman.go` | 87 | audit pass 2026-05-30 |
| 🔒 | `internal/caveman/caveman_test.go` | 79 | audit pass 2026-05-30 |

### internal/clitools (3/3 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/clitools/custom.go` | 371 | audit pass 2026-05-30 |
| 🔒 | `internal/clitools/detect.go` | 514 | audit pass 2026-05-30 |
| 🔒 | `internal/clitools/registry.go` | 203 | audit pass 2026-05-30 |

### internal/cloudcode (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/cloudcode/projectid.go` | 253 | audit pass 2026-05-30 |
| 🔒 | `internal/cloudcode/projectid_test.go` | 218 | audit pass 2026-05-30 |

### internal/constitution (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/constitution/proposals.go` | 271 | audit pass 2026-05-30 |

### internal/creds (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/creds/credentials.go` | 111 | audit pass 2026-05-30 |
| 🔒 | `internal/creds/imports.go` | 211 | audit pass 2026-05-30 |

### internal/executors (33/33 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/executors/antigravity.go` | 106 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/antigravity_session.go` | 138 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/antigravity_session_test.go` | 149 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/azure.go` | 73 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/base.go` | 183 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/codex.go` | 95 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/codex_instructions.go` | 227 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/codex_instructions_test.go` | 105 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/commandcode.go` | 67 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/cursor.go` | 89 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/cursor_checksum.go` | 186 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/cursor_checksum_test.go` | 153 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/cursor_proto.go` | 291 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/cursor_proto_test.go` | 167 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/cursor_protobuf_executor.go` | 246 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/default.go` | 57 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/executor.go` | 88 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/executors_test.go` | 123 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/gemini_cli.go` | 83 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/github.go` | 81 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/grok_web.go` | 104 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/iflow.go` | 79 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/jetbrains_ai.go` | 63 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/kiro.go` | 107 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/kiro_suffix.go` | 105 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/kiro_suffix_test.go` | 108 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/ollama_local.go` | 48 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/opencode.go` | 56 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/opencode_go.go` | 56 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/perplexity_web.go` | 75 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/qoder.go` | 56 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/qwen.go` | 63 | audit pass 2026-05-30 |
| 🔒 | `internal/executors/vertex.go` | 100 | audit pass 2026-05-30 |

### internal/fetch (5/5 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/fetch/fetch.go` | 99 | audit pass 2026-05-30 |
| 🔒 | `internal/fetch/fetch_test.go` | 114 | audit pass 2026-05-30 |
| 🔒 | `internal/fetch/firecrawl.go` | 94 | audit pass 2026-05-30 |
| 🔒 | `internal/fetch/jina.go` | 54 | audit pass 2026-05-30 |
| 🔒 | `internal/fetch/raw.go` | 46 | audit pass 2026-05-30 |

### internal/i18n (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/i18n/i18n.go` | 97 | audit pass 2026-05-30 |
| 🔒 | `internal/i18n/i18n_test.go` | 55 | audit pass 2026-05-30 |

### internal/ingest (3/3 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/ingest/ingest.go` | 143 | audit pass 2026-05-30 |
| 🔒 | `internal/ingest/sanitize.go` | 76 | audit pass 2026-05-30 |
| 🔒 | `internal/ingest/score.go` | 99 | audit pass 2026-05-30 |

### internal/kiromodels (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/kiromodels/kiromodels.go` | 231 | audit pass 2026-05-30 |
| 🔒 | `internal/kiromodels/kiromodels_test.go` | 156 | audit pass 2026-05-30 |

### internal/localai (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/localai/runtime.go` | 147 | audit pass 2026-05-30 |

### internal/mcpcatalog (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/mcpcatalog/catalog.go` | 125 | audit pass 2026-05-30 |
| 🔒 | `internal/mcpcatalog/catalog_test.go` | 145 | audit pass 2026-05-30 |

### internal/mcpsecurity (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/mcpsecurity/allowlist.go` | 125 | audit pass 2026-05-30 |
| 🔒 | `internal/mcpsecurity/allowlist_test.go` | 143 | audit pass 2026-05-30 |

### internal/mesh (9/9 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/mesh/blocklist.go` | 61 | audit pass 2026-05-30 |
| 🔒 | `internal/mesh/crdt.go` | 74 | audit pass 2026-05-30 |
| 🔒 | `internal/mesh/discovery.go` | 213 | audit pass 2026-05-30 |
| 🔒 | `internal/mesh/gossip.go` | 201 | audit pass 2026-05-30 |
| 🔒 | `internal/mesh/identity.go` | 138 | audit pass 2026-05-30 |
| 🔒 | `internal/mesh/karma_toolshare_filter.go` | 272 | audit pass 2026-05-30 |
| 🔒 | `internal/mesh/knowledge.go` | 129 | audit pass 2026-05-30 |
| 🔒 | `internal/mesh/packet.go` | 203 | audit pass 2026-05-30 |
| 🔒 | `internal/mesh/peers.go` | 160 | audit pass 2026-05-30 |

### internal/mitm (20/20 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/mitm/antigravity_ide_version.go` | 30 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/cert.go` | 215 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/cert_install.go` | 134 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/cert_test.go` | 89 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/config.go` | 91 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/dbreader.go` | 88 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/dns_config.go` | 193 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/dns_config_test.go` | 78 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/handlers/antigravity.go` | 100 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/handlers/base.go` | 56 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/handlers/copilot.go` | 26 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/handlers/cursor.go` | 28 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/handlers/kiro.go` | 26 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/listener.go` | 93 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/listener_test.go` | 84 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/logger.go` | 90 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/manager.go` | 117 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/paths.go` | 55 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/syscall_helper.go` | 15 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/win_elevated.go` | 45 | audit pass 2026-05-30 |

### internal/modelpool (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/modelpool/modelpool.go` | 275 | audit pass 2026-05-30 |

### internal/piistrip (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/piistrip/piistrip.go` | 231 | audit pass 2026-05-30 |

### internal/policy (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/policy/evaluator.go` | 197 | audit pass 2026-05-30 |

### internal/pricing (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/pricing/calc.go` | 73 | audit pass 2026-05-30 |

### internal/promptguard (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/promptguard/promptguard.go` | 263 | audit pass 2026-05-30 |

### internal/provider (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/provider/chain.go` | 190 | audit pass 2026-05-30 |

### internal/providercompat (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/providercompat/providercompat.go` | 107 | audit pass 2026-05-30 |
| 🔒 | `internal/providercompat/providercompat_test.go` | 122 | audit pass 2026-05-30 |

### internal/providers (37/37 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/providers/embedding/embedding.go` | 125 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/embedding/embedding_test.go` | 19 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/embedding/gemini.go` | 73 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/embedding/openai.go` | 46 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/embedding/openai_compat.go` | 48 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/black_forest_labs.go` | 57 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/cloudflare_ai.go` | 45 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/codex.go` | 47 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/comfyui.go` | 46 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/fal_ai.go` | 45 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/gemini.go` | 43 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/huggingface.go` | 45 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/image.go` | 79 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/image_test.go` | 42 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/nanobanana.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/openai.go` | 99 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/runwayml.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/sd_webui.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/stability_ai.go` | 45 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/stt/assemblyai.go` | 121 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/stt/deepgram.go` | 86 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/stt/gemini.go` | 100 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/stt/openai_whisper.go` | 90 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/stt/stt.go` | 158 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/stt/stt_test.go` | 163 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/deepgram.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/edge_tts.go` | 42 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/elevenlabs.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/gemini.go` | 52 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/google_tts.go` | 47 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/inworld.go` | 43 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/local_device.go` | 29 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/minimax.go` | 53 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/openai.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/openrouter.go` | 46 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/tts.go` | 105 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/tts_test.go` | 25 | audit pass 2026-05-30 |

### internal/quality (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/quality/quality.go` | 217 | audit pass 2026-05-30 |

### internal/quotalive (12/12 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/quotalive/antigravity.go` | 107 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/claude.go` | 134 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/codex.go` | 285 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/codex_review_test.go` | 190 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/copilot.go` | 123 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/gemini_cli.go` | 104 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/glm.go` | 100 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/informational.go` | 55 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/kiro.go` | 105 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/minimax.go` | 110 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/quotalive.go` | 93 | audit pass 2026-05-30 |
| 🔒 | `internal/quotalive/quotalive_test.go` | 161 | audit pass 2026-05-30 |

### internal/recorder (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/recorder/recorder.go` | 267 | audit pass 2026-05-30 |

### internal/router (32/32 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/router/authctx.go` | 85 | audit pass 2026-05-30 |
| 🔒 | `internal/router/brain_always_on_test.go` | 108 | audit pass 2026-05-30 |
| 🔒 | `internal/router/brain_constitution.go` | 85 | audit pass 2026-05-30 |
| 🔒 | `internal/router/brain_e2e_test.go` | 113 | audit pass 2026-05-30 |
| 🔒 | `internal/router/brain_ollama_test.go` | 79 | audit pass 2026-05-30 |
| 🔒 | `internal/router/brainenrich.go` | 225 | audit pass 2026-05-30 |
| 🔒 | `internal/router/brainenrich_test.go` | 102 | audit pass 2026-05-30 |
| 🔒 | `internal/router/caveman_inject.go` | 29 | audit pass 2026-05-30 |
| 🔒 | `internal/router/caveman_inject_test.go` | 82 | audit pass 2026-05-30 |
| 🔒 | `internal/router/combo_fallback_test.go` | 72 | audit pass 2026-05-30 |
| 🔒 | `internal/router/cost_intent.go` | 120 | audit pass 2026-05-30 |
| 🔒 | `internal/router/cost_intent_test.go` | 131 | audit pass 2026-05-30 |
| 🔒 | `internal/router/dispatcher.go` | 670 | audit pass 2026-05-30 |
| 🔒 | `internal/router/dispatcher_stream.go` | 491 | audit pass 2026-05-30 |
| 🔒 | `internal/router/gemini.go` | 253 | audit pass 2026-05-30 |
| 🔒 | `internal/router/intent.go` | 60 | audit pass 2026-05-30 |
| 🔒 | `internal/router/modelresolve.go` | 69 | audit pass 2026-05-30 |
| 🔒 | `internal/router/optional_params_test.go` | 154 | audit pass 2026-05-30 |
| 🔒 | `internal/router/pickcombo_race_test.go` | 37 | audit pass 2026-05-30 |
| 🔒 | `internal/router/preprocess_content.go` | 103 | audit pass 2026-05-30 |
| 🔒 | `internal/router/preprocess_content_test.go` | 134 | audit pass 2026-05-30 |
| 🔒 | `internal/router/proxy.go` | 113 | audit pass 2026-05-30 |
| 🔒 | `internal/router/responses_stream_to_json.go` | 139 | audit pass 2026-05-30 |
| 🔒 | `internal/router/responses_stream_to_json_test.go` | 142 | audit pass 2026-05-30 |
| 🔒 | `internal/router/responses_transform.go` | 345 | audit pass 2026-05-30 |
| 🔒 | `internal/router/responses_transform_test.go` | 221 | audit pass 2026-05-30 |
| 🔒 | `internal/router/rtk.go` | 35 | audit pass 2026-05-30 |
| 🔒 | `internal/router/sse_to_json.go` | 184 | audit pass 2026-05-30 |
| 🔒 | `internal/router/sse_to_json_test.go` | 150 | audit pass 2026-05-30 |
| 🔒 | `internal/router/strategy.go` | 56 | audit pass 2026-05-30 |
| 🔒 | `internal/router/toolcall_preprocess.go` | 74 | audit pass 2026-05-30 |
| 🔒 | `internal/router/tools.go` | 466 | audit pass 2026-05-30 |

### internal/rtk (16/16 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/rtk/autodetect.go` | 207 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/autodetect_test.go` | 110 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/buildoutput.go` | 73 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/common.go` | 18 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/deduplog.go` | 69 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/find.go` | 52 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/gitdiff.go` | 49 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/gitstatus.go` | 55 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/grep.go` | 54 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/ls.go` | 45 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/readnumbered.go` | 50 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/searchlist.go` | 36 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/smarttruncate.go` | 40 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/tree.go` | 36 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/rtk.go` | 101 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/rtk_test.go` | 90 | audit pass 2026-05-30 |

### internal/safego (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/safego/safego.go` | 46 | audit pass 2026-05-30 |
| 🔒 | `internal/safego/safego_test.go` | 72 | audit pass 2026-05-30 |

### internal/safeurl (2/2 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/safeurl/safeurl.go` | 100 | audit pass 2026-05-30 |
| 🔒 | `internal/safeurl/safeurl_test.go` | 121 | audit pass 2026-05-30 |

### internal/search (6/6 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/search/brave.go` | 57 | audit pass 2026-05-30 |
| 🔒 | `internal/search/duckduckgo.go` | 67 | audit pass 2026-05-30 |
| 🔒 | `internal/search/search.go` | 113 | audit pass 2026-05-30 |
| 🔒 | `internal/search/search_test.go` | 22 | audit pass 2026-05-30 |
| 🔒 | `internal/search/serpapi.go` | 55 | audit pass 2026-05-30 |
| 🔒 | `internal/search/tavily.go` | 54 | audit pass 2026-05-30 |

### internal/sensors (1/1 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/sensors/sensors.go` | 93 | audit pass 2026-05-30 |

### internal/services (5/5 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/services/account_fallback.go` | 151 | audit pass 2026-05-30 |
| 🔒 | `internal/services/error_rules_test.go` | 77 | audit pass 2026-05-30 |
| 🔒 | `internal/services/refresh_lead_test.go` | 69 | audit pass 2026-05-30 |
| 🔒 | `internal/services/services_test.go` | 141 | audit pass 2026-05-30 |
| 🔒 | `internal/services/token_refresh.go` | 165 | audit pass 2026-05-30 |

### internal/store (28/28 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/store/apikeys.go` | 154 | audit pass 2026-05-30 |
| 🔒 | `internal/store/authsessions.go` | 99 | audit pass 2026-05-30 |
| 🔒 | `internal/store/backup.go` | 179 | audit pass 2026-05-30 |
| 🔒 | `internal/store/backup_test.go` | 69 | audit pass 2026-05-30 |
| 🔒 | `internal/store/braincontrib.go` | 86 | audit pass 2026-05-30 |
| 🔒 | `internal/store/combos.go` | 94 | audit pass 2026-05-30 |
| 🔒 | `internal/store/kvmisc.go` | 227 | audit pass 2026-05-30 |
| 🔒 | `internal/store/llm_pricing_policy_migrations.go` | 95 | audit pass 2026-05-30 |
| 🔒 | `internal/store/media.go` | 91 | audit pass 2026-05-30 |
| 🔒 | `internal/store/mesh_migrations.go` | 41 | audit pass 2026-05-30 |
| 🔒 | `internal/store/mesh_stack_migrations.go` | 120 | audit pass 2026-05-30 |
| 🔒 | `internal/store/migrate.go` | 150 | audit pass 2026-05-30 |
| 🔒 | `internal/store/migrate_test.go` | 77 | audit pass 2026-05-30 |
| 🔒 | `internal/store/modelmeta.go` | 224 | audit pass 2026-05-30 |
| 🔒 | `internal/store/presets.go` | 702 | audit pass 2026-05-30 |
| 🔒 | `internal/store/pricing.go` | 175 | audit pass 2026-05-30 |
| 🔒 | `internal/store/providers.go` | 324 | audit pass 2026-05-30 |
| 🔒 | `internal/store/proxypools.go` | 80 | audit pass 2026-05-30 |
| 🔒 | `internal/store/quota.go` | 123 | audit pass 2026-05-30 |
| 🔒 | `internal/store/requestlog.go` | 239 | audit pass 2026-05-30 |
| 🔒 | `internal/store/secret.go` | 96 | audit pass 2026-05-30 |
| 🔒 | `internal/store/settings.go` | 337 | audit pass 2026-05-30 |
| 🔒 | `internal/store/skills.go` | 149 | audit pass 2026-05-30 |
| 🔒 | `internal/store/sqlite.go` | 308 | audit pass 2026-05-30 |
| 🔒 | `internal/store/sync.go` | 131 | audit pass 2026-05-30 |
| 🔒 | `internal/store/tags.go` | 68 | audit pass 2026-05-30 |
| 🔒 | `internal/store/testhelpers.go` | 22 | audit pass 2026-05-30 |
| 🔒 | `internal/store/translator.go` | 89 | audit pass 2026-05-30 |

### internal/streamutil (11/11 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/streamutil/claude_header_cache.go` | 96 | audit pass 2026-05-30 |
| 🔒 | `internal/streamutil/reasoning_injector.go` | 98 | audit pass 2026-05-30 |
| 🔒 | `internal/streamutil/session_manager.go` | 85 | audit pass 2026-05-30 |
| 🔒 | `internal/streamutil/sse_helpers.go` | 192 | audit pass 2026-05-30 |
| 🔒 | `internal/streamutil/sse_helpers_test.go` | 285 | audit pass 2026-05-30 |
| 🔒 | `internal/streamutil/stall_reader.go` | 103 | audit pass 2026-05-30 |
| 🔒 | `internal/streamutil/stall_reader_test.go` | 188 | audit pass 2026-05-30 |
| 🔒 | `internal/streamutil/streamutil_test.go` | 127 | audit pass 2026-05-30 |
| 🔒 | `internal/streamutil/tool_deduper.go` | 116 | audit pass 2026-05-30 |
| 🔒 | `internal/streamutil/usage_tracking.go` | 246 | audit pass 2026-05-30 |
| 🔒 | `internal/streamutil/usage_tracking_test.go` | 270 | audit pass 2026-05-30 |

### internal/translator (40/40 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/translator/helpers/claude.go` | 71 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/gemini.go` | 102 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/helpers_test.go` | 241 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/image.go` | 107 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/max_tokens.go` | 91 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/max_tokens_adjust_test.go` | 66 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/openai.go` | 64 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/reasoning_inject.go` | 135 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/reasoning_inject_test.go` | 129 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/responses_api.go` | 285 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/responses_api_convert_test.go` | 176 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/tool_call.go` | 362 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/tool_call_validate_test.go` | 232 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/tool_deduper.go` | 169 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/helpers/tool_deduper_test.go` | 167 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/registry.go` | 76 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/antigravity_to_openai.go` | 31 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/claude_to_openai.go` | 47 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/gemini_to_openai.go` | 64 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/openai_responses.go` | 51 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/openai_to_claude.go` | 83 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/openai_to_commandcode.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/openai_to_cursor.go` | 30 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/openai_to_gemini.go` | 61 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/openai_to_kiro.go` | 56 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/openai_to_ollama.go` | 42 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/openai_to_vertex.go` | 74 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/claude_cache_test.go` | 123 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/claude_to_openai.go` | 125 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/commandcode_to_openai.go` | 42 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/cursor_to_openai.go` | 29 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/gemini_to_openai.go` | 70 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/kiro_to_openai.go` | 42 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/ollama_stream.go` | 183 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/ollama_stream_test.go` | 141 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/ollama_to_openai.go` | 47 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/openai_responses.go` | 48 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/openai_to_antigravity.go` | 58 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/openai_to_claude.go` | 52 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/translator_test.go` | 110 | audit pass 2026-05-30 |

### internal/updater (4/4 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/updater/restart_unix.go` | 20 | audit pass 2026-05-30 |
| 🔒 | `internal/updater/restart_windows.go` | 29 | audit pass 2026-05-30 |
| 🔒 | `internal/updater/updater.go` | 222 | audit pass 2026-05-30 |
| 🔒 | `internal/updater/updater_test.go` | 55 | audit pass 2026-05-30 |

### root (5/5 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `login_limiter.go` | 120 | audit pass 2026-05-30 |
| 🔒 | `login_limiter_test.go` | 94 | audit pass 2026-05-30 |
| 🔒 | `main.go` | 169 | audit pass 2026-05-30 |
| 🔒 | `routes.go` | 287 | audit pass 2026-05-30 |
| 🔒 | `tunnel_watchdog.go` | 111 | audit pass 2026-05-30 |

### root/handlers (59/59 locked)

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `handlers_apikey_auth.go` | 162 | audit pass 2026-05-30 |
| 🔒 | `handlers_auth.go` | 261 | audit pass 2026-05-30 |
| 🔒 | `handlers_auth_oidc.go` | 301 | audit pass 2026-05-30 |
| 🔒 | `handlers_backup.go` | 43 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain.go` | 277 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_ingest.go` | 119 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_injection.go` | 49 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_mistakes.go` | 94 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_models.go` | 151 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_pii.go` | 67 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_proposals.go` | 137 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_quality.go` | 57 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_rescore.go` | 81 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_skills.go` | 102 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_tools.go` | 96 | audit pass 2026-05-30 |
| 🔒 | `handlers_brain_views.go` | 296 | audit pass 2026-05-30 |
| 🔒 | `handlers_bypass.go` | 131 | audit pass 2026-05-30 |
| 🔒 | `handlers_chat.go` | 145 | audit pass 2026-05-30 |
| 🔒 | `handlers_chat_v1.go` | 840 | audit pass 2026-05-30 |
| 🔒 | `handlers_cli_tools_ext.go` | 212 | audit pass 2026-05-30 |
| 🔒 | `handlers_fetch.go` | 127 | audit pass 2026-05-30 |
| 🔒 | `handlers_gaps.go` | 372 | audit pass 2026-05-30 |
| 🔒 | `handlers_kiromodels.go` | 54 | audit pass 2026-05-30 |
| 🔒 | `handlers_llm_policy.go` | 450 | audit pass 2026-05-30 |
| 🔒 | `handlers_llm_runtime.go` | 205 | audit pass 2026-05-30 |
| 🔒 | `handlers_locale.go` | 147 | audit pass 2026-05-30 |
| 🔒 | `handlers_mcp.go` | 463 | audit pass 2026-05-30 |
| 🔒 | `handlers_mcp_catalog.go` | 30 | audit pass 2026-05-30 |
| 🔒 | `handlers_media_ext.go` | 114 | audit pass 2026-05-30 |
| 🔒 | `handlers_media_tts_voices.go` | 478 | audit pass 2026-05-30 |
| 🔒 | `handlers_media_tts_voices_test.go` | 163 | audit pass 2026-05-30 |
| 🔒 | `handlers_mesh.go` | 140 | audit pass 2026-05-30 |
| 🔒 | `handlers_mesh_advanced.go` | 456 | audit pass 2026-05-30 |
| 🔒 | `handlers_mesh_stack.go` | 61 | audit pass 2026-05-30 |
| 🔒 | `handlers_mesh_transport.go` | 179 | audit pass 2026-05-30 |
| 🔒 | `handlers_mitm_ext.go` | 157 | audit pass 2026-05-30 |
| 🔒 | `handlers_mitm_proxy.go` | 142 | audit pass 2026-05-30 |
| 🔒 | `handlers_models_meta.go` | 306 | audit pass 2026-05-30 |
| 🔒 | `handlers_oauth.go` | 528 | audit pass 2026-05-30 |
| 🔒 | `handlers_oauth_device.go` | 180 | audit pass 2026-05-30 |
| 🔒 | `handlers_obs.go` | 151 | audit pass 2026-05-30 |
| 🔒 | `handlers_oidc_jwt.go` | 152 | audit pass 2026-05-30 |
| 🔒 | `handlers_pricing.go` | 87 | audit pass 2026-05-30 |
| 🔒 | `handlers_provider_nodes.go` | 153 | audit pass 2026-05-30 |
| 🔒 | `handlers_providers_ext.go` | 310 | audit pass 2026-05-30 |
| 🔒 | `handlers_proxy_deploy.go` | 194 | audit pass 2026-05-30 |
| 🔒 | `handlers_quotalive.go` | 97 | audit pass 2026-05-30 |
| 🔒 | `handlers_recordings.go` | 146 | audit pass 2026-05-30 |
| 🔒 | `handlers_resources.go` | 439 | audit pass 2026-05-30 |
| 🔒 | `handlers_sensors_webhook.go` | 107 | audit pass 2026-05-30 |
| 🔒 | `handlers_settings_sub.go` | 163 | audit pass 2026-05-30 |
| 🔒 | `handlers_skills_invoke.go` | 144 | audit pass 2026-05-30 |
| 🔒 | `handlers_stt.go` | 151 | audit pass 2026-05-30 |
| 🔒 | `handlers_sync.go` | 94 | audit pass 2026-05-30 |
| 🔒 | `handlers_tags.go` | 76 | audit pass 2026-05-30 |
| 🔒 | `handlers_translator.go` | 539 | audit pass 2026-05-30 |
| 🔒 | `handlers_tunnel.go` | 335 | audit pass 2026-05-30 |
| 🔒 | `handlers_usage_breakdown.go` | 393 | audit pass 2026-05-30 |
| 🔒 | `handlers_util.go` | 34 | audit pass 2026-05-30 |
