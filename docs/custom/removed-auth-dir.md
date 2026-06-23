# Removed: auth-dir / JSON auth-file subsystem

The `auth_dir` config field and the file-based auth scanning subsystem have been removed from the `custom` branch. The `custom` branch only uses the `claude-api-key` and `openai-compatibility` providers with API keys configured directly in `config.yaml`. The auth-dir scanning feature — which watched a directory for JSON auth files and synthesized auth entries at runtime — is unused under this configuration.

## Why it was removed

The `auth_dir` / `auth-dir` config field pointed the watcher at a directory of JSON auth files (e.g., `claude.json`, `gemini.json`). The watcher would scan this directory on startup, register fsnotify watches on each file, and react to file changes/additions/removals by incrementally updating the auth state. Since the `custom` branch only uses API keys defined in `config.yaml`, this entire subsystem was dead weight.

## Removed config field

| Field | YAML Key | Struct | Notes |
|-------|----------|--------|-------|
| `Config.AuthDir` | `auth_dir` / `auth-dir` | `string` | Path to the directory containing JSON auth files. Already absent from the struct; the key is stripped from stale user configs by the existing `removeRemovedIntegrationKeys` migration helper. |

## Removed watcher fields and methods

| Symbol | File | Kind | Reason |
|--------|------|------|--------|
| `Watcher.authDir` | `internal/watcher/watcher.go` | field | Path to the auth directory. No longer watched or scanned. |
| `Watcher.mirroredAuthDir` | `internal/watcher/watcher.go` | field | Override path from token store `AuthDir()`. No longer used. |
| `NewWatcher(configPath, authDir, callback)` | `internal/watcher/watcher.go` | function | Signature changed to `NewWatcher(configPath, callback)`. |
| `Watcher.addOrUpdateClient(path)` | `internal/watcher/events.go` | method | Handled incremental auth-file change. No longer called. |
| `Watcher.removeClient(path)` | `internal/watcher/events.go` | method | Handled auth-file removal. No longer called. |
| `Watcher.loadFileClients(cfg)` | `internal/watcher/events.go` | method | Walked auth dir and loaded JSON auth files. No longer called. |

## Updated watcher behaviour

- `NewWatcher` now takes only `configPath` and `reloadCallback` (no `authDir`).
- `Watcher.Start` only adds the config file to the fsnotify watcher; it no longer adds an auth directory or walks auth files.
- `Watcher.handleEvent` now only processes config-file change events; auth-file events are no longer dispatched.
- `snapshotCoreAuths(cfg)` now takes only `cfg` (no `authDir` parameter). The `FileSynthesizer.Synthesize` method returns nil (no auth files to scan), though `SynthesizeAuthFile` remains available for direct per-file auth synthesis from the management API.

## Updated synthesizer

- `SynthesisContext` no longer has an `AuthDir` field. The struct now contains only `Config`, `Now`, and `IDGenerator`.
- `FileSynthesizer.Synthesize(ctx)` returns `(nil, nil)` — file-based auth directory scanning is removed.
- `SynthesizeAuthFile(ctx, fullPath, data)` is retained for direct per-file synthesis (used by the management API when persisting individual auth files).

## Updated test files

| File | Changes |
|------|---------|
| `internal/watcher/watcher_test.go` | Removed all test functions that tested auth-file behaviour (`TestAddOrUpdateClientSkipsUnchanged`, `TestAddOrUpdateClientTriggersReloadAndHash`, `TestRemoveClientRemovesHash`, `TestAuthFileEventsDoNotInvokeSnapshotCoreAuths`, `TestAddOrUpdateClientEdgeCases`, `TestLoadFileClientsWalkError`, `TestReloadConfigUsesMirroredAuthDir`, `TestStartFailsWhenAuthDirMissing`, `TestNewWatcherDetectsPersisterAndAuthDir`, `TestAuthSliceToMap`, `TestHandleEventRemovesAuthFile`, `TestHandleEventAuthWriteTriggersUpdate`, `TestHandleEventRemoveDebounceSkips`, `TestHandleEventAtomicReplaceUnchangedSkips`, `TestHandleEventAtomicReplaceChangedTriggersUpdate`, `TestHandleEventRemoveUnknownFileIgnored`, `TestHandleEventRemoveKnownFileDeletes`). Removed `stubStore.authDir` field and `AuthDir()` method. Updated all `NewWatcher` calls to the new signature. Removed `AuthDir` from all `config.Config` struct literals and `Watcher` struct literals. Updated `TestReloadConfigFiltersAffectedOAuthProviders` to `TestReloadConfigPreservesRuntimeAuths` (tests that runtime auths survive config reload). Removed unused `tmpDir`/`tmp` variables. |
| `internal/watcher/diff/config_diff_test.go` | Removed `AuthDir` field from all `config.Config` struct literals. Removed `auth-dir` diff assertions from `TestBuildConfigChangeDetails` and `TestBuildConfigChangeDetails_AllBranches`. |
| `internal/watcher/synthesizer/file_test.go` | Removed `AuthDir` field from all `SynthesisContext` struct literals. Replaced directory-scanning tests with direct `SynthesizeAuthFile` calls (since `FileSynthesizer.Synthesize` now returns nil). Fixed `TestSynthesizeAuthFile_PerAuthExcludedModels` to check `excluded_models_hash` (the actual attribute) instead of raw model names. |
| `internal/api/server_test.go` | Fixed `TestDefaultRequestLoggerFactory_UsesDefaultLogDirectory` to read the logs directory at the correct resolved path (`configDir/logs` instead of `cwd/logs`), matching how `NewFileRequestLogger` resolves a relative `logsDir` against the config file directory. |

## Verification

`gofmt`, `go build`, `go test ./...`, and the standard smoke test all pass.
