---
module: blackwall-voice
status: in_progress
stage: 2
last_updated: 2026-10-15
last_task: "Noise: bounded dedup state, ignored verify rules, voice_max_level upper bound (one message per alert with Brain)"
next_task: "Dashboard (OpenSearch) is deferred; next Voice work is channel routing (Slack/webhook)"
blockers: []
---

## Done

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
- `voice_max_level` is ignored (with a warning) when `brain-analyst` is not running on the host; `bw-lab provision brain` re-runs Voice.

## Decisions

- Wazuh exposes no Prometheus metrics natively — no exporter was built for it.
  "Metrics are arriving" is checked via node_exporter, agent health via the
  Wazuh API (`tests/bin/bw-check wazuh`).
- Prometheus/Grafana live in the lab compose file, not in ansible roles: they
  are test-environment infrastructure, not something rolled out to prod hosts.
  node_exporter is the opposite — a role, installed on real hosts.
- `bw-check voice` gives SKIP when the alerter is not deployed: `bw-lab provision all`
  deliberately leaves voice out — it needs a real Telegram token.
- Dedup: `id` → `sent_ids.txt` (persistent across restarts)
- Inode rotation detection: the service reopens the file when it is replaced (linear file vs log rotate)
- Logging: Python → stdout → systemd `StandardOutput=append:` (one path, no duplication)
- Dashboard (OpenSearch) deferred — not part of Stage 2
