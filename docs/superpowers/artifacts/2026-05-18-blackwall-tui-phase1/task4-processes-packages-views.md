# Task 4: Processes + Packages sub-views with `/` filter

**Date:** 2026-05-18
**Branch:** claude-dev
**Status:** Complete

## What was done

### A. `keyCapturer` interface (subview.go)
Added optional interface `keyCapturer { CapturesKeys() bool }` — not part of `subView`, so Hardware/Network are unaffected.

### B. `detail.go` key forwarding
At the top of the `tea.KeyMsg` branch: if the active sub-view implements `keyCapturer` and returns `true`, all key events are forwarded directly to the sub-view, bypassing tab-switch and esc/refresh hotkeys.

### C. `processes.go`
- `ProcessesClient` interface (single method `ListProcesses`).
- `ProcessesModel` with async `Init()`, `CapturesKeys() bool`, and `/`-triggered filter.
- Filter logic: case-insensitive match on `Name`, `Cmd`, `Euser`.
- Table columns: PID / PPID / USER / RSS(KB) / NAME / COMMAND.
- Filter prompt rendered inline below the table.

### D. `packages.go`
- `PackagesClient` interface (single method `ListPackages`).
- `PackagesModel` — same filter pattern, fields: `Name`, `Vendor`, `Description`.
- Table columns: NAME / VERSION / VENDOR / INSTALL TIME.

### E. Wire-up in `detail.go::NewDetail`
Processes wired as tab 2, Packages as tab 3 (alongside existing Hardware tab 1, Network tab 4).

### F. `Client` interface extended (`list.go`)
Added `ListProcesses` and `ListPackages` to the package-level `Client` interface so `*wazuh.Client` satisfies it end-to-end.

### G. Tests
- `processes_test.go`: 7 tests — happy path, slash→filter, typing, esc clear, enter apply, filter match, error/loading states.
- `packages_test.go`: 8 tests — same coverage plus vendor-based filter.
- `detail_test.go`: 3 new tests — `t` activates `*ProcessesModel`, `k` activates `*PackagesModel`, `h` is consumed by filter when filtering=true.
- `list_test.go`: added `ListProcesses`/`ListPackages` zero-value stubs to `fakeClient`.

## Files changed

- `tools/blackwall/internal/views/agents/subview.go` — `keyCapturer` interface
- `tools/blackwall/internal/views/agents/processes.go` — new
- `tools/blackwall/internal/views/agents/packages.go` — new
- `tools/blackwall/internal/views/agents/detail.go` — key forwarding + wire-up
- `tools/blackwall/internal/views/agents/list.go` — Client interface extended
- `tools/blackwall/internal/views/agents/processes_test.go` — new
- `tools/blackwall/internal/views/agents/packages_test.go` — new
- `tools/blackwall/internal/views/agents/detail_test.go` — 3 new tests
- `tools/blackwall/internal/views/agents/list_test.go` — fakeClient stubs

## Verification

```
make test  → ok (all packages)
make build → clean
go vet     → clean
```

## Commits

- `2c213af` feat(agents): add keyCapturer interface to subview
- `8a1bfd3` feat(agents): add processes subview with filter
- `185d93f` feat(agents): add packages subview with filter
- `963ddcc` refactor(detail): forward keys to filtering subview; wire processes+packages
- `27de92b` test(agents): cover processes+packages filter and detail key routing

## Notes

- Vendor column in packages truncates to 16 runes (matches TUI width budget); test checks prefix "Ubuntu" rather than full "Ubuntu Developers".
- No scroll/sort/pagination per YAGNI spec.
- `truncate` helper reused from `network.go` (same package).
