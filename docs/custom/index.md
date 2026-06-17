# Custom Changes

This directory documents all local changes on the `custom` branch that diverge from upstream.

- **Upstream**: https://github.com/router-for-me/CLIProxyAPI.git
- **Based on upstream commit**: `9dbf4cd0`

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

## Integration Tests

Full pytest integration test suite in `integration/` covering rate limits, persistence, dynamic config, management API, usage API, cost calculations, and headers.

Upstream server is configured via environment variables:
- `CLI_PROXY_TEST_UPSTREAM_URL` — upstream base URL (e.g. `http://127.0.0.1:8098/v1`)
- `CLI_PROXY_TEST_UPSTREAM_KEY` — upstream API key
- `CLI_PROXY_TEST_UPSTREAM_MODEL` — primary upstream model name
- `CLI_PROXY_TEST_UPSTREAM_MODEL_2` — secondary upstream model name (for multi-model tests)

If not set, `get_upstream_api()` raises a `RuntimeError` with a clear message and example export commands. A liveness chat completion is sent before each test session; if the upstream is unreachable, tests are skipped automatically.


