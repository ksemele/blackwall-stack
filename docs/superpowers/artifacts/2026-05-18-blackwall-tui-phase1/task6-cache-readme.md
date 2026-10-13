# Task 6: TTL Cache + README — Completion Artifact

**Date:** 2026-05-18
**Branch:** claude-dev
**Phase:** 1, final task (6/6)

## Status

DONE — all tests green, binary builds, go vet clean.

## Commits

| SHA | Message |
|---|---|
| `1861920` | feat(wazuh): add TTL cache for slow-changing endpoints |
| `c9a2a49` | test(wazuh): cover cache hit/miss/error/invalidate paths |
| `cd43e10` | docs(blackwall): document agent-detail tabs, hotkeys, and inventory cache |

## Files changed

| File | Change |
|---|---|
| `internal/wazuh/cache.go` | new — `clientCache` struct, `cacheEntry`, generic `getCached[T]`, `InvalidateCache()` |
| `internal/wazuh/client.go` | embed `clientCache` in `Client`; init `cache` map in `New()` |
| `internal/wazuh/syscollector.go` | refactored 6 methods to wrap via `getCached`; fetch logic moved to `fetch*` helpers |
| `internal/wazuh/cache_test.go` | new — 5 tests: hit, TTL expiry, miss on error, key isolation, invalidate |
| `tools/blackwall/README.md` | added Agent detail section with tab/key table + caching note |

## Design decisions

- `clientCache` as an embedded struct keeps the cache fields grouped without polluting `Client` directly.
- Generic `getCached[T]` (Go 1.22+) avoids repetitive type-specific helpers.
- Each slow-changing method gets its own `fetch*` private helper; the public method is a one-liner that delegates to `getCached`.
- `InvalidateCache()` is exposed so the TUI `r` key binding can clear all entries and force a fresh fetch.
- Existing `syscollector_test.go` tests were unaffected: each creates a fresh `*Client` via `New()`, so the cache is always empty at test start.

## TTL assignments

| Method | TTL |
|---|---|
| `GetHardware` | 60s |
| `GetOSInfo` | 60s |
| `ListPackages` | 60s |
| `ListNetiface` | 30s |
| `ListNetaddr` | 30s |
| `ListPorts` | 30s |
| `ListProcesses` | not cached |
| `ListAgents`, `GetAgent` | not cached |
| syscheck / rootcheck / sca / stats | not cached |

## Verification

```
make test   → all packages PASS (race detector on)
make build  → bin/blackwall built OK
go vet      → clean
```

## Phase 1 complete

All 6 tasks done:
1. Wazuh REST client (5 endpoint groups)
2. Agent detail with tabbar (6 tabs)
3. Hardware + Network views
4. Processes + Packages views with filter
5. Security composite view (FIM/rootcheck/SCA)
6. TTL cache + README (this task)
