---
module: blackwall-voice
status: in_progress
stage: 2
last_updated: 2026-08-31
last_task: "node-exporter role + Prometheus/Grafana as lab compose services; metrics from three hosts arrive, the Blackwall hosts dashboard is provisioned from the repo"
next_task: "Set a real Telegram bot token + chat_id via ansible-vault and test end-to-end alert delivery"
blockers: []
---

## Done

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

- `telegram_bot_token` and `telegram_chat_id`: in the lab they come from env (`BW_TELEGRAM_*` in `.envrc`, direnv); scrubbed from git history on 2026-10-12. For prod — ansible-vault.
- The state file (`sent_ids.txt`) grows without bound — needs rotation/TTL eventually.

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
