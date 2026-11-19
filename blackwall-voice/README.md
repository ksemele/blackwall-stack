# blackwall-voice — Voice

Notification layer for the Blackwall security platform. Consumes Wazuh alerts and
delivers them to configured channels. Stage 2 delivers a Telegram alerter running
as a systemd service on the Wazuh manager host, plus an optional generic webhook /
Slack channel next to (or instead of) Telegram.

## Stack

- Python 3 + Telegram Bot API + a generic webhook channel (stdlib `urllib.request`)
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

On the test environment the secrets (telegram_bot_token, telegram_chat_id,
optionally alerter_webhook_url) are read from env vars set in `.envrc` — see
[tests/README.md](../tests/README.md#secrets-telegram-claude-api).
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
| `alerter_webhook_url` | `""` (off) | generic webhook / Slack incoming-webhook URL (secret) |
| `alerter_webhook_format` | `generic` | payload shape: `generic` or `slack` |

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

### Webhook / Slack channel

`alerter_webhook_url` adds a second channel next to Telegram (empty = off).
A webhook alone can stand on its own: the role only requires Telegram
credentials when no webhook URL is set. The URL is POSTed the alert as JSON
(`Content-Type: application/json`). Point it at a Slack incoming webhook or
at any endpoint that takes JSON (Mattermost, a chat gateway, your own
receiver):

```yaml
# placeholder — a real Slack URL is a bearer secret, keep it out of the repo
alerter_webhook_url: "https://hooks.example.com/services/T0000000/B0000000/XXXXXXXXXXXXXXXX"
alerter_webhook_format: slack   # or generic (default)
```

`generic` payload — the alert's fields as a JSON object (the log excerpt is
capped at 300 chars):

```json
{
  "level": 10,
  "rule_id": "5710",
  "description": "First time user login",
  "agent": "black-host-01",
  "timestamp": "2026-11-18T12:34:56.789012+00:00",
  "log": "user 'root' login attempt from 192.0.2.10 ..."
}
```

`slack` payload — `{"text": "..."}` with Slack mrkdwn: a `*BLACKWALL ALERT*`
line, the host and log excerpt in backticks, the same 300-char excerpt.

Both channels share one pipeline (same filter, dedup and retry rules): each
configured channel gets up to 3 send attempts, `poll_interval` apart; a
channel that still fails is logged as dropped and only that channel loses the
alert. The dedup ID is written when **every** configured channel accepted —
with two channels, one failing keeps the alert out of `sent_ids.txt` (it is
not retried either: the tail has already moved past the line). One consequence
of the single dedup set: while one channel keeps failing, the ID is not
recorded at all, so if the truncation re-read path fires (the alert log was
rewritten shorter while the alerter was running), alerts the *healthy*
channel already delivered can be delivered a second time.

**Acceptance is Voice's own contract, not a provider guarantee.** The
reference docs define only the request side of chat webhooks (OpenSearch
`webhook` storage type, Wazuh `integration` reference — the offline docs hold
nothing about how a provider *responds*). Voice treats a send as accepted
when the receiver answers 2xx: non-2xx (4xx/5xx, transport errors) is retried
and, if it keeps failing, logged as dropped; a 2xx is logged with its
truncated response body (`Webhook accepted (HTTP 200): …`), so a receiver
that reports a rejected payload *inside* a 2xx at least leaves a trace in
`alerter.log` — but it still counts as sent. If you need stricter
confirmation, point `alerter_webhook_url` at a receiver you control and check
`sent_ids.txt` (that is what `verify-voice.yml` does).

The URL is a secret (Slack-style URLs carry the key in the path): the alerter
logs channel names, error codes and bodies only, and a log filter scrubs the
URL and the bot token from every line — including error text and tracebacks
that echo the URL back (`<redacted>`). The role validates the URL/format with
a `no_log` task, so a failed run does not echo it either.

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
  # optional webhook channel: -e alerter_webhook_url=https://hooks.example.com/services/...
```

Bearer secrets on the command line land in your shell history and in `ps`;
prefer passing all of them through a file that is not in the repo:
`-e @secrets.yml` (or ansible-vault) instead of `-e`.

## Verify

```bash
ansible-playbook ansible/playbooks/verify-voice.yml
```

Injects a synthetic alert (rule 99999, level 10 by default) and checks that
every configured channel accepted the message (its id lands in the dedup
state file). When `voice_max_level` leaves no room for it (Brain on in the
lab), it checks that Voice skipped the alert instead. With a channel
configured (token or webhook URL), a refused send fails the playbook; the
Telegram/webhook error body is in `/var/log/voice-alerter/alerter.log`. A
failed send is retried 3 times per channel, then logged as dropped. For a
live webhook check in the lab, point `BW_WEBHOOK_URL` at a local receiver
(`nc -l 8080` or a few lines of Python) on the manager host.

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
