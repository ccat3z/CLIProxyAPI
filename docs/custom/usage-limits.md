# Per-API-Key Model Usage Limits

Per-API-key sliding-window token and cost limits, enforced at the executor level with 429 responses. When a limit is exceeded, the request is rejected with HTTP 429 and a JSON body describing the limit type, current usage, and limit value. No `Retry-After` header is sent.

## Config

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

## API

429 response body includes limit details:

```json
{
  "error": {
    "message": "rate limit exceeded: input_tokens limit (21000/20000) for window 1h0m0s",
    "type": "rate_limit_error"
  }
}
```

## Code Changes

- `internal/runtime/limiter/` — `ModelLimiter` tracks limits per `authID` with `LimitConfig` entries (each containing `Models []string`); `models: [a,b]` means a+b share one window (combined tokens count toward one limit); `models: []` or omitted means wildcard (all models for that authID); `Check()` queries `usage.UsageStore.QueryUsageMulti` for aggregate usage; `UpdateLimits` replaces all configs for an authID atomically; `RemoveAllForAuth` cleans stale entries
- `internal/watcher/synthesizer/helpers.go` — `wireLimitsToLimiter` builds flat `[]LimitConfig` preserving model groups from config; `wireModelPrices` registers model prices independently of limits (so cost tracking works without limits configured); `claudeModelPrices` / `openAICompatModelPrices` key prices by upstream name
- `internal/config/config.go` — Limit window and price parsing; `ParsedModelLimitWindow` struct
- `internal/config/duration_parser.go` — Parses duration strings with unit suffixes (`h`, `m`, `s`, `d`)
- `internal/runtime/executor/*.go` — All executors call `limiter.CheckRateLimit(authID, baseModel)` before forwarding
