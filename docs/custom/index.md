# Custom Changes

This directory documents all local changes on the `custom` branch that diverge from upstream.

- **Upstream**: https://github.com/router-for-me/CLIProxyAPI.git
- **Based on upstream commit**: `04109920e4e74128e705aa3155164711e2f7b573`

> This directory only documents server-side changes. Web UI and other non-server component changes are excluded.
> Bug fixes for features introduced on the `custom` branch are not documented separately.

## Architecture

- [Architecture Diagram](./architecture.svg) — post-cleanup request flow (clients → Gin API → handlers → conductor → executors → upstream). DOT source at [architecture.dot](./architecture.dot).

## Features

- [Per-API-Key Model Usage Limits](./usage-limits.md)
- [Usage Persistence & API](./usage-persistence.md)
- [Disable Config API](./disable-config-api.md)
- [Model `extra` Field](./model-extra.md)
- [Model-Level `compat` Option](./model-compat.md)
- [Claude Model ID Prefix Handling](./claude-model-id-prefix.md)
- [Docker Compose Support](./docker-compose.md)
- [Server Flags & Config Defaults](./server-flags.md)

## Bug Fixes

- **Claude count_tokens 404** (`internal/runtime/executor/helps/token_helpers.go`): Fall back to local token estimation when upstream returns 404
- **Auth block/unavailable logging** (`internal/runtime/executor/helps/logging_helpers.go`): Diagnostic logging for auth selection errors
- **Streaming translator nil return leaks raw OpenAI chunks** (`internal/translator/openai/claude/openai_claude_response.go`): The OpenAI→Claude streaming translator returned a `nil` `[][]byte` for chunks that map to no Anthropic event — an empty-content delta (commonly sent by the upstream right after a tool_call chunk and before the finish chunk) and a redundant `[DONE]` after the stream had already terminated. The translator registry treats a `nil` return as "no translator produced output" and falls back to forwarding the raw upstream line verbatim, so the raw OpenAI `chat.completion.chunk` (and a duplicate `data: [DONE]`) leaked into the downstream Claude SSE stream. The leaked `data:` line landed between a tool_use `content_block_start` and its `input_json_delta`, merging them into one corrupt SSE block on the client and dropping the tool's input arguments (e.g. an `Edit` whose `file_path`/`old_string`/`new_string` vanished). Fix: the two streaming helpers now return a non-nil (possibly empty) slice, so the registry's raw-passthrough fallback never fires for a registered translator. Regression test: `TestStreaming_NeverReturnsNilChunks`.

## Removed Modules

- [Removed: Plugin System](./removed-plugins.md) — `pluginhost` + `pluginstore`, management routes, and all integration points
- [Removed: Non-Essential Providers, OAuth Flows, Browser Login](./removed-providers-oauth.md) — Provider executors, auth packages, translators, browser/login commands, OAuth management handlers, Claude executor OAuth code paths, and Antigravity residual plumbing
- [Removed: Storage Backends and WebSocket Relay](./removed-storage-relay.md) — Postgres/Git/Object stores, `wsrelay`, and the GeminiCLI shared-credential runtime
- [Removed: TUI and Utility Commands](./removed-tui-commands.md) — Terminal UI, `--tui`/`--standalone` flags, and `fetch_*_models` utilities
- [Removed: Config Schema and Server Flags](./removed-config-flags.md) — `-local-model` flag, OAuth provider config types, dead config fields, and related management routes
- [Removed: utls TLS Fingerprinting](./removed-utls.md) — uTLS Chrome fingerprint spoofing in the Claude executor
- [Removed: TLS / HTTPS Listen Support](./removed-tls-listen.md) — Server `tls:` config block, HTTPS listener branch, and ALPN handshake path; plain-HTTP listen only
- [Removed: pprof HTTP Debug Server](./removed-pprof.md) — `pprof.enable`/`pprof.addr` config block, `sdk/cliproxy/pprof_server.go`, and the `/debug/pprof/*` listener
- [Removed: Redis Support and the Home Control Plane](./removed-redis-home.md) — `go-redis` dependency, the `internal/redisqueue` queue + RESP protocol handler, the redis-backed `internal/home` control-plane client, and all home-mode wiring; local in-memory session-id KV retained
- [Removed: auth-dir / JSON Auth-File Subsystem](./removed-auth-dir.md) — `auth_dir` config field, `Watcher.authDir`/`mirroredAuthDir`, file-scanning watcher methods (`addOrUpdateClient`, `removeClient`, `loadFileClients`), and all related test infrastructure; `NewWatcher` signature simplified
- [Removed: Dead Code and Unused Dependencies](./removed-dead-code-dependencies.md) — Zero-caller directories/files and the final `go mod tidy` pass

## Writing Removal Docs

When documenting a removal (a `removed-*.md` file or a section in `removed-dead-code-dependencies.md`), follow these conventions so the notes stay scannable and consistent.

**Organize by feature, not by cleanup round.** Group related removals under a feature heading (e.g. *Providers & Model Catalog*, *Auth & Credentials*, *Config Schema*). Never structure a doc as a chronological sequence of passes — the cleanup history is in git, not in prose.

**Describe what changed vs. upstream, not the steps taken.** State what was removed and why it is unused under the active config. Do not write procedural narration — avoid `Batch N`, `Step N`, `Phase N`, `Round N`, or "first we… then we…" sequences. The doc describes the *result*; the work order is irrelevant to a reader.

**One doc per subsystem.** Each removed subsystem (TLS listen, pprof, redis/home, auth-dir, proxy-url, etc.) gets its own `removed-<feature>.md`. Small zero-caller symbols that do not warrant a standalone doc go into `removed-dead-code-dependencies.md`, organized by feature.

**For each removed symbol, record the reason it was dead.** Use a table with `Symbol | Kind | Reason` columns. State the concrete evidence (zero callers verified by grep, only caller was a removed provider, config-unreachable under `claude-api-key`/`openai-compatibility`). Record the grep command used when the verification was non-trivial.

**Always document what was *kept* and why.** Adjacent symbols that survived (because they have a live caller, satisfy an interface, or are whitelisted) must be listed under a **Kept** note. This is what prevents a future cleanup from re-flagging them and from breaking the build. Call out false-positive traps explicitly (indirect callers, dot-imports, interface satisfaction, struct-field access via a local variable).

**Keep it concise.** One line of verification per change is enough ("`gofmt`, `go build`, `go test ./...`, `pytest integration/`, smoke test pass") — do not repeat the full checklist at the end of every section.

**Update this index.** Add a one-line entry under *Removed Modules* for every new `removed-*.md`, with a short description of the subsystem removed.

## Integration Tests

Full pytest integration test suite in `integration/` covering rate limits, persistence, dynamic config, management API, usage API, cost calculations, and headers. No environment variables are needed — a mock `llama-server` upstream is auto-downloaded by the test harness.

