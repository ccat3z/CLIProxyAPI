# Removed: TUI and Utility Commands

The terminal management UI and the standalone model-fetching utilities are not used on the `custom` branch. They have been removed along with their flags and dependencies.

## TUI (Terminal User Interface)

- **Directory**: `internal/tui/` — entire directory deleted
  - `app.go`, `auth_tab.go`, `browser.go`, `client.go`, `config_tab.go`, `dashboard.go`, `i18n.go`, `keys_tab.go`, `loghook.go`, `logs_tab.go`, `oauth_tab.go`, `styles.go`
- **Command-line flags removed**:
  - `--tui` — previously started the terminal management UI
  - `--standalone` — previously started an embedded local server with TUI client
- **Code paths removed** from `cmd/server/main.go`:
  - TUI standalone mode (embedded server + TUI client)
  - TUI pure client mode (connects to running server)
  - TUI log hook setup and IO redirection
  - `shouldEnableExampleAPIKeySafeMode` simplified (removed `tuiMode`/`standalone`/`homeMode` parameters); the custom branch no longer runs a separate warning-only server, it starts the normal server with the example-API-key safe-mode middleware (see [removed-tls-listen.md](./removed-tls-listen.md#example-api-key-safe-mode))

## Utility Commands

- **Directory**: `cmd/fetch_codex_models/` — deleted. Standalone utility for fetching Codex model listings.
- **Directory**: `cmd/fetch_antigravity_models/` — deleted. Standalone utility for fetching Antigravity model listings.

## Impact

- The server now always starts in normal proxy mode; there is no TUI option.
- The `--tui` and `--standalone` flags are no longer recognized.
- The fetch utility commands must be run from upstream source if needed.
