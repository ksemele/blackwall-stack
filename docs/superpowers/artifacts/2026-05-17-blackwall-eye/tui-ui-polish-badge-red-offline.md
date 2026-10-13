# TUI polish: drop dup line, context badge, red offline rows
**Status:** DONE
**Module:** blackwall-eye
**Date:** 2026-05-17
**Commits:** f1e8bb1 af54c4b 6a68da5

## What was done
- **Task A**: Removed `→ context: <name>` footer message emitted in `app.go` after a successful context switch. The header statusbar already reflects the new context immediately, so the message was pure duplication. Error case (`⚠ context switch failed: ...`) is preserved.
- **Task B**: Added `ContextBadge` style (`lipgloss` yellow bg #220, black fg, padding 0,1, bold) to `internal/ui/styles/styles.go`. Updated `internal/ui/statusbar/statusbar.go` to wrap the context name value in `styles.ContextBadge.Render(...)`.
- **Task C1**: Updated `internal/views/contexts/list.go` to apply `styles.RowDown` (red foreground) to the text portion of any context row whose reachability is `reachDown`. Selected rows still use `SelectedRow` (inverted cyan bar).
- **Task C2**: Updated `internal/views/agents/list.go` to apply `styles.RowDown` to any agent row where `a.Status != "active"`. Selection takes priority. Also fixed QF1012 lint: replaced `b.WriteString(fmt.Sprintf(...))` with `fmt.Fprintf(&b, ...)`.
- Added `RowDown = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))` to styles (reuses existing color "9" already used by `Disconnected` and `Error`).

## Verification
- `go build ./...` — clean, no errors
- `go test ./...` — all 11 tested packages pass, 2 skipped (no test files)
- Manual render: could not run interactively (no Docker/Wazuh env available in this session). Lipgloss composition reviewed: selected branch always wins first, down-row branch is the else-branch, so inverted bar is never obscured by red style.

## Deviations
- The QF1012 lint fix and Task C2 landed in the same commit since both touched `agents/list.go` on adjacent lines.

## Open questions
- none
