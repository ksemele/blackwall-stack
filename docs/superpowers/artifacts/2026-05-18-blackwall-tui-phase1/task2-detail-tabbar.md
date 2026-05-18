# Task 2: agent-detail tabbar container — artifact

## Status
DONE

## Commits
- `9a36d99` refactor(detail): split into tabbar container with tab hotkeys
- `10efd44` test(detail): cover tab switching via h/o keys and tabbar render

## What was done

### detail.go
- Added `activeTab int` and `width int` fields to `DetailModel`.
- Added `detailTabs []string` — fixed order: Overview, Hardware, Top, Packages, Network, Security.
- Added `tabIndex map[string]int` mapping `o/h/t/k/n/s` to tab indices 0–5.
- `Update()`: handles `tea.WindowSizeMsg` (saves width), tab hotkeys via `tabIndex` lookup, preserves `esc`/`left` (BackMsg) and `r` (refresh).
- `View()`: renders a one-line header (id, name, colored status), then `tabbar.Render(detailTabs, m.activeTab, m.width, "")`, then `renderActiveTab()`. Error shown in place of tab body.
- `renderOverview()`: extracted from old `View()` — renders Status/IP/Version/OS/Last seen/Registered/Manager.
- `renderActiveTab()`: switch on `activeTab`; case 0 → `renderOverview()`, cases 1–5 → placeholder strings.
- `Hints()`: expanded to include `o/h/t/k/n/s tabs`, `esc back`, `r refresh`, `? help`.

### detail_test.go
- Existing tests preserved and passing.
- Added `TestDetail_HKeySwitchesToHardwareTab` — pressing `h` sets `activeTab=1`.
- Added `TestDetail_OKeyReturnsToOverviewTab` — pressing `o` after `h` sets `activeTab=0`.
- Added `TestDetail_TabbarRenderedWithHardwareActive` — `View()` with `activeTab=1` contains "Hardware".

## Verification
```
make test  → all packages pass (including agents 2.202s)
make build → bin/blackwall compiled clean
go vet     → no issues
```

## Deviations
None. Implementation follows spec exactly: no Subview interface/registry introduced, no new exported types, tabbar.Render called with empty version string.

## Open questions
None — placeholders for tasks 3–5 are simple switch cases, ready to be replaced with real subview calls.
