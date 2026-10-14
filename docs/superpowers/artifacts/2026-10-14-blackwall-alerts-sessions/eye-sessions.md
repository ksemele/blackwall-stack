# Eye: session snapshots (rule 100100)

- Status: done
- Module: blackwall-eye
- Date: 2026-10-14
- Commits: see `git log` ("feat(eye): session snapshots via last + rule 100100; explicit ruleset")

## What was done

- Manager `ossec.conf.j2`: explicit `<ruleset>` block (GET /rules no longer fails with 1106).
- New `blackwall_rules.xml.j2` deployed to `/var/ossec/etc/rules/blackwall_rules.xml`; rule 100100 (level 3, groups `blackwall`, `blackwall_sessions`, `check_diff`, child of 530).
- Agent: `full_command` localfile with alias `blackwall-sessions`, `last --time-format iso -w -n 50`, every 60 s (`wazuh_sessions_enabled`, `wazuh_sessions_frequency`).
- `tests/bin/bw-check`: asserts rule 100100 is loaded. README and STATUS updated.

## Captured full_log (for Task 7)

Layout: first line is exactly `ossec: output: 'blackwall-sessions':`, then raw `last` lines separated by `\n`
(no trailing text besides the `wtmp begins` line; the last lines are the `reboot   system boot ...` row and `wtmp begins <iso time>`).
Row format: `user  tty  host  <iso start>  [- <iso end> | still logged in]  [(duration)]`.
Rows with `still logged in` have no end time. Snapshot holds up to 50 rows + reboot/wtmp lines.

repr of the first 600 chars (alert with a `still logged in` row):

```
"ossec: output: 'blackwall-sessions':\nroot     pts/0        172.28.0.1       2026-10-14T00:38:05+00:00   still logged in\nroot     pts/0        172.28.0.1       2026-10-14T00:38:04+00:00 - 2026-10-14T00:38:04+00:00  (00:00)\nroot     pts/0        172.28.0.1       2026-10-14T00:38:02+00:00 - 2026-10-14T00:38:02+00:00  (00:00)\nroot     pts/0        172.28.0.1       2026-10-14T00:37:50+00:00 - 2026-10-14T00:38:02+00:00  (00:00)\nroot     pts/0        172.28.0.1       2026-10-14T00:37:50+00:00 - 2026-10-14T00:37:50+00:00  (00:00)\nroot     pts/0        172.28.0.1       2026-10-14T00:37:50+00:00 - 2026-"
```

Tail of a later snapshot: `...\nroot     pts/0        172.28.0.1       2026-10-14T00:33:49+00:00 - 2026-10-14T00:33:49+00:00  (00:00)\nreboot   system boot  7.0.14-orbstack-00380-ga7e0a2dc9535 2026-10-14T00:33:47+00:00   still running\nwtmp begins 2026-10-14T00:33:47+00:00`

Note: the `reboot` row also contains `still running` (not `still logged in`); the kernel field has spaces-free text but the column is wider than other hosts.

## check_diff first-run behaviour

check_diff DOES alert on the very first output: black-host-01 and black-dead-98 each got an alert at 2026-10-14T00:38:15 (26 lines) right after the lab reset, before any manual login. Subsequent alerts appear only when the output changes (second alert on black-host-01 at 00:39:15 after the test SSH login, 52 lines).

## Verification

- `bash -n tests/bin/bw-check`: ok.
- `tests/bin/bw-lab reset` then `tests/bin/bw-check all`: `PASS rule 100100 (blackwall sessions) is loaded`, 30 passed, 0 failed, 1 skipped (before and after the SSH login).
- Indexer query for rule.id 100100 on black-host-01 returned snapshots as above.

## Deviations

None.

## Open questions

- Lab sessions come from the ansible/bw-check SSH logins, so "still logged in" rows appear only while a login is open at snapshot time; the newest snapshot may have none. The Sessions tab should tolerate that.
