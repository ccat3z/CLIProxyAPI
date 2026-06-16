# Custom Changes

This document summarizes all local changes on the `custom` branch that diverge from upstream.

Detailed documentation: [docs/custom/](./docs/custom/)

## Features

- **[Per-API-Key Usage Limits](./docs/custom/usage-limits.md)** — Sliding-window token/cost limits with 429 responses
- **[Usage Persistence & API](./docs/custom/usage-persistence.md)** — SQLite-backed usage storage and management API
- **[Disable Config API](./docs/custom/disable-config-api.md)** — Block config-modifying management endpoints
- **[Model `extra` Field](./docs/custom/model-extra.md)** — Custom metadata in `/v1/models` response
- **[Server Flags & Defaults](./docs/custom/server-flags.md)** — `-port` flag and default `auth-dir` change

## Bug Fixes

- **Stream usage null handling** (`internal/usage/`): Skip null usage nodes in stream parsers
- **Claude count_tokens 404** (`internal/runtime/executor/helps/`): Fall back to local estimation
- **Auth block/unavailable logging** (`internal/runtime/executor/helps/`): Diagnostic logging

## Integration Tests

Environment-variable-driven pytest suite in `integration/`. See [docs/custom/index.md](./docs/custom/index.md#integration-tests).

## Pi Agent Integration

- **[Auto-Models Extension](./docs/custom/pi-auto-models.md)** — Auto-register providers from `/v1/models`
