# Claude Model ID Prefix Handling

Upstream feature picked from commit `ee71dc52b704ac448fe24043899f251656b9deb1`. Only the Claude model ID prefix handling portion applies to the `custom` branch; the plugin auth disabled-state portion of that upstream commit was dropped because the entire plugin system was removed (see [removed-plugins.md](./removed-plugins.md)).

## What it does

The Claude `/v1/models` listing and `/v1/messages` request handlers now standardize model IDs so the catalog exposed to Anthropic-format clients always begins with the `claude-` prefix, while still routing requests to the real underlying model.

- IDs that already start with `claude-` are returned unchanged.
- All other IDs are rewritten to `claude-fable-5-dd-<reversed>`, where `<reversed>` is the original ID with its rune sequence reversed (e.g. `gpt-4o` → `claude-fable-5-dd-o4-tpg`).
- On inbound `/v1/messages` and `/v1/count_tokens` requests, the rewritten ID is decoded back to the original model name before routing and upstream dispatch. Optional `model(value)` thinking suffixes are preserved through the round trip.

## Affected Files

- `internal/util/claude_model.go` — `EnsureClaudeModelIDPrefix` and `ResolveClaudeModelIDPrefix` helpers (plus `reverseModelID`, `splitModelThinkingSuffix`).
- `sdk/api/handlers/claude/code_handlers.go`:
  - `ClaudeModels` applies `EnsureClaudeModelIDPrefix` to every entry and sorts the list by `display_name` (with `id` as a stable tie-breaker).
  - `ClaudeMessages` and `ClaudeCountTokens` call the new `rewriteClaudeDDModelInBody` helper to decode the prefix before the request is routed.
- Tests: `internal/util/claude_model_test.go`, `sdk/api/handlers/claude/code_handlers_model_test.go`.

## What was dropped from the upstream commit

The upstream commit also introduced "plugin auth disabled state support" — `patchPluginVirtualSourceStatus`, `setSourceAuthFileDisabled`, `applyAuthDisabledState` in `internal/api/handlers/management/auth_files.go`, plus `disabled` propagation through the plugin multi-auth expansion paths in `internal/watcher/synthesizer/file.go` and `sdk/auth/filestore.go`. None of this applies because the `custom` branch removed the plugin host, plugin auth parsers, `PatchAuthFileStatus`, and the plugin multi-auth synthesis paths. The existing single-auth `disabled` handling in `internal/watcher/synthesizer/file.go` already covers the live config. The `auth_files_plugin_oauth_test.go` test file remains deleted.
