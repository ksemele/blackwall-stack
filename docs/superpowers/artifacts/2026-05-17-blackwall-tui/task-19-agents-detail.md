# Task 19 — :agents detail view
**Status:** DONE
**Date:** 2026-05-17
**Commits:** eeed4a6

## What was done
- `internal/views/agents/detail.go`: DetailModel, NewDetail, Init (one-shot GetAgent), DetailLoadedMsg, Esc→BackMsg, r→re-fetch, metadata grid render.
- Tests: render all fields, Esc emits BackMsg. Shares fakeClient from list_test.go.

## Verification
`go test ./internal/views/agents/... -v` → 6/6 PASS (4 list + 2 detail).

## Deviations
- Executed inline.

## Concerns / open questions
none
