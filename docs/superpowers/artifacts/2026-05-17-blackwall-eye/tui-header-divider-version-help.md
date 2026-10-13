# TUI: header divider, version, help view, q-to-quit
**Status:** DONE
**Module:** blackwall-eye
**Date:** 2026-05-17
**Commits:** 1a0a2b6  22281fd  65675e5

## What was done
- Added `HRule` and `HelpHeading` styles to `internal/ui/styles/styles.go`
- Inserted a full-width `─` rule between the header chrome (tab bar + status bar) and the view body in `app.go`; rendered once at the app level, all views inherit it
- Added `tea.WindowSizeMsg` tracking to `app.Model` so the rule and version label are always sized to the real terminal width
- Updated `tabbar.Render` signature to accept `width int` and `version string`; version is rendered right-aligned using a gap of spaces
- Declared `var version = "v0.0.1"` in `cmd/blackwall/main.go`, overridable via `-ldflags "-X main.version=…"`; wired through `app.Deps.Version`
- Created `internal/views/help/view.go` — a `views.View` that renders grouped keyboard shortcuts; bound `?` globally in `app.go` (pushes help via `helpOpen` flag), `ESC` pops back, `q` quits
- Added `{Key: "?", Desc: "help"}` hint to agents list, agents detail, and contexts list `Hints()` methods
- Verified `q` was already bound to `tea.Quit` in `app.go` (confirmed in existing `TestApp_QuitsOnQ` test)

## Verification
- `go build ./...` — clean, no errors
- `go test ./...` — all 13 packages pass (0 failures)
- Manual render check: pending (no interactive terminal in agent environment)

## Deviations
- `app.go` handles the help view via an `helpOpen bool` flag rather than injecting it into the `views` slice; this avoids polluting the `[1]...[N]` tab bar with a `:help` entry that should not be tab-navigable
- `q` from the help view calls `tea.Quit` (not `BackMsg`); `ESC` pops back to the previous view — matches conventional terminal-app behaviour

## Open questions
- none
