# Task 4: Config Resolve (CLI/env/defaults precedence)

## Status
✅ COMPLETE

## Date
2026-05-17

## Commits
- `066aa8a` feat(blackwall): config Resolve (CLI/env precedence)

## What was done

### Files created
1. **`tools/blackwall/internal/config/resolve.go`**
   - Implemented `Resolve()` function enforcing precedence: CLI > env > config file > defaults
   - Implemented `ReadEnv()` to parse BW_CONTEXT, BW_REFRESH, BW_SELECT_CONTEXT_ON_START
   - Defined `Overrides` and `EnvValues` types using pointers to distinguish explicit user values from zero defaults
   - Helper `boolPtr()` utility function

2. **`tools/blackwall/internal/config/resolve_test.go`**
   - 5 new test cases covering all precedence paths:
     - `TestResolve_CLIBeatsConfig` — CLI flag overrides file value
     - `TestResolve_EnvBeatsConfig` — env vars override file values for both context and refresh interval
     - `TestResolve_CLIBeatsEnv` — CLI flag takes precedence over env vars
     - `TestResolve_ExplicitContextDisablesPicker` — explicit --context disables SelectContextOnStart
     - `TestResolve_NoSelectContextFlag` — --no-select-context explicitly disables picker

### Key design decisions
- **Pointer-based override detection**: `Overrides.Refresh` and `SelectContextOnStart` use `*time.Duration` and `*bool` to distinguish "not set" (nil) from explicit zero values
- **Context + picker interaction**: Setting context explicitly (CLI or env) automatically disables picker to avoid UI confusion
- **Validation**: Returns error if `Current` is empty or references a non-existent context after resolve
- **Error handling**: Clear error messages for invalid state

### Test coverage
- All 12 tests pass (7 existing from tasks 2-3 + 5 new)
- Full precedence coverage verified
- Side effects (picker disable) validated

## Verification

```
$ go test ./internal/config/... -v
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
=== RUN   TestResolve_CLIBeatsConfig
--- PASS: TestResolve_CLIBeatsConfig (0.00s)
=== RUN   TestResolve_EnvBeatsConfig
--- PASS: TestResolve_EnvBeatsConfig (0.00s)
=== RUN   TestResolve_CLIBeatsEnv
--- PASS: TestResolve_CLIBeatsEnv (0.00s)
=== RUN   TestResolve_ExplicitContextDisablesPicker
--- PASS: TestResolve_ExplicitContextDisablesPicker (0.00s)
=== RUN   TestResolve_NoSelectContextFlag
--- PASS: TestResolve_NoSelectContextFlag (0.00s)
PASS
ok  	github.com/greengrunt/blackwall/tools/blackwall/internal/config	0.801s
```

## Deviations

The task plan included test code with 3 bugs that were pre-corrected before implementation:

1. **Bug #1 in `TestResolve_EnvBeatsConfig`**: Line had `t.Setenv("BW_REFRESH", "10s")` but test didn't consume from env. Plan's code was missing the `t.Setenv()` calls entirely.

2. **Bug #2 in `TestResolve_ExplicitContextDisablesPicker`**: Plan code attempted to set `ov.SelectContextOnStart` directly in test, but the logic is inside `Resolve()` itself. Test was corrected to verify the output, not the intermediate state.

3. **Bug #3 in `TestResolve_NoSelectContextFlag`**: Plan code didn't define the `no := false` variable, and the assertion was wrong. Corrected to test the Overrides correctly.

These corrections allowed tests to pass immediately after implementation without requiring fixes to resolve.go.

## Concerns

None. Implementation clean and minimal. All tests pass without modification to resolve.go. Ready for Task 5 (template).
