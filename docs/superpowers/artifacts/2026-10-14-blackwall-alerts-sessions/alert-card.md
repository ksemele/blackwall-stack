# Task 9: alert card

- Status: done
- Module: blackwall (tools/blackwall)
- Date: 2026-10-14
- Commits: 3f417d1 (feature), a6df994 (artifact), plus the fix-round commit

## What was done
Replaced the Task 8 stub `internal/views/alerts/card.go` with the detail screen opened by Enter on a feed row. Sections: title (level-styled, wrapped), local time with UTC, Rule (id, level, fired count, groups), Compliance (PCI DSS, GDPR, NIST 800-53, HIPAA, TSC; shown only when present), MITRE ATT&CK, Source (agent, location), Data (flattened `a.b.c: value`, sorted), Full log. The body scrolls with `scroll.List` (j/k, g/G, ctrl+d/u); esc closes the card (handled by view.go). Same unexported API as the stub.

## Verification
- Fix round 1: `go test ./internal/views/alerts/` covers TestCard_ComplianceIsLastSection, TestCard_StripsTerminalControls, TestSanitize; RED shown before implementation.
- `go vet ./... && go test ./...` pass; `go test -race ./internal/views/alerts/` passes.
- TDD: before the implementation, `go test ./internal/views/alerts/ -run 'Card|EnterOpens'` failed (stub rendered only the description; G did not scroll; PCI DSS not shown).
- Lab check (scratch config, tmux 140x40): `3`, then Enter on the top row (`[8] New user added to the system.`) shows all sections; `G` reaches the bottom (`6–39 of 39`); `esc` returns to the feed. Capture: `.superpowers/sdd/2026-10-14-blackwall-alerts-sessions/task-9-capture-card.txt` (git-ignored; the card as first opened, reproduced below).

Card (first open, 140x40):
```
[8] New user added to the system.
2026-10-14 03:44:47 EEST (2026-10-14 00:44:47 UTC)

Rule
────
id 5902 · level 8 · fired 2×
groups: syslog, adduser

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

Full log
────────
Oct 14 00:44:46 black-host-01 useradd[7142]: new user: name=bw-trigger-user, UID=1000, GID=1000, home=/home/bw-trigger-user, shell=/bin/sh,
from=/dev/pts/0

Compliance
──────────
1–34 of 39
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
[esc] back  [j/k] scroll  [g/G] top/bottom
```

## Deviations
- Fix round 1 (controller ruling): Compliance is the last section (spec section 5 order: header, Rule, MITRE, Source, Data, Full log, Compliance). `TestView_EnterOpensCardEscCloses` now asserts the title and rule id after Enter and the feed header `level ≥` after esc, because the Compliance section is below the fold in the 30-row test view.
- Fix round 1 (security): FullLog and Data keys/values pass through `sanitize` before wrapping. Tabs become a space; C0 controls, DEL and C1 controls are dropped. Without it, log text could carry ANSI escapes into the terminal.
- The level-styled title is wrapped as plain text and styled per line. Wrapping the styled string would split ANSI escape sequences for descriptions longer than the width.

## Open questions
- None.
