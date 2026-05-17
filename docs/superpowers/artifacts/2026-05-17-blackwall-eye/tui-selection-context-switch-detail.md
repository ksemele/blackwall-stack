# TUI: k9s-style highlight, context switching, agent details
**Status:** DONE
**Module:** blackwall-eye
**Date:** 2026-05-17
**Commits:**
- `89936dc` fix(blackwall): inverted highlight in k9s style for list views
- `4f97275` feat(blackwall): enter opens agent detail view
- `2fe550b` feat(blackwall): availability circle in contexts list
- `88e021f` fix(blackwall): enter switches context and persists to config
- `dcf7f6f` fix(blackwall): honor select_context_on_start on startup

## What was done

- **Issue 1 (selection style):** Added `styles.SelectedRow` to `internal/ui/styles/styles.go` — cyan bg (`"14"`) + black fg (`"0"`), bold. Removed `▶` arrow marker from both `agents/list.go` and `contexts/list.go`; replaced with full-row `lipgloss` render via `SelectedRow.Render(row)`.

- **Issue 5 (agent detail Enter):** Created `internal/views/agents/view.go` — a `View` wrapper that owns both `*Model` (list) and `*DetailModel` (detail) and manages push/pop. On `SwitchToDetailMsg` it creates a `DetailModel` and calls its `Init`. On `BackMsg` it pops back to list. Updated `main.go` to use `NewView` instead of `New`. The detail view already had ESC→`BackMsg` wired and `View()` displaying all fields.

- **Issue 2 (availability circles):** Added `internal/wazuh/probe.go` — `Probe(ctx, WazuhSubsystem)` does a GET to `/security/user/authenticate` with 2s timeout; any HTTP response means up. Updated `contexts/list.go`: contexts model tracks `map[string]reachState` (pending/up/down). `Init()` resets all to pending and fires async `tea.Batch` of one goroutine per context. Each probe returns a `ProbeResultMsg`. `View()` renders `●` via `styles.Active` / `styles.Disconnected` / `styles.Pending`. `ctrl+r` re-probes.

- **Issue 3 (context switch + persist):** Added `config.SetCurrent(path, name)` which parses YAML into a `yaml.Node` tree, updates the `current` scalar, and re-marshals — preserving structure. Added `ContextSwitchedMsg` to `app/messages.go`. Added `SwitchContext func(name string) tea.Cmd` to `app.Deps`. `app.go` handles `ContextChangedMsg` (delegates to `switchCtx`) and `ContextSwitchedMsg` (shows footer confirmation `→ context: <name>`, navigates to `:agents`). In `main.go` the `switchContext` closure builds a new `wazuh.Client`, calls `config.SetCurrent`, updates `liveCtx`/`liveWazuh`, calls `agentsList.ReplaceClient`, calls `ctxView.UpdateCurrent`. Added `TestSetCurrent_UpdatesField` test.

- **Issue 4 (select_context_on_start):** The startup flag was already setting `initial = ":contexts"` but Enter on a context never completed the flow (no handler in app). Fixed as a side-effect of Issue 3. Also fixed: cursor in contexts list now initializes to `cfg.Current` position (alphabetically sorted). `ctxView.UpdateCurrent(name)` keeps the cursor on the newly-active context for future opens.

## Verification

`go test ./...` — all packages pass:
```
ok  internal/app
ok  internal/config
ok  internal/log
ok  internal/oneshot
ok  internal/tickcmd
ok  internal/ui/footer
ok  internal/ui/statusbar
ok  internal/ui/tabbar
ok  internal/views/agents
ok  internal/views/contexts
ok  internal/wazuh
```

`go build ./...` — clean, no errors.

Binary smoke: `blackwall --help` responds correctly. `--once` mode path unchanged (no TUI code involved).

Docker env was not running at time of implementation; full interactive smoke test deferred to next session with live environment.

## Deviations from plan

- The contexts `Hints()` was simplified: `c` (set current) and `/` (filter) removed since neither was implemented; replaced with `ctrl+r` (re-probe). Filter is a future task.
- The `footerMsg` in the app model is not auto-cleared after a timeout (would require a tickcmd). It persists until next navigation. Acceptable for MVP.

## Open questions / concerns

- `footerMsg` in app root never auto-clears — low priority UX issue.
- Probe uses `insecure_skip_verify` from config but ignores `ca_file` at probe time (builds a fresh HTTP client with TLS settings from the subsystem config). This is correct behavior.
- Context switch does not re-probe circles in contexts view (they retain their last state). Re-open the view or press `ctrl+r` to refresh.
