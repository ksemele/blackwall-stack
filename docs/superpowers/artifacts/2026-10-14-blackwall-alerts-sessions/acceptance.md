# Task 11: documentation and end-to-end acceptance

- Status: DONE
- Module: blackwall (tools/blackwall), blackwall-eye
- Date: 2026-10-14
- Commits: see `git log` (docs + this artifact)

## What was done

- tools/blackwall/README.md: indexer in the Configure example and lab context, `l`/`e` keys,
  `:alerts` palette entry, new Alerts section, Sessions row, PTY caveat.
- tools/blackwall/docs/ARCHITECTURE.md: new layers and a "Keys and ticks" section.
- blackwall-eye/README.md: PTY caveat in the sessions section.
- blackwall-eye/STATUS.md: Done entry for the TUI consumers, `next_task` restored.
- tests/README.md already documented the lab indexer; no change.

## Verification (fresh lab: `tests/bin/bw-lab reset`, then `tests/bin/bw-check all`)

`bw-check all`: 30 passed, 0 failed, 1 skipped (voice not deployed).
TUI: `tools/blackwall/bin/blackwall --config .superpowers/sdd/2026-10-14-blackwall-alerts-sessions/lab-config.yaml`, tmux 150x40.

### 1. Alerts feed (key `3`): no CIS rows

```
[1] :contexts  │  [2] :agents  │   [3] :alerts                                                                                                  v0.0.1
context:  lab  · wazuh: 127.0.0.1:55000 · indexer: 127.0.0.1:9200 · ↻ live (3s)
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
level ≥ 5 · hiding sca, blackwall_sessions · agent: all · last 24h                                                                            29 shown
TIME                       LVL  AGENT             RULE     DESCRIPTION
2026-10-14 04:11 (01:11Z)    8  black-dead-98     5904     Information from the user was changed.
2026-10-14 04:11 (01:11Z)    8  black-dead-98     5902     New user added to the system.
2026-10-14 04:11 (01:11Z)    8  black-dead-98     5901     New group added to the system.
2026-10-14 04:11 (01:11Z)    8  black-host-01     5904     Information from the user was changed.
2026-10-14 04:11 (01:11Z)    8  black-host-01     5902     New user added to the system.
2026-10-14 04:11 (01:11Z)    8  black-host-01     5901     New group added to the system.
2026-10-14 04:11 (01:11Z)    8  black-master-00   5902     New user added to the system.
2026-10-14 04:11 (01:11Z)    8  black-master-00   5904     Information from the user was changed.
2026-10-14 04:11 (01:11Z)    8  black-master-00   5901     New group added to the system.
```

`tests/triggers/bw-trigger user`, 14 s later: new rows on top (31 shown instead of 29)

```
level ≥ 5 · hiding sca, blackwall_sessions · agent: all · last 24h                                                                            31 shown
TIME                       LVL  AGENT             RULE     DESCRIPTION
2026-10-14 04:12 (01:12Z)    8  black-host-01     5902     New user added to the system.
2026-10-14 04:12 (01:12Z)    8  black-host-01     5901     New group added to the system.
2026-10-14 04:11 (01:11Z)    8  black-dead-98     5904     Information from the user was changed.
2026-10-14 04:11 (01:11Z)    8  black-dead-98     5902     New user added to the system.
```

Enter on the top row: alert card (esc returns)

```
[1] :contexts  │  [2] :agents  │   [3] :alerts                                                                                                  v0.0.1
context:  lab  · wazuh: 127.0.0.1:55000 · indexer: 127.0.0.1:9200 · ↻ live (3s)
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
[8] New user added to the system.
2026-10-14 04:12:48 EEST (2026-10-14 01:12:48 UTC)

Rule
────
id 5902 · level 8 · fired 4×
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
Oct 14 01:12:46 black-host-01 useradd[6873]: new user: name=bw-trigger-user, UID=1000, GID=1000, home=/home/bw-trigger-user, shell=/bin/sh, from=/dev/
pts/0

Compliance
──────────
1–34 of 39
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
[esc] back  [j/k] scroll  [g/G] top/bottom
```

### 2. Sessions, active then history

Interactive PTY session: `sshpass -p blackwall ssh -tt -F tests/docker/ssh_config black-host-01 'sleep 120' &`
(plain `bw-lab ssh host cmd` has no PTY and never reaches wtmp). Agent card of black-host-01, key `e`, ~70 s after login:

```
[1] :contexts  │   [2] :agents   │  [3] :alerts                                                                                                 v0.0.1
context:  lab  · wazuh: 127.0.0.1:55000 · indexer: 127.0.0.1:9200 · ↻ live (3s)
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
Agent 002 — black-host-01  active   DOCKER
[O] Overview  │  [H] Hardware  │  [T] Top  │  [K] Packages  │  [N] Network  │  [S] Security  │   [E] Sessions
Sessions — snapshot 2026-10-14 04:13 (01:13Z)

USER        TTY      FROM              LOGIN                      LOGOUT                     DURATION
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
root        pts/0    172.28.0.1        2026-10-14 04:13 (01:13Z)  still logged in            1m ●
root        pts/0    172.28.0.1        2026-10-14 04:12 (01:12Z)  2026-10-14 04:12 (01:12Z)  4s
root        pts/0    172.28.0.1        2026-10-14 04:12 (01:12Z)  2026-10-14 04:12 (01:12Z)  —
root        pts/0    172.28.0.1        2026-10-14 04:12 (01:12Z)  2026-10-14 04:12 (01:12Z)  —
root        pts/0    172.28.0.1        2026-10-14 04:12 (01:12Z)  2026-10-14 04:12 (01:12Z)  —
```

After the ssh exited and ~70 s passed (esc, Enter, e): moved to history with a duration.

```
[1] :contexts  │   [2] :agents   │  [3] :alerts                                                                                                 v0.0.1
context:  lab  · wazuh: 127.0.0.1:55000 · indexer: 127.0.0.1:9200 · ↻ live (3s)
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
Agent 002 — black-host-01  active   DOCKER
[O] Overview  │  [H] Hardware  │  [T] Top  │  [K] Packages  │  [N] Network  │  [S] Security  │   [E] Sessions
Sessions — snapshot 2026-10-14 04:15 (01:15Z)

USER        TTY      FROM              LOGIN                      LOGOUT                     DURATION
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
root        pts/0    172.28.0.1        2026-10-14 04:13 (01:13Z)  2026-10-14 04:15 (01:15Z)  2m
root        pts/0    172.28.0.1        2026-10-14 04:12 (01:12Z)  2026-10-14 04:12 (01:12Z)  4s
root        pts/0    172.28.0.1        2026-10-14 04:12 (01:12Z)  2026-10-14 04:12 (01:12Z)  —
root        pts/0    172.28.0.1        2026-10-14 04:12 (01:12Z)  2026-10-14 04:12 (01:12Z)  —
root        pts/0    172.28.0.1        2026-10-14 04:12 (01:12Z)  2026-10-14 04:12 (01:12Z)  —
```

### 3. Typing q, 1, p into the `/` prompt in `:alerts`

The prompt shows `/q1p`; the TUI neither quit nor switched tabs. "no alerts yet" is the
filter `q1p` matching no rows.

```
[1] :contexts  │  [2] :agents  │   [3] :alerts                                                                                                  v0.0.1
context:  lab  · wazuh: 127.0.0.1:55000 · indexer: 127.0.0.1:9200 · ↻ live (3s)
──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
level ≥ 5 · hiding sca, blackwall_sessions · agent: all · last 24h                                                                             0 shown
/q1p
TIME                       LVL  AGENT             RULE     DESCRIPTION
no alerts yet

```

## Deviations

None. The first `e` press hit agent 000 (the cursor keys were sent too fast); redone
stepwise for black-host-01.

## Open questions

None.
