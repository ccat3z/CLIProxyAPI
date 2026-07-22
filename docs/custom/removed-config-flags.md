# Removed: Config Schema and Server Flags

The config schema is now restricted to the providers that still have runtime support (`claude-api-key` and `openai-compatibility`). The `-local-model` flag, the OAuth-era provider config types (`gemini-api-key`, `codex-api-key`, `vertex-api-key`, `oauth-excluded-models`, `oauth-model-alias`), and a set of dead config fields have been removed. The `models.json` catalog is always served from the embedded asset — there is no remote refresh of `models.json`. The `codex_client_models.json` catalog, by contrast, is remote-refreshed in the background (see [Codex Client Model Catalog Refresh](#codex-client-model-catalog-refresh) below).

## `-local-model` Flag and Remote Model Catalog Refresh

- `internal/registry/model_updater.go` (deleted) — the `StartModelsUpdater` goroutine that polled GitHub for a remote `models.json` and reloaded the catalog at runtime. Also contained the embedded-catalog reader helpers, which were preserved (see below).
- `cmd/server/main.go` — removed the `-local-model` flag definition, the local variable, the conditional that called `registry.StartModelsUpdater(...)`, and the `internal/registry` import.
- `internal/registry/model_definitions.go` — package comment trimmed to drop the "can be refreshed from network" sentence.

The embedded-catalog helpers (`embeddedModelsJSON`, `modelStore`, `modelsCatalogStore`, `loadModelsFromBytes`, `getModels`, `validateModelsCatalog`, `validateModelSection`) were preserved by moving them into the new `internal/registry/model_catalog.go`. Runtime behaviour is unchanged: `models.json` is loaded once from the embedded asset.

## Codex Client Model Catalog Refresh

Unlike `models.json`, the Codex client model catalog (`codex_client_models.json`) IS remote-refreshed on the `custom` branch, cherry-picked from upstream commit `4fe2c60c` (feat(registry): remote-refresh Codex client model catalog (#4276)):

- `internal/registry/codex_client_models.go` — rewritten to back the catalog by a revisioned in-memory store (`codexClientCatalogStore`) with validation (`ValidateCodexClientModelsJSON`), snapshot reads (`GetCodexClientModelsSnapshot`), and atomic replacement (`loadCodexClientModelsFromBytes`). The embedded `codex_client_models.json` seeds the store at init.
- `internal/registry/codex_client_models_updater.go` (new) — `StartCodexClientModelsUpdater` polls the upstream model repository every 3 hours, validates fetched bytes, and swaps the catalog revision when content changes. The `modelsRefreshInterval` / `modelsFetchTimeout` constants live here (the upstream `model_updater.go` that held them was deleted on `custom`).
- `cmd/server/main.go` — calls `registry.StartCodexClientModelsUpdater(context.Background())` unconditionally before starting the proxy service. The upstream `startModelCatalogUpdaters`/`modelCatalogUpdaterPlan` helpers were NOT taken: they gated on the removed `-local-model` flag and `cfg.Home.Enabled` (both absent on `custom`). There is no `-local-model` flag and no Home gate; the Codex client catalog refresh always runs.
- `cmd/server/main_test.go` — the upstream `TestModelCatalogUpdaterPlan` test was NOT taken (it exercises the removed `modelCatalogUpdaterPlan` helper).
- `cmd/validate_codex_models/main.go` (new) — standalone validator utility used by CI to bake a validated catalog at build time.
- `sdk/api/handlers/openai/codex_client_models.go` — template loader switched from `sync.Once` to a revision-keyed mutex so handler reloads pick up refreshed catalogs without restart. Upstream commit `ceaeb75d` (gate tool search by model providers) plus follow-up `e73aad2e` (require model template for tool search) were cherry-picked: `buildCodexClientModels` now takes a `codexClientModelProvidersFunc` and `applyCodexClientSearchToolSupport` disables `supports_search_tool` on any non-template model and on template models whose providers are not all `codex`.
- `.github/scripts/refresh-model-catalogs.sh` (new) + workflow changes — CI now refreshes both `models.json` and `codex_client_models.json` via the validate utility.
- `cmd/fetch_codex_models/` remains deleted (see [removed-tui-commands.md](./removed-tui-commands.md)); the upstream modifications to that standalone fetch utility were dropped.

## OAuth Provider Config Types

The following fields and struct types were removed from `internal/config/config.go`:

- `GeminiKey []GeminiKey` and the `GeminiKey` / `GeminiModel` structs
- `CodexKey []CodexKey` and the `CodexKey` / `CodexModel` structs
- `VertexCompatAPIKey []VertexCompatKey` and the `VertexCompatKey` / `VertexCompatModel` structs
- `OAuthExcludedModels map[string][]string`
- `OAuthModelAlias map[string][]OAuthModelAlias` and the `OAuthModelAlias` struct

`internal/config/vertex_compat.go` was deleted entirely (it held the `VertexCompatKey` type). `internal/config/parse.go` was updated to drop the matching `sanitize*` calls. `sdk/config/config.go` dropped the type aliases for the removed types.

## Other Dead Config Fields

| Field | YAML Key | Struct | Notes |
|-------|----------|--------|-------|
| `Plugins` | `plugins` | `PluginsConfig`, `PluginInstanceConfig` | Plugin system was removed; config was dead code |
| `CodexHeaderDefaults` | `codex-header-defaults` | `CodexHeaderDefaults` | Codex executor was removed; header defaults were unused |
| `Codex` | `codex` | `CodexConfig` (`IdentityConfuse`) | Identity confuse was never used in execution logic |
| `AntigravitySignatureCacheEnabled` | `antigravity-signature-cache-enabled` | `*bool` | Signature cache toggle was unused after provider removal |
| `AntigravitySignatureBypassStrict` | `antigravity-signature-bypass-strict` | `*bool` | Signature cache strict mode was unused after provider removal |
| `QuotaExceeded.AntigravityCredits` | `quota-exceeded.antigravity-credits` | `bool` | Credits fallback removed from conductor |

### Config sanitization

- `NormalizePluginsConfig()` — removed from `internal/config/config.go` and `internal/config/parse.go`
- `SanitizeCodexHeaderDefaults()` — removed from `internal/config/config.go` and `internal/config/parse.go`

### YAML config cleanup

- `removeRemovedIntegrationKeys()` in `internal/config/config.go` now removes `codex`, `codex-header-defaults`, `antigravity-signature-cache-enabled`, `antigravity-signature-bypass-strict`, and `plugins` from existing config files on save

### Signature cache wiring

- `configuredSignatureCacheEnabled()`, `configuredSignatureBypassStrict()`, `applySignatureCacheConfig()` — removed from `internal/api/server.go`
- `internal/cache` import removed from `internal/api/server.go`

### Watcher diff

- `quota-exceeded.antigravity-credits` and `codex.identity-confuse` diff detection removed from `internal/watcher/diff/config_diff.go`

### Test files

- `internal/config/plugin_config_test.go` — deleted
- `internal/config/codex_websocket_header_defaults_test.go` — deleted

## Service Helpers (`sdk/cliproxy/service.go`)

Removed the per-provider resolve helpers and their callers in `registerModelsForAuth`:

- `resolveConfigGeminiKey`, `resolveConfigVertexCompatKey`, `resolveConfigCodexKey`
- `oauthExcludedModels`
- `buildGeminiConfigModels`, `buildCodexConfigModels`, `buildVertexCompatConfigModels`
- The `case "gemini":` / `case "vertex":` / `case "codex":` branches no longer consult per-key model overrides; they fall back to the registry catalog and the synthesizer-provided `excluded_models` auth attribute.
- The OAuth model-alias runtime path (`applyOAuthModelAlias`, `registerModelRefreshCallback`, `refreshModelRegistrationForAuth`, `latestAuthForModelRegistration`, `SetOAuthModelAlias`) was also deleted.

`sdk/cliproxy/builder.go` dropped the `SetOAuthModelAlias` call.

## Auth Conductor (`sdk/cliproxy/auth/`)

- `sdk/cliproxy/auth/oauth_model_alias.go` (+ test) — deleted. The runtime `oauthModelAlias` table, `SetOAuthModelAlias`, `OAuthModelAliasChannel`, and the alias-resolution helpers that walked that table were removed.
- `sdk/cliproxy/auth/model_alias.go` (new) — retains the provider-agnostic alias-pool helpers that were never tied to the `oauthModelAlias` table: `modelAliasEntry`, `modelAliasLookupCandidates`, `preserveResolvedModelSuffix`, `resolveModelAliasPoolFromConfigModels`, `resolveModelAliasFromConfigModels`.
- `sdk/cliproxy/auth/conductor.go` — removed the `resolve*APIKeyConfig` helpers for Gemini/Codex/Vertex, the `resolveUpstreamModelFor*` functions, the `oauthModelAlias` `atomic.Value` field, and the `applyOAuthModelAlias` calls inside `executionModelCandidates` / `selectionModelForAuth`.
- `sdk/cliproxy/auth/conductor_oauth_alias_suspension_test.go` and `sdk/cliproxy/auth/oauth_model_alias_test.go` — deleted (the runtime feature they tested no longer exists). The `TestManager_ShouldRetryAfterError_UsesOAuthModelAliasForCooldown` case was removed from `conductor_overrides_test.go`.

## Watcher Diff and Synthesizer

- `internal/watcher/diff/oauth_model_alias.go`, `internal/watcher/diff/oauth_excluded.go` (+ tests) — deleted.
- `internal/watcher/diff/config_diff.go` — dropped the Gemini, Codex, Vertex, OAuth-excluded, and OAuth-model-alias diff sections.
- `internal/watcher/diff/models_summary.go` — kept only `ClaudeModelsSummary` and `ExcludedModelsSummary`. The shared `expectContains` test helper (previously in the deleted `oauth_excluded_test.go`) was moved here so other diff tests can still use it.
- `internal/watcher/diff/model_hash.go` — removed `ComputeGeminiModelsHash`, `ComputeCodexModelsHash`, `ComputeVertexModelsHash`.
- `internal/watcher/synthesizer/config.go` — removed `synthesizeGeminiKeys`, `synthesizeCodexKeys`, `synthesizeVertexCompat`.
- `internal/watcher/synthesizer/helpers.go` — dropped the `OAuthExcludedModels` merge in `ApplyAuthExcludedModelsMeta`.
- `internal/watcher/clients.go` — `BuildAPIKeyClients` now returns `(0, 0, claudeCount, 0, openAICompatCount)`. The function signature is unchanged so callers do not need to be updated.
- `internal/watcher/config_reload.go` — removed the `reflect` import and the OAuth-diff / model-alias plumbing; only the material change log via `diff.BuildConfigChangeDetails` remains.

## Management API Routes

The following routes were removed from `internal/api/server.go` (and their handlers from `internal/api/handlers/management/`):

- `GET/PUT/PATCH/DELETE /v0/management/gemini-api-key`
- `GET/PUT/PATCH/DELETE /v0/management/codex-api-key`
- `GET/PUT/PATCH/DELETE /v0/management/vertex-api-key`
- `GET/PUT/PATCH/DELETE /v0/management/oauth-excluded-models`
- `GET/PUT/PATCH/DELETE /v0/management/oauth-model-alias`

The `claude-api-key` and `openai-compatibility` routes are unchanged. `config_apikey_disable.go` now only iterates `cfg.ClaudeKey` when applying the `*` exclusion wildcard.

## config.example.yaml Cleanup

The example config template (`config.example.yaml`) was swept to remove entries that no longer correspond to a live config struct field. The following commented-out sections were removed because their underlying config types were deleted in prior cleanup rounds:

| Removed Example Entry | Reason |
|-----------------------|--------|
| `gemini-api-key` section | `GeminiKey` struct removed |
| `codex-api-key` section | `CodexKey` struct removed |
| `codex` section (identity-confuse) | `CodexConfig` struct removed |
| `codex-header-defaults` section | `CodexHeaderDefaults` struct removed |
| `vertex-api-key` section | `VertexCompatKey` struct removed |
| `oauth-model-alias` section | `OAuthModelAlias` map removed |
| `oauth-excluded-models` section | `OAuthExcludedModels` map removed |
| `antigravity-signature-cache-enabled` | Field removed from struct |
| `antigravity-signature-bypass-strict` | Field removed from struct |
| `gpt-image-2-base-model` | Field removed from struct |
| `quota-exceeded.antigravity-credits` | Field removed from `QuotaExceeded` struct |
| `claude-header-defaults` sub-fields `os`, `arch`, `stabilize-device-profile` | Fields removed from `ClaudeHeaderDefaults` struct |
| Plugin provider reference in oauth-model-alias comment | Plugin system removed |

The `claude-header-defaults` comment was also updated to remove references to the deleted `os`/`arch`/`stabilize-device-profile` fields.

## Config Fields Kept for Compatibility

The following config fields are still accepted because they are used by runtime code, management handlers, or the conductor:

- `ClaudeHeaderDefaults` — still used by `claude_executor.go`
- `DisableClaudeCloakMode` — still used by `claude_executor.go`
- `WebsocketAuth` — still used by websocket handlers and selector
- `CommercialMode` — still used by `server.go` and logging helpers
