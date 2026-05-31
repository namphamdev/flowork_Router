# flowork_Router — Audit Inventory

> Auto-generated file manifest for the full bug-audit pass. Dibuat: 2026-05-30.
> Tujuan: tau persis ada file apa aja + jumlahnya, biar audit bisa dilacak file-per-file.

## Ringkasan jumlah

| Kategori | Jumlah |
|---|---|
| Go files (exclude referensifile) | 396 |
| Go test files | 66 |
| Go files di root (package main) | 64 |
| internal/ subpackages | 39 |
| GUI (web/static) | 1 |
| referensifile Go (read-only ref) | 100 |
| skills/ dirs | 8 |
| cmd/ binaries | 2 |

## Root package main — Go files (64)

- [ ] 🔒 `handlers_apikey_auth.go` (162 LOC)
- [ ] 🔒 `handlers_auth.go` (261 LOC)
- [ ] 🔒 `handlers_auth_oidc.go` (301 LOC)
- [ ] 🔒 `handlers_backup.go` (43 LOC)
- [ ] 🔒 `handlers_brain.go` (277 LOC)
- [ ] 🔒 `handlers_brain_ingest.go` (119 LOC)
- [ ] 🔒 `handlers_brain_injection.go` (49 LOC)
- [ ] 🔒 `handlers_brain_mistakes.go` (94 LOC)
- [ ] 🔒 `handlers_brain_models.go` (151 LOC)
- [ ] 🔒 `handlers_brain_pii.go` (67 LOC)
- [ ] 🔒 `handlers_brain_proposals.go` (137 LOC)
- [ ] 🔒 `handlers_brain_quality.go` (57 LOC)
- [ ] 🔒 `handlers_brain_rescore.go` (81 LOC)
- [ ] 🔒 `handlers_brain_skills.go` (102 LOC)
- [ ] 🔒 `handlers_brain_tools.go` (96 LOC)
- [ ] 🔒 `handlers_brain_views.go` (296 LOC)
- [ ] 🔒 `handlers_bypass.go` (131 LOC)
- [ ] 🔒 `handlers_chat.go` (145 LOC)
- [ ] 🔒 `handlers_chat_v1.go` (840 LOC)
- [ ] 🔒 `handlers_cli_tools_ext.go` (212 LOC)
- [ ] 🔒 `handlers_fetch.go` (127 LOC)
- [ ] 🔒 `handlers_gaps.go` (372 LOC)
- [ ] 🔒 `handlers_kiromodels.go` (54 LOC)
- [ ] 🔒 `handlers_llm_policy.go` (450 LOC)
- [ ] 🔒 `handlers_llm_runtime.go` (205 LOC)
- [ ] 🔒 `handlers_locale.go` (147 LOC)
- [ ] 🔒 `handlers_mcp_catalog.go` (30 LOC)
- [ ] 🔒 `handlers_mcp.go` (463 LOC)
- [ ] 🔒 `handlers_media_ext.go` (114 LOC)
- [ ] 🔒 `handlers_media_tts_voices.go` (478 LOC)
- [ ] 🔒 `handlers_media_tts_voices_test.go` (163 LOC)
- [ ] 🔒 `handlers_mesh_advanced.go` (456 LOC)
- [ ] 🔒 `handlers_mesh.go` (140 LOC)
- [ ] 🔒 `handlers_mesh_stack.go` (61 LOC)
- [ ] 🔒 `handlers_mesh_transport.go` (179 LOC)
- [ ] 🔒 `handlers_mitm_ext.go` (157 LOC)
- [ ] 🔒 `handlers_mitm_proxy.go` (142 LOC)
- [ ] 🔒 `handlers_models_meta.go` (306 LOC)
- [ ] 🔒 `handlers_oauth_device.go` (180 LOC)
- [ ] 🔒 `handlers_oauth.go` (528 LOC)
- [ ] 🔒 `handlers_obs.go` (151 LOC)
- [ ] 🔒 `handlers_oidc_jwt.go` (152 LOC)
- [ ] 🔒 `handlers_pricing.go` (87 LOC)
- [ ] 🔒 `handlers_provider_nodes.go` (153 LOC)
- [ ] 🔒 `handlers_providers_ext.go` (310 LOC)
- [ ] 🔒 `handlers_proxy_deploy.go` (194 LOC)
- [ ] 🔒 `handlers_quotalive.go` (97 LOC)
- [ ] 🔒 `handlers_recordings.go` (146 LOC)
- [ ] 🔒 `handlers_resources.go` (439 LOC)
- [ ] 🔒 `handlers_sensors_webhook.go` (107 LOC)
- [ ] 🔒 `handlers_settings_sub.go` (163 LOC)
- [ ] 🔒 `handlers_skills_invoke.go` (144 LOC)
- [ ] 🔒 `handlers_stt.go` (151 LOC)
- [ ] 🔒 `handlers_sync.go` (94 LOC)
- [ ] 🔒 `handlers_tags.go` (76 LOC)
- [ ] 🔒 `handlers_translator.go` (539 LOC)
- [ ] 🔒 `handlers_tunnel.go` (335 LOC)
- [ ] 🔒 `handlers_usage_breakdown.go` (393 LOC)
- [ ] 🔒 `handlers_util.go` (34 LOC)
- [ ] 🔒 `login_limiter.go` (120 LOC)
- [ ] 🔒 `login_limiter_test.go` (94 LOC)
- [ ] 🔒 `main.go` (169 LOC)
- [ ] 🔒 `routes.go` (287 LOC)
- [ ] 🔒 `tunnel_watchdog.go` (111 LOC)

## internal/ packages (39)

- [ ] **brain** — 15 file, 2105 LOC
- [ ] **bypass** — 2 file, 269 LOC
- [ ] **caveman** — 2 file, 166 LOC
- [ ] **clitools** — 3 file, 1088 LOC
- [ ] **cloudcode** — 2 file, 471 LOC
- [ ] **constitution** — 1 file, 271 LOC
- [ ] **creds** — 2 file, 322 LOC
- [ ] **executors** — 33 file, 3727 LOC
- [ ] **fetch** — 5 file, 407 LOC
- [ ] **i18n** — 2 file, 152 LOC
- [ ] **ingest** — 3 file, 318 LOC
- [ ] **kiromodels** — 2 file, 387 LOC
- [ ] **localai** — 1 file, 147 LOC
- [ ] **mcpcatalog** — 2 file, 270 LOC
- [ ] **mcpsecurity** — 2 file, 268 LOC
- [ ] **mesh** — 9 file, 1451 LOC
- [ ] **mitm** — 20 file, 1653 LOC
- [ ] **modelpool** — 1 file, 275 LOC
- [ ] **piistrip** — 1 file, 231 LOC
- [ ] **policy** — 1 file, 197 LOC
- [ ] **pricing** — 1 file, 73 LOC
- [ ] **promptguard** — 1 file, 263 LOC
- [ ] **provider** — 1 file, 190 LOC
- [ ] **providercompat** — 2 file, 229 LOC
- [ ] **providers** — 37 file, 2328 LOC
- [ ] **quality** — 1 file, 217 LOC
- [ ] **quotalive** — 12 file, 1567 LOC
- [ ] **recorder** — 1 file, 267 LOC
- [ ] **router** — 32 file, 5127 LOC
- [ ] **rtk** — 16 file, 1085 LOC
- [ ] **safego** — 2 file, 118 LOC
- [ ] **safeurl** — 2 file, 221 LOC
- [ ] **search** — 6 file, 368 LOC
- [ ] **sensors** — 1 file, 93 LOC
- [ ] **services** — 5 file, 603 LOC
- [ ] **store** — 28 file, 4549 LOC
- [ ] **streamutil** — 11 file, 1806 LOC
- [ ] **translator** — 40 file, 4126 LOC
- [ ] **updater** — 4 file, 326 LOC

## cmd/ binaries

- [ ] flow-cli — 18 file
- [ ] flow-tray — 2 file

## GUI

- [ ] web/static/index.html (5082 LOC) — single-file dashboard, 20 tab
