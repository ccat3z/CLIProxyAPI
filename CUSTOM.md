# Custom Changes

This document summarizes all local changes on the `custom` branch that diverge from upstream.

## Per-API-Key Model Usage Limits

Per-API-key sliding-window token and cost limits, enforced at the executor level with 429 responses. When a limit is exceeded, the request is rejected with HTTP 429 and a JSON body describing the limit type, current usage, and limit value. No `Retry-After` header is sent.

### Config

Limit windows are defined per `api-key-entry`:

```yaml
openai-compatibility:
  - name: "my-upstream"
    base-url: "..."
    api-key-entries:
      - api-key: "sk-xxx"
        limits:
          - window: 1h          # duration: 1h, 30m, 1d, 3600s
            models: [upstream-model]   # upstream model names only (no aliases)
            input_tokens: 20k    # optional, 0 = unlimited
            output_tokens: 5k    # optional, 0 = unlimited
            cache_tokens: 10k    # optional, 0 = unlimited
            price: 1.5           # cost limit in USD, optional, 0 = unlimited
    models:
      - name: "upstream-model"
        alias: "my-alias"
        input_price_m: 3        # prices defined on model entries, not inside limit windows
        output_price_m: 15
        cache_price_m: 0.3
```

### API

429 response body includes limit details:

```json
{
  "error": {
    "message": "rate limit exceeded: input_tokens limit (21000/20000) for window 1h0m0s",
    "type": "rate_limit_error"
  }
}
```

### Code Changes

- `internal/runtime/limiter/` — `ModelLimiter` tracks usage per `authID|model` key with configurable sliding windows; `Check()` queries `usage.UsageStore` for current usage; `SyncLimitsForAuth` / `RemoveAllForAuth` clean stale entries
- `internal/watcher/synthesizer/helpers.go` — `wireLimitsToLimiter` registers limits directly under the model names specified in config (must be upstream names); `claudeModelPrices` / `openAICompatModelPrices` key prices by upstream name
- `internal/config/config.go` — Limit window and price parsing; `ParsedModelLimitWindow` struct
- `internal/config/duration_parser.go` — Parses duration strings with unit suffixes (`h`, `m`, `s`, `d`)
- `internal/runtime/executor/*.go` — All executors call `limiter.CheckRateLimit(authID, baseModel)` before forwarding

## Usage Persistence & API

SQLite-backed usage storage so rate limits and usage data survive restarts. Records include token counts, cost, request ID, latency, and timestamps. Supports per-API, per-model, per-day/hour aggregation queries.

### Config

```yaml
usage-statistics-enabled: true
usage-db: ./data/usage.db
```

### API

`GET /v0/management/usage?window=N` returns full usage report with a `limits` field:

```json
{
  "usage": {
    "total_requests": 10,
    "success_count": 10,
    "failure_count": 0,
    "total_tokens": 50000,
    "apis": { ... },
    "requests_by_day": { ... },
    "cost_by_day": { ... }
  },
  "failed_requests": 0,
  "limits": [{
    "source": "auths/xxx.yaml",
    "auth_index": "0",
    "config": { "window": 3600, "models": ["upstream-model"], "input_tokens": 20000, ... },
    "current": { "input_tokens": 5000, "output_tokens": 1200, "cache_tokens": 0, "price": 0.03 }
  }]
}
```

- `window=N` query param: time range in hours (default 24)
- `limits` is keyed by the model names specified in the limits config; `current` reflects actual usage from SQLite

### Code Changes

- `internal/usage/persist_plugin.go` — `PersistPlugin`: stores records with `auth_id`, `model`, `timestamp`, tokens, `cost`, `provider`, `source`, `request_id`; `QueryUsage(authID, model, from, to)` for limit checks; `QueryFullUsageReport(from, to)` for API; `SetModelPrices(authID, model, prices)` for cost computation
- `internal/api/handlers/management/usage.go` — `GetUsageStatistics` handler with `limits` field via `buildLimitsResponse`; `buildPersistResponse` for SQLite path
- `internal/runtime/limiter/limiter.go` — `GetAllLimits()` exposes configured limits; `LimitEntry` / `LimitConfig` structs

## Disable Config API

Block all config-modifying management endpoints (PUT /config, PUT /api-keys) while keeping read-only endpoints accessible.

### Config

```yaml
disable-config-api: true
```

### Code Changes

- `internal/config/config.go` — `DisableConfigAPI` field
- `internal/api/server.go` — Blocks write endpoints when flag is set

## Server Flags & Config Defaults

- `-port` flag: override config port from command line
- Default `auth-dir` changed from `~/.cli-proxy-api` to `./auth`

### Code Changes

- `cmd/server/main.go` — `-port` flag handling
- `internal/config/config.go` — Default auth-dir change

## Bug Fixes

- **Stream usage null handling** (`internal/usage/`): Skip null usage nodes in stream parsers instead of crashing
- **Claude count_tokens 404** (`internal/runtime/executor/helps/token_helpers.go`): Fall back to local token estimation when upstream returns 404
- **Auth block/unavailable logging** (`internal/runtime/executor/helps/logging_helpers.go`): Diagnostic logging for auth selection errors

## Integration Tests

Full pytest integration test suite in `integration/` covering rate limits, persistence, dynamic config, management API, usage API, cost calculations, and headers.
