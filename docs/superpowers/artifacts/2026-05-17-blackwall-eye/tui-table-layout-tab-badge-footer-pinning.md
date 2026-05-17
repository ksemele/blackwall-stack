# TUI: k9s-style table layout, tab badge, footer pinning
**Status:** DONE
**Module:** blackwall-eye
**Date:** 2026-05-17
**Commits:** 3df5fb9  73568f6  d7810a2  e8aac25  04c153a

## What was done
- Added `internal/ui/table` helper — `Col{Title, Min, Flex}` plus `Widths/Header/Row`. Flex columns split leftover horizontal space; the trailing column is right-padded so a row's printable width always equals the configured total, letting full-row highlights extend cleanly to the right edge.
- Refactored `internal/views/agents/list.go` to use `table.Header`/`table.Row` with `agentCols` (`ID`, `NAME`/flex, `IP`, `STATUS`, `LAST SEEN`/flex). Added `tea.WindowSizeMsg` handling on the model so rows render at the real terminal width. Fixes the bug where the selected-row highlight stopped at the last printable character (e.g. master row's `—`).
- Refactored `internal/views/contexts/list.go` similarly; the leading reachability circle stays outside the table grid (kept in its own colour even when the row is highlighted), and the text columns are flex-distributed.
- Replaced `TabActive` style: dropped underline, switched to a yellow badge (`bg 220 / fg 0 / padding(0,1) / bold`) — matches the existing `ContextBadge` so the selection language is consistent across the chrome.
- Pinned the footer to the bottom of the terminal: `app.Model` now tracks `height` as well as `width`, broadcasts `WindowSizeMsg` to every view (including the help overlay) so inactive views have correct dimensions on first activation, and pads the body with blank lines so the bottom horizontal rule + footer occupy the last rows. A second `─` divider now separates the body from the footer, mirroring the existing top divider.

## Verification
- `go build ./... && go vet ./...` — clean
- `go test ./...` — all packages pass; new tests:
  - `table.TestWidths_*` / `table.TestRow_*` / `table.TestHeader_*`
  - `agents.TestListView_RowsStretchToTerminalWidth` — asserts every rendered line equals the terminal width at `WindowSizeMsg{Width:120}`
  - `app.TestApp_PinsFooterToBottomOnResize` — asserts rendered height matches `WindowSizeMsg.Height` and the second-to-last line is the horizontal rule

## Deviations
- Tab badge uses the same colour as the context badge (yellow) per the user's request "сделать выделение также как контекст, фоном". Two yellow elements coexist in the top chrome; can be split later if it reads as visually noisy.
- Columns are not yet user-configurable — the request was forward-looking ("колонки будут потом меняться или конфигурироваться"); the helper is structured so column specs are passed in by each view, which is the natural extension point.

## Open questions
- нет
