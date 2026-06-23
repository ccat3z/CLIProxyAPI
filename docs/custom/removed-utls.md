# Removed: utls TLS Fingerprinting (Phase 7 Batch 4)

## Summary

Removed the uTLS Chrome TLS-fingerprint spoofing code path from the Claude
executor. The Claude executor now uses the standard `http.Client` (with
proxy support) provided by `helps.NewProxyAwareHTTPClient`, matching the
behaviour of every other provider executor.

## What was removed

- `internal/runtime/executor/helps/utls_client.go` (deleted) — contained
  the `utlsRoundTripper`, `fallbackRoundTripper`, `utlsProtectedHosts`
  map (`api.anthropic.com`, `chatgpt.com`), and the `NewUtlsHTTPClient`
  constructor that wrapped both round trippers behind a single client.
- `internal/runtime/executor/helps/utls_client_test.go` (deleted) — the
  associated unit tests.
- The four `helps.NewUtlsHTTPClient(ctx, e.cfg, auth, 0)` call sites in
  `internal/runtime/executor/claude_executor.go` (around lines 151, 272,
  465, 700) were replaced with
  `helps.NewProxyAwareHTTPClient(ctx, e.cfg, auth, 0)`.

## What replaced it

`helps.NewProxyAwareHTTPClient` (defined in
`internal/runtime/executor/helps/proxy_helpers.go`) is the same constructor
used by the other executors. It honours `auth.ProxyURL` /
`cfg.ProxyURL` and falls back to the context-provided `RoundTripper`. The
function signature is identical to the removed `NewUtlsHTTPClient`, so the
Claude executor now behaves like the rest of the runtime: standard
`http.Client` with proxy awareness and no TLS fingerprint spoofing.

## Rationale

The `custom` branch targets `mcli.sankuai.com` and `aigc.sankuai.com` as
upstream hosts. Neither host appears in `utlsProtectedHosts`, so the
`fallbackRoundTripper` always delegated to the standard transport — the
uTLS code path was dead weight that pulled in the
`github.com/refraction-networking/utls` dependency for no runtime benefit.

## Trade-off

If the proxy is later pointed directly at `api.anthropic.com` (or any
other Cloudflare-protected Anthropic endpoint), requests may be blocked by
Cloudflare's TLS-fingerprint check because the client now presents a
standard Go TLS fingerprint. If that becomes necessary, the uTLS round
tripping can be re-introduced — re-adding `utls_client.go` and switching
the affected call sites back to `NewUtlsHTTPClient` is a self-contained
change.

## Note on dependency cleanup

`github.com/refraction-networking/utls` remains listed in `go.mod` for
now; it will be dropped in the final cleanup batch (Phase 7 Batch 6) via
`go mod tidy`.

## Verification

- `gofmt -w .`
- `go build -o test-output ./cmd/server` succeeds
- `go test ./...` — all modified packages pass; pre-existing unrelated
  failures in `sdk/cliproxy/auth` (OpenAI-compat pool suspension tests)
  are not affected by this change.
- `grep -rn "utls\|refraction-networking" --include="*.go" .` returns no
  matches.
- Smoke test against a live server:
  - `GET /v1/models` → 200
  - `POST /v1/chat/completions` → 200 with valid completion
  - `GET /v0/management/config` → 200
