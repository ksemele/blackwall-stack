# Task 21 — --once text printer
**Status:** DONE
**Date:** 2026-05-17
**Commits:** 391d8c1

## What was done
- `internal/oneshot/oneshot.go`: Lister interface, Print(ctx, out, c) — fetches agents and prints tab table.
- Test verifies all expected columns + row data present.

## Verification
`go test ./internal/oneshot/... -v` → 1/1 PASS.

## Deviations
none

## Concerns / open questions
none
