# Removed: Non-Essential Providers, OAuth Flows, Browser Login

Phase 4 of the cleanup plan removed all non-essential provider executors, their auth packages, translators, the browser helper, login commands, and the OAuth management handlers. Only Claude and OpenAI-compatibility executors remain.

## Removed Executors

The following executor implementations were deleted from `internal/runtime/executor/`:

- `codex_executor.go` and related codex files (`codex_websockets_executor.go`, `codex_openai_images.go`, reasoning replay cache)
- `gemini_executor.go`, `gemini_vertex_executor.go`
- `gemini-cli` runtime support (already removed in Phase 3)
- `antigravity_executor.go`
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

## Claude Executor Inlining

The Claude device profile stabilization helpers (`helps/claude_device_profile.go`, `helps/claude_builtin_tools.go`, `helps/claude_system_prompt.go`, `helps/cloak_obfuscate.go`, `helps/cloak_utils.go`, `helps/user_id_cache.go`) were removed. The still-needed constants and helpers (Claude Code static system prompt sections, `shouldCloak`, `generateFakeUserID`, `isValidUserID`, `buildSensitiveWordMatcher`, `obfuscateSensitiveWords`, `augmentClaudeBuiltinToolRegistry`) were inlined into a new file:

- `internal/runtime/executor/claude_cloak_helpers.go`

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
