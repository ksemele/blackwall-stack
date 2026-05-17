# Task 5: First-Run Config Template

## Status
✅ Complete

## Date
2026-05-17

## Commits
- `be167d6` — feat(blackwall): embedded first-run config template

## What was done

Created embedded first-run configuration template for the blackwall TUI, enabling automatic creation of a default config file on first run.

**Files created:**

1. **`tools/blackwall/internal/config/template.yaml`** (29 lines)
   - Embedded YAML default configuration with example Wazuh context
   - Includes UI settings (refresh interval, theme, vim keys)
   - Includes logging configuration
   - Uses environment variable placeholder for password security

2. **`tools/blackwall/internal/config/template.go`** (29 lines)
   - `EnsureExists(path string) (bool, error)` function
   - Uses Go's `go:embed` directive to embed template.yaml at compile time
   - Returns `(true, nil)` when template was written
   - Returns `(false, nil)` when file already existed
   - Creates parent directories with mode 0o700 if needed
   - Writes config file with restrictive mode 0o600

3. **`tools/blackwall/internal/config/template_test.go`** (34 lines)
   - `TestEnsureExists_CreatesFileWhenAbsent` test
   - Verifies template is created on first call
   - Verifies template contains expected content (example-master section)
   - Verifies second call returns `created=false` (idempotent)

## Verification

All 13 tests in `./internal/config/...` PASS:

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
=== RUN   TestEnsureExists_CreatesFileWhenAbsent
--- PASS: TestEnsureExists_CreatesFileWhenAbsent (0.00s)
PASS
ok  	github.com/greengrunt/blackwall/tools/blackwall/internal/config	0.791s
```

## Deviations
None. Implementation matches specification exactly.

## Concerns
None. The `go:embed` directive embeds the YAML at compile time, ensuring the template is always available and consistent. The idempotent design allows safe repeated calls.

---

Artifact created: `/Users/akrugliak/coding/blackwall/docs/superpowers/artifacts/2026-05-17-blackwall-tui/task-05-template.md`
