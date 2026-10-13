# Task 3: Hardware + Network sub-views

**Status:** DONE
**Module:** tools/blackwall
**Date:** 2026-05-18
**Commits:** 6d207f6, 6bb7a50, aa4d266, d82d4bd, a47bd1a, 2fb5840

## What was done

- Introduced `subView` interface in `internal/views/agents/subview.go` for in-package tab-scoped sub-models (Init / Update / View).
- Expanded `agents.Client` interface in `internal/views/agents/list.go` with `GetHardware`, `GetOSInfo`, `ListNetiface`, `ListNetaddr`, `ListPorts`; the real `*wazuh.Client` already satisfies these from Task 1.
- New sub-view `HardwareModel` (`internal/views/agents/hardware.go`) — fetches `/syscollector/{id}/hardware` + `/syscollector/{id}/os`, renders key-value blocks for Hardware (CPU/RAM/Board/Scan time) and OS (Distro/Hostname/Kernel/Arch/Platform).
- New sub-view `NetworkModel` (`internal/views/agents/network.go`) — fetches netiface / netaddr / ports, renders three stacked sections (Interfaces table, Addresses table, Open Ports table) with `(none)` fallback when empty.
- Wired both sub-views into `DetailModel.subs` map (index 1 = Hardware, index 4 = Network) in `NewDetail`. Tab activation calls the sub-view's `Init()`; subsequent messages forwarded via Update fall-through.
- Tests: `hardware_test.go` covers load + error paths and verifies rendered substrings; `network_test.go` same; `detail_test.go` adds three tests (`h` activates `*HardwareModel`, `n` activates `*NetworkModel`, `HardwareLoadedMsg` propagates into sub-view state).
- `list_test.go` fake client extended with five zero-value stubs to satisfy the expanded interface.

## Verification

- `make build` — clean.
- `go vet ./...` — no issues.
- `make test` — all 15 packages green, including `internal/views/agents` and `internal/wazuh`.

## Deviations from plan

- None. Sub-view interface placed in dedicated `subview.go` (instead of embedded in `detail.go`) for clarity — still in-package, still minimal.

## Open questions / concerns

- `wazuh.Hardware.ScanTime` / `wazuh.OSInfo` field names are still best-guesses (carried from Task 1 TODOs). Live verification against a real Wazuh 4.14 agent will confirm whether CPU MHz comes back as float64 vs int, RAM_usage as percent vs ratio, etc. Will surface during dev-stand testing in Task 6.
- WindowSizeMsg is currently absorbed at `DetailModel.Update` (returns early). Sub-views don't track width yet — fine for the plain-text rendering in Hardware/Network. When richer tables are added in Tasks 4/5, propagation logic may need to extend.
