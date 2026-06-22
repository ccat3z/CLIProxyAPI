# AGENTS.md

Go 1.26+ proxy server providing OpenAI/Claude-compatible APIs with API-key auth and round-robin load balancing.

## Repository
- GitHub: https://github.com/router-for-me/CLIProxyAPI

## Commands
```bash
gofmt -w . # Format (required after Go changes)
go build -o cli-proxy-api ./cmd/server # Build
go run ./cmd/server # Run dev server
go test ./... # Run all tests
go test -v -run TestName ./path/to/pkg # Run single test
pytest integration/ # Run integration test
go build -o test-output ./cmd/server && rm test-output # Verify compile (REQUIRED after changes)
```
- Common flags: `--config <path>`, `--local-model`

## Config
- Default config: `config.yaml` (template: `config.example.yaml`)
- `.env` is auto-loaded from the working directory
- Auth material defaults under `auths/`
- Storage backend: file-based (under `auths/`)

## Verification (REQUIRED after every code change)

Run through this checklist after every code change, before reporting the work as complete. Do not skip steps. If any step fails, fix the issue before continuing.

1. **Format**: `gofmt -w .`
2. **Build**: `go build -o test-output ./cmd/server && rm test-output`
3. **Unit tests**: `go test ./...`
4. **Integration tests**: `pytest integration/` (no env vars needed; `llama-server` is auto-downloaded as mock upstream — do NOT set `CLI_PROXY_TEST_UPSTREAM_URL`/`_KEY`/`_MODEL`)
5. **Smoke test**:
   - Pick a free port (e.g. via `PORT=$(python -c 'import socket; s=socket.socket(); s.bind(("",0)); print(s.getsockname()[1])')`) and use it consistently.
   - Start: `go run ./cmd/server --config data/config.yaml --port $PORT` (wait for "API server started", then kill after tests)
   - `/v1/models`: `curl -s http://localhost:$PORT/v1/models -H "Authorization: Bearer sk-123"`
   - `/v1/chat/completions`: `curl -s http://localhost:$PORT/v1/chat/completions -H "Authorization: Bearer sk-123" -H "Content-Type: application/json" -d '{"model":"glm-5.1","messages":[{"role":"user","content":"hi"}],"max_tokens":10}'`
   - Management API: `curl -s http://localhost:$PORT/v0/management/config`

The upstream may not respond successfully — focus on the server accepting requests without crashing or returning 500/unknown-provider errors.

## Custom Changes
- See [docs/custom/index.md](docs/custom/index.md) for all local changes on the `custom` branch that diverge from upstream.
- Every code change on the `custom` branch must be reflected in `docs/custom/`. Update it in the same commit as the code change.

## Architecture
- `cmd/server/` — Server entrypoint
- `internal/api/` — Gin HTTP API (routes, middleware, modules)
- `internal/api/modules/amp/` — Amp integration (Amp-style routes + reverse proxy)
- `internal/thinking/` — Main thinking/reasoning pipeline. `ApplyThinking()` (apply.go) parses suffixes (`suffix.go`, suffix overrides body), normalizes config to canonical `ThinkingConfig` (`types.go`), normalizes and validates centrally (`validate.go`/`convert.go`), then applies provider-specific output via `ProviderApplier`. Do not break this "canonical representation → per-provider translation" architecture.
- `internal/runtime/executor/` — Per-provider runtime executors (Claude, OpenAI-compat)
- `internal/translator/` — Provider protocol translators (and shared `common`)
- `internal/registry/` — Model registry + remote updater (`StartModelsUpdater`); `--local-model` disables remote updates
- `internal/managementasset/` — Config snapshots and management assets
- `internal/cache/` — Request signature caching
- `internal/watcher/` — Config hot-reload and watchers
- `internal/usage/` — Usage and token accounting
- `sdk/cliproxy/` — Embeddable SDK entry (service/builder/watchers/pipeline)
- `test/` — Cross-module integration tests

## Code Conventions
- Keep changes small and simple (KISS)
- Comments in English only
- If editing code that already contains non-English comments, translate them to English (don’t add new non-English comments)
- For user-visible strings, keep the existing language used in that file/area
- New Markdown docs should be in English unless the file is explicitly language-specific (e.g. `README_CN.md`)
- As a rule, do not make standalone changes to `internal/translator/`. You may modify it only as part of broader changes elsewhere.
- If a task requires changing only `internal/translator/`, run `gh repo view --json viewerPermission -q .viewerPermission` to confirm you have `WRITE`, `MAINTAIN`, or `ADMIN`. If you do, you may proceed; otherwise, file a GitHub issue including the goal, rationale, and the intended implementation code, then stop further work.
- `internal/runtime/executor/` should contain executors and their unit tests only. Place any helper/supporting files under `internal/runtime/executor/helps/`.
- Follow `gofmt`; keep imports goimports-style; wrap errors with context where helpful
- Do not use `log.Fatal`/`log.Fatalf` (terminates the process); prefer returning errors and logging via logrus
- Shadowed variables: use method suffix (`errStart := server.Start()`)
- Wrap defer errors: `defer func() { if err := f.Close(); err != nil { log.Errorf(...) } }()`
- Use logrus structured logging; avoid leaking secrets/tokens in logs
- Avoid panics in HTTP handlers; prefer logged errors and meaningful HTTP status codes
- Timeouts are allowed only during credential acquisition; after an upstream connection is established, do not set timeouts for any subsequent network behavior. Intentional exceptions that must remain allowed: the management APICall timeout in `internal/api/handlers/management/api_tools.go`.
