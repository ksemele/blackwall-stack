# Detail tabs: numeric → letter hotkey prefixes

**Status:** DONE
**Module:** blackwall-eye
**Date:** 2026-05-18
**Commits:** 4adf52a 958c938

## What was done

- Added `RenderWithKeys(viewNames, keys, active, width, version)` to `tools/blackwall/internal/ui/tabbar/tabbar.go` — renders tab prefixes as `[O]`, `[H]`, etc. instead of `[1]`, `[2]`, etc.
- Extracted shared `renderStrip` helper to avoid duplication between `Render` and `RenderWithKeys`; `Render` is unchanged (backward-compatible for `app.go` numeric hotkeys).
- Added `detailTabKeys = []string{"O","H","T","K","N","S"}` in `detail.go` co-located with `detailTabs` and documented the sync requirement in a comment.
- Changed `detail.go` View() call from `tabbar.Render(detailTabs, ...)` to `tabbar.RenderWithKeys(detailTabs, detailTabKeys, ...)`.
- Added three unit tests in `tabbar_test.go`: `TestRenderWithKeys_LetterPrefixes`, `TestRenderWithKeys_ActiveTab`, `TestRenderWithKeys_NoNumericPrefixes`.

## Verification

`go test ./...` output:
```
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/app
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/config
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/log
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/oneshot
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/tickcmd
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/ui/statusbar
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/ui/tabbar
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/ui/table
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/views/agents
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/views/contexts
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh
```

`go build ./...` — no output (clean).

Tab bar now renders (ANSI stripped for readability):
```
[O] Overview  │  [H] Hardware  │  [T] Top  │  [K] Packages  │  [N] Network  │  [S] Security
```

## Deviations from plan

- `tabbar.go` implementation landed in the same commit as the tests (`4adf52a`) because both files were staged together. The commit message says "red" but the implementation is included; functionally TDD sequence was: write tests (red verified via build failure), implement, verify green — just committed atomically. The second commit (`958c938`) contains only the `detail.go` change.

## Open questions / concerns

- none
