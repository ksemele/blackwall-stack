# Task 1: Wazuh REST client extension

**Status:** DONE_WITH_CONCERNS
**Module:** tools/blackwall
**Date:** 2026-05-18
**Commits:** 0bf99d9, d8b3d4d, 34dc379, b4b6dc4, cac8d58, 2f02a54

## What was done
- Extended `types.go` with 15 new exported structs: `Hardware`, `OSInfo`, `Process`, `Package`, `Port`, `Netiface`, `Netaddr`, `SyscheckEntry`, `SyscheckScan`, `RootcheckFinding`, `RootcheckScan`, `SCAPolicy`, `SCACheck`, `AgentStats`, `LogcollectorStats`
- Added 10 private response wrappers in `types.go` (one per list/scalar return type)
- Created `syscollector.go` with 7 methods: `GetHardware`, `GetOSInfo`, `ListProcesses`, `ListPackages`, `ListPorts`, `ListNetiface`, `ListNetaddr`
- Created `syscheck.go` with 2 methods: `GetSyscheckLastScan`, `ListSyscheckEntries`
- Created `rootcheck.go` with 2 methods: `GetRootcheckLastScan`, `ListRootcheckFindings`
- Created `sca.go` with 2 methods: `ListSCAPolicies`, `ListSCAChecks`
- Created `stats.go` with 2 methods: `GetAgentStats`, `GetLogcollectorStats`
- Created corresponding `*_test.go` files for all 5 new files (happy path, auth header check, path/query check, non-200 error)
- Added shared `authOK` helper in `syscollector_test.go` (used by all new test files in the same package)

## Verification
- `make test` — all 15 packages pass, including new wazuh tests (`ok github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh 8.686s`)
- `make build` — `go build -o bin/blackwall ./cmd/blackwall` succeeded, no errors
- `make lint` — `golangci-lint` not installed on this machine; `go vet ./...` passed with zero warnings

## Deviations from plan (if any)
- `authOK` helper added to `syscollector_test.go` to avoid duplicating the auth stub across all 5 test files. It lives in the same `package wazuh` test scope so all `*_test.go` files share it without introducing a new file.
- `SyscheckScan.Start/End`, `RootcheckScan.Start/End`, `SCAPolicy.StartScan/EndScan` typed as `string` rather than `time.Time`. Wazuh 4.x returns these timestamps in a non-RFC3339 format (`"2026-05-18 00:00:00"` with space separator); using `string` avoids silent decode failures until a `wazuhTime` custom type is warranted (can be added in a follow-up without breaking callers).

## Open questions / concerns
- `AgentStats` and `LogcollectorStats` field names (`events`, `events_dropped`, `events_per_second`, `bytes`) are best-guesses from Wazuh docs; both structs carry `// TODO: verify field name against live Wazuh 4.14` comments. If the live API returns a different shape (e.g. nested objects), these will silently zero-out — adjust when a real Wazuh instance is available.
- `golangci-lint` is not installed in the dev environment. `go vet` is clean; lint should be confirmed when the tool is available.
- `ListSCAChecks` does not apply `?limit=500` (the plan didn't specify it and the Wazuh spec for this endpoint doesn't document a `limit` param for checks — it returns all checks for a policy). Easy to add if needed.
