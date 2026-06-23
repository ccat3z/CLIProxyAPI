# Removed: proxy-url Config and HTTP(S)/SOCKS Proxy Support

The `proxy-url` configuration field and all associated HTTP(S)/SOCKS proxy infrastructure have been removed from the `custom` branch. The server no longer routes outbound upstream requests through a proxy.

## What Was Removed

### Config fields

- `proxy-url` top-level field in `SDKConfig` (`internal/config/sdk_config.go`) — global proxy for all outbound requests.
- `proxy-url` per-entry field in `ClaudeKey` and `OpenAICompatibilityAPIKey` (`internal/config/config.go`) — per-credential proxy override.
- `ProxyURL` field in `coreauth.Auth` (`sdk/cliproxy/auth/types.go`) — runtime representation propagated from config and auth-file metadata.
- `ProxyInfo()` method on `coreauth.Auth` — logging helper that derived a short proxy description from `ProxyURL`.

### Management API

- `GET/PUT/PATCH/DELETE /v0/management/proxy-url` routes (`internal/api/server.go`) and their handler methods (`internal/api/handlers/management/config_basic.go`).

### HTTP client proxy plumbing

- `sdk/proxyutil/` package (deleted entirely) — `BuildHTTPTransport`, `Redact`, and the HTTP-connect/SOCKS5 dialer implementations.
- `internal/util/proxy.go` `SetProxy` function — applied `SDKConfig.ProxyURL` to an `http.Client` transport.
- `sdk/cliproxy/rtprovider.go` `defaultRoundTripperProvider` — cached per-proxy-URL transports and supplied them as `RoundTripperFor(auth)`.
- `internal/runtime/executor/helps/proxy_helpers.go` `NewProxyAwareHTTPClient` — resolved proxy priority (auth > global > context RoundTripper) and built a proxied transport. Replaced by `NewHTTPClient(ctx, timeout)` which only applies a context-provided `RoundTripper` (a generic SDK extension hook).

### Config diff and synthesizer

- `proxy-url` / `proxy_url` comparison blocks in `internal/watcher/diff/config_diff.go` and `auth_diff.go`.
- `formatProxyURL` helper in `config_diff.go`.
- `ProxyURL` propagation in `internal/watcher/synthesizer/config.go` and `file.go`.
- `proxy_url` sync in `internal/api/handlers/management/auth_files.go`.

### Template and examples

- `proxy-url` entries in `config.example.yaml`.
- Proxy logic in `examples/custom-provider/main.go` `buildHTTPClient`.
- Proxy-related example code in `docs/sdk-usage.md` and `docs/sdk-usage_CN.md`.

### Tests

- `sdk/proxyutil/proxy_test.go` (deleted with the package).
- `sdk/cliproxy/rtprovider_test.go` (proxy-only test, deleted).
- `internal/runtime/executor/helps/proxy_helpers_test.go` (proxy-only test, deleted).
- `TestFormatProxyURL` in `internal/watcher/diff/config_diff_test.go` (deleted).
- `ProxyURL` assertions in synthesizer, diff, and management handler tests (removed).

### Stale config key cleanup

- `removeMapKey(root, "proxy-url")` added to `removeRemovedIntegrationKeys` in `internal/config/config.go` so that old user config files are cleaned up on next save.

## What Replaced It

- `helps.NewHTTPClient(ctx, timeout)` creates a plain `http.Client`. If the context carries a `"cliproxy.roundtripper"` value (set by an SDK `RoundTripperProvider`), it is used as the transport; otherwise the default transport is used. The proxy priority chain is gone.
- The `RoundTripperProvider` interface and `roundTripperFor` plumbing in `sdk/cliproxy/auth/conductor.go` are retained as a generic SDK extension point, but no default provider is wired (the `defaultRoundTripperProvider` is removed).

## Rationale

The `custom` branch targets direct or cloud-deployed upstream hosts where an outbound proxy is not needed. The proxy feature added configuration surface, runtime complexity (proxy URL parsing, SOCKS5/HTTP-connect dialers, credential redaction, per-auth transport caching), and a transitive dependency footprint (`golang.org/x/net/proxy` via the SOCKS5 dialer) — all for functionality that is unused in the target deployment.

## Trade-off

If outbound proxying becomes necessary (e.g., routing through a corporate HTTP proxy or SOCKS5 gateway), the feature must be re-introduced. The simplest restoration path would be to cherry-pick the proxy-related commits from upstream and re-add the `proxy-url` config field, the `sdk/proxyutil` package, and the `NewProxyAwareHTTPClient` constructor.
