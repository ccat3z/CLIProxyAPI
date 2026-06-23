# Removed: Dead Code and Unused Dependencies

After the feature-level removals (plugins, providers, storage, TUI, config, utls), a final sweep deleted directories, files, and standalone functions with zero callers, then ran `go mod tidy` to drop dependencies that were no longer imported by the build.

## Removed Directories

| Path | Reason |
| --- | --- |
| `internal/htmlsanitize/` | Zero importers after the browser-login / OAuth flows were removed. |
| `internal/cache/` | Zero importers — request-signature cache wiring was removed. |
| `sdk/pluginabi/` | Plugin host was removed; package had no non-test, non-example callers. |
| `examples/plugin/` | Entire plugin examples tree. Only built against the removed plugin host. |

### Note on `sdk/pluginapi/`

`sdk/pluginapi/` is **not** dead: it remains imported by the intentionally-retained extension-point interfaces listed in [removed-plugins.md](./removed-plugins.md) (`PluginInterceptorHost`, `PluginModelRouterHost`, `PluginExecutorHost`, `PluginScheduler`, `PluginAuthParser`, `PluginHooks`) and is in the `cmd/server` dependency tree. It was kept.

## Removed: `internal/runtime/executor/helps/cache_helpers.go`

After the Codex executor was removed in an earlier cleanup, the Codex prompt-cache helpers in this file had zero callers. Both the implementation and its test were deleted in full.

| Symbol | Kind | Reason |
| --- | --- | --- |
| `CodexCache` | type | Zero callers outside the file. |
| `GetCodexCache` | function | Zero callers. |
| `GetCodexCacheRequired` | function | Zero callers. |
| `SetCodexCache` | function | Zero callers. |
| `SetCodexCacheRequired` | function | Zero callers. |
| `SetCodexCacheBestEffort` | function | Zero callers. |
| `CodexPromptCacheKey` | function | Zero callers. |

Each symbol was re-verified with `grep -rn "<symbol>" --include="*.go" . | grep -v _test.go | grep -v cache_helpers.go` before deletion — all returned no matches. The file's other private helpers (`codexCacheMap`, `codexCacheMu`, `startCodexCacheCleanup`, `purgeExpiredCodexCache`, `codexCacheCleanupOnce`, `codexCacheCleanupInterval`) supported only the removed symbols and went with them.

The companion `cache_helpers_test.go` only exercised `SetCodexCacheRequired` against the (now-removed) Codex cache, so it was deleted in full.

Note: `homekv` (`internal/home`) is still imported by `session_id_cache.go` in the same package, so removing this file did not orphan that dependency.

## Removed Files in `internal/misc/`

| File | Symbols verified with zero callers |
| --- | --- |
| `mime-type.go` | `misc.MimeTypes` |
| `oauth.go` | `misc.GenerateRandomState`, `misc.ParseOAuthCallback`, `misc.OAuthCallback`, `misc.AsyncPrompt` |
| `copy-example-config.go` | `misc.CopyConfigTemplate` |
| `claude_code_instructions.go` + `claude_code_instructions.txt` | `misc.ClaudeCodeInstructions` |

## Trimmed: `internal/misc/credentials.go`

The file previously held three helpers. Two had zero callers and were removed; the third is still used by the model registry and was kept.

| Function | Disposition |
| --- | --- |
| `LogSavingCredentials` | Removed (zero callers) |
| `MergeMetadata` | Removed (zero callers) |
| `LogCredentialSeparator` | **Kept** — called from `internal/registry/model_registry.go` |

The file now contains only `credentialSeparator` and `LogCredentialSeparator`.

## AGENTS.md Updates

Removed two stale architecture lines that referenced deleted code:

- `internal/api/modules/amp/` — directory never existed.
- `internal/cache/` — package deleted.

## Key Deleted Paths

- `internal/htmlsanitize/htmlsanitize.go`, `internal/htmlsanitize/htmlsanitize_test.go`
- `internal/cache/signature_cache.go`, `internal/cache/signature_cache_test.go`
- `sdk/pluginabi/types.go`, `sdk/pluginabi/types_test.go`
- `examples/plugin/` (full tree: `auth/`, `cli/`, `executor/`, `host-callback/`, `host-model-callback/`, `management-api/`, `model/`, `protocol-format/`, `request-normalizer/`, `request-translator/`, `response-normalizer/`, `response-translator/`, `scheduler/`, `simple/`, `thinking/`, `usage/`, `claude-web-search-router/`, `codex-service-tier/`, `frontend-auth/`, `frontend-auth-exclusive/`, `host-callback-auth-files/`, plus `Makefile`, `README.md`, `README_CN.md`, `scripts/`)
- `internal/misc/mime-type.go`
- `internal/misc/oauth.go`
- `internal/misc/copy-example-config.go`
- `internal/misc/claude_code_instructions.go`
- `internal/misc/claude_code_instructions.txt`

## Unused Go Dependencies

After the code that imported these packages was deleted, the entries lingered in `go.mod` / `go.sum`. `go mod tidy` drops them, leaving only dependencies that are still actually imported by the build.

| Dependency | Type | Why it is now unused |
| --- | --- | --- |
| `golang.org/x/oauth2` | direct | Last user was the management API tool handler in `internal/api/handlers/management/api_tools.go`, deleted with the Antigravity residual cleanup. The remaining `internal/misc/oauth.go` helpers were deleted as dead code. No Go source imports this package anymore. |
| `cloud.google.com/go/compute/metadata` | indirect | Pulled in transitively by `golang.org/x/oauth2/google`. With `oauth2` gone, the indirect is gone too. |
| `github.com/refraction-networking/utls` | direct | Last user was `internal/runtime/executor/helps/utls_client.go`, deleted when the Claude executor switched to the standard `http.Client` via `helps.NewProxyAwareHTTPClient`. See [removed-utls.md](./removed-utls.md). |
| `github.com/charmbracelet/bubbletea` | direct | TUI framework. See [removed-tui-commands.md](./removed-tui-commands.md). |
| `github.com/charmbracelet/bubbles` | direct | TUI components. See [removed-tui-commands.md](./removed-tui-commands.md). |
| `github.com/charmbracelet/lipgloss` | direct | TUI styling. See [removed-tui-commands.md](./removed-tui-commands.md). |
| `github.com/atotto/clipboard` | direct | Clipboard for TUI. See [removed-tui-commands.md](./removed-tui-commands.md). |
| `github.com/go-git/go-git/v6` | direct | Git store backend. See [removed-storage-relay.md](./removed-storage-relay.md). |
| `github.com/jackc/pgx/v5` | direct | PostgreSQL store backend. See [removed-storage-relay.md](./removed-storage-relay.md). |
| `github.com/minio/minio-go/v7` | direct | Object store backend. See [removed-storage-relay.md](./removed-storage-relay.md). |
| `github.com/skratchdot/open-golang` | direct | Browser login. See [removed-providers-oauth.md](./removed-providers-oauth.md). |

## What Was Kept

`github.com/redis/go-redis/v9` stays in `go.mod`. It is still imported by `internal/home/` (Redis-based control-plane client), which was retained on the `custom` branch. The associated indirects (`github.com/cespare/xxhash/v2`, `go.uber.org/atomic`) also remain.

## Removed: dead usage parsers

After the Codex / Gemini / Gemini-CLI / Antigravity executors were removed in earlier cleanups, the `Parse*Usage` / `Parse*StreamUsage` helpers in `internal/runtime/executor/helps/usage_helpers.go` that existed only to serve those executors had zero callers. The `custom` branch only uses the `claude-api-key` and `openai-compatibility` providers, so the OpenAI and Claude parsers are the only ones still wired in.

| Function | Disposition | Reason |
| --- | --- | --- |
| `ParseCodexUsage` | Removed | Zero callers outside the file and tests. |
| `ParseCodexImageToolUsage` | Removed | Zero callers. |
| `ParseGeminiUsage` | Removed | Zero callers. |
| `ParseGeminiStreamUsage` | Removed | Zero callers. |
| `ParseGeminiCLIUsage` | Removed | Zero callers. |
| `ParseGeminiCLIStreamUsage` | Removed | Zero callers. |
| `ParseAntigravityUsage` | Removed | Zero callers. |
| `ParseAntigravityStreamUsage` | Removed | Zero callers. |

The following private helpers were used **only** by the removed parsers and were deleted alongside them:

| Helper | Reason |
| --- | --- |
| `parseGeminiFamilyUsageDetail` | Only callers were the removed Gemini-family / Antigravity parsers. |
| `hasGeminiFamilyUsageTokenFields` | Only caller was the removed `ParseGeminiCLIStreamUsage`. |
| `firstExistingUsageNode` | Only callers were the removed `ParseGeminiCLIUsage` and `ParseGeminiCLIStreamUsage`. |

Each symbol was re-verified with `grep -rn "<symbol>" --include="*.go" . | grep -v _test.go | grep -v usage_helpers.go` before deletion — all returned no matches.

**Kept** (still have live callers in the OpenAI-compat and Claude executors):

- `ParseOpenAIUsage`, `ParseOpenAIStreamUsage`, plus their shared helpers `hasOpenAIStyleUsageTokenFields` / `parseOpenAIStyleUsageNode`.
- `ParseClaudeUsage`, `ParseClaudeStreamUsage`, plus `parseClaudeUsageNode`.

The matching test cases in `usage_helpers_test.go` (`TestParseGeminiStreamUsage_NullUsageMetadata`, `TestParseGeminiCLIStreamUsage_NullUsageMetadata`, `TestParseAntigravityStreamUsage_NullUsageMetadata`, `TestParseGeminiCLIUsage_TopLevelUsageMetadata`, `TestParseGeminiCLIStreamUsage_ResponseSnakeCaseUsageMetadata`, `TestParseGeminiCLIStreamUsage_IgnoresTrafficTypeOnlyUsageMetadata`) were removed; the OpenAI and Claude tests were left untouched.

## Removed: dead thinking providers

After the Codex / Gemini / Gemini-CLI / Kimi / xAI executors were removed in earlier cleanups, their thinking-config appliers in `internal/thinking/provider/` still self-registered at startup via blank imports in `internal/runtime/executor/helps/thinking_providers.go`. No live caller dispatched to those provider names, so the packages were dead weight.

### Deleted directories

| Path | Reason |
| --- | --- |
| `internal/thinking/provider/codex/` | Only self-registers via blank import; no live caller dispatches to `"codex"`. |
| `internal/thinking/provider/gemini/` | Only self-registers via blank import; no live caller dispatches to `"gemini"`. |
| `internal/thinking/provider/geminicli/` | Only self-registers via blank import; no live caller dispatches to `"gemini-cli"`. |
| `internal/thinking/provider/kimi/` | Only self-registers via blank import; no live caller dispatches to `"kimi"`. |
| `internal/thinking/provider/xai/` | Embeds `codex.Applier`; only self-registers via blank import; no live caller dispatches to `"xai"`. |

`internal/thinking/provider/claude/` and `internal/thinking/provider/openai/` were kept (live providers).

### `internal/runtime/executor/helps/thinking_providers.go`

The five blank imports for the deleted packages were removed; the `claude` and `openai` imports were kept.

### `internal/thinking/apply.go` trims

- `nativeProviderAppliers` map: dropped keys `gemini`, `gemini-cli`, `codex`, `antigravity`, `kimi`, `xai`. Only `claude` and `openai` remain.
- `extractThinkingConfig` switch: dropped the `case "gemini", "gemini-cli", "antigravity":`, `case "codex", "xai":`, and `case "kimi":` branches (unreachable — corresponding executors are gone). Kept `case "claude":`, `case "openai":`, and the `default:` branch.
- `extractGeminiConfig` helper: deleted. After the gemini switch branch was removed it had zero callers (re-verified with `grep -rn "extractGeminiConfig" --include="*.go" .` — only the definition remained).

### Kept: `extractCodexConfig`

`extractCodexConfig` is **not** dead: it is still called as a fallback from `ExtractReasoningEffort` and `ExtractTranslatedReasoningEffort` for `openai` / `openai-response` providers (which are live in the custom branch). It was intentionally kept, along with `extractOpenAIConfig` and `extractClaudeConfig`.

### Tests

No tests in `internal/thinking/` exercised the deleted providers (the per-provider `apply_test.go` files lived inside the deleted packages and went with them). The top-level `apply_user_defined_test.go` and `reasoning_effort_test.go` only cover the retained `claude` / `openai` paths and were left untouched.

## Removed: dead quota config (SwitchProject/SwitchPreviewModel)

`QuotaExceeded.SwitchProject` and `QuotaExceeded.SwitchPreviewModel` were parsed, diffed, and exposed via the management API, but no runtime code read them to make routing decisions — they were leftovers from the removed provider infrastructure. The `custom` branch only uses `claude-api-key` and `openai-compatibility` providers with API keys, so the automatic project / preview-model failover paths are unreachable.

| Item | Disposition |
| --- | --- |
| `config.QuotaExceeded.SwitchProject` field | Removed from `internal/config/config.go`. |
| `config.QuotaExceeded.SwitchPreviewModel` field | Removed from `internal/config/config.go`. |
| `GetSwitchProject`, `PutSwitchProject`, `GetSwitchPreviewModel`, `PutSwitchPreviewModel` handlers | Removed — file `internal/api/handlers/management/quota.go` deleted in full (it contained only these four handlers). |
| `GET/PUT/PATCH /v0/management/quota-exceeded/switch-project` routes | Removed from `internal/api/server.go`. |
| `GET/PUT/PATCH /v0/management/quota-exceeded/switch-preview-model` routes | Removed from `internal/api/server.go`. |
| `quota-exceeded.switch-project` / `quota-exceeded.switch-preview-model` diff entries | Removed from `internal/watcher/diff/config_diff.go`. |
| `config.example.yaml` `switch-project` / `switch-preview-model` lines | Removed. |

`removeRemovedIntegrationKeys` in `internal/config/config.go` was extended (via a new `removeNestedMapKey` helper) to strip `switch-project` and `switch-preview-model` from the `quota-exceeded` mapping of existing user config files on the next save, so stale values are cleaned up automatically.

The matching assertions in `internal/watcher/diff/config_diff_test.go` (two `QuotaExceeded{...}` struct literals and four `expectContains(...)` lines for the removed diff keys) were trimmed. The `QuotaExceeded` struct is now empty but retained so existing YAML `quota-exceeded:` blocks (e.g. `antigravity-credits`) continue to deserialize without error.

## Removed: `internal/auth` package + `Auth.Storage` field

The `custom` branch only uses the `claude-api-key` and `openai-compatibility` providers with API keys. After the OAuth removal, the `internal/auth/` package existed only to define the `TokenStorage` interface, and it had no live implementations. The `Auth.Storage` field (typed `baseauth.TokenStorage`) was read in `sdk/auth/filestore.go` but never assigned anywhere — it was always `nil`, so the `case auth.Storage != nil:` branch in `FileTokenStore.Save` was dead code.

| Item | Disposition |
| --- | --- |
| `internal/auth/` directory (`models.go`) | Deleted in full — only defined `TokenStorage`, which had no live implementations or importers after the field removal. |
| `Auth.Storage` field (`sdk/cliproxy/auth/types.go`) | Removed. |
| `baseauth "github.com/router-for-me/CLIProxyAPI/v7/internal/auth"` import (`sdk/cliproxy/auth/types.go`) | Removed (only used by the deleted field). |
| `case auth.Storage != nil:` branch in `FileTokenStore.Save` (`sdk/auth/filestore.go`) | Removed — dead branch (field was always `nil`). |
| Local `metadataSetter` interface in `FileTokenStore.Save` (`sdk/auth/filestore.go`) | Removed — only used by the deleted branch. |
| `sdk/auth/filestore_disabled_test.go` | Deleted in full — only exercised the removed `Storage`-based branch via a `testTokenStorage` stub. |

The live `case auth.Metadata != nil:` branch (which persists auth metadata as JSON) was kept and is the only remaining save path. The `Storage` field deletion and the dead-branch removal shipped in the same commit so the build never broke.

Each reference was re-verified with `grep -rn "internal/auth\|TokenStorage\|baseauth" --include="*.go" . | grep -v "internal/auth/"` after the edits — zero matches.

## Removed: dead signature helpers (antigravity/gemini)

The `custom` branch only uses the `claude-api-key` and `openai-compatibility` providers with API keys. After the Gemini and Antigravity providers were removed in earlier cleanups, the signature helpers below had zero callers outside `internal/signature/` and were deleted.

| Symbol | File | Kind | Reason |
| --- | --- | --- | --- |
| `CompatibleAntigravityClaudeThinkingSignature` | `internal/signature/provider_compatibility.go` | function | Only Antigravity replay used it; Antigravity provider was removed. |
| `SanitizeGeminiRequestThoughtSignatures` | `internal/signature/gemini_sanitize.go` | function | Only Gemini upstream replay used it; Gemini provider was removed. |
| `GeminiReplaySignatureOrBypass` | `internal/signature/gemini_sanitize.go` | function | Only called by `SanitizeGeminiRequestThoughtSignatures`. |
| `logGeminiThoughtSignatureSanitize` | `internal/signature/gemini_sanitize.go` | helper | Only called by `SanitizeGeminiRequestThoughtSignatures`. |
| `geminiPartThoughtSignature` | `internal/signature/gemini_sanitize.go` | helper | Only called by `SanitizeGeminiRequestThoughtSignatures`. |
| `deleteGeminiPartThoughtSignatureFields` | `internal/signature/gemini_sanitize.go` | helper | Only called by `SanitizeGeminiRequestThoughtSignatures`. |

After removing the two public functions, `gemini_sanitize.go` contained only those two functions plus their private helpers, so the entire file was deleted. The companion `gemini_sanitize_test.go` only exercised the removed symbols (plus its local `newSignatureDebugHook`/`assertSignatureDebugDoesNotLeak` helpers, which had no other callers) and was deleted in full. The two `TestCompatibleAntigravityClaudeThinkingSignature_*` cases in `provider_compatibility_test.go` were trimmed; the rest of that file is untouched.

Each symbol was re-verified with `grep -rn "<symbol>" --include="*.go" . | grep -v _test.go | grep -v internal/signature/` before deletion — all returned zero matches. The rest of `internal/signature/` is retained: it is still imported by `claude_executor.go`, `openai_responses_signature.go`, and the `internal/translator/` packages.

## Removed: dead translator helpers (GeminiCLI leftovers)

The `custom` branch only uses the `claude-api-key` and `openai-compatibility` providers with API keys. After the GeminiCLI provider was removed in an earlier cleanup, two JSON-building helpers in `internal/translator/common/bytes.go` existed only to shape that provider's responses and had zero callers.

| Symbol | File | Kind | Reason |
| --- | --- | --- | --- |
| `WrapGeminiCLIResponse` | `internal/translator/common/bytes.go` | function | Wrapped a raw payload as `{"response": <body>}` for the GeminiCLI HTTP shape; no live caller after the provider was removed. |
| `GeminiTokenCountJSON` | `internal/translator/common/bytes.go` | function | Built the GeminiCLI `countTokens` response JSON (`{"totalTokens": ..., "promptTokensDetails": [...]}`); no live caller after the provider was removed. |

The `github.com/tidwall/sjson` import in `bytes.go` was used solely by `WrapGeminiCLIResponse` and was removed alongside it. The `strconv` import was kept — `ClaudeInputTokensJSON` (still called by `internal/translator/openai/claude/openai_claude_response.go`) uses `strconv.AppendInt`. There are no `_test.go` files in `internal/translator/common/` and no test references to either symbol elsewhere.

Each symbol was re-verified with `grep -rn "<symbol>" --include="*.go" . | grep -v _test.go | grep -v common/bytes.go` before deletion — all returned zero matches. The remaining functions in `bytes.go` were kept: `ClaudeInputTokensJSON` (live caller noted above), `SSEEventData`, `AppendSSEEventString`, and `AppendSSEEventBytes`.

## Removed: `sdk/cliproxy/pipeline` package

The entire `sdk/cliproxy/pipeline/` directory (containing only `context.go`) was a leftover from the removed plugin execution pipeline. None of the symbols it defined had any live callers after the plugin host was removed in an earlier cleanup — it was kept around as dead weight.

| Symbol | Kind | Reason |
| --- | --- | --- |
| `Context` | struct | Encapsulated execution state shared across middleware/translators/executors; zero callers outside the package. |
| `Hook` | interface | Middleware callback contract (`BeforeExecute`/`AfterExecute`/`OnStreamChunk`); zero implementors or dispatchers. |
| `HookFunc` | struct | Functional adapter implementing `Hook`; only used by the (removed) plugin pipeline. |
| `RoundTripperProvider` | interface | Per-auth HTTP transport injection point; zero implementors or callers. |

The directory was deleted in full (no `_test.go` files existed). Each symbol was re-verified before deletion with:

```
grep -rn "cliproxy/pipeline" --include="*.go" . | grep -v _test.go | grep -v "sdk/cliproxy/pipeline/"
grep -rn "pipeline\.Context\|pipeline\.Hook\|pipeline\.HookFunc\|pipeline\.RoundTripperProvider" --include="*.go" . | grep -v "sdk/cliproxy/pipeline/"
```

Both returned zero matches. `gofmt`, `go build`, `go test ./...`, `pytest integration/`, and the standard smoke test (`/v1/models`, `/v1/chat/completions`, `/v0/management/config`) all pass.

## Removed: dead websocket and credits logging helpers

`internal/runtime/executor/helps/logging_helpers.go` carried two clusters of helpers with zero live callers on the `custom` branch: the upstream-websocket request-log recorders, and the AI-credits flag accessors. Both are leftovers from the removed provider/OAuth infrastructure. The retained HTTP request-log helpers (`RecordAPIRequest`, `RecordAPIResponseMetadata`, `RecordAPIResponseError`, `AppendAPIResponseChunk`) are unchanged and still wired into the live executors.

### Websocket logging

No live executor opens an upstream websocket on the `custom` branch, so the websocket-timeline recorders were never reached. The six public functions below were removed, along with the two private helpers that only they called.

| Symbol | Kind | Reason |
| --- | --- | --- |
| `RecordAPIWebsocketRequest` | function | No callers outside the file. |
| `RecordAPIWebsocketHandshake` | function | No callers outside the file. |
| `RecordAPIWebsocketUpgradeRejection` | function | No callers outside the file. |
| `WebsocketUpgradeRequestURL` | function | No callers outside the file. |
| `AppendAPIWebsocketResponse` | function | No callers outside the file. |
| `RecordAPIWebsocketError` | function | No callers outside the file. |
| `appendAPIWebsocketTimeline` | helper | Only called by the six functions above. |
| `apiWebsocketTimelineSource` | helper | Only called by `appendAPIWebsocketTimeline`. |

The `net/url` import was used solely by `WebsocketUpgradeRequestURL` and was removed with it. The `apiWebsocketTimelineKey` context-key constant (`"API_WEBSOCKET_TIMELINE"`) was only written by `appendAPIWebsocketTimeline` and was removed; the gin-context reader `ResponseWriterWrapper.extractAPIWebsocketTimeline` in `internal/api/middleware/response_writer.go` reads the same key as a string literal and is out of scope for this change — it now simply returns `nil`, matching the pre-existing behaviour where nothing populated that key. The separate `logging.APIWebsocketTimelineSourceContextKey` / `FileBodySource` plumbing (still driven by `request_logging.go`) is unrelated and untouched.

### Credits tracking

`MarkCreditsUsed` / `CreditsUsed` flagged a request as having consumed AI credits. `MarkCreditsUsed` had no callers, so the flag was never set and `CreditsUsed` always returned `false`. The gin logger in `internal/logging/gin_logger.go` reads the same key via its own identically-valued private `creditsUsedKey` constant, which is unaffected by this change.

| Symbol | Kind | Reason |
| --- | --- | --- |
| `MarkCreditsUsed` | function | No callers outside the file; the flag was never set. |
| `CreditsUsed` | function | No callers outside the file. |
| `creditsUsedKey` | constant | Only used by the two functions above. |

### Verification

Each public symbol was re-verified before deletion with `grep -rn "<symbol>" --include="*.go" . | grep -v helps/logging_helpers.go` — all returned zero matches (the apparent `apiWebsocketTimelineSource` hits in `response_writer.go` / `request_logger.go` are unrelated local variables and parameters, not calls). `gofmt`, `go build`, `go test ./...`, `pytest integration/` (37 passed), and the standard smoke test (`/v1/models` → 200, `/v1/chat/completions` → 200, `/v0/management/config` → 401) all pass.

## Removed: dead Google API retry parser and unused JSON field deleter

Two single-export helper files in `internal/runtime/executor/helps/` had zero callers on the `custom` branch and were deleted in full.

### Google API 429 retry-delay parser

`retry_delay.go` exported `ParseRetryDelay`, which extracted the retry delay from a Google API 429 error response (parsing `RetryInfo.retryDelay`, `ErrorInfo.metadata.quotaResetDelay`, and a human-readable "Your quota will reset after Xs" fallback). Its only callers were the removed Codex and Gemini executors; after those were deleted in earlier cleanups, the function was unreachable.

| Symbol | File | Kind | Reason |
| --- | --- | --- | --- |
| `ParseRetryDelay` | `internal/runtime/executor/helps/retry_delay.go` | function | Zero callers outside the file. Codex/Gemini callers removed earlier. |

The file's `github.com/tidwall/gjson`, `regexp`, `strconv`, `strings`, and `fmt` imports were used only by this function. `gjson` remains widely imported elsewhere.

### JSON field deleter

`json_helpers.go` exported `DeleteJSONField`, a thin wrapper around `sjson.DeleteBytes`. No live caller remained on the `custom` branch.

| Symbol | File | Kind | Reason |
| --- | --- | --- | --- |
| `DeleteJSONField` | `internal/runtime/executor/helps/json_helpers.go` | function | Zero callers outside the file. |

The file's `github.com/tidwall/sjson` import was used solely by this function. `sjson` remains widely imported elsewhere.

### Verification

Each symbol was re-verified before deletion with `grep -rn "ParseRetryDelay\|DeleteJSONField" --include="*.go" .` — only the definitions themselves appeared, with zero external callers and zero test references. `gofmt`, `go build`, `go test ./...`, `pytest integration/` (37 passed), and the standard smoke test (`/v1/models` → 200, `/v1/chat/completions` → 200, `/v0/management/config` → 401) all pass.

