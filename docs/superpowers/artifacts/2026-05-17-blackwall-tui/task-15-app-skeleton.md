# Task 15: app.Model Skeleton + Global Hotkeys

Status: DONE
Date: 2026-05-17
Commits: f571358 feat(blackwall): app.Model + global hotkeys + tab nav

## What was done

- Created `tools/blackwall/internal/app/app_test.go` with four table-driven tests covering `SwitchViewMsg` routing, Tab cycling, number-key jumping, and `q` quit.
- Created `tools/blackwall/internal/app/app.go` implementing `Model`, `New(Deps)`, `Active()`, `Init()`, `Update()`, and `View()` with full global hotkey handling (q/ctrl+c quit, tab/shift+tab cycle, 1-9 jump, p pause toggle, SwitchViewMsg routing) and delegation to the active view.
- All 4 tests pass; full suite (`go test ./...`) is green across all 9 packages.

## Verification

```
go test ./internal/app/... -v
# TestApp_SwitchViewMsg  PASS
# TestApp_TabCyclesForward PASS
# TestApp_NumberKeyJumps  PASS
# TestApp_QuitsOnQ        PASS

go test ./...
# all packages ok
```

## Deviations

Removed the `var _ = fmt.Sprintf` no-op hack present in an earlier plan draft — it was a leftover workaround for an unused-import error that no longer exists in the cleaned implementation.

## Concerns

None.
