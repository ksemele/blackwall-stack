# Task 20 — Logging (slog JSON)
**Status:** DONE
**Date:** 2026-05-17
**Commits:** 542cc23

## What was done
- `internal/log/log.go`: Options{File, Level}, Setup() creates parent dir (0o700), opens file (0o600), returns slog JSON logger + io.Closer.
- Tests: writes JSON, parses all 4 levels, unknown level defaults to warn.

## Verification
`go test ./internal/log/... -v` → 3/3 PASS.

## Deviations
none

## Concerns / open questions
none
