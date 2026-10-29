# blackwall-voice — Voice

Notification layer for the Blackwall security platform. Consumes Wazuh alerts and
delivers them to configured channels. Stage 2 delivers a Telegram alerter running
as a systemd service on the Wazuh manager host.

## Stack

- Python 3 + Telegram Bot API
- Wazuh alert log: `/var/ossec/logs/alerts/alerts.json`
- Ansible role `telegram-alerter` deploys to `wazuh_managers` group

## Test Environment

Shared with blackwall-eye. Start containers from the **repo root**:

```bash
tests/bin/bw-lab up --no-provision   # or: bw-lab up, to provision straight away
```

Wait for SSH before running Ansible:

```bash
cd blackwall-voice
ansible all -m wait_for_connection
```

See [tests/README.md](../tests/README.md) for full environment documentation.

## Configure

On the test environment the secrets (telegram_bot_token, telegram_chat_id)
are read from env vars set in `.envrc` — see [tests/README.md](../tests/README.md#secrets-telegram-claude-api).
For other targets, set these vars before deploying (use Ansible vault for secrets):

```yaml
telegram_bot_token: "your-bot-token"
telegram_chat_id:   "your-chat-id"
severity_threshold: 5   # forward alerts with rule.level >= this value
```

`telegram_chat_id` is the chat the bot writes to — your own user id, not the
bot's (the number before `:` in the token is the bot itself; Telegram answers
403 "the bot can't send messages to the bot"). Send `/start` to the bot, then
read the id from `https://api.telegram.org/bot<token>/getUpdates`
(`result[].message.chat.id`).

| Variable | Default | Purpose |
|---|---|---|
| `severity_threshold` | `5` | lower bound: forward `rule.level >=` this |
| `voice_max_level` | `""` (no upper bound) | upper bound: forward only `level <= voice_max_level` |
| `alerter_ignore_rule_ids` | `["99998"]` | `rule.id`s never forwarded (Brain's verify rule) |
| `alerter_state_max_ids` | `10000` | dedup IDs kept in `sent_ids.txt` |
| `voice_cover_unit` | `brain-analyst` | unit that must be active for the upper bound to apply |
| `voice_cover_script` | `/etc/brain-analyst/brain.env` | read at deploy time for `BRAIN_ANALYSIS_THRESHOLD` (bound clamp) |

### One message per alert with Brain

With Brain on, an alert at or above its `analysis_threshold` already reaches
Telegram as an analysis. Set `voice_max_level` to `analysis_threshold - 1` so
Voice stops below that line (alerts above it are logged as `Skipped alert`).
With Brain off leave it empty. The two roles do not import each other, so the
link lives in the inventory; the lab does it with a Jinja expression:

```yaml
voice_max_level: "{{ (analysis_threshold | int - 1) if (brain_enabled | default(true) | bool) else '' }}"
```

Fail-safe, two layers (`voice_cover_unit`, default `brain-analyst`):

- Deploy time: when `voice_max_level` is set, the play checks that the unit is
  enabled and active. If not, the bound is ignored (Voice forwards everything
  `>= severity_threshold`) and the play prints `voice_max_level ignored: ...`.
  It also reads `ANALYSIS_THRESHOLD` from the deployed Brain script
  (`voice_cover_script`) and clamps the bound to `analysis_threshold - 1`
  (with a `clamped` warning if it differed).
- Runtime: while a bound is set the alerter runs `systemctl is-active` on the
  unit at most every 15 s. While it is not active the bound is lifted and
  everything `>= severity_threshold` is forwarded; both transitions are logged
  once. Residual window: an alert above the bound that arrives within 15 s
  after Brain stops (or while Brain is "activating" after a crash) can be
  skipped by Voice and not analyzed; Brain's own retry/fallback covers alerts
  it has already picked up.

Both roles are assumed to run on the same host (`wazuh_managers`): the unit
check and the threshold read are local.

`brain_enabled` is Brain's toggle (default `true`). If you switch it with
`-e` in the lab, `bw-lab provision brain ...` re-deploys Voice with the same
args by itself (when Voice is installed; Voice first when Brain is being
disabled, after Brain otherwise).

### Dedup state

`sent_ids.txt` keeps only the newest `alerter_state_max_ids` IDs. It is
compacted at startup and at most once an hour (atomic rewrite: tmp + rename);
the in-memory set is trimmed to match. Older IDs are useless because the
alerter seeks to the end of the alert log on start.

### Ignored synthetic alerts

`alerter_ignore_rule_ids` lists `rule.id`s Voice drops silently. The default
`["99998"]` is the synthetic alert of Brain's `verify-brain.yml`, so Brain's
self-test does not also page through Voice (Brain ignores Voice's `99999`).

The lab inventory sets `severity_threshold: 10`: provisioning alone produces
hundreds of level-7 alerts a day.

## Deploy

```bash
cd blackwall-voice
ansible-playbook ansible/playbooks/voice.yml \
  -e telegram_bot_token=<token> \
  -e telegram_chat_id=<chat_id>
```

## Verify

```bash
ansible-playbook ansible/playbooks/verify-voice.yml
```

Injects a synthetic alert (rule 99999, level 10 by default) and checks that
Telegram accepted the message (its id lands in the dedup state file). When
`voice_max_level` leaves no room for it (Brain on in the lab), it checks that
Voice skipped the alert instead. With a token set, a refused
send fails the playbook; the Telegram error body is in
`/var/log/voice-alerter/alerter.log`. A failed send is retried 3 times, then
logged as dropped.

## Metrics (node_exporter + Prometheus + Grafana)

Host metrics are collected by the `node-exporter` role and scraped by the
Prometheus/Grafana pair that runs as compose services of the test environment
(`tests/docker/prometheus/`, `tests/docker/grafana/`).

```bash
cd blackwall-voice
ansible-playbook ansible/playbooks/metrics.yml
ansible-playbook ansible/playbooks/verify-metrics.yml
```

| Variable | Default | Purpose |
|---|---|---|
| `node_exporter_install_method` | `apt` | `apt` (package) or `tarball` (GitHub release) |
| `node_exporter_version` | `1.8.2` | version used by the tarball method |
| `node_exporter_listen_address` | `0.0.0.0:9100` | bind address |

The exporter must be reachable on `:9100` from the Prometheus container, so the
`nftables` role opens that port for both host groups.

Wazuh exposes no Prometheus metrics of its own, so it is absent from Grafana on
purpose — agent health is checked through the Wazuh API instead.

Grafana ships one provisioned dashboard, **Blackwall hosts** (hosts up, load
average, memory used) at http://localhost:3000 (`admin` / `blackwall`).

## Docs

- `CLAUDE.md` — full module context, design decisions, agent rules
- `STATUS.md` — current implementation state
