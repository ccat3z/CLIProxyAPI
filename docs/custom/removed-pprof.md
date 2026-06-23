# Removed: pprof HTTP debug server

The optional Go `net/http/pprof` debug server (`pprof.enable` / `pprof.addr` config block) has been removed. It exposed the standard `/debug/pprof/*` profiling endpoints on a separate HTTP listener bound to `127.0.0.1:8316` by default, started by `sdk/cliproxy/Service` when enabled, and restarted on config hot-reload. It was a development/diagnostics aid, not part of any request path or management API surface.

## Why it was removed

The `custom` branch deployment does not use pprof-based profiling: nothing in `cmd/server`, `internal/api/`, the request handlers, or the management UI reads or depends on these endpoints, and no integration test exercises them. The feature added a second long-lived HTTP listener, a hot-reload apply path, and a config block that served no production purpose here.

## Removed file

| Path | Symbols |
| --- | --- |
| `sdk/cliproxy/pprof_server.go` | `pprofServer` struct, `newPprofServer`, `(*Service).applyPprofConfig`, `(*Service).shutdownPprof`, `(*pprofServer).Apply`, `(*pprofServer).Shutdown`, `(*pprofServer).startServer`, `(*pprofServer).stopServer`, `(*pprofServer).stopServerWithContext`, `newPprofMux`, and the `net/http/pprof` import that registered `/debug/pprof/{index,cmdline,profile,symbol,trace,allocs,block,goroutine,heap,mutex,threadcreate}`. |

The whole file was self-contained: no other package imported `pprofServer` or called its methods directly — the only entrypoints were the `Service` methods listed above.

## Removed `Service` wiring (`sdk/cliproxy/service.go`)

| Item | Disposition |
| --- | --- |
| `Service.pprofServer *pprofServer` field | Removed. |
| `s.applyPprofConfig(newCfg)` call in the hot-reload path | Removed. |
| `s.applyPprofConfig(s.cfg)` call after server start | Removed. |
| `s.shutdownPprof(ctx)` block in `Service.Shutdown` | Removed (along with its `errShutdownPprof` error-propagation branch). |

## Removed config (`internal/config/config.go`, `internal/config/parse.go`)

| Item | Disposition |
| --- | --- |
| `DefaultPprofAddr = "127.0.0.1:8316"` constant | Removed. |
| `Config.Pprof PprofConfig` field (`yaml:"pprof"`) | Removed. |
| `PprofConfig` struct (`Enable`, `Addr`) | Removed. |
| `cfg.Pprof.Enable` / `cfg.Pprof.Addr` defaults in `LoadConfigOptional` | Removed. |
| `cfg.Pprof.Addr` trim-space + default-back normalization in `LoadConfigOptional` | Removed. |
| Same defaults/normalization in `ParseConfigBytes` (`parse.go`) | Removed. |
| `case "pprof.addr":` in `isKnownDefaultValue` (default-value pruning on save) | Removed. |

## Removed config diff (`internal/watcher/diff/config_diff.go`)

The two `pprof.enable` and `pprof.addr` change-report entries were removed from `BuildConfigChangeDetails`.

## Removed example config

The `pprof:` block (and its `# Enable pprof HTTP debug server ...` comment) was removed from `config.example.yaml`. The deployment's working `data/config.yaml` was not modified.

## Config migration

`pprof` was added to `removeRemovedIntegrationKeys` in `internal/config/config.go`, so a stale top-level `pprof:` block in an existing user config file is stripped automatically on the next save (the server tolerates an unknown `pprof:` key at load time regardless, since YAML decoding ignores unmapped fields, but this keeps on-disk configs clean).

## What was kept

- The `/v0/management/debug` GET/PUT/PATCH routes and the `Config.Debug` boolean are **unrelated** to pprof — they toggle debug-level logging. They are retained.
- All other `Service` lifecycle wiring (server start/stop, watcher, auth queue, shutdown error propagation) is unchanged.

## Verification

Each removed symbol was re-verified before deletion with `grep -rn 'pprof\|Pprof\|DefaultPprofAddr' --include='*.go' --include='*.yaml' .` — after the edits the only remaining hit is the intentional `removeMapKey(root, "pprof")` line in `removeRemovedIntegrationKeys`. `gofmt`, `go build`, `go test ./...`, `pytest integration/` (37 passed), and the standard smoke test (`/v1/models` → 200, `/v1/chat/completions` → 200, `/v0/management/config` → 401) all pass.
