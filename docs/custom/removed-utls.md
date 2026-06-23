# Removed: utls TLS Fingerprinting

The uTLS Chrome TLS-fingerprint spoofing code path has been removed from the Claude executor. The Claude executor now uses the standard `http.Client` provided by `helps.NewHTTPClient`, matching the behaviour of every other provider executor.

## What Was Removed

- `internal/runtime/executor/helps/utls_client.go` (deleted) — contained the `utlsRoundTripper`, `fallbackRoundTripper`, `utlsProtectedHosts` map (`api.anthropic.com`, `chatgpt.com`), and the `NewUtlsHTTPClient` constructor that wrapped both round trippers behind a single client.
- `internal/runtime/executor/helps/utls_client_test.go` (deleted) — the associated unit tests.
- The four `helps.NewUtlsHTTPClient(ctx, e.cfg, auth, 0)` call sites in `internal/runtime/executor/claude_executor.go` (around lines 151, 272, 465, 700) were replaced with `helps.NewHTTPClient(ctx, 0)`.

## What Replaced It

`helps.NewHTTPClient` (defined in `internal/runtime/executor/helps/proxy_helpers.go`) is the same constructor used by the other executors. It creates a standard `http.Client` with an optional context-provided `RoundTripper`. The proxy feature (`auth.ProxyURL` / `cfg.ProxyURL`) has been removed from the `custom` branch — see [removed-proxy-url.md](./removed-proxy-url.md).

## Rationale

The `custom` branch targets `mcli.sankuai.com` and `aigc.sankuai.com` as upstream hosts. Neither host appears in `utlsProtectedHosts`, so the `fallbackRoundTripper` always delegated to the standard transport — the uTLS code path was dead weight that pulled in the `github.com/refraction-networking/utls` dependency for no runtime benefit.

## Trade-off

If the proxy is later pointed directly at `api.anthropic.com` (or any other Cloudflare-protected Anthropic endpoint), requests may be blocked by Cloudflare's TLS-fingerprint check because the client now presents a standard Go TLS fingerprint. If that becomes necessary, the uTLS round tripping can be re-introduced — re-adding `utls_client.go` and switching the affected call sites back to `NewUtlsHTTPClient` is a self-contained change.
