---
module: blackwall-voice
status: in_progress
stage: 2
last_updated: 2026-11-18
last_task: "Generic webhook / Slack channel next to Telegram (#22)"
next_task: "Confirm the webhook channel in the live lab (bw-check voice with BW_WEBHOOK_URL set); dashboard (OpenSearch) stays deferred"
blockers: []
---

## Done

### 2026-11-18 — generic webhook / Slack channel next to Telegram (#22)

- `alerter_webhook_url` (empty = off) adds a JSON webhook channel next to
  Telegram; `alerter_webhook_format`: `generic` (level, rule_id, description,
  agent, timestamp, 300-char log excerpt) or `slack` (`{"text": ...}` mrkdwn).
  Payload builders are pure functions; stdlib `urllib.request` only.
- Two channels share one pipeline: up to 3 attempts per channel, `poll_interval`
  apart; a still-failing channel is logged as dropped and loses only its copy.
  The dedup ID is written when *every* configured channel accepted.
- A webhook may stand alone: the role now requires "a channel", not specifically
  Telegram credentials. Without webhook vars the old behavior is unchanged.
- Secret hygiene: a logging filter scrubs the webhook URL and the bot token from
  every log line (error text and tracebacks included); URL/format validation runs
  in a `no_log` assert task.
- Lab: `BW_WEBHOOK_URL` env wiring (`hosts.yml`, `.envrc.example`, tests README);
  `verify-voice.yml` delivery assert is strict when any channel is configured.
- Verified offline: `tests/unit/voice_webhook_test.py` (payloads, redaction,
  multi-channel "sent" rule, acceptance rule against a stdlib `http.server`
  fixture: 404 → dropped/not recorded, 2xx → recorded, body logged) and
  `tests/unit/voice_role_asserts_test.py` (the three guard assert tasks run
  verbatim on localhost across the five var combinations). Live `bw-check
  voice` with a real receiver still to run in the lab.
- Review round 1 (2026-11-19): the URL check used `| regex(...)` — Ansible has
  no `regex` filter (it is a test), so every deploy failed with
  "No filter named 'regex'"; now `is match('^https?://')`, split into separate
  URL/format assert tasks with distinct `fail_msg`s, pinned by the new
  role-asserts test (real `ansible-playbook -c local` run). Webhook 2xx
  responses now log their truncated body; "no channel configured" warning is
  once per run.

### 2026-11-18 — unit check for the alert filters (#19)

- `tests/unit/voice_filter_test.py` renders the alerter template with the role
  defaults (plus `voice_max_level` 9) and calls `handle_line()` with synthetic
  alerts: non-JSON lines and ignored rule IDs dropped, `severity_threshold` and
  `voice_max_level` edges, dedup by alert ID (same run and after a restart), a
  failed send retried 3 times then dropped. `send_telegram`, `save_sent_id`,
  `current_max_level` and `time.sleep` are stubbed — no Telegram, no host.
- Before this, only the formatter had a unit test; the filters were verified in
  the lab only.

### 2026-10-28 — no alerts lost around rotation (#23)

- The tail checks the path only at EOF of the open file: on a new inode it
  drains the old file (still open), then reads the new one from its start
  instead of its end. A shrunk file is reread from 0; a line whose newline is
  not written yet is held instead of parsed half-written (it used to be dropped
  as non-JSON).
- `tests/unit/alerts_tail_test.py` covers rotation, truncation and split lines
  for Voice and Brain; the previous loop fails it.
- Still by design: alerts written while the service is down are not replayed
  (the first open seeks to EOF).

### 2026-10-15 — less noise

- `sent_ids.txt` is capped at `alerter_state_max_ids` (10000): compacted at
  startup and hourly, atomic rewrite, in-memory set trimmed to match.
- `alerter_ignore_rule_ids` (default `["99998"]`, Brain's verify rule) are
  dropped; Brain ignores Voice's `99999`.
- `voice_max_level` (default empty = no upper bound): forward only
  `severity_threshold <= level <= voice_max_level`. The lab inventory sets it
  to `analysis_threshold - 1` when `brain_enabled` is true, so each alert gives
  one message. `verify-voice.yml` reads the deployed bound and asserts skip
  instead of delivery when the window is empty.
- Verified in the lab: Brain on -> one Telegram message (Brain's), Voice logs
  `Skipped alert`; Brain off -> Voice sends it; `bw-check all` green.

### 2026-10-14 — end-to-end Telegram delivery

- Verified in the lab with a real bot: `bw-check voice` PASS, the synthetic
  level-10 alert arrived in Telegram.
- Root cause of the earlier 403: `telegram_chat_id` was the bot's own id
  (Telegram: "the bot can't send messages to the bot"). README explains how to
  get the right id.
- `alerter.py`: logs the Telegram error body; a failed send is retried
  3 times and then logged as dropped (the old "will retry next cycle" never
  retried — the tail had already moved past the line).
- `verify-voice.yml`: the delivery assert is strict when a token is set.
- Lab inventory: `severity_threshold: 10` (level-7 provisioning noise,
  ~280 alerts/day at the default 5).

### Stage 2 — host metrics

- **`ansible/roles/node-exporter/`** — installs `prometheus-node-exporter` from apt
  (`node_exporter_install_method: apt`), with a fallback to the GitHub tarball.
  Listens on `0.0.0.0:9100`, systemd enabled + started.
- **`ansible/playbooks/metrics.yml`** — rollout to the whole `all` group.
- **`ansible/playbooks/verify-metrics.yml`** — unit active + `/metrics` serves
  `node_load1` and `node_memory_MemTotal_bytes`.
- **Prometheus + Grafana as lab compose services** (`black-metrics-00`,
  `black-grafana-00`), configs in `tests/docker/prometheus/` and
  `tests/docker/grafana/`: the scrape config, datasource and the
  "Blackwall hosts" dashboard are provisioned from the repo, no manual steps.
- Verified on the live lab: `metrics.yml` ok=7 on three hosts, Prometheus
  scrapes `black-master-00` and `black-host-01`, `node_load1` returns data.

### Stage 2 — Telegram Alerter

- **`ansible/roles/telegram-alerter/defaults/main.yml`** — all variables with defaults:
  `severity_threshold`, `poll_interval`, `alerter_state_file`, `alerter_log_file`,
  `alerter_install_dir`, `telegram_bot_token`, `telegram_chat_id`

- **`ansible/roles/telegram-alerter/tasks/main.yml`** — tasks:
  create directories, deploy the script + unit, `daemon_reload` + enable + start

- **`ansible/roles/telegram-alerter/handlers/main.yml`** — handlers:
  `Reload systemd`, `Restart voice-alerter`

- **`ansible/roles/telegram-alerter/templates/alerter.py.j2`** — Python 3 script:
  - Tail-follows `/var/ossec/logs/alerts/alerts.json` (seeks to EOF at start)
  - Filters on `rule.level >= severity_threshold`
  - Deduplicates by `id` via a state file
  - Formats the Telegram message (hostname, level, description, timestamp)
  - Sends via `urllib.request` (no external pip dependencies)
  - Detects file replacement (inode rotation) — reopens the log automatically
  - Logs to stdout (systemd captures it into `alerter_log_file`)

- **`ansible/roles/telegram-alerter/templates/alerter.service.j2`** — systemd unit:
  `User=root`, `Restart=on-failure`, `RestartSec=10s`, `PYTHONUNBUFFERED=1`

- **`ansible/playbooks/voice.yml`** — deploys `telegram-alerter` to `wazuh_managers`

- **`ansible/playbooks/verify-voice.yml`** — verification:
  - `systemctl is-active voice-alerter`
  - Injects a synthetic alert (level=10) into `alerts.json` via `printf >> `
  - Waits `poll_interval + 3` seconds
  - Checks the alerter log (grep for the synthetic ID)
  - Checks the state file (ignore_errors=true with an invalid token)

### Test results

- `ansible-playbook voice.yml` → `ok=9, changed=2, failed=0`
- `ansible-playbook verify-voice.yml` → `ok=13, changed=1, failed=0`
- The `voice-alerter` service is active on `black-master-00` (Docker test env; it was called `black-00` at the time of the test)
- The alerter correctly reads real Wazuh alerts from `alerts.json`
- Synthetic alerts (level=10) are picked up and logged
- Telegram 404 is expected (the token in hosts.yml was invalid)

## Open questions

- `telegram_bot_token` and `telegram_chat_id`: in the lab they come from env (`BW_TELEGRAM_*` in `.envrc`, direnv); scrubbed from git history on 2026-10-12. For prod — supplied from the owner's secret management (no ansible-vault).
- `voice_max_level` is ignored when `brain-analyst` is not active: at deploy time (warning, clamp to the deployed analysis_threshold - 1) and at runtime (15 s check). `bw-lab provision brain` re-runs Voice (first when disabling Brain).

## Decisions

- Channel "sent" rule (#22): one dedup set, not per-channel state. An alert is
  attempted once per configured channel (3 attempts each, in the same inline
  loop); the ID lands in `sent_ids.txt` only when all channels accepted. A
  failed channel's copy is unrecoverable anyway — the tail never replays a
  line it passed — so per-channel state files would add complexity for no
  delivery gain. Accepted limitation: while one channel keeps failing no ID is
  recorded, so the truncation re-read path can resend what the healthy channel
  already delivered (documented in README).
- Webhook acceptance (#22): Voice's own receiver contract, not a sourced
  provider fact — the offline docs (`opensearch-docs/_security/audit-logs/
  storage-types.md`, `wazuh-docs/.../ossec-conf/integration.rst`) define only
  the request side of chat webhooks, nothing about responses. 2xx = accepted
  (the truncated response body is logged so a refusal inside a 2xx leaves a
  trace in alerter.log), non-2xx retried then dropped. Operators needing a
  stronger confirmation point the URL at their own receiver and check
  `sent_ids.txt` (`verify-voice.yml` does exactly this).
- The webhook URL is never logged: `_RedactSecrets` on the alerter logger
  scrubs URL + bot token from formatted messages and tracebacks (urllib's
  `InvalidURL` / "unknown url type" errors and some providers' error bodies
  echo the full URL back). Unit tests assert the URL never appears in output.
- Wazuh exposes no Prometheus metrics natively — no exporter was built for it.
  "Metrics are arriving" is checked via node_exporter, agent health via the
  Wazuh API (`tests/bin/bw-check wazuh`).
- Prometheus/Grafana live in the lab compose file, not in ansible roles: they
  are test-environment infrastructure, not something rolled out to prod hosts.
  node_exporter is the opposite — a role, installed on real hosts.
- `bw-check voice` gives SKIP when the alerter is not deployed: `bw-lab provision all`
  deliberately leaves voice out — it needs a real Telegram token.
- Dedup: `id` → `sent_ids.txt` (persistent across restarts)
- Rotation: on a new inode the old file is drained, the new one is read from its start (2026-10-28)
- Logging: Python → stdout → systemd `StandardOutput=append:` (one path, no duplication)
- Dashboard (OpenSearch) deferred — not part of Stage 2
