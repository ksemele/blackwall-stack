# Task 06 — Wazuh Client: Types + Authenticate

## Status

COMPLETE

## Date

2026-05-17

## Commits

- `e9ec441` — feat(blackwall): wazuh client Authenticate + token cache

## What was done

Created three new files in `tools/blackwall/internal/wazuh/`:

- **types.go** — Defines `Agent`, `AgentDetail`, and unexported response types (`agentsResponse`, `singleAgentResponse`, `authResponse`) used by the client and future list/get methods.
- **client.go** — Implements `Client` struct with `New()` constructor (using existing `buildHTTPClient`), `Authenticate()` (POST to `/security/user/authenticate` with Basic Auth, stores JWT + 13-minute expiry), `doAuthed()` (JWT bearer requests with single 401 retry), `ensureToken()`, and `invalidateToken()`. The `insecureScheme` field allows test override from "https" to "http".
- **client_test.go** — Two table-style tests: `TestAuthenticate_OK` (verifies correct path, Authorization header, and token stored) and `TestAuthenticate_401` (verifies error returned on HTTP 401). Uses `httptest.Server` with `insecureScheme = "http"` override.

## Verification

```
=== RUN   TestAuthenticate_OK
--- PASS: TestAuthenticate_OK (0.00s)
=== RUN   TestAuthenticate_401
--- PASS: TestAuthenticate_401 (0.00s)
=== RUN   TestBuildHTTPClient_InsecureSkipVerify
--- PASS: TestBuildHTTPClient_InsecureSkipVerify (0.00s)
=== RUN   TestBuildHTTPClient_VerifyFailsWithoutSkip
--- PASS: TestBuildHTTPClient_VerifyFailsWithoutSkip (0.01s)
=== RUN   TestBuildHTTPClient_CAFileMissing
--- PASS: TestBuildHTTPClient_CAFileMissing (0.00s)
=== RUN   TestBuildHTTPClient_CAFileLoaded
--- PASS: TestBuildHTTPClient_CAFileLoaded (0.00s)
PASS
ok  	github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh	0.471s
```

All 6 wazuh package tests pass. Task 7's `tls.go`/`tls_test.go` were not modified.

## Deviations

None. Files match the plan exactly.

## Concerns

None.
