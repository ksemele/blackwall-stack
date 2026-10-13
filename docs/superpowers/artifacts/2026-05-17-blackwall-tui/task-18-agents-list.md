# Task 18 — :agents list view + tick loop
**Status:** DONE
**Date:** 2026-05-17
**Commits:** 26a7ce2

## What was done
- `internal/views/agents/list.go`: Model, Client interface, New(client, refresh), Init/Update/View, AgentsLoadedMsg, fetchCmd, tick re-arm, pause/refresh/Enter hotkeys.
- Tests: render, tick async (no sync fetch), pause skips fetch, Enter→SwitchToDetailMsg.

## Verification
`go test ./internal/views/agents/... -v` → 4/4 PASS (after T18).

## Deviations
- Executed inline (subagent rate limit).
- Minor lint suggestion (QF1012 WriteString(Sprintf) → Fprintf) ignored — stylistic, not a bug.

## Concerns / open questions
none
