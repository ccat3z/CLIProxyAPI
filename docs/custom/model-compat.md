# Model-Level `compat` Option

Per-model compatibility transforms for non-standard API behavior. Currently supports extracting images from `tool_result` into separate user messages.

## Config

```yaml
claude-api-key:
  - api-key: "sk-xxx"
    models:
      - name: "kimi-k2.6"
        compat:
          - extract-tool-result-images
```

## `extract-tool-result-images`

Some Claude-compatible APIs (e.g., kimi-k2.6) do not support image blocks inside `tool_result` content but do support them in user messages. When enabled, images are automatically extracted from `tool_result` into separate user messages with a placeholder text.

## Code Changes

- `internal/config/config.go` — `Compat []string` field on `ClaudeModel`
- `internal/runtime/executor/claude_executor.go` — `extractToolResultImages` transform applied before forwarding
- `internal/runtime/executor/extract_tool_result_images_test.go` — Unit tests
