# Move scroll list to internal/ui/scroll

**Status:** Done
**Module:** tools/blackwall (TUI)
**Date:** 2026-10-14
**Commits:** e40608d refactor(blackwall): move scroll list to internal/ui/scroll

## What was done

- `scrollist` moved from `internal/views/agents/scrollist.go` to
  `internal/ui/scroll/scroll.go` as the exported `scroll.List`.
- `HeaderRows` and `FooterRows` are now exported fields.
- `Height()` and `Reset()` added. `Reset()` replaces the `m.sl.scroll = 0` writes.
- `PackagesModel` and `ProcessesModel` use `scroll.List`, with the `scroll` import.
- `view_test.go` reads `pk.sl.Height()` instead of the private `height` field.
- Behaviour is unchanged. `SetSize`, `HandleKey`, `Indicator` and `Render` have the same logic.

## Verification

- RED: `go test ./internal/ui/scroll/` failed with `undefined: List` (package had no implementation).
- GREEN: `go vet ./...` clean. `go test ./...` passes, including `internal/ui/scroll` and
  `internal/views/agents`. `go test -race ./...` passes.
- `gofmt -l` reports nothing for the new and touched test/processes files.

## Deviations

- `packages.go` keeps its pre-existing alignment of `client:`/`agentID:` in
  `NewPackages`. `gofmt -l` flags that file for that reason. The lines are outside this
  change, so I left them alone.
- Commit message has no Co-Authored-By trailer, per the global constraints file.

## Open questions

- None.
