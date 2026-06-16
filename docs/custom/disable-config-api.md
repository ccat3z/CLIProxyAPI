# Disable Config API

Block all config-modifying management endpoints (PUT /config, PUT /api-keys) while keeping read-only endpoints accessible.

## Config

```yaml
disable-config-api: true
```

## Code Changes

- `internal/config/config.go` — `DisableConfigAPI` field
- `internal/api/server.go` — Blocks write endpoints when flag is set
