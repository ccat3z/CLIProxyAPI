# Server Flags & Config Defaults

## `-port` Flag

Override config port from command line:

```bash
cli-proxy-api --config config.yaml --port 9000
```

## Default `auth-dir`

The default auth-dir is `~/.cli-proxy-api`, matching upstream behavior (applied via `ResolveAuthDir` when the config value is empty).

## Code Changes

- `cmd/server/main.go` — `-port` flag handling
