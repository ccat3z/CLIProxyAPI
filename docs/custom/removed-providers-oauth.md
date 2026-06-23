# Removed: Non-Essential Providers, OAuth Flows, Browser Login

The `custom` branch only proxies to Claude-compatible and OpenAI-compatible upstreams using API keys. Every other provider executor, OAuth flow, browser login helper, and OAuth management route has been removed, along with the OAuth-specific code paths that lingered inside the Claude executor and the residual Antigravity plumbing.

## Removed Provider Executors

The following executor implementations were deleted from `internal/runtime/executor/`:

- `codex_executor.go` and related codex files (`codex_websockets_executor.go`, `codex_openai_images.go`, reasoning replay cache)
- `gemini_executor.go`, `gemini_vertex_executor.go`
- `gemini-cli` runtime support (executor and shared state)
- `antigravity_executor.go`
- `aistudio_executor.go` (and test) — depended on `internal/wsrelay/`
- `gemini_cli_executor.go` (and test) — depended on `internal/runtime/geminicli/`
- `kimi_executor.go`, `xai_executor.go`, `xai_websockets_executor.go`

`baselineExecutorAuths()` in `sdk/cliproxy/service.go` now only seeds `claude` and `openai-compatibility`, and `registerExecutorForAuth` only branches on `claude` (plus the OpenAI-compat fallback).

## Removed Auth Packages

All provider-specific auth packages were removed:

- `internal/auth/codex/`, `internal/auth/gemini/`, `internal/auth/antigravity/`, `internal/auth/kimi/`, `internal/auth/xai/`, `internal/auth/vertex/`, `internal/auth/empty/`, `internal/auth/claude/`
- `sdk/auth/codex.go`, `sdk/auth/codex_device.go`, `sdk/auth/antigravity.go`, `sdk/auth/kimi.go`, `sdk/auth/xai.go`, `sdk/auth/gemini.go`, `sdk/auth/claude.go`, `sdk/auth/refresh_registry.go`

`sdk/cliproxy/service.go`'s `newDefaultAuthManager` was simplified to `sdkAuth.NewManager(sdkAuth.GetTokenStore())` with no provider-specific authenticators.

## Removed Translators

Provider-specific translator packages under `internal/translator/` were removed:

- `codex/`, `gemini/`, `gemini-cli/`, `antigravity/`
- `claude/gemini/`, `claude/gemini-cli/`
- `openai/gemini/`, `openai/gemini-cli/`

`internal/translator/init.go` now only imports:

```go
_ "github.com/router-for-me/CLIProxyAPI/v7/internal/translator/claude/openai/chat-completions"
_ "github.com/router-for-me/CLIProxyAPI/v7/internal/translator/claude/openai/responses"
_ "github.com/router-for-me/CLIProxyAPI/v7/internal/translator/openai/claude"
_ "github.com/router-for-me/CLIProxyAPI/v7/internal/translator/openai/openai/chat-completions"
_ "github.com/router-for-me/CLIProxyAPI/v7/internal/translator/openai/openai/responses"
```

## Removed Browser + Login Commands

- `internal/browser/` directory deleted.
- `internal/cmd/login.go`, `anthropic_login.go`, `openai_login.go`, `openai_device_login.go`, `vertex_import.go`, `antigravity_login.go`, `kimi_login.go`, `xai_login.go`, `auth_manager.go` deleted.

`cmd/server/main.go` no longer registers or handles login-related flags:

- Removed: `--login`, `--codex-login`, `--codex-device-login`, `--claude-login`, `--antigravity-login`, `--kimi-login`, `--xai-login`, `--no-browser`, `--oauth-callback-port`, `--project_id`, `--vertex-import`, `--vertex-import-prefix`

## Removed OAuth Management Handlers

OAuth callback routes and token request endpoints were removed from `internal/api/server.go` and `internal/api/handlers/management/`:

- Routes: `/anthropic/callback`, `/codex/callback`, `/google/callback`, `/antigravity/callback`, `/xai/callback`, `/vertex/import`, `/oauth-callback`, and the per-provider `-auth-url` management endpoints.
- Handler files: `vertex_import.go`, `oauth_callback.go`, `oauth_sessions.go`.
- The shared callback forwarder HTTP server (`startCallbackForwarder`, `stopCallbackForwarderInstance`, etc.) was removed from `auth_files.go`.
- All `Request*Token` helpers, `extractCodexIDTokenClaims`, `ServePluginAuthURL`, and the `managementCallbackURL` plumbing were removed.
- `GetAuthStatus` was simplified to a pass-through status response.

`sdk/api/management.go` was trimmed to only `NewHandler`, `NewHandlerWithoutConfigFilePath`, `WriteConfig`, and `PopulateAuthContext`. The `ManagementTokenRequester` interface, OAuth session helpers, and OAuth callback file helpers are gone.

## Claude Executor OAuth Code Paths

All OAuth-only behaviour inside the Claude executor is gone: tool name remapping to defeat Anthropic fingerprinting, automatic cch signing for `sk-ant-oat` tokens, and OAuth session credential fallback.

### OAuth tool name remapping (fingerprint evasion)

Anthropic uses tool name patterns to detect third-party clients on OAuth traffic and bill them extra usage. The entire tool name remapping machinery is gone:

- `oauthToolRenameMap`, `oauthToolsToRemove`, `claudeToolPrefix` constants/vars
- `isClaudeOAuthToken(apiKey)` helper (detected `sk-ant-oat` tokens)
- `prepareClaudeOAuthToolNamesForUpstream`
- `restoreClaudeOAuthToolNamesFromResponse`
- `restoreClaudeOAuthToolNamesFromStreamLine`
- `remapOAuthToolNames`, `reverseRemapOAuthToolNames`, `reverseRemapOAuthToolNamesFromStreamLine`
- `applyClaudeToolPrefix`, `stripClaudeToolPrefixFromResponse`, `stripClaudeToolPrefixFromStreamLine`
- `sanitizeForwardedSystemPrompt` (only ever called when `oauthMode=true`)
- `defaultClaudeBuiltinToolNames`, `newClaudeBuiltinToolRegistry`, `augmentClaudeBuiltinToolRegistry` from `claude_cloak_helpers.go` (only consumed by the prefix logic)

The conditional branches that invoked these helpers in `Execute`, `ExecuteStream`, and `CountTokens` were deleted.

### `oauthMode` plumbing in system-prompt injection

`checkSystemInstructionsWithSigningMode` lost its `oauthMode bool` parameter (it was always set from `isClaudeOAuthToken` and is now always false). The `sanitizeForwardedSystemPrompt` short-circuit that depended on it is removed; third-party system context is forwarded verbatim (subject to the existing `cloak.strict-mode` opt-in).

### Automatic cch signing for OAuth tokens

The `if oauthToken || experimentalCCHSigningEnabled(...)` branches in `Execute`, `ExecuteStream`, and `applyCloaking` now read:

```go
if experimentalCCHSigningEnabled(e.cfg, auth) {
    bodyForUpstream = signAnthropicMessagesBody(bodyForUpstream)
}
```

Per-credential cch signing is still available via `claude-api-key.experimental-cch-signing`.

### `access_token` fallback in `claudeCreds`

`claudeCreds(auth)` no longer reads `auth.Metadata["access_token"]`. The OAuth flow that populated that field is gone; only `auth.Attributes["api_key"]` and `auth.Attributes["base_url"]` are read.

### Tests removed from `internal/runtime/executor/claude_executor_test.go`

- All `TestApplyClaudeToolPrefix*` cases (10 tests)
- All `TestStripClaudeToolPrefix*` cases (4 tests)
- All `TestRemapOAuthToolNames*` cases (3 tests)
- `TestReverseRemapOAuthToolNamesFromStreamLine_HonorsPerRequestMap`
- `TestPrepareClaudeOAuthToolNamesForUpstream_MixedCaseWithPrefix`
- `TestRestoreClaudeOAuthToolNamesFromResponse_MixedCaseWithPrefix`
- `TestRestoreClaudeOAuthToolNamesFromStreamLine_MixedCaseWithPrefix`

### What was kept in the Claude executor

- **Usage tracking** (request/response logging, `ExecutorUsageReporter`, `ParseClaudeStreamUsage`/`ParseClaudeUsage`).
- **Thinking pipeline** (`thinking.ApplyThinking`, `disableThinkingIfToolChoiceForced`, `normalizeClaudeTemperatureForThinking`).
- **Payload and cache helpers** (`ensureCacheControl`, `enforceCacheControlLimit`, `normalizeCacheControlTTL`, `inject*CacheControl`, `extractAndRemoveBetas`, `ensureModelMaxTokens`, `extractToolResultImages`).
- **Cloak/obfuscation** (`applyCloaking`, `shouldCloak`, `generateFakeUserID`, `obfuscateSensitiveWords`, `claudeCode*` system prompt constants). Cloaking is still configurable per credential (`claude-api-key.cloak.*`) or globally (`disable-claude-cloak-mode`). It now serves only to make non-Claude-Code clients look like Claude Code for compatibility — it no longer has any OAuth-specific branches.
- **Device profile headers** (`applyClaudeLegacyDeviceHeaders`, `mapStainlessOS`, `mapStainlessArch`). These emit Stainless/User-Agent headers that match Claude Code 2.1.63. They remain valuable when proxying to `api.anthropic.com` and harmless for custom base URLs.
- **`signAnthropicMessagesBody` and `experimentalCCHSigningEnabled`** — opt-in cch signing for `claude-api-key.experimental-cch-signing: true`.

### Key modified files

- `internal/runtime/executor/claude_executor.go` — ~500 lines removed (~2768 → ~2265).
- `internal/runtime/executor/claude_executor_test.go` — 23 OAuth tool-renaming tests removed (~1705 → ~1287 lines).
- `internal/runtime/executor/claude_cloak_helpers.go` — built-in tool registry helpers removed (~308 → ~269 lines).

## Antigravity Residual Plumbing

After the Antigravity executor was deleted, the supporting plumbing had no remaining users. It was removed:

### Files deleted

- `internal/misc/antigravity_version.go` (+ test) — background version updater that polled `storage.googleapis.com/antigravity-public/` and the legacy `antigravity-auto-updater-...run.app/releases` endpoint to keep a cached Antigravity client version. Exposed `StartAntigravityVersionUpdater`, `AntigravityLatestVersion`, `AntigravityUserAgent`, `AntigravityRequestUserAgent`, `AntigravityLoadCodeAssistUserAgent`, `AntigravityVersionFromUserAgent`, and the `AntigravityNodeAPIClientUA` / `AntigravityGoogAPIClientUA` constants.
- `sdk/cliproxy/antigravity_models.go` — runtime fetch of Antigravity's `fetchAvailableModels` capability hints (web-search-capable model IDs) from `daily-cloudcode-pa.googleapis.com` / `cloudcode-pa.googleapis.com`.
- `internal/api/handlers/management/api_tools.go` (+ test) — the `POST /v0/management/api-call` management endpoint and its OAuth refresh helpers for `gemini-cli` and `antigravity` access tokens. This file was the **only** place in the codebase that imported `golang.org/x/oauth2` (and `golang.org/x/oauth2/google`).
- `internal/thinking/provider/antigravity/` (entire directory) — the Antigravity thinking-config applier registered via blank import in `internal/runtime/executor/helps/thinking_providers.go`.

### Files updated

- `cmd/server/main.go` — dropped the `misc.StartAntigravityVersionUpdater(...)` call at startup and the now-unused `internal/misc` import.
- `sdk/cliproxy/service.go` — dropped the `case "antigravity":` branch in `registerModelsForAuth` (it called `registry.GetAntigravityModels()` and the deleted `applyAntigravityFetchedModelCapabilities` helper).
- `sdk/cliproxy/service_excluded_models_test.go` — removed the `TestRegisterModelsForAuth_AntigravityFetchesWebSearchCapability` test and its now-unused `net/http` / `net/http/httptest` imports.
- `internal/api/server.go` — removed the `mgmt.POST("/api-call", s.mgmt.APICall)` route registration.
- `internal/runtime/executor/helps/thinking_providers.go` — removed the blank import of the deleted antigravity thinking provider.
- `internal/registry/models/models.json` — removed the `"antigravity"` section (12 model entries). All other providers (`claude`, `gemini`, `vertex`, `gemini-cli`, `aistudio`, `codex-*`, `kimi`, `xai`) are unchanged.

### Antigravity credits fallback (conductor)

The conductor no longer runs the Antigravity credits fallback path. Removed from `sdk/cliproxy/auth/conductor.go`:

- `findAllAntigravityCreditsCandidateAuths()`, `hasAntigravityProvider()`, `shouldAttemptAntigravityCreditsFallback()`, `tryAntigravityCreditsExecute()`, `tryAntigravityCreditsExecuteStream()`, `antigravityCreditsKVUnavailableError()`, `creditsCandidateEntry` type
- Antigravity credits fallback calls in `Execute()` and `ExecuteStream()`

Deleted files:

- `sdk/cliproxy/auth/antigravity_credits.go`
- `sdk/cliproxy/auth/antigravity_credits_test.go`
- `sdk/cliproxy/auth/conductor_credits_candidates_test.go`

## Claude Executor Inlining

The Claude device profile stabilization helpers (`helps/claude_device_profile.go`, `helps/claude_builtin_tools.go`, `helps/claude_system_prompt.go`, `helps/cloak_obfuscate.go`, `helps/cloak_utils.go`, `helps/user_id_cache.go`) were removed. The still-needed constants and helpers (Claude Code static system prompt sections, `shouldCloak`, `generateFakeUserID`, `isValidUserID`, `buildSensitiveWordMatcher`, `obfuscateSensitiveWords`, `augmentClaudeBuiltinToolRegistry`) were inlined into `internal/runtime/executor/claude_cloak_helpers.go`.

`applyClaudeLegacyDeviceHeaders`, `mapStainlessOS`, and `mapStainlessArch` were inlined directly into `claude_executor.go`. The per-apiKey `user_id` cache is gone; callers that set `cache-user-id: true` now receive a freshly generated ID on every request (the config field is still accepted for compatibility).

The `Refresh` method on `ClaudeExecutor` no longer performs OAuth refresh; it returns the auth unchanged. Configure API-key auth for Claude instead.

## Tests Updated

- `internal/runtime/executor/claude_executor_test.go` lost its device-profile and user-id-cache test block. The remaining tests import `_ "internal/translator"` to ensure translator registration side effects still run (the side effect previously came from deleted codex/xai test files).
- `sdk/cliproxy/service_codex_executor_binding_test.go` deleted (only tested codex/xai binding behavior).
- `test/usage_logging_test.go` deleted (only tested Gemini executor usage logging).
- `test/thinking_conversion_test.go` deleted (matrix tested thinking conversions across codex/gemini/antigravity; coverage of the remaining providers is in `internal/thinking`).
- `test/builtin_tools_translation_test.go` lost its `TestOpenAIToCodex_PreservesBuiltinTools` case.
- `integration/conftest.py` no longer passes `--no-browser` when starting the server.

## What Still Works

- Claude API proxy via API key (`ClaudeKey` config entries).
- OpenAI-compatibility proxy for any provider that speaks the OpenAI Chat Completions / Responses API.
- All management endpoints unrelated to OAuth (auth file upload/list/delete, config, usage, logs, model definitions).
- Cloaking (system prompt injection, sensitive-word obfuscation, fake user ID) for non-Claude-Code clients.
