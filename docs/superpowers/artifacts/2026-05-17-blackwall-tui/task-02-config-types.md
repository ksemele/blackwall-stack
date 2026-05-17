# Task 2: Config Types + YAML Parsing

## Status
**COMPLETED**

## Date
2026-05-17

## Commits
- `d17b0e7` feat(blackwall): config types + YAML load

## What was done

Implemented the configuration type system and YAML unmarshalling for the blackwall TUI:

1. **Created config type definitions** (`tools/blackwall/internal/config/config.go`):
   - `Config`: top-level config struct with current context, contexts map, UI settings, logging settings
   - `Context`: per-context struct with description and optional Wazuh subsystem
   - `WazuhSubsystem`: Wazuh server connection (host, port, user, password, TLS)
   - `TLS`: TLS settings (insecure_skip_verify, ca_file)
   - `UI`: UI settings (refresh_interval, theme, vim_keys)
   - `Logging`: logging settings (level, file)

2. **Implemented config loading** (`tools/blackwall/internal/config/load.go`):
   - `Load(path string)`: reads YAML file from disk, unmarshals into Config
   - `writeFile(path, content string)`: test helper for writing temp config files
   - Error wrapping for file read and YAML parse failures

3. **Added YAML dependency**: `gopkg.in/yaml.v3 v3.0.1`

4. **Created comprehensive test** (`tools/blackwall/internal/config/load_test.go`):
   - `TestLoad_HappyPath`: validates YAML unmarshalling with multi-context config
   - Tests current context selection, UI settings, Wazuh subsystem config, TLS settings
   - Uses temp file for isolated test data

## Verification

Test output (all passing):
```
=== RUN   TestLoad_HappyPath
--- PASS: TestLoad_HappyPath (0.00s)
PASS
ok  	github.com/greengrunt/blackwall/tools/blackwall/internal/config	1.209s
```

Test coverage:
- YAML parsing of top-level fields (current, select_context_on_start)
- Contexts map unpacking with nested Wazuh subsystem
- Integer field marshalling (port)
- Boolean field marshalling (vim_keys, insecure_skip_verify)
- Duration field parsing (3s -> 3*time.Second)
- String array and nested struct handling

## Deviations
None. Followed the plan exactly as specified.

## Concerns
None. The implementation is straightforward YAML unmarshalling with well-defined types. Validation and defaults (Task 3) will handle any runtime concerns.
