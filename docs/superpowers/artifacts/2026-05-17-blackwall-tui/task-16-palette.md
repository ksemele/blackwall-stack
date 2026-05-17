# Task 16 — Command Palette

## Status
DONE

## Date
2026-05-17

## Commits
- `8ceaff6` feat(blackwall): command palette parser + UI

## What was done
- **parsePalette**: implemented in `palette.go` — parses `:q`/`:quit` → `QuitMsg`, `:ctx <name>` → `ContextChangedMsg`, bare view names → `SwitchViewMsg`, unknown input → error.
- **Model palette state**: extended `Model` struct with `paletteOpen bool`, `paletteInput string`, `paletteErr string`; added palette-mode branch in `Update` (`:` opens, `esc` closes, `enter` dispatches, `backspace` edits, runes accumulate).
- **View rendering**: palette prompt (`> <input>`) and optional error line (styled via `styles.Error`) appended to body when `paletteOpen`; added `viewNames` helper to extract view name slice.

## Verification

```
=== RUN   TestApp_SwitchViewMsg
--- PASS: TestApp_SwitchViewMsg (0.00s)
=== RUN   TestApp_TabCyclesForward
--- PASS: TestApp_TabCyclesForward (0.00s)
=== RUN   TestApp_NumberKeyJumps
--- PASS: TestApp_NumberKeyJumps (0.00s)
=== RUN   TestApp_QuitsOnQ
--- PASS: TestApp_QuitsOnQ (0.00s)
=== RUN   TestParsePalette_BareViewName
--- PASS: TestParsePalette_BareViewName (0.00s)
=== RUN   TestParsePalette_CtxShortcut
--- PASS: TestParsePalette_CtxShortcut (0.00s)
=== RUN   TestParsePalette_Quit
--- PASS: TestParsePalette_Quit (0.00s)
=== RUN   TestParsePalette_Unknown
--- PASS: TestParsePalette_Unknown (0.00s)
PASS
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/app  0.411s
```

Full `go test ./...` green (all packages).

## Deviations
None. Implemented exactly as specified.

## Concerns
None.
