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
