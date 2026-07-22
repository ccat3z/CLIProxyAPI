# Removed: Dead Code and Unused Dependencies

This document catalogs the zero-caller code and dead subsystems removed from the `custom` branch after the feature-level removals ([plugins](./removed-plugins.md), [providers/OAuth](./removed-providers-oauth.md), [storage/relay](./removed-storage-relay.md), [TUI](./removed-tui-commands.md), [config flags](./removed-config-flags.md), [utls](./removed-utls.md), [TLS listen](./removed-tls-listen.md), [pprof](./removed-pprof.md), [redis/home](./removed-redis-home.md), [auth-dir](./removed-auth-dir.md), [proxy-url](./removed-proxy-url.md)).

The `custom` branch only configures the `claude-api-key` and `openai-compatibility` providers with plain API keys. Everything below had zero callers under that configuration (verified with `grep -rn '<symbol>' --include='*.go' .` before each deletion), or was a subsystem whose only config-reachable entry was a provider type absent from the deployment.

> Verification: every removal passed `gofmt`, `go build`, `go test ./...`, `pytest integration/` (37 passed), and a smoke test (`/v1/models`, `/v1/chat/completions`, `/v0/management/config`) before commit. Removed routes confirmed to return 404 afterward.

---

## Providers & Model Catalog

These leftovers existed only to serve the removed providers (codex, gemini, gemini-cli, antigravity, kimi, xai, aistudio, vertex). None can be configured on the `custom` branch, so the code paths were unreachable.

### Provider usage parsers

The `Parse*Usage` / `Parse*StreamUsage` helpers in `internal/runtime/executor/helps/usage_helpers.go` served only the removed executors. Only the OpenAI and Claude parsers have live callers.

| Removed | Reason |
| --- | --- |
| `ParseCodexUsage`, `ParseCodexImageToolUsage` | Codex executor removed. |
| `ParseGeminiUsage`, `ParseGeminiStreamUsage` | Gemini executor removed. |
| `ParseGeminiCLIUsage`, `ParseGeminiCLIStreamUsage` | Gemini-CLI executor removed. |
| `ParseAntigravityUsage`, `ParseAntigravityStreamUsage` | Antigravity executor removed. |
| `parseGeminiFamilyUsageDetail`, `hasGeminiFamilyUsageTokenFields` (private) | Only callers were the removed parsers above. |

**Kept** (live callers in the OpenAI-compat and Claude executors): `ParseOpenAIUsage`, `ParseOpenAIStreamUsage`, `ParseClaudeUsage`, `ParseClaudeStreamUsage`, their shared helpers, `StreamUsageBuffer` (used by the OpenAI-compat executor to track the last observed stream usage before publishing), and `firstExistingUsageNode` (now used by the retained `parseOpenAIStyleUsageNode` to normalize cache-token aliases).

### Dead thinking-config providers

The thinking-config appliers under `internal/thinking/provider/` self-registered at startup via blank imports but had no live caller dispatching to their provider names. Deleted directories:

| Path | Reason |
| --- | --- |
| `internal/thinking/provider/codex/` | Self-registers for `"codex"`; no live dispatch. |
| `internal/thinking/provider/gemini/` | Self-registers for `"gemini"`; no live dispatch. |
| `internal/thinking/provider/geminicli/` | Self-registers for `"gemini-cli"`; no live dispatch. |
| `internal/thinking/provider/kimi/` | Self-registers for `"kimi"`; no live dispatch. |
| `internal/thinking/provider/xai/` | Self-registers for `"xai"`; no live dispatch. |

The five blank imports in `internal/runtime/executor/helps/thinking_providers.go` were removed (`claude`/`openai` kept).

`internal/thinking/apply.go` trims: the `nativeProviderAppliers` map dropped keys `gemini`/`gemini-cli`/`codex`/`antigravity`/`kimi`/`xai` (only `claude`/`openai` remain); the `extractThinkingConfig` switch dropped the `gemini`/`codex`/`kimi` branches (unreachable); the `extractGeminiConfig` helper was deleted (zero callers after its branch was removed).

**Kept**: `extractCodexConfig` — still called as a fallback from `ExtractReasoningEffort` / `ExtractTranslatedReasoningEffort` for the live `openai` / `openai-response` providers. `internal/thinking/provider/claude/` and `.../openai/` are the only live providers.

### Signature validation helpers (`internal/signature/`)

The package still carried validation/sanitization entrypoints exercised only by the removed Gemini / Antigravity providers. The only live callers are `SanitizeClaudeMessagesForClaudeUpstream` (from `claude_executor.go`) and the cross-provider prefix/compatibility helpers used by the translators.

**`claude.go` — deleted in full** (plus `claude_test.go`): `StripInvalidClaudeThinkingBlocks`, `StripInvalidClaudeThinkingBlocksAndEmptyMessages`, and their private helpers `shouldStripClaudeThinkingBlock`, `isEmptyClaudeThinkingPlaceholder`, `claudeThinkingBlockText`. All had zero external callers.

**`claude_messages_sanitize.go` — trimmed**: removed `SanitizeClaudeMessagesSignaturesForModel` and `SanitizeClaudeMessagesSignaturesForTarget` (zero external callers). `ForTarget` was inlined into the kept `SanitizeClaudeMessagesForClaudeUpstream` with the Claude-upstream options hardcoded (`DropEmptyMessages`/`DropToolSignatures`/`DropEmptyThinkingPlaceholders` all `true`), which made two branches provably dead and removed the dependency on `isEmptyClaudeThinkingPlaceholder`. Two orphaned symbols went with it: `ClaudeMessagesSignatureSanitizeOptions`, `sanitizeClaudeToolUseSignature`.

**`gemini_validation.go` — trimmed**: removed `ValidateGeminiThoughtSignatures`, `ValidateGeminiFunctionCallPairing`, and their private helpers (`geminiContents`, `geminiFunctionCallRef`, `geminiFunctionResponseRef`). The `gjson` import was dropped.

**`gemini_sanitize.go` — deleted in full** (plus `gemini_sanitize_test.go`): `SanitizeGeminiRequestThoughtSignatures`, `GeminiReplaySignatureOrBypass`, and private helpers (`logGeminiThoughtSignatureSanitize`, `geminiPartThoughtSignature`, `deleteGeminiPartThoughtSignatureFields`).

**`provider_compatibility.go` — trimmed**: removed `CompatibleAntigravityClaudeThinkingSignature`.

**Kept despite an initial removal plan**: `InspectGeminiThoughtSignature`, `IsValidGeminiThoughtSignature`, `IsGeminiThoughtSignatureBypass` are retained — they are called from `provider_compatibility.go` (itself called by translators), so deleting them would break the build. The full envelope/decoder helper tree `InspectGeminiThoughtSignature` depends on is also kept. Also kept: `provider_compatibility.go`, `claude_validation.go`, `gpt_validation.go`, `CompatibleSignatureForProvider`, and the retained `SignatureSanitizeReport` / `stripClaudeToolUseSignatureFields` used by the inlined Claude-upstream path.

Tests trimmed: `claude_test.go` deleted; `gemini_validation_test.go` reduced to the Inspect/IsValid cases; `provider_compatibility_test.go` dropped the five `ForModel` cases.

### Model catalog definitions (`internal/registry/`)

Static model definitions for all removed providers lingered in the embedded catalog. The empty `antigravity` section fired a startup warning (`models catalog: antigravity section is empty`). Since no auth ever registers these providers' models, the static sections could never be served — only `claude` corresponds to a live provider format. (The deployment's real models — `glm-5.1` etc. — are registered dynamically per-auth from `config.yaml`, not from the static catalog.)

| Item | Disposition |
| --- | --- |
| `models.json` sections: `gemini`, `vertex`, `gemini-cli`, `aistudio`, `codex-free/team/plus/pro`, `kimi`, `xai`, `antigravity` | Removed — only `claude` remains (13 models). |
| `model_catalog.go` `requiredSections` | Trimmed to `claude` only — startup warning gone. |
| `model_definitions.go` struct fields (`Gemini`/`Vertex`/`GeminiCLI`/`AIStudio`/`Codex*`/`Kimi`/`Antigravity`/`XAI`) | Removed; only `Claude` remains. |
| `model_definitions.go` accessors (`GetGeminiModels`, `GetGeminiVertexModels`, `GetGeminiCLIModels`, `GetAIStudioModels`, `GetCodexFree/Team/Plus/ProModels`, `GetKimiModels`, `GetAntigravityModels`, `GetXAIModels`) | Removed. |
| `AntigravityWebSearchModelFor`, `normalizeAntigravityCapabilityModelID` | Removed. |
| Builtin-injection helpers (`WithCodexBuiltins`, `WithXAIBuiltins`, `xaiBuiltinImage*ModelInfo`, `codexBuiltinImageModelInfo`, `upsertModelInfos`, and their constants) | Removed — only used by deleted accessors. |
| `GetStaticModelDefinitionsByChannel` / `LookupStaticModelInfo` switches | Trimmed to `claude` + default. |
| `sdk/cliproxy/service.go` `registerModelsForAuth` dead provider cases | Removed — only `claude` + OpenAI-compat `default` remain. |
| `internal/registry/model_registry.go` doc comments | Trimmed of removed-provider references. |

Tests: `model_definitions_test.go` deleted; `service_excluded_models_test.go`'s fixture migrated from `gemini-cli` to the live `claude` provider.

**Kept**: the `claude` catalog section + `GetClaudeModels`; `GetStaticModelDefinitionsByChannel` / `LookupStaticModelInfo` (functions retained, switches trimmed — still called by the management endpoint); all dynamic registry functions (`RegisterClient`, `GetModelInfo`, `GetModelProviders`, `GetAvailableModels`, `GetAvailableModelsByProvider`).

### Video generation subsystem (XAI/Sora)

Every video route hardcoded an XAI/Sora model (`grok-imagine-video` / `sora-2`) under the `openai-video` handler type, and no configured provider registers those models — so every video request returned `502 unknown provider` before any upstream call.

| Item | Disposition |
| --- | --- |
| `sdk/api/handlers/openai/openai_videos_handlers.go` (+ test) | Deleted in full — all 7 video handlers, `videoAuthBindings` store, normalization helpers. |
| `/v1/videos*` routes (xAI native, 5 routes) | Removed from `internal/api/server.go`. |
| `/openai/v1` route group | Deleted entirely — it contained only 3 video routes. |
| `SDKConfig.VideoResultAuthCacheTTL` field + `config.example.yaml` entry | Removed (only reader was the deleted video handler). |
| `TestVideosRoutesKeepXAINativeAndExposeOpenAIPrefix` | Removed from `internal/api/server_test.go`. |

**Kept**: `openai_images_handlers.go` + `imagesModelParts` (shared with the live `/v1/images/*` handlers); `codex_client_models.go`; the `openai-image` half of the conductor check (still live).

---

## Auth & Credentials

### `internal/auth` package + `Auth.Storage` field

After the OAuth removal, `internal/auth/` existed only to define the `TokenStorage` interface with no live implementations. `Auth.Storage` (typed `baseauth.TokenStorage`) was read in `sdk/auth/filestore.go` but never assigned — always `nil`, so the `case auth.Storage != nil:` branch in `FileTokenStore.Save` was dead.

| Item | Disposition |
| --- | --- |
| `internal/auth/` directory | Deleted in full. |
| `Auth.Storage` field + `baseauth` import (`sdk/cliproxy/auth/types.go`) | Removed. |
| `case auth.Storage != nil:` branch + local `metadataSetter` interface in `FileTokenStore.Save` | Removed. |
| `sdk/auth/filestore_disabled_test.go` | Deleted (only exercised the removed branch). |

The live `case auth.Metadata != nil:` branch (persists auth metadata as JSON) is the only remaining save path.

### Dead home KV helpers (`internal/home/kv_helpers.go`)

Two parallel families of KV accessors over the home Redis client existed. The only live callers on the `custom` branch are `HashKeyPart` and `CurrentKVClient` (both from `session_id_cache.go`).

| Removed (`*Required` family) | Removed (`*BestEffort` family) | Removed (private) |
| --- | --- | --- |
| `KVGetJSONRequired`, `KVSetJSONRequired`, `KVSetBytesRequired`, `KVSetNXRequired`, `KVDelRequired`, `KVExpireRequired` | `KVGetJSONBestEffort`, `KVSetJSONBestEffort`, `KVSetBytesBestEffort`, `KVSetNXBestEffort`, `KVDelBestEffort`, `KVExpireBestEffort` | `kvSetOptionsForTTL`, `kvLogPrefix`, `firstKVKey` |

`kv_helpers_test.go` was trimmed to `TestHashKeyPart` + `TestCurrentKVClientUnavailableErrors`.

**Kept**: `HashKeyPart`, `CurrentKVClient` (live callers in `session_id_cache.go`). Note: the broader redis-backed `internal/home` client was removed in [removed-redis-home.md](./removed-redis-home.md); the local session-id KV mode is retained.

---

## Config Schema

### Quota failover config (`SwitchProject` / `SwitchPreviewModel`)

`QuotaExceeded.SwitchProject` and `.SwitchPreviewModel` were parsed, diffed, and exposed via the management API, but no runtime code read them — leftovers from the removed provider failover infrastructure.

| Item | Disposition |
| --- | --- |
| `QuotaExceeded.SwitchProject` / `.SwitchPreviewModel` fields | Removed from `internal/config/config.go`. |
| `Get/Put SwitchProject` / `Get/Put SwitchPreviewModel` handlers (`internal/api/handlers/management/quota.go`) | File deleted in full. |
| `GET/PUT/PATCH /v0/management/quota-exceeded/switch-{project,preview-model}` routes | Removed from `internal/api/server.go`. |
| `switch-project` / `switch-preview-model` diff entries | Removed from `internal/watcher/diff/config_diff.go`. |
| `config.example.yaml` entries | Removed. |

`removeRemovedIntegrationKeys` was extended (via `removeNestedMapKey`) to strip the two keys from existing user configs on next save. The `QuotaExceeded` struct is now empty but retained so existing YAML `quota-exceeded:` blocks deserialize without error.

### Dead SDK config fields

| Field | Disposition |
| --- | --- |
| `SDKConfig.GPTImage2BaseModel` (`gpt-image-2-base-model`) | Removed from `sdk_config.go` + diff entry + `config.example.yaml`; added to removal-key migration. No executor reads it. |
| `ClaudeHeaderDefaults.OS` / `.Arch` / `.StabilizeDeviceProfile` | Removed from `internal/config/config.go`, sanitizer, `config.example.yaml`, and test assertions; added to removal-key migration. |

**Kept**: `ClaudeHeaderDefaults.UserAgent` / `.PackageVersion` / `.RuntimeVersion` / `.Timeout` — all have live callers in `claude_executor.go` (`Timeout` seeds the `X-Stainless-Timeout` header). `Timeout` was initially considered for removal but re-verification (checking accesses via the local `hd` variable, not the type name) found its live caller, so it stays.

---

## HTTP API & Handlers

### Disabled gemini-cli `/v1internal` endpoint

`POST /v1internal:method` was a localhost passthrough that gated entirely on `EnableGeminiCLIEndpoint` — which defaults to `false`, is unset in `data/config.yaml`, and was force-set to `false` by `forceHomeRuntimeConfig`. Every request returned `403`.

| Item | Disposition |
| --- | --- |
| `sdk/api/handlers/gemini/gemini-cli_handlers.go` | Deleted in full (`GeminiCLIAPIHandler`, `CLIHandler`, etc.). |
| `POST /v1internal:method` route + handler instantiation | Removed from `internal/api/server.go`. |
| `SDKConfig.EnableGeminiCLIEndpoint` field + `forceHomeRuntimeConfig` write + `config.example.yaml` entry | Removed. |

**Kept**: the regular `gemini.NewGeminiAPIHandler` (`/v1beta`) at the time; the `GeminiCLI` constant (used by registry/header_util); translator `FormatGeminiCLI` and `gemini-cli` auth branches (translator/auth-infra layer, out of scope).

### Gemini `/v1beta` handler subsystem

The `/v1beta/*` routes serve the `gemini` handler type, requiring a `gemini` provider in the auth conductor — absent from `data/config.yaml`. The entire subsystem was unreachable (no integration test exercises it; all code upstream-native).

| Item | Disposition |
| --- | --- |
| `sdk/api/handlers/gemini/` directory | Deleted in full (`GeminiAPIHandler` + methods). |
| `GET /v1beta/models`, `POST/GET /v1beta/models/*action` routes | Removed from `internal/api/server.go`. |
| `geminiModelsHandler`, `geminiGetHandler`, `handleHomeGeminiModels`, `handleHomeGeminiModel`, `formatHomeGeminiModels`, `formatHomeGeminiModel`, `homeGeminiModelMatches` | Removed — only called from the deleted routes. |

**Kept**: `FormatGemini` constant + the `gemini` case in `auth/conductor.go` (single case statements in otherwise-live switches); `internal/signature/gemini_validation.go` (called by `provider_compatibility.go` → Claude translator); all home-model infrastructure (serves the live `/v1/models` endpoint).

### Websocket + credits logging helpers

`internal/runtime/executor/helps/logging_helpers.go` carried two clusters with zero live callers. No live executor opens an upstream websocket on the `custom` branch, so the websocket-timeline recorders were never reached; `MarkCreditsUsed` had no callers so the flag was never set.

| Removed (websocket logging) | Removed (credits tracking) |
| --- | --- |
| `RecordAPIWebsocketRequest`, `RecordAPIWebsocketHandshake`, `RecordAPIWebsocketUpgradeRejection`, `WebsocketUpgradeRequestURL`, `AppendAPIWebsocketResponse`, `RecordAPIWebsocketError`, `appendAPIWebsocketTimeline`, `apiWebsocketTimelineSource`, `apiWebsocketTimelineKey` constant, `net/url` import | `MarkCreditsUsed`, `CreditsUsed`, `creditsUsedKey` constant |

**Kept**: the retained HTTP request-log helpers (`RecordAPIRequest`, `RecordAPIResponseMetadata`, `RecordAPIResponseError`, `AppendAPIResponseChunk`) are unchanged and wired into the live executors. The gin logger's own `creditsUsedKey` constant (in `internal/logging/gin_logger.go`) is unaffected.

---

## Executor Helpers (`internal/runtime/executor/helps/`)

### Codex cache helpers (`cache_helpers.go`)

The Codex prompt-cache helpers had zero callers after the Codex executor was removed.

| Removed |
| --- |
| `CodexCache` (type), `GetCodexCache`, `GetCodexCacheRequired`, `SetCodexCache`, `SetCodexCacheRequired`, `SetCodexCacheBestEffort`, `CodexPromptCacheKey`, and the private cleanup helpers (`codexCacheMap`, `codexCacheMu`, `startCodexCacheCleanup`, `purgeExpiredCodexCache`, etc.) |

`cache_helpers.go` + `cache_helpers_test.go` deleted in full.

### Session-id cache wrapper (`session_id_cache.go`)

`CachedSessionID` was a convenience wrapper around `CachedSessionIDRequired` that swallowed errors and fell back to a fresh UUID. After the request-time path became the only entrypoint wired into `claude_executor.go`, it had zero callers and was removed.

**Kept**: `CachedSessionIDRequired` (live caller at `claude_executor.go:1052`) and all helpers it uses (`startSessionIDCacheCleanup`, `purgeExpiredSessionIDs`, `sessionIDCacheKey`, `claudeSessionIDKVKey`, `currentClaudeIDKVClient`, `homekv.HashKeyPart`/`CurrentKVClient` usage).

### Payload config wrapper (`payload_helpers.go`)

`ApplyPayloadConfigWithRoot` was a thin wrapper forwarding to `ApplyPayloadConfigWithRequest` with empty `fromProtocol`/nil `headers`. Only `ApplyPayloadConfigWithRequest` (4 live callers) remained in use.

| Item | Disposition |
| --- | --- |
| `ApplyPayloadConfigWithRoot` | Removed. |
| `payload_helpers_disable_image_generation_test.go` | Deleted (every test called the removed function). |

**Kept**: `ApplyPayloadConfigWithRequest` and all its internal helpers.

### Google API retry parser + JSON field deleter

Two single-export helper files with zero callers.

| File | Symbol | Reason |
| --- | --- | --- |
| `retry_delay.go` | `ParseRetryDelay` | Parsed Google API 429 `RetryInfo.retryDelay`; only callers were the removed Codex/Gemini executors. |
| `json_helpers.go` | `DeleteJSONField` | Thin `sjson.DeleteBytes` wrapper; no live caller. |

Both files deleted in full.

### SSE usageMetadata filtering helpers (`usage_helpers.go`)

These stripped Gemini-style `usageMetadata` from non-terminal SSE chunks and existed only for the removed aistudio/antigravity executors.

| Removed |
| --- |
| `FilterSSEUsageMetadata`, `StripUsageMetadataFromJSON`, `JSONPayload`, and the orphaned sibling helpers `stopChunkWithoutUsage` (var), `rememberStopWithoutUsage`, `isStopChunkWithoutUsage`, `hasUsageMetadata`. |

The `sjson` import (used only by `StripUsageMetadataFromJSON`) was removed; the private `jsonPayload` helper was kept (still called by the live stream parsers).

---

## SDK & Plugin Plumbing

### `sdk/cliproxy/pipeline` package

The entire directory (containing only `context.go`) was a leftover from the removed plugin execution pipeline. None of its symbols had live callers.

| Removed | Kind |
| --- | --- |
| `Context` | struct |
| `Hook` | interface |
| `HookFunc` | struct |
| `RoundTripperProvider` | interface |

Directory deleted in full.

### Thinking plugin-provider registry (`internal/thinking/apply.go`)

A parallel registry let plugin-owned providers register thinking-config appliers alongside the built-in `claude`/`openai` ones. With no production code importing `sdk/pluginapi`, nothing ever called the register/unregister entrypoints — the `pluginProviderAppliers` map was always empty.

| Removed |
| --- |
| `pluginProviderApplier` (struct), `pluginProviderAppliers` (map var), `RegisterPluginProvider`, `UnregisterPluginProviders`, `ClearPluginProviders`. |

`GetProviderApplier` was simplified to return `nativeProviderAppliers[provider]` directly — behaviour unchanged (the plugin map was always empty). The retained `nativeProviderAppliers` map and `RegisterProvider` function are unchanged.

> Note: `sdk/pluginapi/` itself was removed in [removed-plugins.md](./removed-plugins.md).

---

## Usage

### `PersistPlugin.QueryUsage` (`internal/usage/persist_plugin.go`)

`QueryUsage` (one authID + one model) overlapped with `QueryUsageMulti` (one authID + a model list, empty list = wildcard). All live callers go through `QueryUsageMulti`.

| Item | Disposition |
| --- | --- |
| `PersistPlugin.QueryUsage` method | Removed. |
| `TestPersistStoreQueryUsage` | Removed. |

Four other tests that used `QueryUsage` purely as a read-back helper were migrated to the equivalent `QueryUsageMulti(authID, []string{model}, from, to)` form (assertions unchanged). `QueryUsageMulti` and `QueryFullUsageReport` retained — both have live callers.

---

## Translation Helpers (`internal/translator/common/bytes.go`)

Two JSON-building helpers existed only for the removed GeminiCLI provider's HTTP shape.

| Removed | Reason |
| --- | --- |
| `WrapGeminiCLIResponse` | Wrapped a payload as `{"response": <body>}`; no live caller. |
| `GeminiTokenCountJSON` | Built the GeminiCLI `countTokens` response; no live caller. |

The `sjson` import (used only by `WrapGeminiCLIResponse`) was removed; `strconv` kept (`ClaudeInputTokensJSON` still uses it). **Kept**: `ClaudeInputTokensJSON`, `SSEEventData`, `AppendSSEEventString`, `AppendSSEEventBytes`.

---

## Misc utilities (`internal/misc/`)

| File | Removed symbols |
| --- | --- |
| `mime-type.go` | `misc.MimeTypes` |
| `oauth.go` | `misc.GenerateRandomState`, `misc.ParseOAuthCallback`, `misc.OAuthCallback`, `misc.AsyncPrompt` |
| `copy-example-config.go` | `misc.CopyConfigTemplate` |
| `claude_code_instructions.go` + `.txt` | `misc.ClaudeCodeInstructions` |

`credentials.go` trimmed: removed `LogSavingCredentials` and `MergeMetadata` (zero callers); **kept** `LogCredentialSeparator` (called from `model_registry.go`).

---

## Removed Directories & Go Dependencies

### Removed directories

| Path | Reason |
| --- | --- |
| `internal/htmlsanitize/` | Zero importers after browser-login/OAuth removal. |
| `internal/cache/` | Zero importers — request-signature cache wiring removed. |
| `sdk/pluginabi/` | Plugin host removed; no non-test callers. |
| `examples/plugin/` | Entire plugin examples tree; only built against the removed plugin host. |

### Unused Go dependencies (`go mod tidy`)

After the importing code was deleted, these lingered in `go.mod`/`go.sum`:

| Dependency | Last user |
| --- | --- |
| `golang.org/x/oauth2` | management API tool handler + `internal/misc/oauth.go` (both deleted). |
| `cloud.google.com/go/compute/metadata` | indirect via `oauth2/google`. |
| `github.com/refraction-networking/utls` | `utls_client.go` (deleted — see [removed-utls.md](./removed-utls.md)). |
| `github.com/charmbracelet/bubbletea`, `bubbles`, `lipgloss`, `atotto/clipboard` | TUI (see [removed-tui-commands.md](./removed-tui-commands.md)). |
| `github.com/go-git/go-git/v6`, `jackc/pgx/v5`, `minio/minio-go/v7` | storage backends (see [removed-storage-relay.md](./removed-storage-relay.md)). |
| `github.com/skratchdot/open-golang` | browser login (see [removed-providers-oauth.md](./removed-providers-oauth.md)). |

> Note: `github.com/redis/go-redis/v9` and its indirects were later removed when the redis-backed `internal/home` was deleted — see [removed-redis-home.md](./removed-redis-home.md).

### AGENTS.md update

Two stale architecture lines referencing deleted code were removed: `internal/api/modules/amp/` (never existed) and `internal/cache/` (deleted).
