# Task 5: Security Sub-View (FIM / Rootcheck / SCA)

**Status:** complete  
**Branch:** claude-dev  
**Date:** 2026-05-18

## Files

| File | Action |
|---|---|
| `internal/views/agents/security.go` | new — SecurityModel, SecurityClient interface, SecurityLoadedMsg |
| `internal/views/agents/security_test.go` | new — happy path, empty path, partial errors, loading state |
| `internal/views/agents/detail.go` | subs[5] wired; placeholder case 5 removed from renderActiveTab |
| `internal/views/agents/list.go` | Client interface extended with 5 security methods |
| `internal/views/agents/list_test.go` | fakeClient extended with zero-value stubs for 5 security methods |
| `internal/views/agents/detail_test.go` | TestDetail_SKeyActivatesSecuritySubview added |

## Commits

- `17a07b4` feat(agents): add security subview (FIM/rootcheck/SCA)
- `1bf3af7` refactor(detail): wire security subview + extend Client interface
- `359a4a2` test(agents): cover security subview empty+happy+partial paths

## Design notes

- All 5 endpoints called in one Init() cmd; errors silently ignored per spec — each section shows `(no data)` / `(none)` / `(no scan yet)` when empty.
- FIM entries sorted by Mtime DESC (fallback to Date), capped at 10.
- Rootcheck filters to `status != "solved"` before display.
- SCA pass % coloured green ≥85%, yellow ≥70%, red <70%.
- `SecurityClient` interface is a strict subset of the extended `Client`, so `*wazuh.Client` satisfies it automatically.

## Test results

```
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/views/agents  1.847s
```

All tests pass. `make build` and `go vet ./...` clean.
