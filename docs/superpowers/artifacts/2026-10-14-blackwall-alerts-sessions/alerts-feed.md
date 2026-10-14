# Task 8: :alerts view (feed)

- Status: done
- Module: blackwall (tools/blackwall)
- Date: 2026-10-14
- Commits: 9991473

## What was done
New `internal/views/alerts` (view, feed merge/dedupe, render, stub card for Task 9), `styles.LevelCritical`, and main.go wiring (indexer client, view registration, context switch, status line indexer address).

## Verification
`go vet ./... && go test ./...` pass (also -race for the view). TDD: tests failed with undefined symbols before implementation. Lab check used a scratch config (not ~/.config/blackwall/config.yaml; no owner action needed).

Initial load (tab 3):
```
[1] :contexts  │  [2] :agents  │   [3] :alerts                                                                                        v0.0.1
context:  lab  · wazuh: 127.0.0.1:55000 · indexer: 127.0.0.1:9200 · ↻ live (3s)
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
level ≥ 5 · hiding sca, blackwall_sessions · agent: all · last 24h                                                                  19 shown
TIME                       LVL  AGENT             RULE     DESCRIPTION
2026-10-14 03:38 (00:38Z)    8  black-master-00   5904     Information from the user was changed.
2026-10-14 03:38 (00:38Z)    8  black-master-00   5902     New user added to the system.
2026-10-14 03:38 (00:38Z)    8  black-master-00   5901     New group added to the system.
2026-10-14 03:38 (00:38Z)    7  black-master-00   550      Integrity checksum changed.
```

After `tests/triggers/bw-trigger user` (new rows on top, count 19 to 23):
```
[1] :contexts  │  [2] :agents  │   [3] :alerts                                                                                        v0.0.1
context:  lab  · wazuh: 127.0.0.1:55000 · indexer: 127.0.0.1:9200 · ↻ live (3s)
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
level ≥ 5 · hiding sca, blackwall_sessions · agent: all · last 24h                                                                  23 shown
TIME                       LVL  AGENT             RULE     DESCRIPTION
2026-10-14 03:44 (00:44Z)    8  black-host-01     5902     New user added to the system.
2026-10-14 03:44 (00:44Z)    8  black-host-01     5901     New group added to the system.
2026-10-14 03:44 (00:44Z)    7  black-host-01     550      Integrity checksum changed.
2026-10-14 03:44 (00:44Z)    7  black-host-01     550      Integrity checksum changed.
```

After pressing `s` (groups no longer hidden, 195 shown):
```
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
level ≥ 5 · all groups · agent: all · last 24h                                                                                     195 shown
TIME                       LVL  AGENT             RULE     DESCRIPTION
2026-10-14 03:44 (00:44Z)    8  black-host-01     5902     New user added to the system.
```

## Deviations
None. Brief code used verbatim; gofmt applied to the card stub.

## Open questions
None.
