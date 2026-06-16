# Server Flags & Config Defaults

## `-port` Flag

Override config port from command line:

```bash
cli-proxy-api --config config.yaml --port 9000
```

## Default `auth-dir`

Changed from `~/.cli-proxy-api` to `./auth`.

## Code Changes

- `cmd/server/main.go` — `-port` flag handling
- `internal/config/config.go` — Default auth-dir change
