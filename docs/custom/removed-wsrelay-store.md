# Removed: WebSocket Relay, GeminiCLI Runtime, and Non-File Store Backends

## What Was Removed

The following modules were removed from the `custom` branch because they are not used:

- **WebSocket Relay** (`internal/wsrelay/`): WebSocket relay manager used exclusively by the AI Studio provider for real-time streaming via WebSocket connections. Not used on the custom branch.
- **GeminiCLI Runtime State** (`internal/runtime/geminicli/`): Shared credential and virtual credential types for multi-project Gemini CLI logins. Not used on the custom branch — all callers were removed alongside the gemini-cli executor and the multi-project virtual-auth synthesizer.
- **Postgres Store** (`internal/store/postgresstore.go`): PostgreSQL-backed token store for persistent auth storage. Not used on the custom branch.
- **Git Store** (`internal/store/gitstore.go`, `internal/store/gitstore_test.go`): Git-backed token store for version-controlled auth storage. Not used on the custom branch.
- **Object Store** (`internal/store/objectstore.go`): S3-compatible object store backend for auth storage. Not used on the custom branch.
- **AI Studio and Gemini CLI executors** (`internal/runtime/executor/aistudio_executor*.go`, `internal/runtime/executor/gemini_cli_executor*.go`): These executors depend on `wsrelay` (AI Studio) and `internal/runtime/geminicli` (Gemini CLI). They were removed together with the modules they depend on. Phase 4 will remove the remaining provider executors.

## Key Deleted Files/Directories

- `internal/wsrelay/` (entire directory: `manager.go`, `session.go`, `http.go`, `message.go`)
- `internal/runtime/geminicli/` (entire directory: `state.go`)
- `internal/store/` (entire directory, including `gitstore.go`, `gitstore_test.go`, `objectstore.go`, `postgresstore.go`)
- `internal/runtime/executor/aistudio_executor.go` and `internal/runtime/executor/aistudio_executor_test.go`
- `internal/runtime/executor/gemini_cli_executor.go` and `internal/runtime/executor/gemini_cli_executor_test.go`
- `internal/runtime/executor/antigravity_executor_credits_test.go` (tested the removed `parseRetryDelay` helper)

## Key Modified Files

- `cmd/server/main.go`: Removed all `PGSTORE_*`, `GITSTORE_*`, and `OBJECTSTORE_*` environment variable parsing, `PostgresStore`/`GitTokenStore`/`ObjectTokenStore` initialization, and the conditional `RegisterTokenStore()` branches. Only the default `FileTokenStore` remains. Removed unused imports (`io/fs`, `net/url`, `internal/store`).
- `sdk/cliproxy/service.go`: Removed the `wsGateway *wsrelay.Manager` field, `ensureWebsocketGateway()` method, `wsOnConnected()`/`wsOnDisconnected()` callbacks, the `aistudio` executor registration branch (which required `wsGateway`), the `gemini-cli` executor registration branch (executor deleted), WebSocket route attachment and auth-change handler wiring in `Run()`, and `wsGateway.Stop()` in `Shutdown()`. Removed the `internal/wsrelay` import.
- `internal/api/server.go`: Removed `AttachWebsocketRoute()`, `SetWebsocketAuthChangeHandler()`, the `wsRoutes`/`wsRouteMu`/`wsAuthChanged`/`wsAuthEnabled` fields and their initialization. Removed the now-unused `sync` import.
- `internal/api/handlers/management/api_tools.go`: Removed `geminicli.ResolveSharedCredential` lookups from `tokenValueForAuth` and `geminiOAuthMetadata`.
- `internal/watcher/synthesizer/file.go`: Removed `SynthesizeGeminiVirtualAuths` and its helpers (`splitGeminiProjectIDs`, `buildGeminiVirtualID`), along with the `geminicli` import. The multi-project Gemini virtual-auth synthesizer is no longer wired into the file synthesizer.
- `internal/managementasset/updater.go`: Removed the `GITSTORE_GIT_URL`/`GITSTORE_GIT_TOKEN` based Authorization header fallback when fetching management assets.
- `.gitignore` and `.env.example`: Removed `pgstore/*`, `gitstore/*`, `objectstore/*` entries and the commented-out `PGSTORE_*`/`GITSTORE_*`/`OBJECTSTORE_*` examples.

## Helpers Moved to `internal/runtime/executor/helps/`

Two functions previously defined inside deleted executor files are still used by the surviving antigravity executor (slated for removal in Phase 4). They were exported from `internal/runtime/executor/helps/`:

- `ParseRetryDelay` (`helps/retry_delay.go`) — moved from `internal/runtime/executor/gemini_cli_executor.go`. Used by `internal/runtime/executor/antigravity_executor.go`.
- `DeleteJSONField` (`helps/json_helpers.go`) — moved from `internal/runtime/executor/gemini_cli_executor.go`. Used by `internal/runtime/executor/antigravity_executor.go`.

A small antigravity test helper file (`internal/runtime/executor/antigravity_test_helpers_test.go`) was added to keep the surviving antigravity executor tests compilable after the original `_credits_test.go` file was removed.

## Rationale

The custom branch does not use the AI Studio provider (which requires `wsrelay`) or any non-file storage backends (Postgres, Git, Object Store). Only file-based storage is needed. Removing these modules reduces binary size, dependency surface, and maintenance burden.

## Removed Environment Variables

- `PGSTORE_DSN`, `PGSTORE_SCHEMA`, `PGSTORE_LOCAL_PATH` (Postgres store configuration)
- `GITSTORE_GIT_URL`, `GITSTORE_GIT_USERNAME`, `GITSTORE_GIT_TOKEN`, `GITSTORE_LOCAL_PATH`, `GITSTORE_GIT_BRANCH` (Git store configuration)
- `OBJECTSTORE_ENDPOINT`, `OBJECTSTORE_ACCESS_KEY`, `OBJECTSTORE_SECRET_KEY`, `OBJECTSTORE_BUCKET`, `OBJECTSTORE_LOCAL_PATH` (Object store configuration)
