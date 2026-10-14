# App plumbing: key capture, pause, alerts jump message, status line

**Status:** Done
**Module:** tools/blackwall (TUI)
**Date:** 2026-10-14
**Commits:** d3c2135 fix(blackwall): let views capture keys and own p; add alerts jump msg

## What was done

- `views.KeyCapturer` interface added in `internal/views/view.go`. While a view's
  `CapturesKeys()` returns true, the app forwards keys to it and skips global
  shortcuts (q, Tab, digits, `:`, `?`, `p`). `ctrl+c` always quits.
- `p` is consumed by the app only when a pause hook is wired. Otherwise it reaches
  the active view.
- `app.ShowAgentAlertsMsg{AgentID, AgentName}` added. The app activates the view
  named `:alerts` and passes it the message. No-op if no such view is registered.
- `statusbar.State.Indexer` added. The status line shows `indexer: <addr>` only when set.
- `(*agents.View).CapturesKeys()` added. It reports true while the active detail
  tab (the packages or processes filter) is capturing keys.

## Verification

- RED (app): `go test ./internal/app/ -run 'Capturing|CtrlC|PReaches|ShowAgentAlerts'`
  failed with `undefined: ShowAgentAlertsMsg`.
- RED (statusbar): failed with `unknown field Indexer in struct literal of type State`.
- RED (agents): failed with `v.CapturesKeys undefined`.
- GREEN: `go vet ./...` clean. `go test -race ./...` all packages pass.
  `go build ./cmd/blackwall` succeeds.

## Deviations

- The `ctrl+c` check sits after the palette block, as the brief specifies. While the
  `:` palette is open, ctrl+c is consumed by the palette and does not quit. Pre-existing
  palette behaviour, left as specified.
- `cmd/blackwall/main.go` status func is not changed. `internal/config` has no indexer
  setting, so there is no value to pass into `statusbar.State.Indexer` yet. Wiring
  belongs with the config work in a later task.
- `gofmt -w` was applied to `internal/app/app_test.go`. It had pre-existing misaligned
  one-line methods, and this task added one more.
- The commit has no Co-Authored-By trailer, following the plan's constraints file.

## Open questions

- Should the ctrl+c quit check move above the palette block so it always quits?
