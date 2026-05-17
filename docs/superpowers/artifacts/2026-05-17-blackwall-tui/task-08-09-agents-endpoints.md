# Tasks 8 + 9 — Wazuh Agent Endpoints

## Status
COMPLETE

## Date
2026-05-17

## Commits

| SHA | Message |
|---|---|
| `f6b79ae` | feat(blackwall): wazuh ListAgents + GetAgent |
| `173184a` | test(blackwall): wazuh GetAgent + 401 retry |

## What was done

### Task 8 — ListAgents
- Created `tools/blackwall/internal/wazuh/agents.go` with `ListAgents` (GET `/agents?limit=500`) and `GetAgent` (GET `/agents?agents_list=<id>`), plus `ErrAgentNotFound` sentinel and `neverExpire` helper.
- Created `tools/blackwall/internal/wazuh/agents_test.go` with 2 T8 tests:
  - `TestListAgents_OK` — verifies auth handshake + correct agent list returned.
  - `TestListAgents_ContextCancel` — verifies cancelled context propagates to error.

### Task 9 — GetAgent + 401 retry
- Appended 3 T9 tests to `agents_test.go`:
  - `TestGetAgent_OK` — verifies `agents_list` query param and correct agent returned.
  - `TestGetAgent_NotFound` — verifies empty `affected_items` maps to `ErrAgentNotFound`.
  - `TestDoAuthed_RetriesOn401` — verifies `doAuthed` re-authenticates and retries once on 401.

Both methods delegate to the existing `doAuthed` helper in `client.go`, which already handles token lifecycle and the 401 retry loop. No changes to existing files were needed.

## Verification

### After T8 commit (2 new tests):
```
=== RUN   TestListAgents_OK
--- PASS: TestListAgents_OK (0.00s)
=== RUN   TestListAgents_ContextCancel
--- PASS: TestListAgents_ContextCancel (0.00s)
PASS
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh  1.300s
```

### After T9 commit (all 11 wazuh tests):
```
=== RUN   TestListAgents_OK
--- PASS: TestListAgents_OK (0.00s)
=== RUN   TestListAgents_ContextCancel
--- PASS: TestListAgents_ContextCancel (0.00s)
=== RUN   TestGetAgent_OK
--- PASS: TestGetAgent_OK (0.00s)
=== RUN   TestGetAgent_NotFound
--- PASS: TestGetAgent_NotFound (0.00s)
=== RUN   TestDoAuthed_RetriesOn401
--- PASS: TestDoAuthed_RetriesOn401 (0.00s)
=== RUN   TestAuthenticate_OK
--- PASS: TestAuthenticate_OK (0.00s)
=== RUN   TestAuthenticate_401
--- PASS: TestAuthenticate_401 (0.00s)
=== RUN   TestBuildHTTPClient_InsecureSkipVerify
--- PASS: TestBuildHTTPClient_InsecureSkipVerify (0.00s)
=== RUN   TestBuildHTTPClient_VerifyFailsWithoutSkip
--- PASS: TestBuildHTTPClient_VerifyFailsWithoutSkip (0.02s)
=== RUN   TestBuildHTTPClient_CAFileMissing
--- PASS: TestBuildHTTPClient_CAFileMissing (0.00s)
=== RUN   TestBuildHTTPClient_CAFileLoaded
--- PASS: TestBuildHTTPClient_CAFileLoaded (0.00s)
PASS
ok  github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh  0.950s
```

## Deviations

T8 and T9 were dispatched together in a single subagent session as instructed, since they share files and sequential commits were simpler than parallel. The two-commit split was preserved by writing only T8 tests first, committing, then appending T9 tests and committing separately.

## Concerns

None. The `agents.go` implementation is minimal by design — no paging, no config fetch. Both are noted as future enhancements in code comments.
