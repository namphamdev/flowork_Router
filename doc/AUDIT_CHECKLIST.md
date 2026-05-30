# Flowork Router — AUDIT CHECKLIST

> Auto-generated 2026-05-30 per Mr.Dev mandate "audit setiap file, cari bug, perbaiki, lock".
> Status: 🔒 LOCKED + verified · ⏳ pending audit.

## Inventory

- **Go files**: 396 total
- **Locked**: 97 (24%)
- **Pending**: 299

## File-by-file checklist

### cmd

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `cmd/flow-cli/api/client.go` | 100 | pending |
| ⏳ | `cmd/flow-cli/autostart.go` | 188 | pending |
| ⏳ | `cmd/flow-cli/cli_test.go` | 102 | pending |
| ⏳ | `cmd/flow-cli/main.go` | 241 | pending |
| ⏳ | `cmd/flow-cli/menus/apikeys.go` | 108 | pending |
| ⏳ | `cmd/flow-cli/menus/clitools.go` | 41 | pending |
| ⏳ | `cmd/flow-cli/menus/combos.go` | 107 | pending |
| ⏳ | `cmd/flow-cli/menus/providers.go` | 140 | pending |
| ⏳ | `cmd/flow-cli/menus/settings.go` | 46 | pending |
| 🔒 | `cmd/flow-cli/terminal_ui.go` | 29 | audit pass 2026-05-30 |
| ⏳ | `cmd/flow-cli/tray.go` | 68 | pending |
| ⏳ | `cmd/flow-cli/utils/clipboard.go` | 46 | pending |
| ⏳ | `cmd/flow-cli/utils/display.go` | 80 | pending |
| 🔒 | `cmd/flow-cli/utils/endpoint.go` | 41 | audit pass 2026-05-30 |
| ⏳ | `cmd/flow-cli/utils/format.go` | 56 | pending |
| ⏳ | `cmd/flow-cli/utils/input.go` | 87 | pending |
| 🔒 | `cmd/flow-cli/utils/menuhelper.go` | 37 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-cli/utils/model_selector.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `cmd/flow-tray/main.go` | 45 | audit pass 2026-05-30 |
| ⏳ | `cmd/flow-tray/tray_native.go` | 66 | pending |

### internal/brain

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/brain/brain.go` | 126 | pending |
| ⏳ | `internal/brain/crud.go` | 153 | pending |
| ⏳ | `internal/brain/explore.go` | 106 | pending |
| ⏳ | `internal/brain/fts.go` | 46 | pending |
| ⏳ | `internal/brain/init.go` | 233 | pending |
| 🔒 | `internal/brain/live_test.go` | 43 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/mistakes.go` | 233 | audit pass 2026-05-30 |
| 🔒 | `internal/brain/rescore.go` | 235 | audit pass 2026-05-30 |
| ⏳ | `internal/brain/retrieve.go` | 128 | pending |
| ⏳ | `internal/brain/skills.go` | 131 | pending |
| 🔒 | `internal/brain/skills_test.go` | 42 | audit pass 2026-05-30 |
| ⏳ | `internal/brain/stats.go` | 55 | pending |
| 🔒 | `internal/brain/tool_patterns.go` | 174 | audit pass 2026-05-30 |
| ⏳ | `internal/brain/views.go` | 71 | pending |
| ⏳ | `internal/brain/write.go` | 259 | pending |

### internal/bypass

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/bypass/bypass.go` | 144 | pending |
| ⏳ | `internal/bypass/bypass_test.go` | 111 | pending |

### internal/caveman

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/caveman/caveman.go` | 80 | pending |
| ⏳ | `internal/caveman/caveman_test.go` | 72 | pending |

### internal/clitools

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/clitools/custom.go` | 364 | pending |
| ⏳ | `internal/clitools/detect.go` | 507 | pending |
| ⏳ | `internal/clitools/registry.go` | 196 | pending |

### internal/cloudcode

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/cloudcode/projectid.go` | 246 | pending |
| ⏳ | `internal/cloudcode/projectid_test.go` | 211 | pending |

### internal/constitution

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/constitution/proposals.go` | 271 | audit pass 2026-05-30 |

### internal/creds

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/creds/credentials.go` | 104 | pending |
| ⏳ | `internal/creds/imports.go` | 204 | pending |

### internal/executors

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/executors/antigravity.go` | 99 | pending |
| ⏳ | `internal/executors/antigravity_session.go` | 131 | pending |
| ⏳ | `internal/executors/antigravity_session_test.go` | 142 | pending |
| ⏳ | `internal/executors/azure.go` | 66 | pending |
| ⏳ | `internal/executors/base.go` | 176 | pending |
| ⏳ | `internal/executors/codex.go` | 88 | pending |
| ⏳ | `internal/executors/codex_instructions.go` | 220 | pending |
| ⏳ | `internal/executors/codex_instructions_test.go` | 98 | pending |
| ⏳ | `internal/executors/commandcode.go` | 60 | pending |
| ⏳ | `internal/executors/cursor.go` | 82 | pending |
| ⏳ | `internal/executors/cursor_checksum.go` | 179 | pending |
| ⏳ | `internal/executors/cursor_checksum_test.go` | 146 | pending |
| ⏳ | `internal/executors/cursor_proto.go` | 284 | pending |
| ⏳ | `internal/executors/cursor_proto_test.go` | 160 | pending |
| ⏳ | `internal/executors/cursor_protobuf_executor.go` | 239 | pending |
| ⏳ | `internal/executors/default.go` | 50 | pending |
| ⏳ | `internal/executors/executor.go` | 81 | pending |
| ⏳ | `internal/executors/executors_test.go` | 116 | pending |
| ⏳ | `internal/executors/gemini_cli.go` | 76 | pending |
| ⏳ | `internal/executors/github.go` | 74 | pending |
| ⏳ | `internal/executors/grok_web.go` | 97 | pending |
| ⏳ | `internal/executors/iflow.go` | 72 | pending |
| ⏳ | `internal/executors/jetbrains_ai.go` | 56 | pending |
| ⏳ | `internal/executors/kiro.go` | 100 | pending |
| ⏳ | `internal/executors/kiro_suffix.go` | 98 | pending |
| ⏳ | `internal/executors/kiro_suffix_test.go` | 101 | pending |
| ⏳ | `internal/executors/ollama_local.go` | 41 | pending |
| ⏳ | `internal/executors/opencode.go` | 49 | pending |
| ⏳ | `internal/executors/opencode_go.go` | 49 | pending |
| ⏳ | `internal/executors/perplexity_web.go` | 68 | pending |
| ⏳ | `internal/executors/qoder.go` | 49 | pending |
| ⏳ | `internal/executors/qwen.go` | 56 | pending |
| ⏳ | `internal/executors/vertex.go` | 93 | pending |

### internal/fetch

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/fetch/fetch.go` | 92 | pending |
| ⏳ | `internal/fetch/fetch_test.go` | 107 | pending |
| ⏳ | `internal/fetch/firecrawl.go` | 87 | pending |
| ⏳ | `internal/fetch/jina.go` | 47 | pending |
| ⏳ | `internal/fetch/raw.go` | 39 | pending |

### internal/i18n

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/i18n/i18n.go` | 90 | pending |
| ⏳ | `internal/i18n/i18n_test.go` | 48 | pending |

### internal/ingest

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/ingest/ingest.go` | 143 | audit pass 2026-05-30 |
| 🔒 | `internal/ingest/sanitize.go` | 76 | audit pass 2026-05-30 |
| 🔒 | `internal/ingest/score.go` | 99 | audit pass 2026-05-30 |

### internal/kiromodels

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/kiromodels/kiromodels.go` | 224 | pending |
| ⏳ | `internal/kiromodels/kiromodels_test.go` | 149 | pending |

### internal/localai

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/localai/runtime.go` | 147 | audit pass 2026-05-30 |

### internal/mcpcatalog

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/mcpcatalog/catalog.go` | 118 | pending |
| ⏳ | `internal/mcpcatalog/catalog_test.go` | 138 | pending |

### internal/mcpsecurity

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/mcpsecurity/allowlist.go` | 118 | pending |
| ⏳ | `internal/mcpsecurity/allowlist_test.go` | 136 | pending |

### internal/mesh

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

### internal/mitm

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/mitm/antigravity_ide_version.go` | 30 | audit pass 2026-05-30 |
| ⏳ | `internal/mitm/cert.go` | 208 | pending |
| ⏳ | `internal/mitm/cert_install.go` | 127 | pending |
| ⏳ | `internal/mitm/cert_test.go` | 82 | pending |
| ⏳ | `internal/mitm/config.go` | 84 | pending |
| ⏳ | `internal/mitm/dbreader.go` | 81 | pending |
| ⏳ | `internal/mitm/dns_config.go` | 186 | pending |
| ⏳ | `internal/mitm/dns_config_test.go` | 71 | pending |
| ⏳ | `internal/mitm/handlers/antigravity.go` | 93 | pending |
| ⏳ | `internal/mitm/handlers/base.go` | 49 | pending |
| 🔒 | `internal/mitm/handlers/copilot.go` | 26 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/handlers/cursor.go` | 28 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/handlers/kiro.go` | 26 | audit pass 2026-05-30 |
| ⏳ | `internal/mitm/listener.go` | 86 | pending |
| ⏳ | `internal/mitm/listener_test.go` | 77 | pending |
| ⏳ | `internal/mitm/logger.go` | 83 | pending |
| ⏳ | `internal/mitm/manager.go` | 110 | pending |
| ⏳ | `internal/mitm/paths.go` | 48 | pending |
| 🔒 | `internal/mitm/syscall_helper.go` | 15 | audit pass 2026-05-30 |
| 🔒 | `internal/mitm/win_elevated.go` | 45 | audit pass 2026-05-30 |

### internal/modelpool

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/modelpool/modelpool.go` | 275 | audit pass 2026-05-30 |

### internal/piistrip

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/piistrip/piistrip.go` | 231 | audit pass 2026-05-30 |

### internal/policy

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/policy/evaluator.go` | 197 | audit pass 2026-05-30 |

### internal/pricing

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/pricing/calc.go` | 73 | audit pass 2026-05-30 |

### internal/promptguard

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/promptguard/promptguard.go` | 263 | audit pass 2026-05-30 |

### internal/provider

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/provider/chain.go` | 190 | audit pass 2026-05-30 |

### internal/providercompat

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/providercompat/providercompat.go` | 100 | pending |
| ⏳ | `internal/providercompat/providercompat_test.go` | 115 | pending |

### internal/providers

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/providers/embedding/embedding.go` | 118 | pending |
| 🔒 | `internal/providers/embedding/embedding_test.go` | 19 | audit pass 2026-05-30 |
| ⏳ | `internal/providers/embedding/gemini.go` | 66 | pending |
| ⏳ | `internal/providers/embedding/openai.go` | 39 | pending |
| ⏳ | `internal/providers/embedding/openai_compat.go` | 41 | pending |
| ⏳ | `internal/providers/image/black_forest_labs.go` | 50 | pending |
| 🔒 | `internal/providers/image/cloudflare_ai.go` | 45 | audit pass 2026-05-30 |
| ⏳ | `internal/providers/image/codex.go` | 40 | pending |
| ⏳ | `internal/providers/image/comfyui.go` | 39 | pending |
| 🔒 | `internal/providers/image/fal_ai.go` | 45 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/gemini.go` | 43 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/huggingface.go` | 45 | audit pass 2026-05-30 |
| ⏳ | `internal/providers/image/image.go` | 72 | pending |
| 🔒 | `internal/providers/image/image_test.go` | 42 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/nanobanana.go` | 44 | audit pass 2026-05-30 |
| ⏳ | `internal/providers/image/openai.go` | 92 | pending |
| 🔒 | `internal/providers/image/runwayml.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/image/sd_webui.go` | 44 | audit pass 2026-05-30 |
| ⏳ | `internal/providers/image/stability_ai.go` | 38 | pending |
| ⏳ | `internal/providers/stt/assemblyai.go` | 114 | pending |
| ⏳ | `internal/providers/stt/deepgram.go` | 79 | pending |
| ⏳ | `internal/providers/stt/gemini.go` | 93 | pending |
| ⏳ | `internal/providers/stt/openai_whisper.go` | 83 | pending |
| ⏳ | `internal/providers/stt/stt.go` | 151 | pending |
| ⏳ | `internal/providers/stt/stt_test.go` | 156 | pending |
| 🔒 | `internal/providers/tts/deepgram.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/edge_tts.go` | 42 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/elevenlabs.go` | 44 | audit pass 2026-05-30 |
| ⏳ | `internal/providers/tts/gemini.go` | 45 | pending |
| ⏳ | `internal/providers/tts/google_tts.go` | 40 | pending |
| 🔒 | `internal/providers/tts/inworld.go` | 43 | audit pass 2026-05-30 |
| 🔒 | `internal/providers/tts/local_device.go` | 29 | audit pass 2026-05-30 |
| ⏳ | `internal/providers/tts/minimax.go` | 46 | pending |
| 🔒 | `internal/providers/tts/openai.go` | 44 | audit pass 2026-05-30 |
| ⏳ | `internal/providers/tts/openrouter.go` | 39 | pending |
| ⏳ | `internal/providers/tts/tts.go` | 98 | pending |
| 🔒 | `internal/providers/tts/tts_test.go` | 25 | audit pass 2026-05-30 |

### internal/quality

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/quality/quality.go` | 217 | audit pass 2026-05-30 |

### internal/quotalive

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/quotalive/antigravity.go` | 100 | pending |
| ⏳ | `internal/quotalive/claude.go` | 127 | pending |
| ⏳ | `internal/quotalive/codex.go` | 278 | pending |
| ⏳ | `internal/quotalive/codex_review_test.go` | 183 | pending |
| ⏳ | `internal/quotalive/copilot.go` | 116 | pending |
| ⏳ | `internal/quotalive/gemini_cli.go` | 97 | pending |
| ⏳ | `internal/quotalive/glm.go` | 93 | pending |
| ⏳ | `internal/quotalive/informational.go` | 48 | pending |
| ⏳ | `internal/quotalive/kiro.go` | 98 | pending |
| ⏳ | `internal/quotalive/minimax.go` | 103 | pending |
| ⏳ | `internal/quotalive/quotalive.go` | 86 | pending |
| ⏳ | `internal/quotalive/quotalive_test.go` | 154 | pending |

### internal/recorder

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/recorder/recorder.go` | 267 | audit pass 2026-05-30 |

### internal/router

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/router/authctx.go` | 78 | pending |
| ⏳ | `internal/router/brain_always_on_test.go` | 101 | pending |
| ⏳ | `internal/router/brain_constitution.go` | 78 | pending |
| ⏳ | `internal/router/brain_e2e_test.go` | 106 | pending |
| ⏳ | `internal/router/brain_ollama_test.go` | 72 | pending |
| ⏳ | `internal/router/brainenrich.go` | 218 | pending |
| ⏳ | `internal/router/brainenrich_test.go` | 95 | pending |
| 🔒 | `internal/router/caveman_inject.go` | 29 | audit pass 2026-05-30 |
| ⏳ | `internal/router/caveman_inject_test.go` | 75 | pending |
| ⏳ | `internal/router/combo_fallback_test.go` | 65 | pending |
| ⏳ | `internal/router/cost_intent.go` | 113 | pending |
| ⏳ | `internal/router/cost_intent_test.go` | 124 | pending |
| ⏳ | `internal/router/dispatcher.go` | 663 | pending |
| ⏳ | `internal/router/dispatcher_stream.go` | 484 | pending |
| ⏳ | `internal/router/gemini.go` | 246 | pending |
| ⏳ | `internal/router/intent.go` | 53 | pending |
| ⏳ | `internal/router/modelresolve.go` | 62 | pending |
| ⏳ | `internal/router/optional_params_test.go` | 147 | pending |
| 🔒 | `internal/router/pickcombo_race_test.go` | 37 | audit pass 2026-05-30 |
| ⏳ | `internal/router/preprocess_content.go` | 96 | pending |
| ⏳ | `internal/router/preprocess_content_test.go` | 127 | pending |
| ⏳ | `internal/router/proxy.go` | 106 | pending |
| ⏳ | `internal/router/responses_stream_to_json.go` | 132 | pending |
| ⏳ | `internal/router/responses_stream_to_json_test.go` | 135 | pending |
| ⏳ | `internal/router/responses_transform.go` | 338 | pending |
| ⏳ | `internal/router/responses_transform_test.go` | 214 | pending |
| 🔒 | `internal/router/rtk.go` | 35 | audit pass 2026-05-30 |
| ⏳ | `internal/router/sse_to_json.go` | 177 | pending |
| ⏳ | `internal/router/sse_to_json_test.go` | 143 | pending |
| ⏳ | `internal/router/strategy.go` | 49 | pending |
| ⏳ | `internal/router/toolcall_preprocess.go` | 67 | pending |
| ⏳ | `internal/router/tools.go` | 459 | pending |

### internal/rtk

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/rtk/autodetect.go` | 200 | pending |
| ⏳ | `internal/rtk/autodetect_test.go` | 103 | pending |
| ⏳ | `internal/rtk/filters/buildoutput.go` | 66 | pending |
| 🔒 | `internal/rtk/filters/common.go` | 18 | audit pass 2026-05-30 |
| ⏳ | `internal/rtk/filters/deduplog.go` | 62 | pending |
| ⏳ | `internal/rtk/filters/find.go` | 45 | pending |
| ⏳ | `internal/rtk/filters/gitdiff.go` | 42 | pending |
| ⏳ | `internal/rtk/filters/gitstatus.go` | 48 | pending |
| ⏳ | `internal/rtk/filters/grep.go` | 47 | pending |
| ⏳ | `internal/rtk/filters/ls.go` | 38 | pending |
| ⏳ | `internal/rtk/filters/readnumbered.go` | 43 | pending |
| 🔒 | `internal/rtk/filters/searchlist.go` | 36 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/smarttruncate.go` | 40 | audit pass 2026-05-30 |
| 🔒 | `internal/rtk/filters/tree.go` | 36 | audit pass 2026-05-30 |
| ⏳ | `internal/rtk/rtk.go` | 94 | pending |
| ⏳ | `internal/rtk/rtk_test.go` | 83 | pending |

### internal/safego

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/safego/safego.go` | 39 | pending |
| ⏳ | `internal/safego/safego_test.go` | 65 | pending |

### internal/safeurl

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/safeurl/safeurl.go` | 93 | pending |
| ⏳ | `internal/safeurl/safeurl_test.go` | 114 | pending |

### internal/search

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/search/brave.go` | 50 | pending |
| ⏳ | `internal/search/duckduckgo.go` | 60 | pending |
| ⏳ | `internal/search/search.go` | 106 | pending |
| 🔒 | `internal/search/search_test.go` | 22 | audit pass 2026-05-30 |
| ⏳ | `internal/search/serpapi.go` | 48 | pending |
| ⏳ | `internal/search/tavily.go` | 47 | pending |

### internal/sensors

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/sensors/sensors.go` | 93 | audit pass 2026-05-30 |

### internal/services

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/services/account_fallback.go` | 144 | pending |
| ⏳ | `internal/services/error_rules_test.go` | 70 | pending |
| ⏳ | `internal/services/refresh_lead_test.go` | 62 | pending |
| ⏳ | `internal/services/services_test.go` | 134 | pending |
| ⏳ | `internal/services/token_refresh.go` | 158 | pending |

### internal/store

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/store/apikeys.go` | 147 | pending |
| ⏳ | `internal/store/authsessions.go` | 92 | pending |
| ⏳ | `internal/store/backup.go` | 172 | pending |
| ⏳ | `internal/store/backup_test.go` | 62 | pending |
| ⏳ | `internal/store/braincontrib.go` | 79 | pending |
| ⏳ | `internal/store/combos.go` | 87 | pending |
| ⏳ | `internal/store/kvmisc.go` | 220 | pending |
| 🔒 | `internal/store/llm_pricing_policy_migrations.go` | 95 | audit pass 2026-05-30 |
| ⏳ | `internal/store/media.go` | 84 | pending |
| 🔒 | `internal/store/mesh_migrations.go` | 41 | audit pass 2026-05-30 |
| 🔒 | `internal/store/mesh_stack_migrations.go` | 120 | audit pass 2026-05-30 |
| ⏳ | `internal/store/migrate.go` | 143 | pending |
| ⏳ | `internal/store/migrate_test.go` | 70 | pending |
| ⏳ | `internal/store/modelmeta.go` | 217 | pending |
| ⏳ | `internal/store/presets.go` | 695 | pending |
| ⏳ | `internal/store/pricing.go` | 168 | pending |
| ⏳ | `internal/store/providers.go` | 317 | pending |
| ⏳ | `internal/store/proxypools.go` | 73 | pending |
| ⏳ | `internal/store/quota.go` | 116 | pending |
| ⏳ | `internal/store/requestlog.go` | 232 | pending |
| ⏳ | `internal/store/secret.go` | 89 | pending |
| ⏳ | `internal/store/settings.go` | 330 | pending |
| ⏳ | `internal/store/skills.go` | 142 | pending |
| ⏳ | `internal/store/sqlite.go` | 301 | pending |
| ⏳ | `internal/store/sync.go` | 124 | pending |
| ⏳ | `internal/store/tags.go` | 61 | pending |
| 🔒 | `internal/store/testhelpers.go` | 22 | audit pass 2026-05-30 |
| ⏳ | `internal/store/translator.go` | 82 | pending |

### internal/streamutil

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/streamutil/claude_header_cache.go` | 89 | pending |
| ⏳ | `internal/streamutil/reasoning_injector.go` | 91 | pending |
| ⏳ | `internal/streamutil/session_manager.go` | 78 | pending |
| ⏳ | `internal/streamutil/sse_helpers.go` | 185 | pending |
| ⏳ | `internal/streamutil/sse_helpers_test.go` | 278 | pending |
| ⏳ | `internal/streamutil/stall_reader.go` | 96 | pending |
| ⏳ | `internal/streamutil/stall_reader_test.go` | 181 | pending |
| ⏳ | `internal/streamutil/streamutil_test.go` | 120 | pending |
| ⏳ | `internal/streamutil/tool_deduper.go` | 109 | pending |
| ⏳ | `internal/streamutil/usage_tracking.go` | 239 | pending |
| ⏳ | `internal/streamutil/usage_tracking_test.go` | 263 | pending |

### internal/translator

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `internal/translator/helpers/claude.go` | 64 | pending |
| ⏳ | `internal/translator/helpers/gemini.go` | 95 | pending |
| ⏳ | `internal/translator/helpers/helpers_test.go` | 234 | pending |
| ⏳ | `internal/translator/helpers/image.go` | 100 | pending |
| ⏳ | `internal/translator/helpers/max_tokens.go` | 84 | pending |
| ⏳ | `internal/translator/helpers/max_tokens_adjust_test.go` | 59 | pending |
| ⏳ | `internal/translator/helpers/openai.go` | 57 | pending |
| ⏳ | `internal/translator/helpers/reasoning_inject.go` | 128 | pending |
| ⏳ | `internal/translator/helpers/reasoning_inject_test.go` | 122 | pending |
| ⏳ | `internal/translator/helpers/responses_api.go` | 278 | pending |
| ⏳ | `internal/translator/helpers/responses_api_convert_test.go` | 169 | pending |
| ⏳ | `internal/translator/helpers/tool_call.go` | 355 | pending |
| ⏳ | `internal/translator/helpers/tool_call_validate_test.go` | 225 | pending |
| ⏳ | `internal/translator/helpers/tool_deduper.go` | 162 | pending |
| ⏳ | `internal/translator/helpers/tool_deduper_test.go` | 160 | pending |
| ⏳ | `internal/translator/registry.go` | 69 | pending |
| 🔒 | `internal/translator/request/antigravity_to_openai.go` | 31 | audit pass 2026-05-30 |
| ⏳ | `internal/translator/request/claude_to_openai.go` | 40 | pending |
| ⏳ | `internal/translator/request/gemini_to_openai.go` | 57 | pending |
| ⏳ | `internal/translator/request/openai_responses.go` | 44 | pending |
| ⏳ | `internal/translator/request/openai_to_claude.go` | 76 | pending |
| 🔒 | `internal/translator/request/openai_to_commandcode.go` | 44 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/request/openai_to_cursor.go` | 30 | audit pass 2026-05-30 |
| ⏳ | `internal/translator/request/openai_to_gemini.go` | 54 | pending |
| ⏳ | `internal/translator/request/openai_to_kiro.go` | 49 | pending |
| 🔒 | `internal/translator/request/openai_to_ollama.go` | 42 | audit pass 2026-05-30 |
| ⏳ | `internal/translator/request/openai_to_vertex.go` | 67 | pending |
| ⏳ | `internal/translator/response/claude_cache_test.go` | 116 | pending |
| ⏳ | `internal/translator/response/claude_to_openai.go` | 118 | pending |
| 🔒 | `internal/translator/response/commandcode_to_openai.go` | 42 | audit pass 2026-05-30 |
| 🔒 | `internal/translator/response/cursor_to_openai.go` | 29 | audit pass 2026-05-30 |
| ⏳ | `internal/translator/response/gemini_to_openai.go` | 63 | pending |
| 🔒 | `internal/translator/response/kiro_to_openai.go` | 42 | audit pass 2026-05-30 |
| ⏳ | `internal/translator/response/ollama_stream.go` | 176 | pending |
| ⏳ | `internal/translator/response/ollama_stream_test.go` | 134 | pending |
| ⏳ | `internal/translator/response/ollama_to_openai.go` | 40 | pending |
| ⏳ | `internal/translator/response/openai_responses.go` | 41 | pending |
| ⏳ | `internal/translator/response/openai_to_antigravity.go` | 51 | pending |
| ⏳ | `internal/translator/response/openai_to_claude.go` | 45 | pending |
| ⏳ | `internal/translator/translator_test.go` | 103 | pending |

### internal/updater

| Status | File | LOC | Notes |
|---|---|---|---|
| 🔒 | `internal/updater/restart_unix.go` | 20 | audit pass 2026-05-30 |
| 🔒 | `internal/updater/restart_windows.go` | 29 | audit pass 2026-05-30 |
| ⏳ | `internal/updater/updater.go` | 215 | pending |
| ⏳ | `internal/updater/updater_test.go` | 48 | pending |

### root

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `login_limiter.go` | 113 | pending |
| ⏳ | `login_limiter_test.go` | 87 | pending |
| ⏳ | `main.go` | 162 | pending |
| ⏳ | `routes.go` | 280 | pending |
| ⏳ | `tunnel_watchdog.go` | 104 | pending |

### root/handlers

| Status | File | LOC | Notes |
|---|---|---|---|
| ⏳ | `handlers_apikey_auth.go` | 155 | pending |
| ⏳ | `handlers_auth.go` | 254 | pending |
| ⏳ | `handlers_auth_oidc.go` | 294 | pending |
| 🔒 | `handlers_backup.go` | 43 | audit pass 2026-05-30 |
| ⏳ | `handlers_brain.go` | 270 | pending |
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
| ⏳ | `handlers_brain_views.go` | 289 | pending |
| ⏳ | `handlers_bypass.go` | 124 | pending |
| ⏳ | `handlers_chat.go` | 138 | pending |
| ⏳ | `handlers_chat_v1.go` | 833 | pending |
| ⏳ | `handlers_cli_tools_ext.go` | 205 | pending |
| ⏳ | `handlers_fetch.go` | 120 | pending |
| ⏳ | `handlers_gaps.go` | 365 | pending |
| ⏳ | `handlers_kiromodels.go` | 47 | pending |
| 🔒 | `handlers_llm_policy.go` | 450 | audit pass 2026-05-30 |
| 🔒 | `handlers_llm_runtime.go` | 205 | audit pass 2026-05-30 |
| ⏳ | `handlers_locale.go` | 140 | pending |
| ⏳ | `handlers_mcp.go` | 456 | pending |
| 🔒 | `handlers_mcp_catalog.go` | 30 | audit pass 2026-05-30 |
| ⏳ | `handlers_media_ext.go` | 107 | pending |
| ⏳ | `handlers_media_tts_voices.go` | 471 | pending |
| ⏳ | `handlers_media_tts_voices_test.go` | 156 | pending |
| 🔒 | `handlers_mesh.go` | 140 | audit pass 2026-05-30 |
| 🔒 | `handlers_mesh_advanced.go` | 456 | audit pass 2026-05-30 |
| 🔒 | `handlers_mesh_stack.go` | 61 | audit pass 2026-05-30 |
| 🔒 | `handlers_mesh_transport.go` | 179 | audit pass 2026-05-30 |
| ⏳ | `handlers_mitm_ext.go` | 150 | pending |
| ⏳ | `handlers_mitm_proxy.go` | 135 | pending |
| ⏳ | `handlers_models_meta.go` | 299 | pending |
| ⏳ | `handlers_oauth.go` | 521 | pending |
| ⏳ | `handlers_oauth_device.go` | 173 | pending |
| ⏳ | `handlers_obs.go` | 144 | pending |
| ⏳ | `handlers_oidc_jwt.go` | 145 | pending |
| ⏳ | `handlers_pricing.go` | 80 | pending |
| ⏳ | `handlers_provider_nodes.go` | 146 | pending |
| ⏳ | `handlers_providers_ext.go` | 303 | pending |
| ⏳ | `handlers_proxy_deploy.go` | 187 | pending |
| ⏳ | `handlers_quotalive.go` | 90 | pending |
| 🔒 | `handlers_recordings.go` | 146 | audit pass 2026-05-30 |
| ⏳ | `handlers_resources.go` | 432 | pending |
| 🔒 | `handlers_sensors_webhook.go` | 107 | audit pass 2026-05-30 |
| ⏳ | `handlers_settings_sub.go` | 156 | pending |
| ⏳ | `handlers_skills_invoke.go` | 137 | pending |
| ⏳ | `handlers_stt.go` | 144 | pending |
| ⏳ | `handlers_sync.go` | 87 | pending |
| ⏳ | `handlers_tags.go` | 69 | pending |
| ⏳ | `handlers_translator.go` | 532 | pending |
| ⏳ | `handlers_tunnel.go` | 328 | pending |
| ⏳ | `handlers_usage_breakdown.go` | 386 | pending |
| 🔒 | `handlers_util.go` | 34 | audit pass 2026-05-30 |


## Methodology

Per file: security (SQL/path/cmd/secret), race (mu/defer), memory (close/leak), edge (nil/empty/bound), anti-pattern.