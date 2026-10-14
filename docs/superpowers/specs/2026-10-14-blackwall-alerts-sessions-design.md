# `blackwall` TUI — alerts feed and host sessions — design

- **Date:** 2026-10-14
- **Status:** approved in chat, pending written-spec review
- **Modules:** `tools/blackwall`, `blackwall-eye` (agent + manager roles), `tests/`

## 1. Problem

The TUI shows agent state and inventory, but not what actually happened on the
hosts. Two questions cannot be answered from it today:

1. **What fired?** — a live feed of Wazuh alerts, filtered so that signal is not
   buried under routine events and CIS audit results.
2. **Who is on this host?** — who is logged in right now, from where, since when,
   and the recent login history with durations.

## 2. Decisions (made by the owner on 2026-10-14)

| Question | Decision |
|---|---|
| Order | Indexer client + `:alerts` first, then the Sessions tab |
| Alerts use case | Live feed + alert card (k9s-style), not investigation or a summary dashboard |
| Default noise filter | `rule.level >= 5`, SCA results hidden; both toggleable, defaults in config |
| Alert data source | Wazuh indexer (OpenSearch) — the manager REST API has no alerts endpoint in 4.x |
| Feed refresh | Incremental: query only alerts newer than the newest one shown |
| Sessions data source | `last` executed by the agent (`full_command`), shipped as an alert on change |

## 3. Background: where the data lives

```
agents ──1514──▶ wazuh-manager ──filebeat──▶ wazuh-indexer ◀── wazuh-dashboard
                 (rules; REST API :55000)     (OpenSearch :9200)   (web UI :5601)
```

The TUI already talks to the manager API. Alerts are only stored in the indexer
(`wazuh-alerts-4.x-YYYY.MM.DD`), which is what the Wazuh dashboard reads too.
Both services run on `black-master-00` in the lab; they use different ports and
credentials (`wazuh` for the API, `admin` for the indexer).

Facts verified on the lab (2026-10-14):

- ~450 alerts/day on three hosts: level 3 ≈ 56 %, level 7 ≈ 40 % (almost all
  CIS benchmark failures from SCA), levels 8–9 rare (new user, FIM changes,
  logins out of hours).
- An alert document nests its fields: `rule.{id,level,description,groups,mitre,firedtimes}`,
  `agent.{id,name}`, `data.*`, `full_log`, `location`, `timestamp`.
- Successful SSH logins match different rules depending on the time of day
  (17101 at night, 5715/5501 by day), and login/logout pairs are linked only by
  the sshd PID inside `full_log` — too fragile to build sessions from.
- `syscollector/{agent}/users` exists in 4.14 but only gives a "logged in at
  scan time" flag every 5 minutes, without history or durations.
- `last --time-format iso -w` on Ubuntu 22.04 prints user, tty, source, ISO
  login/logout times and duration, and `still logged in` for open sessions.
- Wazuh ships rule 533, which watches `netstat` output with `<check_diff />` —
  the same mechanism fits `last`.

## 4. Components

### 4.1 `internal/indexer` — Wazuh indexer client

- Basic auth; TLS via the same helper as `internal/wazuh` (`insecure_skip_verify`, `ca_file`).
- One public operation:

  ```go
  func (c *Client) SearchAlerts(ctx context.Context, q AlertQuery) ([]Alert, error)

  type AlertQuery struct {
      MinLevel      int
      ExcludeGroups []string   // rule.groups to drop (e.g. sca, blackwall_sessions)
      AgentID       string     // "" = all agents
      RuleID        string     // "" = any rule (Sessions uses 100100)
      Since         time.Time  // timestamp > Since; zero = now-24h
      Size          int        // max hits
  }
  ```

- Builds an OpenSearch `_search` against `wazuh-alerts-*`: a `bool` filter with
  `range rule.level gte`, `range timestamp gt`, `term agent.id`, `term rule.id`,
  `must_not terms rule.groups`; sorted by `timestamp desc`.
- `Alert` mirrors the real nested document (rule, agent, manager, data as
  `map[string]any`, `full_log`, `location`, `timestamp`, `id`).
- A missing index (`index_not_found_exception`) returns an empty result, not an error.

### 4.2 Configuration

New optional per-context section, same shape as `wazuh:` plus alert defaults:

```yaml
contexts:
  lab:
    wazuh:   { ... }
    indexer:
      server:   127.0.0.1
      port:     9200
      user:     admin
      password: ${BW_LAB_INDEXER_PASSWORD}
      tls:
        insecure_skip_verify: true
      alerts:
        min_level:      5                          # default 5
        exclude_groups: [sca, blackwall_sessions]  # default
```

- Absent `indexer:` → `:alerts` and the Sessions tab show
  "indexer not configured for context <name> — see README"; everything else works.
- `config.Validate` checks `server`/`port` when the section is present.
- Context switching (`:ctx`) rebuilds the indexer client alongside the Wazuh client.
- The status line shows `indexer: host:port` when configured.
- The generated config template and the `lab` context gain the section.

### 4.3 Lab changes (`tests/`)

- `docker-compose.yml` publishes the indexer on loopback only: `127.0.0.1:9200:9200`
  (nftables on the manager already allows 9200).
- `bw-check ports` probes 9200; `bw-check wazuh` asserts the indexer answers and
  rule 100100 is loaded on the manager.
- `.envrc.example` gains `BW_LAB_INDEXER_PASSWORD` (lab value is public:
  `Blackwall1!`, see `tests/README.md`).

### 4.4 Session snapshots (`blackwall-eye`)

- **Agent role** — a command localfile in `ossec.conf.j2`:

  ```xml
  <localfile>
    <log_format>full_command</log_format>
    <command>last --time-format iso -w -n 50</command>
    <alias>blackwall-sessions</alias>
    <frequency>{{ wazuh_sessions_frequency }}</frequency>   <!-- default 60 -->
  </localfile>
  ```

  Toggle: `wazuh_sessions_enabled` (default `true`).

- **Manager role** — new file `/var/ossec/etc/rules/blackwall_rules.xml`
  (template in the role, notifies a manager restart):

  ```xml
  <group name="blackwall,">
    <rule id="100100" level="3">
      <if_sid>530</if_sid>
      <match>ossec: output: 'blackwall-sessions'</match>
      <check_diff />
      <description>Blackwall: login sessions changed (last).</description>
      <group>blackwall_sessions,</group>
    </rule>
  </group>
  ```

  `check_diff` stores an alert only when the output changes (a login or a
  logout), so the indexer gets one snapshot per event instead of one per minute.
  Level 3 is the minimum stored as an alert.

## 5. The `:alerts` view

New package `internal/views/alerts`, registered after `:agents` (tab `[3]`,
palette `:alerts`).

```
level ≥ 5 · SCA hidden · agent: all · last 24h                      142 shown  +3 new
TIME                       LVL  AGENT            RULE    DESCRIPTION
2026-10-14 02:49 (23:49Z)    9  black-host-01    17101   Successful login during non-business hours.
2026-10-14 02:12 (23:12Z)    8  black-master-00  5902    New user added to the system.
```

**Feed behaviour**

- First load: alerts from the last 24 hours, newest 200.
- Every `ui.refresh_interval`: query `Since = newest shown timestamp`, prepend
  the result, keep at most 500 rows in memory (drop the oldest).
- If the cursor is not on the first row it stays on the same alert when rows
  are prepended. The filter line shows `+N new` since the user last sat on the top row.
- Times: local with UTC in parentheses (`wazuh.FormatLocalUTC`).
- Row colour by level: ≥ 12 bold red, 7–11 red, 5–6 yellow, < 5 default.

**Keys**

| Key | Action | Re-queries |
|---|---|---|
| `↑↓` / `j k` | move | no |
| `↵` | open alert card | no |
| `+` / `-` | min level ±1 (clamped 0–15) | yes |
| `s` | toggle `exclude_groups` on/off | yes |
| `a` | cycle agent filter over agents present in the feed, then "all" | yes |
| `/` | substring filter over loaded rows (description, agent, rule id, full_log) | no |
| `p` | pause auto-refresh | — |
| `ctrl+r` | refresh now | yes |

**Alert card** (`↵`, `esc` back; scrollable with the existing `scrollist`):
header (level + description) · Rule (id, level, groups, firedtimes) ·
MITRE (tactic, technique, id) · Agent · Location · Time (local + UTC) ·
Data (decoded fields, sorted) · Full log (wrapped) · Compliance (one line each,
last).

**Jump from an agent:** `l` in the agent card opens `:alerts` filtered to that agent.

## 6. The Sessions tab

New tab `[E] Sessions` in the agent card (`E` for entries; `S` is Security).

- Fetches the newest alert with `rule.id = 100100` for the agent
  (`SearchAlerts{RuleID: "100100", AgentID: id, Size: 1}`, no level/group filter)
  and parses the `last` output from `full_log`.
- Parser (`internal/sessions`): one record per line —
  `user, tty, from, login, logout | still logged in | down | crash, duration`;
  `reboot` pseudo-users and the `wtmp begins` footer are recognised; any
  unrecognised line is kept verbatim as a raw row.

```
Sessions — snapshot 2026-10-14 02:49 (23:49Z)
USER   TTY     FROM          LOGIN                      LOGOUT                     DURATION
root   pts/1   10.0.0.5      2026-10-14 02:40 (23:40Z)  still logged in            9m ●
root   pts/0   172.28.0.1    2026-10-14 02:49 (23:49Z)  2026-10-14 02:49 (23:49Z)  0s
```

- Open sessions first, highlighted, duration counted up to now; then history,
  newest first, scrollable.
- A login gets `⚠` when an alert of level ≥ 7 for that agent and user lies
  within ±1 minute of its login time (one extra `SearchAlerts` query over the
  snapshot's time span).

## 7. Error handling

| Situation | Shown |
|---|---|
| No `indexer:` in the context | "indexer not configured for context X — see README" in `:alerts` and Sessions |
| Indexer unreachable / TLS error / 401 | `⚠ indexer: <error>` above the table, last data kept, retried next tick |
| No `wazuh-alerts-*` index yet | empty feed with "no alerts yet" |
| No session snapshot for the agent | "no session snapshot yet — appears after the next login/logout" |
| Unparseable `last` line | rendered as a raw row; never dropped, never fatal |

## 8. Testing

TDD throughout, with **fixtures captured from the live lab** in `testdata/`
(not structs marshalled by the test — that is how the syscollector schema bug
of 2026-10-14 went unnoticed).

- `internal/indexer`: real `_search` responses (hits / empty / index missing);
  query-builder tests for level, excluded groups, agent, rule, since, size;
  401 and transport errors.
- `internal/sessions`: real `last` output incl. `still logged in`, `down`,
  `reboot`, `wtmp begins`, garbage lines.
- `internal/views/alerts`: incremental prepend keeps the cursor, 500-row cap,
  `+/-`, `s`, `a` re-query, `/` does not, `+N new` counter, card rendering.
- Sessions tab: open sessions first, `⚠` marking, "no snapshot" state.
- `internal/config`: `indexer:` parsing, defaults, validation, env expansion.

## 9. Acceptance

1. `tests/bin/bw-lab reset` brings everything up, including the indexer on
   `127.0.0.1:9200`, rule 100100 and the `last` command on agents;
   `tests/bin/bw-check all` passes.
2. `blackwall --context lab` → `[3] :alerts` shows level ≥ 5 alerts without SCA.
   After `tests/triggers/bw-trigger user`, the new alert appears on top within
   one or two refresh intervals.
3. `tests/bin/bw-lab ssh black-host-01` → agent card → `E`: the session is
   listed as `still logged in`; after exit it moves to history with a duration.

Verified the same way as the 2026-10-14 fixes: the TUI driven in tmux and the
screen captured.

## 10. Documentation (same commits as the behaviour)

- `tools/blackwall/README.md` — `:alerts`, Sessions tab, keys, `indexer:` section.
- `tools/blackwall/docs/ARCHITECTURE.md` — `internal/indexer`, `internal/sessions`.
- `tests/README.md` — port 9200, new `bw-check` assertions, access table.
- `blackwall-eye/README.md` — rule 100100, the sessions command, variables.
- `blackwall-eye/STATUS.md` — done entries.

## 11. Out of scope

- Alert investigation features: arbitrary time ranges, aggregations, paging beyond 500.
- Acting on alerts (acknowledge, mute, create rules).
- Sessions on non-Linux agents; parsing Windows logon events.
- Pairing SSH login/logout alerts into sessions (rejected in §3 as fragile).
- Pushing alerts anywhere (that is `blackwall-voice`).

## 12. Risks

- **`check_diff` on the first output.** If Wazuh does not alert on the very
  first run, the Sessions tab stays empty until the first login/logout. Verified
  during implementation; if confirmed, `bw-lab` triggers one login after
  provisioning so the lab always has a snapshot.
- **Lab noise in sessions.** Almost every session in the lab is Ansible or
  `bw-check` from `172.28.0.1`. Shown as is — that is real host activity.
