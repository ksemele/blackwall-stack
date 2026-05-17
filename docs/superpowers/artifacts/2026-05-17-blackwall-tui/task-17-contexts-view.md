# Task 17 — :contexts picker view
**Status:** DONE
**Date:** 2026-05-17
**Commits:** e5fe7e1

## What was done
- Created `internal/views/contexts/list.go` (Model, New, Update, View, Hints, contextAt).
- Tests: render, Enter→ContextChangedMsg, Down→cursor move.

## Verification
`go test ./internal/views/contexts/... -v` → 3/3 PASS.

## Deviations
- Executed inline (subagent rate limit hit during T17-T19 batch dispatch).

## Concerns / open questions
none
