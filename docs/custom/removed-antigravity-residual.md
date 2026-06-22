# Removed: Antigravity Residual Code (Phase 7 Batch 2)

## Summary

Removed the remaining Antigravity plumbing that was left behind after the
Antigravity provider executor was deleted in Phase 4. None of these files had
any users left at runtime; they were just dead code paths, data entries, and
the `golang.org/x/oauth2` dependency.

## What was removed

### Files deleted

- `internal/misc/antigravity_version.go` (+ test) — background version updater
  that polled `storage.googleapis.com/antigravity-public/` and the legacy
  `antigravity-auto-updater-...run.app/releases` endpoint to keep a cached
  Antigravity client version. Exposed `StartAntigravityVersionUpdater`,
  `AntigravityLatestVersion`, `AntigravityUserAgent`,
  `AntigravityRequestUserAgent`, `AntigravityLoadCodeAssistUserAgent`,
  `AntigravityVersionFromUserAgent`, and the `AntigravityNodeAPIClientUA` /
  `AntigravityGoogAPIClientUA` constants.
- `sdk/cliproxy/antigravity_models.go` — runtime fetch of Antigravity's
  `fetchAvailableModels` capability hints (web-search-capable model IDs) from
  `daily-cloudcode-pa.googleapis.com` / `cloudcode-pa.googleapis.com`.
- `internal/api/handlers/management/api_tools.go` (+ test) — the
  `POST /v0/management/api-call` management endpoint and its OAuth refresh
  helpers for `gemini-cli` and `antigravity` access tokens. This file was the
  **only** place in the codebase that imported `golang.org/x/oauth2` (and
  `golang.org/x/oauth2/google`).
- `internal/thinking/provider/antigravity/` (entire directory) — the
  Antigravity thinking-config applier registered via blank import in
  `internal/runtime/executor/helps/thinking_providers.go`.

### Files updated

- `cmd/server/main.go` — dropped the `misc.StartAntigravityVersionUpdater(...)`
  call at startup and the now-unused `internal/misc` import.
- `sdk/cliproxy/service.go` — dropped the `case "antigravity":` branch in
  `registerModelsForAuth` (it called `registry.GetAntigravityModels()` and the
  deleted `applyAntigravityFetchedModelCapabilities` helper).
- `sdk/cliproxy/service_excluded_models_test.go` — removed the
  `TestRegisterModelsForAuth_AntigravityFetchesWebSearchCapability` test and
  its now-unused `net/http` / `net/http/httptest` imports.
- `internal/api/server.go` — removed the `mgmt.POST("/api-call", s.mgmt.APICall)`
  route registration.
- `internal/runtime/executor/helps/thinking_providers.go` — removed the blank
  import of the deleted antigravity thinking provider.
- `internal/registry/models/models.json` — removed the `"antigravity"` section
  (12 model entries). All other providers (`claude`, `gemini`, `vertex`,
  `gemini-cli`, `aistudio`, `codex-*`, `kimi`, `xai`) are unchanged.

## Rationale

The Antigravity provider executor was removed in Phase 4. The files above were
not executable on their own — they only existed to support that executor. With
no remaining caller they were just binary bloat, an attack surface for the
embedded OAuth client secrets, and the sole reason `golang.org/x/oauth2` was
in `go.mod`.

## Dependency impact

Drops `golang.org/x/oauth2` (and `golang.org/x/oauth2/google`) from the set of
packages actually imported by the build. `go.mod` still lists
`golang.org/x/oauth2` as a direct dependency; the final `go mod tidy` in the
Batch 6 cleanup pass will remove it along with
`cloud.google.com/go/compute/metadata`.

## Verification

- `gofmt -w .`
- `go build -o test-output ./cmd/server` succeeds
- `go test ./...` — all unit tests pass
- `pytest integration/` — 37/37 integration tests pass
- Smoke test against a live server:
  - `GET /v1/models` → 200
  - `POST /v1/chat/completions` → 200 with valid completion
  - `GET /v0/management/config` → 401 (auth required, route still present)
  - `POST /v0/management/api-call` → **404** (route removed)
