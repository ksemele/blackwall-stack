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

Injects a synthetic level-10 alert and checks that Telegram accepted the
message (its id lands in the dedup state file). With a token set, a refused
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
