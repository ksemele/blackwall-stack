# Task 22 — main.go wiring
**Status:** DONE
**Date:** 2026-05-17
**Commits:** fa1686f

## What was done
- `cmd/blackwall/main.go`: parse flags+env, EnsureExists config, Load→ApplyDefaults→ExpandEnv→Resolve→Validate chain, slog setup, wazuh.New from current context, --once → oneshot.Print, otherwise build views (contexts + agents) and tea.NewProgram with AltScreen.
- Mutual exclusion check for `--select-context-on-start` + `--no-select-context`.
- Initial view selection respects SelectContextOnStart.

## Verification
- `go build ./...` → success
- `go test ./...` → all 12 packages PASS

## Deviations
none

## Concerns / open questions
none
