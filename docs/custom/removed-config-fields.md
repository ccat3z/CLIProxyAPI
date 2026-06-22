# Removed: Unused Config Fields and Dependencies

## Summary

Removed dead config fields, antigravity credits fallback logic, signature cache config wiring, and unused Go dependencies.

## Removed Config Fields

| Field | YAML Key | Struct | Notes |
|-------|----------|--------|-------|
| `Plugins` | `plugins` | `PluginsConfig`, `PluginInstanceConfig` | Plugin system was removed in Phase 2; config was dead code |
| `CodexHeaderDefaults` | `codex-header-defaults` | `CodexHeaderDefaults` | Codex executor was removed; header defaults were unused |
| `Codex` | `codex` | `CodexConfig` (`IdentityConfuse`) | Identity confuse was never used in execution logic |
| `AntigravitySignatureCacheEnabled` | `antigravity-signature-cache-enabled` | `*bool` | Signature cache toggle was unused after provider removal |
| `AntigravitySignatureBypassStrict` | `antigravity-signature-bypass-strict` | `*bool` | Signature cache strict mode was unused after provider removal |
| `QuotaExceeded.AntigravityCredits` | `quota-exceeded.antigravity-credits` | `bool` | Credits fallback removed from conductor |

## Kept for Compatibility

The following config fields were **not** removed because they are still actively used by runtime code, management handlers, or the conductor:

- `CodexKey []CodexKey` — still used by management API and conductor routing
- `GeminiKey []GeminiKey` — still used by management API and conductor routing
- `ClaudeHeaderDefaults` — still used by claude_executor.go
- `OAuthExcludedModels` — still used by conductor, watchers, and management API
- `OAuthModelAlias` — still used by conductor, watchers, and management API
- `VertexCompatAPIKey []VertexCompatKey` — still used by management API and conductor
- `DisableClaudeCloakMode` — still used by claude_executor.go
- `WebsocketAuth` — still used by websocket handlers and selector
- `CommercialMode` — still used by server.go and logging helpers

## Removed Code

### Config sanitization
- `NormalizePluginsConfig()` — removed from `internal/config/config.go` and `internal/config/parse.go`
- `SanitizeCodexHeaderDefaults()` — removed from `internal/config/config.go` and `internal/config/parse.go`

### YAML config cleanup
- `removeRemovedIntegrationKeys()` in `internal/config/config.go` now removes `codex`, `codex-header-defaults`, `antigravity-signature-cache-enabled`, `antigravity-signature-bypass-strict`, and `plugins` from existing config files on save

### Signature cache wiring
- `configuredSignatureCacheEnabled()`, `configuredSignatureBypassStrict()`, `applySignatureCacheConfig()` — removed from `internal/api/server.go`
- `internal/cache` import removed from `internal/api/server.go`

### Conductor antigravity credits
- `findAllAntigravityCreditsCandidateAuths()` — removed from `sdk/cliproxy/auth/conductor.go`
- `hasAntigravityProvider()` — removed
- `shouldAttemptAntigravityCreditsFallback()` — removed
- `tryAntigravityCreditsExecute()` — removed
- `tryAntigravityCreditsExecuteStream()` — removed
- `antigravityCreditsKVUnavailableError()` — removed
- `creditsCandidateEntry` type — removed
- Antigravity credits fallback calls removed from `Execute()` and `ExecuteStream()`
- `sdk/cliproxy/auth/antigravity_credits.go` — deleted
- `sdk/cliproxy/auth/antigravity_credits_test.go` — deleted
- `sdk/cliproxy/auth/conductor_credits_candidates_test.go` — deleted

### Test files
- `internal/config/plugin_config_test.go` — deleted
- `internal/config/codex_websocket_header_defaults_test.go` — deleted

### Watcher diff
- `quota-exceeded.antigravity-credits` and `codex.identity-confuse` diff detection removed from `internal/watcher/diff/config_diff.go`

## Removed Go Dependencies

The following dependencies were removed by `go mod tidy` after the config and code cleanup:

| Dependency | Reason |
|------------|--------|
| `github.com/charmbracelet/bubbletea` | TUI framework (removed in Phase 1) |
| `github.com/charmbracelet/bubbles` | TUI components (removed in Phase 1) |
| `github.com/charmbracelet/lipgloss` | TUI styling (removed in Phase 1) |
| `github.com/atotto/clipboard` | Clipboard for TUI (removed in Phase 1) |
| `github.com/go-git/go-git/v6` | Git store backend (removed in Phase 3) |
| `github.com/jackc/pgx/v5` | PostgreSQL store backend (removed in Phase 3) |
| `github.com/minio/minio-go/v7` | Object store backend (removed in Phase 3) |
| `github.com/skratchdot/open-golang` | Browser login (removed in Phase 4) |

## Removed Environment Variables

No environment variables were removed in this phase. The `PGSTORE_*`, `GITSTORE_*`, and `OBJECTSTORE_*` env vars were already removed in Phase 3.

## Key Modified Files

- `internal/config/config.go` — removed config fields, types, and sanitization functions
- `internal/config/parse.go` — removed calls to deleted sanitization functions
- `internal/api/server.go` — removed signature cache config wiring
- `sdk/cliproxy/auth/conductor.go` — removed antigravity credits fallback logic
- `internal/watcher/diff/config_diff.go` — removed diff detection for deleted fields
- `go.mod` / `go.sum` — removed unused dependencies
