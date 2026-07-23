# Usage Persistence & API

SQLite-backed usage storage so rate limits and usage data survive restarts. Records include token counts, cost, request ID, latency, and timestamps. Supports per-API, per-model, per-day/hour aggregation queries.

## Config

```yaml
usage-statistics-enabled: true
usage-db: ./data/usage.db
```

## API

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
    "source": "sk-xxx",
    "auth_index": "0",
    "config": { "window": 3600, "models": ["upstream-model"], "input_tokens": 20000, ... },
    "current": { "input_tokens": 5000, "output_tokens": 1200, "cache_tokens": 0, "price": 0.03 }
  }]
}
```

- `window=N` query param: time range in hours (default 24)
- `limits` is keyed by the model names specified in the limits config; `current` reflects actual usage from SQLite

## Code Changes

- `internal/usage/persist_plugin.go` — `PersistPlugin`: stores records with `auth_id`, `model`, `timestamp`, tokens, `cost`, `provider`, `source`, `request_id`, `request_service_tier`, `response_service_tier` (the `request_service_tier` column is populated from `record.ServiceTier`, following the upstream collapse of the deprecated `RequestServiceTier` alias); `QueryUsageMulti(authID, models, from, to)` for limit checks (aggregates across models; empty models = all); `QueryFullUsageReport(from, to)` for API; `SetModelPrices(authID, model, prices)` for cost computation; `ClearStaleModelPrices(authID, currentModels)` removes prices for models no longer in config
- `internal/api/handlers/management/usage.go` — `GetUsageStatistics` handler with `limits` field via `buildLimitsResponse` (uses `helps.ResolveUsageSource` for source, consistent with apis details); `buildPersistResponse` for SQLite path
- `internal/runtime/executor/helps/usage_helpers.go` — `ResolveUsageSource` (exported) resolves the source identifier for an auth record (api_key, email, project_id, etc.); used by both detail recording and limits response
- `internal/api/handlers/management/api_key_usage.go` — `apiKeyUsageProviderKey` resolves the provider bucket key for API key usage grouping; uses `compat_name` attribute when present (lowercased), otherwise falls back to `auth.Provider`
- `internal/runtime/limiter/limiter.go` — `GetAllLimits()` exposes configured limits; `LimitEntry` includes `Models []string`; `LimitConfig` includes `Models []string` (empty = wildcard)
- `internal/api/handlers/management/usage.go` — `GetUsageQueue` handler pops queued usage records from the in-memory Redis-compatible usage queue (route: `/usage-queue` with `?count=N` param); distinct from `GetUsageStatistics` which queries SQLite
