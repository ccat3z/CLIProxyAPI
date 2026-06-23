# Removed: Unused Dependencies (Phase 7 Batch 6)

## Summary

Final dependency cleanup pass. After the code that imported these
packages was deleted in earlier Phase 7 batches, the entries lingered in
`go.mod` / `go.sum`. `go mod tidy` now drops them, leaving only
dependencies that are still actually imported by the build.

## What was removed

| Dependency | Type | Why it is now unused |
| --- | --- | --- |
| `golang.org/x/oauth2` | direct | Last user was the management API tool handler in `internal/api/handlers/management/api_tools.go`, deleted in Phase 7 Batch 2 (Antigravity residual cleanup). The remaining `internal/misc/oauth.go` helpers were deleted in Phase 7 Batch 1. No Go source imports this package anymore. |
| `cloud.google.com/go/compute/metadata` | indirect | Pulled in transitively by `golang.org/x/oauth2/google`. With `oauth2` gone, the indirect is gone too. |
| `github.com/refraction-networking/utls` | direct | Last user was `internal/runtime/executor/helps/utls_client.go`, deleted in Phase 7 Batch 4 when the Claude executor switched to the standard `http.Client` via `helps.NewProxyAwareHTTPClient`. See [removed-utls.md](./removed-utls.md) for the full rationale. |

## What was kept

`github.com/redis/go-redis/v9` stays in `go.mod`. It is still imported by
`internal/home/` (Redis-based control-plane client), which was retained
on the `custom` branch. The associated indirects
(`github.com/cespare/xxhash/v2`, `go.uber.org/atomic`) also remain.

## How it was done

```
go mod tidy
go mod tidy   # second run confirms idempotency — no further changes
```

`go.sum` was updated in lockstep: the three removed packages' `h1:` and
`/go.mod` checksums were deleted; no other entries were touched.

## Verification

- `go build -o test-output ./cmd/server` succeeds.
- `go test ./...` — all packages pass except the pre-existing
  `sdk/cliproxy/auth` OpenAI-compat pool-suspension failures, which are
  unrelated to this change.
- `pytest integration/` — all 37 integration tests pass.
- Smoke test against a live server:
  - `GET /v1/models` → 200
  - `POST /v1/chat/completions` (model `glm-5.1`) → 200 with valid
    completion
  - `GET /v0/management/config` → 401 (route wired up; auth required as
    expected)
- `grep -E "golang.org/x/oauth2|cloud.google.com/go/compute/metadata|github.com/refraction-networking/utls" go.mod` returns no matches.
- `grep "redis/go-redis" go.mod` still returns the v9 entry.
