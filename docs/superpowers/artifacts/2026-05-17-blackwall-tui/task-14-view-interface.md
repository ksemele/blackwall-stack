# Task 14: View Interface + App Messages

Status: DONE
Date: 2026-05-17
Commits: fb3250f feat(blackwall): View interface + app messages

## What was done

- Created `tools/blackwall/internal/views/view.go` defining the `View` interface that all top-level screens implement (`tea.Model` + `Name()` + `Hints()`).
- Created `tools/blackwall/internal/app/messages.go` declaring the four app-level message types: `SwitchViewMsg`, `SwitchToDetailMsg`, `BackMsg`, `ContextChangedMsg`.
- `go build ./...` passed clean with no errors.

## Verification

```
cd tools/blackwall && go build ./...
# exit 0, no output
```

## Deviations

None.

## Concerns

None.
