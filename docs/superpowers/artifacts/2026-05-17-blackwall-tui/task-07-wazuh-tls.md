# Task 7: Wazuh TLS Configuration

## Status
**COMPLETED**

## Date
2026-05-17

## Commits
- `fc139d3` feat(blackwall): wazuh TLS config (skip-verify + ca_file)

## What was done

Implemented TLS configuration support for Wazuh HTTP client in the `tools/blackwall/internal/wazuh` package.

### Files created:
1. **`tools/blackwall/internal/wazuh/tls.go`** — Main implementation
   - `buildHTTPClient(t config.TLS)` function that creates HTTP clients with TLS configuration
   - Supports `InsecureSkipVerify` flag for testing/development
   - Supports custom CA certificate file via `CAFile` config parameter
   - Loads CA certificates from PEM-encoded files
   - Configures 10-second timeout on HTTP client
   - Returns fully configured `*http.Client` ready for use

2. **`tools/blackwall/internal/wazuh/tls_test.go`** — Comprehensive test suite (4 tests)
   - `TestBuildHTTPClient_InsecureSkipVerify` — Validates TLS connections with verification disabled
   - `TestBuildHTTPClient_VerifyFailsWithoutSkip` — Validates that unverified certificates are rejected by default
   - `TestBuildHTTPClient_CAFileMissing` — Validates error handling for missing CA files
   - `TestBuildHTTPClient_CAFileLoaded` — Validates successful loading of CA certificates

## Verification

All 4 tests pass:
```
=== RUN   TestBuildHTTPClient_InsecureSkipVerify
--- PASS: TestBuildHTTPClient_InsecureSkipVerify (0.01s)
=== RUN   TestBuildHTTPClient_VerifyFailsWithoutSkip
--- PASS: TestBuildHTTPClient_VerifyFailsWithoutSkip (0.02s)
=== RUN   TestBuildHTTPClient_CAFileMissing
--- PASS: TestBuildHTTPClient_CAFileMissing (0.00s)
=== RUN   TestBuildHTTPClient_CAFileLoaded
--- PASS: TestBuildHTTPClient_CAFileLoaded (0.00s)
PASS
ok  	github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh	1.304s
```

## Deviations

**Execution order note:** Task 7 was executed BEFORE Task 6 (Authenticate) due to a dependency constraint. Task 6's `client.go` depends on the `buildHTTPClient` function provided by this task. Executing in plan order would have left the wazuh package uncompilable between commits. The task controller approved executing Task 7 first to unblock Task 6.

## Concerns

None. All tests pass, implementation is complete, and the function is ready to be imported and used by the authentication client in Task 6.
