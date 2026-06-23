# Removed: `-local-model` Flag and OAuth Provider Config Types (Phase 7 Batch 3)

## Summary

Removed the `-local-model` command-line flag and the OAuth-era provider
config types (`gemini-api-key`, `codex-api-key`, `vertex-api-key`,
`oauth-excluded-models`, `oauth-model-alias`). The model catalog is now
always served from the embedded `models.json` (no remote updates), and the
config schema is restricted to the providers that still have runtime
support: `claude-api-key` and `openai-compatibility`.

## What was removed

### `-local-model` flag and remote model catalog refresh

- `internal/registry/model_updater.go` (deleted) — the
  `StartModelsUpdater` goroutine that polled GitHub for a remote
  `models.json` and reloaded the catalog at runtime. Also contained the
  embedded-catalog reader helpers, which were preserved (see below).
- `cmd/server/main.go` — removed the `-local-model` flag definition, the
  local variable, the conditional that called
  `registry.StartModelsUpdater(...)`, and the `internal/registry` import.
- `internal/registry/model_definitions.go` — package comment trimmed to
  drop the "can be refreshed from network" sentence.

The embedded-catalog helpers (`embeddedModelsJSON`, `modelStore`,
`modelsCatalogStore`, `loadModelsFromBytes`, `getModels`,
`validateModelsCatalog`, `validateModelSection`) were preserved by moving
them into the new `internal/registry/model_catalog.go`. Runtime behaviour
is unchanged: `models.json` is loaded once from the embedded asset.

### OAuth provider config types

The following fields and struct types were removed from
`internal/config/config.go`:

- `GeminiKey []GeminiKey` and the `GeminiKey` / `GeminiModel` structs
- `CodexKey []CodexKey` and the `CodexKey` / `CodexModel` structs
- `VertexCompatAPIKey []VertexCompatKey` and the `VertexCompatKey` /
  `VertexCompatModel` structs
- `OAuthExcludedModels map[string][]string`
- `OAuthModelAlias map[string][]OAuthModelAlias` and the
  `OAuthModelAlias` struct

`internal/config/vertex_compat.go` was deleted entirely (it held the
`VertexCompatKey` type). `internal/config/parse.go` was updated to drop
the matching `sanitize*` calls. `sdk/config/config.go` dropped the type
aliases for the removed types.

### Service helpers (`sdk/cliproxy/service.go`)

Removed the per-provider resolve helpers and their callers in
`registerModelsForAuth`:

- `resolveConfigGeminiKey`, `resolveConfigVertexCompatKey`,
  `resolveConfigCodexKey`
- `oauthExcludedModels`
- `buildGeminiConfigModels`, `buildCodexConfigModels`,
  `buildVertexCompatConfigModels`
- The `case "gemini":` / `case "vertex":` / `case "codex":` branches no
  longer consult per-key model overrides; they fall back to the registry
  catalog and the synthesizer-provided `excluded_models` auth attribute.
  The OAuth model-alias runtime path (`applyOAuthModelAlias`,
  `registerModelRefreshCallback`, `refreshModelRegistrationForAuth`,
  `latestAuthForModelRegistration`, `SetOAuthModelAlias`) was also
  deleted.

`sdk/cliproxy/builder.go` dropped the `SetOAuthModelAlias` call.

### Auth conductor (`sdk/cliproxy/auth/`)

- `sdk/cliproxy/auth/oauth_model_alias.go` (+ test) — deleted. The
  runtime `oauthModelAlias` table, `SetOAuthModelAlias`,
  `OAuthModelAliasChannel`, and the alias-resolution helpers that walked
  that table were removed.
- `sdk/cliproxy/auth/model_alias.go` (new) — retains the
  provider-agnostic alias-pool helpers that were never tied to the
  `oauthModelAlias` table: `modelAliasEntry`,
  `modelAliasLookupCandidates`, `preserveResolvedModelSuffix`,
  `resolveModelAliasPoolFromConfigModels`,
  `resolveModelAliasFromConfigModels`.
- `sdk/cliproxy/auth/conductor.go` — removed the
  `resolve*APIKeyConfig` helpers for Gemini/Codex/Vertex, the
  `resolveUpstreamModelFor*` functions, the `oauthModelAlias`
  `atomic.Value` field, and the `applyOAuthModelAlias` calls inside
  `executionModelCandidates` / `selectionModelForAuth`.
- `sdk/cliproxy/auth/conductor_oauth_alias_suspension_test.go` and
  `sdk/cliproxy/auth/oauth_model_alias_test.go` — deleted (the runtime
  feature they tested no longer exists). The
  `TestManager_ShouldRetryAfterError_UsesOAuthModelAliasForCooldown`
  case was removed from `conductor_overrides_test.go`.

### Watcher diff and synthesizer

- `internal/watcher/diff/oauth_model_alias.go`,
  `internal/watcher/diff/oauth_excluded.go` (+ tests) — deleted.
- `internal/watcher/diff/config_diff.go` — dropped the Gemini, Codex,
  Vertex, OAuth-excluded, and OAuth-model-alias diff sections.
- `internal/watcher/diff/models_summary.go` — kept only
  `ClaudeModelsSummary` and `ExcludedModelsSummary`. The shared
  `expectContains` test helper (previously in the deleted
  `oauth_excluded_test.go`) was moved here so other diff tests can still
  use it.
- `internal/watcher/diff/model_hash.go` — removed
  `ComputeGeminiModelsHash`, `ComputeCodexModelsHash`,
  `ComputeVertexModelsHash`.
- `internal/watcher/synthesizer/config.go` — removed
  `synthesizeGeminiKeys`, `synthesizeCodexKeys`, `synthesizeVertexCompat`.
- `internal/watcher/synthesizer/helpers.go` — dropped the
  `OAuthExcludedModels` merge in `ApplyAuthExcludedModelsMeta`.
- `internal/watcher/clients.go` — `BuildAPIKeyClients` now returns
  `(0, 0, claudeCount, 0, openAICompatCount)`. The function signature is
  unchanged so callers do not need to be updated.
- `internal/watcher/config_reload.go` — removed the `reflect` import and
  the OAuth-diff / model-alias plumbing; only the material change log
  via `diff.BuildConfigChangeDetails` remains.

### Management API

The following routes were removed from `internal/api/server.go` (and
their handlers from `internal/api/handlers/management/`):

- `GET/PUT/PATCH/DELETE /v0/management/gemini-api-key`
- `GET/PUT/PATCH/DELETE /v0/management/codex-api-key`
- `GET/PUT/PATCH/DELETE /v0/management/vertex-api-key`
- `GET/PUT/PATCH/DELETE /v0/management/oauth-excluded-models`
- `GET/PUT/PATCH/DELETE /v0/management/oauth-model-alias`

The `claude-api-key` and `openai-compatibility` routes are unchanged.
`config_apikey_disable.go` now only iterates `cfg.ClaudeKey` when
applying the `*` exclusion wildcard.

## Rationale

After Phase 4 removed the Codex/Gemini/Vertex OAuth executors and Phase 5
removed the antigravity executor, the only providers with runtime
executors are Claude, Codex (registry catalog only — no per-key
overrides), Gemini (registry catalog only), and OpenAI-compatibility.
The removed config fields were either unused at runtime
(`OAuthExcludedModels`, `OAuthModelAlias`, `VertexCompatAPIKey`) or
duplicated registry data with no remaining executor
(`GeminiKey.Models`, `CodexKey.Models`).

The `-local-model` flag was the only toggle for remote model-catalog
updates; with no remaining caller, the embedded catalog is now the
single source of truth.

## Verification

- `gofmt -w .`
- `go build -o test-output ./cmd/server` succeeds
- `go test ./...` — all modified packages pass; pre-existing unrelated
  failures in `sdk/cliproxy/auth` (OpenAI-compat pool suspension tests)
  are not affected by this change.
- Smoke test against a live server:
  - `GET /v1/models` → 200
  - `POST /v1/chat/completions` → 200 with valid completion
  - `GET /v0/management/gemini-api-key` → **404** (route removed)
  - `GET /v0/management/oauth-model-alias` → **404** (route removed)
  - `cli-proxy-api --local-model` → **flag rejected** (flag removed)
