# Indexer client: alert search over the Wazuh indexer

**Status:** Done
**Module:** tools/blackwall (TUI)
**Date:** 2026-10-14
**Commits:** e91cef0 feat(blackwall): wazuh indexer client for alert search; plus the completion artifact commit that adds this file

## What was done

- `internal/httpclient/httpclient.go` added. `httpclient.New(config.TLS)` builds the
  TLS-aware `*http.Client` (10 s timeout, `insecure_skip_verify`, `ca_file`). The body
  is moved unchanged from `wazuh.buildHTTPClient`.
- `internal/wazuh/tls.go` now delegates to `httpclient.New`. `buildHTTPClient` stays as a
  thin wrapper, so the wazuh tests are unchanged and green.
- `internal/indexer` package added:
  - `types.go`: `AlertQuery`, `Alert`, `Rule`, `MITRE`, `AgentRef` as specified. `ParseTime`
    accepts Wazuh's `+0000` offset and RFC3339. `Alert.UnmarshalJSON` fills `Time` from
    `timestamp`.
  - `query.go`: `BuildSearch(AlertQuery)` returns the OpenSearch body. It has a level filter
    (`gte`), a timestamp filter (`now-24h` by default, or `gte` the `Since` value), optional
    `agent.id` and `rule.id` terms, a `must_not` terms clause for excluded groups, size
    defaulting to 200, and sort by timestamp descending.
  - `client.go`: `New(config.IndexerSubsystem)`, `Addr()` returning `host:port`, and
    `SearchAlerts`. It POSTs to `/wazuh-alerts-*/_search` with basic auth and
    `Content-Type: application/json`. A non-200 response returns an error that includes the
    status code and up to 200 bytes of body. A pattern that matches no index returns an empty
    slice with no error.

## Verification

- RED: `go test ./internal/indexer/` failed to build with `undefined: Client`,
  `undefined: New`, `undefined: AlertQuery`, `undefined: BuildSearch`. The package had no
  non-test code yet.
- GREEN:
  - `go vet ./...` clean.
  - `go test ./internal/indexer/ ./internal/wazuh/` pass.
  - `go test -race ./...` passes for every package.

## Deviations

- `internal/httpclient` has no test file. The brief specifies none, and its behaviour is
  covered through the wazuh TLS tests.
- `AlertQuery` lives in `types.go`, next to the other public types. The brief did not say
  where it goes.
- `gofmt -w` was applied only to the new `internal/indexer` and `internal/httpclient`
  files. Several pre-existing files are not gofmt-clean. They were left alone.
- The commit has no Co-Authored-By trailer, following the plan's constraints file.

## Open questions

- `SearchAlerts` does not de-duplicate. Callers that poll with `Since` use `Alert.ID` to
  avoid duplicates, as the `gte` comment in `query.go` describes. Task 8 or 10 should
  confirm this is the behaviour they want.
