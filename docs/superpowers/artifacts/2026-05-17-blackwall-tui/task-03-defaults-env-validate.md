# Task 3: Config Defaults + Env Expansion + Validation

## Status

✅ **COMPLETE**

## Date

2026-05-17

## Commits

- `d1b0b15` — feat(blackwall): config defaults + env expand + validation

## What Was Done

Implemented three new functions in `tools/blackwall/internal/config/load.go` to handle post-load config processing:

1. **ApplyDefaults(cfg Config) Config** — Fills missing scalar fields with sensible defaults:
   - `UI.RefreshInterval` → 3s (if 0)
   - `UI.Theme` → "dark" (if empty)
   - `UI.VimKeys` → true (if false)
   - `Logging.Level` → "warn" (if empty)
   - `Logging.File` → `~/.local/state/blackwall/blackwall.log` (if empty)
   - `SelectContextOnStart` → true (if false, with caveat for MVP)

2. **ExpandEnv(cfg Config) (Config, error)** — Substitutes `${VAR}` syntax in Wazuh subsystem fields from process environment:
   - Walks all `Contexts.*.Wazuh` fields (server, user, password, tls.ca_file)
   - Uses regexp to detect and expand `${VAR}` placeholders
   - Returns error if any field with placeholder syntax resolves to empty after expansion (indicates missing env var)

3. **Validate(cfg Config) error** — Checks structural invariants:
   - `current` field is required and must reference a defined context
   - For each context with Wazuh subsystem: server, port, user, password are required
   - If TLS CAFile is specified, file must exist on disk

Added comprehensive test coverage with 6 new tests (after existing TestLoad_HappyPath):
- `TestApplyDefaults_FillsMissingFields`
- `TestExpandEnv_SubstitutesVars`
- `TestExpandEnv_ErrorsOnEmptyAfterExpand`
- `TestValidate_OK`
- `TestValidate_UnknownCurrent`
- `TestValidate_CAFileNotFound`

## Verification (Test Output)

```
=== RUN   TestLoad_HappyPath
--- PASS: TestLoad_HappyPath (0.00s)
=== RUN   TestApplyDefaults_FillsMissingFields
--- PASS: TestApplyDefaults_FillsMissingFields (0.00s)
=== RUN   TestExpandEnv_SubstitutesVars
--- PASS: TestExpandEnv_SubstitutesVars (0.00s)
=== RUN   TestExpandEnv_ErrorsOnEmptyAfterExpand
--- PASS: TestExpandEnv_ErrorsOnEmptyAfterExpand (0.00s)
=== RUN   TestValidate_OK
--- PASS: TestValidate_OK (0.00s)
=== RUN   TestValidate_UnknownCurrent
--- PASS: TestValidate_UnknownCurrent (0.00s)
=== RUN   TestValidate_CAFileNotFound
--- PASS: TestValidate_CAFileNotFound (0.00s)
PASS
ok  	github.com/greengrunt/blackwall/tools/blackwall/internal/config	1.354s
```

All 7 tests pass (1 existing + 6 new).

## Deviations

None. All requirements implemented as specified.

## Concerns

None. Implementation is straightforward and well-covered by tests. The ApplyDefaults MVP limitation for `SelectContextOnStart` (cannot distinguish explicit false from zero-value) is acknowledged in the code comment and will be handled by Task 4 (Resolve) which reads raw YAML separately.
