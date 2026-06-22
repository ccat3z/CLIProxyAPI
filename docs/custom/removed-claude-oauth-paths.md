# Removed: Claude Executor OAuth Code Paths

Phase 5 of the cleanup plan removed the remaining OAuth-specific code paths from the Claude executor. The custom branch only supports API-key auth; all OAuth-only behaviour (tool name remapping to defeat Anthropic fingerprinting, automatic cch signing for `sk-ant-oat` tokens, OAuth session credential fallback) is gone.

Some of this work was already started by Phase 4 (which inlined the cloak helpers and simplified `ClaudeExecutor.Refresh`). Phase 5 finishes the job by deleting the call sites and the now-dead helpers.

## What Was Removed

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

### Tests

Removed from `internal/runtime/executor/claude_executor_test.go`:

- All `TestApplyClaudeToolPrefix*` cases (10 tests)
- All `TestStripClaudeToolPrefix*` cases (4 tests)
- All `TestRemapOAuthToolNames*` cases (3 tests)
- `TestReverseRemapOAuthToolNamesFromStreamLine_HonorsPerRequestMap`
- `TestPrepareClaudeOAuthToolNamesForUpstream_MixedCaseWithPrefix`
- `TestRestoreClaudeOAuthToolNamesFromResponse_MixedCaseWithPrefix`
- `TestRestoreClaudeOAuthToolNamesFromStreamLine_MixedCaseWithPrefix`

## What Was Kept

- **uTLS HTTP client** (`helps.NewUtlsHTTPClient`) — Cloudflare bypass on `api.anthropic.com` and other strict upstreams.
- **Usage tracking** (request/response logging, `ExecutorUsageReporter`, `ParseClaudeStreamUsage`/`ParseClaudeUsage`).
- **Thinking pipeline** (`thinking.ApplyThinking`, `disableThinkingIfToolChoiceForced`, `normalizeClaudeTemperatureForThinking`).
- **Payload and cache helpers** (`ensureCacheControl`, `enforceCacheControlLimit`, `normalizeCacheControlTTL`, `inject*CacheControl`, `extractAndRemoveBetas`, `ensureModelMaxTokens`, `extractToolResultImages`).
- **Cloak/obfuscation** (`applyCloaking`, `shouldCloak`, `generateFakeUserID`, `obfuscateSensitiveWords`, `claudeCode*` system prompt constants). Cloaking is still configurable per credential (`claude-api-key.cloak.*`) or globally (`disable-claude-cloak-mode`). It now serves only to make non-Claude-Code clients look like Claude Code for compatibility — it no longer has any OAuth-specific branches.
- **Device profile headers** (`applyClaudeLegacyDeviceHeaders`, `mapStainlessOS`, `mapStainlessArch`). These emit Stainless/User-Agent headers that match Claude Code 2.1.63. They remain valuable when proxying to `api.anthropic.com` and harmless for custom base URLs.
- **`signAnthropicMessagesBody` and `experimentalCCHSigningEnabled`** — opt-in cch signing for `claude-api-key.experimental-cch-signing: true`.

## Key Modified Files

- `internal/runtime/executor/claude_executor.go` — ~500 lines removed (~2768 → ~2265).
- `internal/runtime/executor/claude_executor_test.go` — 23 OAuth tool-renaming tests removed (~1705 → ~1287 lines).
- `internal/runtime/executor/claude_cloak_helpers.go` — built-in tool registry helpers removed (~308 → ~269 lines).

## Rationale

The custom branch only supports API-key auth against Claude-compatible upstreams (e.g. `https://api.anthropic.com` or `https://mcli.sankuai.com`). OAuth flows, browser login, and the OAuth callback handlers were removed in Phase 4, so:

- `sk-ant-oat` tokens can no longer be obtained by this server.
- Tool name remapping, automatic cch signing, and `access_token` fallback were all dead branches gated on `isClaudeOAuthToken(apiKey)`.
- The `oauthMode` parameter to `checkSystemInstructionsWithSigningMode` was always false in practice.

Removing them makes the executor's API-key-only behaviour the single, auditable code path. Cloaking and device headers are kept because they remain useful for API-key clients that want to look like Claude Code (e.g. to avoid the `Anthropic-Dangerous-Direct-Browser-Access` requirement or to keep prompt-cache breakpoints aligned with Claude Code's structure).

## Phase 4 Pre-Work

Phase 4 already:

- Created `internal/runtime/executor/claude_cloak_helpers.go` (inlined constants/helpers from deleted `helps/` files).
- Inlined `applyClaudeLegacyDeviceHeaders`, `mapStainlessOS`, `mapStainlessArch` into `claude_executor.go`.
- Stripped OAuth refresh logic from `ClaudeExecutor.Refresh` (it now delegates to `helps.RefreshAuthViaHome`, which is a no-op when home is disabled, and otherwise returns the auth unchanged).

Phase 5 builds on that foundation by deleting the remaining OAuth-only call sites and helpers.
