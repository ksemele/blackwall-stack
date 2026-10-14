# Indexer section in context config

**Status:** Done
**Module:** tools/blackwall (TUI)
**Date:** 2026-10-14
**Commits:** bc02bd7 feat(blackwall): indexer section in context config

## What was done

- `Context` gained an optional `Indexer *IndexerSubsystem` (`indexer:` in YAML).
- New types `IndexerSubsystem` (server, port, user, password, tls, alerts) and
  `AlertDefaults` (`MinLevel *int`, `ExcludeGroups []string`).
- `DefaultMinLevel = 5` and `DefaultExcludeGroups()` (returns a fresh
  `[]string{"sca", "blackwall_sessions"}`) added to `config.go`.
- `ApplyDefaults` fills `Alerts.MinLevel` and `Alerts.ExcludeGroups` for every
  non-nil indexer. Explicit `0` and explicit `[]` are kept, because only nil is
  treated as absent.
- `ExpandEnv` expands the indexer's server, user, password and tls.ca_file through
  the same closure as the Wazuh block. Error paths are `contexts.<name>.wazuh.<field>`
  and `contexts.<name>.indexer.<field>`.
- `Validate` now checks the Wazuh and indexer blocks independently, so the indexer
  check runs even when a context has no Wazuh block. The indexer check covers
  required fields, `ca_file` existence and `min_level` in 0–15.
- Lab context in `template.yaml` carries the indexer block on 127.0.0.1:9200
  (admin / lab-only password, insecure_skip_verify for the self-signed cert).
- `tools/blackwall/README.md` documents the `indexer:` section and its defaults.

## Verification

- RED: `go test ./internal/config/` failed to compile with
  `Context has no field or method Indexer` and `undefined: DefaultMinLevel`.
- GREEN: `go test ./internal/config/` passes. The new tests are
  `TestIndexer_ParsedDefaultedExpanded`, `TestIndexer_ExplicitZeroAndEmptyKept`,
  `TestIndexer_ValidateRequiresFields`, `TestIndexer_ValidateLevelRange` and
  `TestTemplate_HasLabIndexer`.
- `go vet ./...` exits 0.
- `go test -race -count=1 ./...` passes for every package.

## Deviations

- The brief's `writeTemp` helper was not added. The new tests call the existing
  `writeTempFile(t, "config.yaml", body)` in `load_test.go`, per controller ruling.
- The template test uses `string(defaultTemplate)`, since `templateYAML` does not exist.
- `load_test.go` gained only the `strings` import. `os` and `path/filepath` are not needed.
- The README was updated in the same commit as the code. The brief's `git add`
  listed only `internal/config`, but the docs-sync rule requires the README change.
- `ExpandEnv` field labels for Wazuh are now `wazuh.server` and similar, passed to
  the shared closure. The emitted messages are unchanged.
- `ExpandEnv` records only the first empty-expansion error. Previously a later field
  could overwrite it.
- The commit has no Co-Authored-By trailer, following the global constraints.
- Pre-existing, not touched: `gofmt -l` flags `internal/config/load.go` for a comment
  indent in `ApplyDefaults`, and flags `internal/config/template.go`. Both predate this task.

## Open questions

- None.
