# Model `extra` Field

Per-model custom metadata included in the `/v1/models` response. Any `map[string]any` key-value pairs defined in config are passed through verbatim.

## Config

```yaml
openai-compatibility:
  - name: "my-provider"
    base-url: "..."
    models:
      - name: "upstream-model"
        alias: "my-alias"
        extra:
          contextWindow: 1000000
          maxTokens: 384000
          reasoning: true
          cost:
            input: 3
            output: 15
            cacheRead: 0.3
            cacheWrite: 0.3
          compat:
            supportsReasoningEffort: true
            maxTokensField: "max_tokens"
```

Also supported on `claude-api-key.*.models[]`, `codex-api-key.*.models[]`, `gemini-api-key.*.models[]`, and `vertex-api-key.*.models[]`.

## API Response

```json
{
  "data": [
    {
      "id": "my-alias",
      "object": "model",
      "owned_by": "my-provider",
      "extra": {
        "contextWindow": 1000000,
        "maxTokens": 384000,
        "reasoning": true,
        "cost": { "input": 3, "output": 15, "cacheRead": 0.3, "cacheWrite": 0.3 },
        "compat": { "supportsReasoningEffort": true, "maxTokensField": "max_tokens" }
      }
    }
  ]
}
```

## Code Changes

- `internal/config/config.go` — `Extra map[string]any` field on all model config structs + `GetExtra()` method
- `internal/config/vertex_compat.go` — `Extra` field on `VertexCompatModel` + `GetExtra()` method
- `internal/registry/model_registry.go` — `Extra` field on `ModelInfo`; `convertModelToMap` includes `extra` in openai/claude/default handler output
- `sdk/cliproxy/service.go` — `buildConfigModels` copies `Extra`; inline OpenAI-compat construction copies `m.Extra`; `modelEntry` interface adds `GetExtra()`
- `sdk/api/handlers/openai/openai_handlers.go` — `OpenAIModels` includes `extra` in filtered output
