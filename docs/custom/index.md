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

- [Removed: TUI and Utility Commands](./removed-tui.md)
- [Removed: Plugin System (pluginhost + pluginstore)](./removed-plugins.md)
- [Removed: WebSocket Relay, GeminiCLI Runtime, and Non-File Store Backends](./removed-wsrelay-store.md)
- [Removed: Non-Essential Providers, OAuth Flows, Browser Login](./removed-providers-oauth.md)
- [Removed: Claude Executor OAuth Code Paths](./removed-claude-oauth-paths.md)
- [Removed: Unused Config Fields and Dependencies](./removed-config-fields.md)
- [Removed: Dead Code (Phase 7 Batch 1)](./removed-dead-code.md)
- [Removed: Antigravity Residual Code (Phase 7 Batch 2)](./removed-antigravity-residual.md)
- [Removed: `-local-model` Flag and OAuth Provider Config Types (Phase 7 Batch 3)](./removed-local-model-oauth-config.md)
- [Removed: utls TLS Fingerprinting (Phase 7 Batch 4)](./removed-utls.md)
- [Removed: Unused Dependencies (Phase 7 Batch 6)](./removed-dependencies.md)

## Integration Tests

Full pytest integration test suite in `integration/` covering rate limits, persistence, dynamic config, management API, usage API, cost calculations, and headers. No environment variables are needed — a mock `llama-server` upstream is auto-downloaded by the test harness.

