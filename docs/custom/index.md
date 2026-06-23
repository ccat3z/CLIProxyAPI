# Custom Changes

This directory documents all local changes on the `custom` branch that diverge from upstream.

- **Upstream**: https://github.com/router-for-me/CLIProxyAPI.git
- **Based on upstream commit**: `a5cb8832`

> This directory only documents server-side changes. Web UI and other non-server component changes are excluded.
> Bug fixes for features introduced on the `custom` branch are not documented separately.

## Features

- [Per-API-Key Model Usage Limits](./usage-limits.md)
- [Usage Persistence & API](./usage-persistence.md)
- [Disable Config API](./disable-config-api.md)
- [Model `extra` Field](./model-extra.md)
- [Model-Level `compat` Option](./model-compat.md)
- [Docker Compose Support](./docker-compose.md)
- [Server Flags & Config Defaults](./server-flags.md)

## Bug Fixes

- **Claude count_tokens 404** (`internal/runtime/executor/helps/token_helpers.go`): Fall back to local token estimation when upstream returns 404
- **Auth block/unavailable logging** (`internal/runtime/executor/helps/logging_helpers.go`): Diagnostic logging for auth selection errors

## Removed Modules

- [Removed: Plugin System](./removed-plugins.md) — `pluginhost` + `pluginstore`, management routes, and all integration points
- [Removed: Non-Essential Providers, OAuth Flows, Browser Login](./removed-providers-oauth.md) — Provider executors, auth packages, translators, browser/login commands, OAuth management handlers, Claude executor OAuth code paths, and Antigravity residual plumbing
- [Removed: Storage Backends and WebSocket Relay](./removed-storage-relay.md) — Postgres/Git/Object stores, `wsrelay`, and the GeminiCLI shared-credential runtime
- [Removed: TUI and Utility Commands](./removed-tui-commands.md) — Terminal UI, `--tui`/`--standalone` flags, and `fetch_*_models` utilities
- [Removed: Config Schema and Server Flags](./removed-config-flags.md) — `-local-model` flag, OAuth provider config types, dead config fields, and related management routes
- [Removed: utls TLS Fingerprinting](./removed-utls.md) — uTLS Chrome fingerprint spoofing in the Claude executor
- [Removed: Dead Code and Unused Dependencies](./removed-dead-code-dependencies.md) — Zero-caller directories/files and the final `go mod tidy` pass

## Integration Tests

Full pytest integration test suite in `integration/` covering rate limits, persistence, dynamic config, management API, usage API, cost calculations, and headers. No environment variables are needed — a mock `llama-server` upstream is auto-downloaded by the test harness.

