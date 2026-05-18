# Top decode error + Packages scrolling
**Status:** DONE
**Module:** blackwall-eye (TUI)
**Date:** 2026-05-18
**Commits:** 0d80136 da7addf

## What was done

- **FlexInt added** (`tools/blackwall/internal/wazuh/flexint.go`): custom JSON unmarshaler that accepts both `"1"` (quoted string) and `1` (bare int), stores as int64. Handles null and empty string as 0.
- **Process.PID switched to FlexInt**: Wazuh syscollector returns pid as a JSON string; other numeric fields (ppid, tgid, etc.) remain plain int. Port.PID also switched defensively — same syscollector origin, same schema quirk.
- **PPID decision**: kept as `int`. Tested against live Wazuh responses; ppid always arrives unquoted. Switching it to FlexInt would be defensive but adds noise with no observed benefit.
- **scrollist helper** (`tools/blackwall/internal/views/agents/scrollist.go`): ~100 LOC shared scroll logic — scroll state, clamp, key handling (j/k/ctrl+d/ctrl+u/g/G), and Render() with sticky header. Shared between PackagesModel and ProcessesModel to avoid duplication.
- **sizedView + hintProvider interfaces** added to `subview.go`; DetailModel now forwards WindowSizeMsg to all sub-views that implement sizedView, and picks up footer hints from hintProvider.
- **PackagesModel and ProcessesModel rewritten** with scrollist: sticky header, windowed rows, scroll indicator `N–M of total` embedded in count line, filter resets scroll to 0.

## Verification

### go build ./...
```
(no output — clean build)
```

### go test ./...
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

### Rendered Packages view (height=20, 100 packages)
```
NAME                                  VERSION               VENDOR            INSTALL TIME
──────────────────────────────────────────────────────────────────────────────────────────
pkg-000                               1.0.0                 TestVendor        2025-01-01
pkg-001                               1.1.0                 TestVendor        2025-01-01
pkg-002                               1.2.0                 TestVendor        2025-01-01
pkg-003                               1.3.0                 TestVendor        2025-01-01
pkg-004                               1.4.0                 TestVendor        2025-01-01
pkg-005                               1.5.0                 TestVendor        2025-01-01
pkg-006                               1.6.0                 TestVendor        2025-01-01
pkg-007                               1.7.0                 TestVendor        2025-01-01
pkg-008                               1.8.0                 TestVendor        2025-01-01
pkg-009                               1.9.0                 TestVendor        2025-01-01
pkg-010                               1.10.0                TestVendor        2025-01-01
pkg-011                               1.11.0                TestVendor        2025-01-01
pkg-012                               1.12.0                TestVendor        2025-01-01
pkg-013                               1.13.0                TestVendor        2025-01-01
pkg-014                               1.14.0                TestVendor        2025-01-01
pkg-015                               1.15.0                TestVendor        2025-01-01

100/100  1–16 of 100
```
Total: 20 lines. Header sticky. Footer shows count + scroll range.

### Rendered Top (Processes) view (height=20, 50 processes, PIDs as FlexInt)
```
PID      PPID     USER        RSS(KB)     NAME                  COMMAND
────────────────────────────────────────────────────────────────────────────────
1        1        root        1024        proc-00               /usr/bin/proc-00
2        1        root        2048        proc-01               /usr/bin/proc-01
...
16       1        root        16384       proc-15               /usr/bin/proc-15

50/50  1–16 of 50
```
PIDs render as integers (FlexInt int64 backed — format verbs work natively).

## Deviations from plan (if any)

- `footerRows=2` structure: rather than a bare indicator appended by Render(), the indicator is embedded in the count line (`100/100  1–16 of 100`) so the total line budget is strictly `headerRows + visibleRows + footerRows`. Cleaner accounting.
- Pre-resize fallback (height=0): Render() falls back to full-list output unchanged, matching previous behavior before any WindowSizeMsg arrives.

## Open questions / concerns
- none
