# Removed: Redis support and the redis-backed Home control plane

The entire Redis support and the redis-backed Home control plane have been removed from the `custom` branch. The Home control plane (`internal/home/`) was a remote control plane client that spoke the Redis protocol (`github.com/redis/go-redis/v9`) for config distribution, auth dispatch (`RPopAuth`), usage forwarding (`LPushUsage`), request/app-log streaming, and a shared KV store. Redis was its only transport — there was no separate non-Redis backend — so removing Redis removes the Home control plane wholesale.

The local session-id KV mode that the Claude executor depends on is **retained**: `CachedSessionIDRequired` (`internal/runtime/executor/helps/session_id_cache.go`) now uses an in-process in-memory cache instead of the Home KV client, so it continues to work with no external dependency.

## Why it was removed

The `custom` branch deployment does not operate a Home control plane and does not expose usage data over the Redis RESP protocol. The Redis stack added: a direct `go-redis` dependency, an in-process Redis RESP protocol server multiplexed onto the API port, an in-process usage/error queue (`internal/redisqueue`), a TLS-capable remote control-plane client, and home-mode auth-dispatch / runtime-config plumbing woven through the auth manager and service lifecycle — none of which is exercised by `cmd/server`, the request handlers, the management UI, or the integration tests.

## Removed packages and files

| Path | What it was |
| --- | --- |
| `internal/redisqueue/` (`queue.go`, `plugin.go`, `usage_toggle.go`) | In-process queue backing the Redis RESP usage output, the `SubscribeErrors`/`SubscribeUsage` fan-out, and the HTTP `/usage-queue` endpoint. |
| `internal/api/redis_queue_protocol.go` | Redis RESP protocol handler (`isRedisRESPPrefix`, `handleRedisConnection`) that the connection multiplexer dispatched to. |
| `internal/api/redis_queue_protocol_integration_test.go` | Integration test for the RESP handler. |
| `internal/home/` (`client.go`, `global.go`, `kv_helpers.go`, `requests.go`, `certificate.go`) | The redis-backed Home control-plane client (`New`, `Current`, `SetCurrent`, `ClearCurrent`, `KVGet/KVSet/KVSetNX/KVExpire`, `RPopAuth`, `LPushUsage`, `RPushRequestLog`, config subscriber, heartbeat). |
| `internal/config/home.go` | `HomeConfig` and `HomeTLSConfig` (client-side TLS for the Home Redis connection) and the `Config.Home` field. |
| `internal/config/home_test.go` | Tests for the Home config block. |
| `internal/logging/home_app_log_forwarder.go` | `HomeAppLogForwarder` / `StartHomeAppLogForwarder` (streamed app logs to Home). |
| `internal/logging/home_app_log_forwarder_test.go`, `internal/logging/request_logger_home_test.go` | Tests for the Home log forwarder / request-log client. |
| `internal/runtime/executor/helps/home_refresh.go` | `RefreshAuthViaHome` (credential refresh via the Home control plane). |
| `internal/runtime/executor/helps/home_refresh_test.go` | Tests for Home-based credential refresh. |
| `sdk/cliproxy/auth/error_events.go` | Auth error-event publisher whose only consumer was `redisqueue.EnqueueError`. |
| `sdk/cliproxy/auth/error_events_test.go` | Tests for the error-event publisher. |
| `sdk/cliproxy/auth/home_retry_loop_test.go`, `sdk/cliproxy/auth/home_websocket_reuse_test.go` | Tests for the Home auth-dispatch retry loop and websocket auth pinning. |
| `internal/api/handlers/management/usage_test.go` | Tests for the removed `GetUsageQueue` handler. |

The `github.com/redis/go-redis/v9` module dependency was dropped (`go mod tidy`).

## Removed wiring and entry points

| File | Disposition |
| --- | --- |
| `internal/api/protocol_multiplexer.go` | Removed the Redis RESP protocol-detection branch from `routeMuxConnection`. The multiplexer now serves HTTP only (the byte-peek that guards against idle connections is retained). |
| `internal/api/server.go` | Removed the `/v0/management/usage-queue` route and the "HTTP and Redis" listener comment. |
| `internal/api/handlers/management/usage.go` | Removed the `GetUsageQueue` handler, `usageQueueRecord`, `parseUsageQueueCount`, and the `redisqueue` import. |
| `sdk/cliproxy/service.go` | Removed the `home.Client` subscriber/forwarder (`startHomeSubscriber`, `startHomeUsageForwarder`), `applyHomeOverlay`, `forceHomeRuntimeConfig`, `logHomeConfigChanges`, the `homeClient`/`homeCancel`/`homeLogForwarder` fields, all `redisqueue` usage, and every `homeEnabled`/`cfg.Home.Enabled` branch in `Run`/`Shutdown`. |
| `sdk/cliproxy/auth/conductor.go` | The redis-backed Home auth dispatch is inert: `Manager.HomeEnabled()` now reports `false`, `currentHomeDispatcher` returns `nil`, and the `publishErrorEvent` call was removed (the error-event publisher was deleted). The legacy home-mode retry/dispatch code paths remain compiled but are unreachable. Upstream commit `36ed0ca5` added a new `recordAvailabilityNeutralResult` helper that also called `m.publishErrorEvent`; when cherry-picked, that call (and the now-unused `authSnapshot` local) was dropped to match the custom `MarkResult` tail. |
| `sdk/api/handlers/handlers.go` | Removed the `HomeEnabled()` provider-routing branches; model resolution always uses the registry-based path. |
| `internal/watcher/clients.go` | Removed `redisqueue.NotifyUsageRefresh` calls. |
| `internal/watcher/diff/config_diff.go` | Removed the `redis-usage-queue-retention-seconds` change-report entry. |
| `internal/runtime/executor/claude_executor.go`, `internal/runtime/executor/openai_compat_executor.go` | Removed the `RefreshAuthViaHome` probe from credential refresh. |
| `internal/logging/request_logger.go` | Removed the Home request-log forwarding client. |
| `internal/managementasset/updater.go` | Removed the Home "cluster mode" skip branch. |
| `cmd/server/main_test.go` | Removed the `homeMode` case from the example-API-key warning test. |
| `internal/api/server_test.go` | Removed the `/usage-queue` test, the home-endpoint-hiding test, and the home-models helper tests. |
| `internal/watcher/watcher_test.go` | Removed the `redisqueue.SubscribeUsage` tests. |
| `internal/managementasset/updater_test.go` | Removed the Home "cluster mode" case. |

## Removed config

| File | Disposition |
| --- | --- |
| `config.example.yaml` | Removed the `redis-usage-queue-retention-seconds` block. The deployment's working `data/config.yaml` was not modified. |
| `internal/config/config.go` | The `Config.Home` field and `RedisUsageQueueRetentionSeconds` field were already absent from the struct; the `redis-usage-queue-retention-seconds` key is still stripped from stale user configs by the existing `removeMapKey` migration helper (kept intentionally). |

## What was kept

- **Local session-id KV**: `helps.CachedSessionIDRequired` (used by the Claude executor) holds per-API-key session UUIDs in an in-process TTL cache. It no longer touches `internal/home` or Redis, and the local mode is fully functional.
- **Config migration**: the `removeMapKey(root, "redis-usage-queue-retention-seconds")` line in `internal/config/config.go` is retained so stale user configs are cleaned up on the next save.
- **Home-mode auth dispatch code in `conductor.go`**: `Manager.HomeEnabled()` is preserved as a constant-`false` method and the `pickNextViaHome` dispatch path is left compiled-but-unreachable, because `GetExecutionSessionAuthByID` (which reads the Home runtime-auth map) is still consulted by the OpenAI Responses websocket handler for session-pinned auth lookups (it falls back to `GetByID` when there is no match). Excising the remaining dead home-mode dispatch helpers is tracked as a follow-up cleanup; it has no runtime effect today.

## Verification

Each removed symbol was re-verified with `grep -rn 'redis\|Redis\|go-redis\|internal/home\|internal/redisqueue\|HomeEnabled\|cfg\.Home' --include='*.go' --include='*.yaml' .`. After the edits the only remaining hits are the intentional config-migration `removeMapKey` line and explanatory comments. `gofmt`, `go build`, `go test ./...`, `pytest integration/` (37 passed), and the standard smoke test (`/v1/models` → 200, `/v1/chat/completions` → 200, `/v0/management/config` → 401) all pass.
