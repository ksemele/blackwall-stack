# Task 9: alert card

- Status: done
- Module: blackwall (tools/blackwall)
- Date: 2026-10-14
- Commits: 3f417d1 (feature), plus this artifact commit

## What was done
Replaced the Task 8 stub `internal/views/alerts/card.go` with the detail screen opened by Enter on a feed row. Sections: title (level-styled, wrapped), local time with UTC, Rule (id, level, fired count, groups), Compliance (PCI DSS, GDPR, NIST 800-53, HIPAA, TSC; shown only when present), MITRE ATT&CK, Source (agent, location), Data (flattened `a.b.c: value`, sorted), Full log. The body scrolls with `scroll.List` (j/k, g/G, ctrl+d/u); esc closes the card (handled by view.go). Same unexported API as the stub.

## Verification
- `go vet ./... && go test ./...` pass; `go test -race ./internal/views/alerts/` passes.
- TDD: before the implementation, `go test ./internal/views/alerts/ -run 'Card|EnterOpens'` failed (stub rendered only the description; G did not scroll; PCI DSS not shown).
- Lab check (scratch config, tmux 140x40): `3`, then Enter on the top row (`[8] New user added to the system.`) shows all sections; `G` reaches the bottom (`6–39 of 39`); `esc` returns to the feed. Capture: `.superpowers/sdd/2026-10-14-blackwall-alerts-sessions/task-9-capture-card.txt` (git-ignored; the card as first opened, reproduced below).

Card (first open):
```
[8] New user added to the system.
2026-10-14 03:44:47 EEST (2026-10-14 00:44:47 UTC)

Rule
────
id 5902 · level 8 · fired 2×
groups: syslog, adduser

Compliance
──────────
PCI DSS: 10.2.7, 10.2.5, 8.1.2
GDPR: IV_35.7.d, IV_32.2
NIST 800-53: AU.14, AC.7, AC.2, IA.4
HIPAA: 164.312.b, 164.312.a.2.I, 164.312.a.2.II
TSC: CC6.8, CC7.2, CC7.3

MITRE ATT&CK
────────────
tactic:    Persistence
technique: Create Account
id:        T1136

Source
──────
agent:    black-host-01 (002)
location: /var/log/auth.log

Data
────
dstuser: bw-trigger-user
gid: 1000
home: /home/bw-trigger-user
shell: /bin/sh,
uid: 1000
1–34 of 39
[esc] back  [j/k] scroll  [g/G] top/bottom
```

## Deviations
- Compliance section moved to directly after Rule (brief order: Rule, MITRE, Source, Data, Full log, Compliance). Reason: with the 120x30 test view the Compliance section fell outside the visible 29 rows, failing the brief's `TestView_EnterOpensCardEscCloses` (`PCI DSS` must be on screen). Full log stays last.
- The level-styled title is wrapped as plain text and styled per line. Wrapping the styled string would split ANSI escape sequences for descriptions longer than the width.

## Open questions
- None.
