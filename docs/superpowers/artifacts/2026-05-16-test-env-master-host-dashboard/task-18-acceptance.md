# Task 18 — End-to-end acceptance

**Status:** DONE
**Module:** blackwall-eye
**Date:** 2026-05-16
**Commits during this task:**
- `902a81d` — fix(eye): dashboard race — flush handlers before start, wait for 302
- `e12ed6b` — docs/test: replace stale black-00 refs in eye README + verify fixtures

## Criteria

| # | Criterion | Result | Notes |
|---|---|---|---|
| 1 | Two containers from clean compose up | ✅ | `black-host-01`, `black-master-00` after `down -v && up --build` |
| 2 | all.yml clean fresh deploy | ✅ | `failed=0` on both; runtime ~7 min; auditd ignored (kernel netlink not in container, expected) |
| 3 | Dashboard 302 | ✅ | After fix 902a81d: stable 302 on first deploy, no manual `.kibana_1` intervention needed |
| 4 | Agent Active (manager side) | ✅ | `agent_control -l` shows `ID: 001, Name: black-host-01, IP: any, Active` |
| 5 | verify-agents passes | ✅ | `At least one external agent is Active — enrollment and connectivity verified.` |
| 6 | Override + revert works | ✅ | `-e wazuh_manager_host=192.0.2.1 -e wazuh_enrollment_password=hunter2` wrote `<address>192.0.2.1</address>`, `<authorization_pass_path>...</authorization_pass_path>`, and `authd.pass=hunter2`. Revert removed authd.pass and restored 172.28.0.10. |
| 7 | Tear down + re-up cycle | ✅ | `down -v` removed both containers + network; `up --build` recreated both cleanly |
| 8 | No stale name references | ✅ | After cleanup commit e12ed6b. Excludes `docs/superpowers/` (historical specs), `docs/stages/stage-1-test-env.md` (frozen Stage-1 design doc), `STATUS.md` |

## Root-cause + fix discovered during acceptance

**First acceptance run blocked at Criterion 3.** Dashboard stuck on `.kibana_1` migration:

> Another OpenSearch Dashboards instance appears to be migrating the index. Waiting for that migration to complete. If no other OpenSearch Dashboards instance is attempting migrations, you can get past this message by deleting index `.kibana_1` and restarting OpenSearchDashboards.

**Root cause:** the wazuh-dashboard role started the dashboard service BEFORE Ansible flushed pending `Restart wazuh-dashboard` handlers (notified by cert/config/keystore tasks earlier in the play). Sequence:
1. Service started by "Enable and start wazuh-dashboard service" task; dashboard begins `.kibana_1` creation/migration.
2. `wait_for port:443` succeeds within ~3s (port opens before Node.js finishes init).
3. End-of-play handler flush fires `Restart wazuh-dashboard`, killing the dashboard mid-migration.
4. New dashboard instance sees `.kibana_1` in mid-creation state, trips OpenSearch's "migration in progress" guard.

The earlier subagent diagnosis ("Docker volume not clearing") was wrong — `docker compose down -v` does correctly clear the indexer's data; the issue was an in-play race.

**Fix (commit 902a81d):**
- `meta: flush_handlers` added BEFORE the systemd start task. Any prior config-induced restarts now fire when the service isn't yet running, so they become no-op or initial-start, eliminating the mid-migration race.
- `wait_for port:443` replaced with `uri: status_code: 302` polling (retries=60, delay=5s) — confirms full Node.js init, not just socket listening.

## Stale-reference cleanup (commit e12ed6b)

Found 3 live files (excluded historical design docs) still mentioning `black-00`:
- `blackwall-eye/README.md` — comment + SSH tunnel example
- `blackwall-brain/ansible/playbooks/verify-brain.yml` — alert fixture
- `blackwall-voice/ansible/playbooks/verify-voice.yml` — alert fixture

All updated to `black-master-00`. Historical Stage-1 design doc at `blackwall-eye/docs/stages/stage-1-test-env.md` retained as-is (frozen, not authoritative).

## Manual steps still required (user)

These need a human/browser, were not run:
- Browser login at https://localhost:5601 with `admin / Blackwall1!` to confirm dashboard UI renders.
- Visual check that `black-host-01` appears as `Active` on the dashboard's Agents / Endpoint Summary page.

## Known caveats (acceptable)

- `internal_users.yml.j2` is non-idempotent due to bcrypt random salt — every `security-node.yml` run re-pushes the security config (`changed=1` on indexer template). Functionally idempotent (same password verifies). Documented in tests/README.md §13.
- auditd service fails to start in containers (kernel netlink unavailable) — `ignored: 2` in PLAY RECAP, expected.
