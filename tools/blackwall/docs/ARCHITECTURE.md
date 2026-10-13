# blackwall — architecture

A Bubbletea TUI in three layers:

- `internal/config`  — YAML + defaults + env expansion + CLI overrides.
- `internal/wazuh`   — Wazuh Manager API client (JWT auth, TLS-aware).
- `internal/app`     — root `tea.Model` with view router, command palette, global hotkeys.
- `internal/views/*` — top-level views (`:contexts`, `:agents`). Each implements
                       `views.View` and is registered in `cmd/blackwall/main.go`.
- `internal/ui/*`    — render helpers (`tabbar`, `statusbar`, `footer`, `styles`).

## Adding a new view

1. New package under `internal/views/<name>/`.
2. Implement the `views.View` interface (`Init`, `Update`, `View`, `Name`, `Hints`).
3. Append it to the `registered` slice in `cmd/blackwall/main.go`.
4. It becomes available as a `[N]` tab and as `:<name>` in the command palette automatically.

## Asynchronous work

All network calls run in `tea.Cmd` goroutines. The view stores any
state-changing result via a message handled in `Update`. The UI thread
never blocks on I/O.

## Refresh loop

`agents.list` re-arms `tickcmd.Every(refreshInterval)` after each tick
so the interval does not drift behind slow responses.
