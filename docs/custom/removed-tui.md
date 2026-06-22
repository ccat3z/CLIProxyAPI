# Removed: TUI and Utility Commands

## What Was Removed

The following modules were removed from the `custom` branch because they are not used:

### TUI (Terminal User Interface)

- **Directory**: `internal/tui/` — entire directory deleted
  - `app.go`, `auth_tab.go`, `browser.go`, `client.go`, `config_tab.go`, `dashboard.go`, `i18n.go`, `keys_tab.go`, `loghook.go`, `logs_tab.go`, `oauth_tab.go`, `styles.go`
- **Command-line flags removed**:
  - `--tui` — previously started the terminal management UI
  - `--standalone` — previously started an embedded local server with TUI client
- **Code paths removed** from `cmd/server/main.go`:
  - TUI standalone mode (embedded server + TUI client)
  - TUI pure client mode (connects to running server)
  - TUI log hook setup and IO redirection
  - `shouldStartExampleAPIKeyWarningServer` simplified (removed `tuiMode`/`standalone` parameters)

### Utility Commands

- **Directory**: `cmd/fetch_codex_models/` — deleted
  - Standalone utility for fetching Codex model listings
- **Directory**: `cmd/fetch_antigravity_models/` — deleted
  - Standalone utility for fetching Antigravity model listings

## Rationale

The TUI terminal interface and the fetch utility commands are not used on the `custom` branch. Removing them reduces binary size, eliminates unused dependencies (e.g., Bubbletea), and simplifies the server startup path.

## Impact

- The server now always starts in normal proxy mode; there is no TUI option.
- The `--tui` and `--standalone` flags are no longer recognized.
- The fetch utility commands must be run from upstream source if needed.
