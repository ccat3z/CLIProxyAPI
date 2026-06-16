# Pi Agent Auto-Models Extension

Extension at `~/.pi/agent/extensions/auto-models.ts` that fetches `/v1/models` from a CLIProxyAPI and auto-registers providers/models with the pi agent.

## Configuration

Environment variables:
- `PI_PROXY_URL` — Base URL of the CLIProxyAPI (e.g. `http://127.0.0.1:8098/v1`)
- `PI_PROXY_KEY` — API key for the proxy

If `PI_PROXY_URL` is not set, the extension does nothing.

## How It Works

1. On startup, fetches `/v1/models` from the configured proxy URL
2. For each model with an `extra` field, extracts metadata (`contextWindow`, `maxTokens`, `reasoning`, `cost`, `compat`, `thinking`, etc.)
3. Groups models by `owned_by` and registers each group as a provider via `pi.registerProvider()`

## Required `extra` Fields

For a model to be registered, its `extra` must include at minimum:
- `contextWindow` — context window size in tokens
- `maxTokens` — maximum output tokens

Other recognized fields: `reasoning`, `cost`, `compat`, `thinking`, `thinkingLevelMap`, `maxTokensField`, `api`.
