# blackwall-voice — Voice

**Role:** Notification and visualization layer.  
Voice consumes Wazuh alerts and delivers them to humans via configured channels.  
It does NOT perform analysis (that is Brain) and does NOT perform detection (that is Eye).

---

## Purpose

Eye sees. Voice speaks. When Wazuh fires an alert, Voice routes it to the right human channel
at the right severity threshold — no alert left unheard, no noise amplified.

---

## Stage 2 Scope

### 1. Telegram Alerter (primary deliverable)

A Python script deployed as a systemd service on the manager host that:

- Reads from `/var/ossec/logs/alerts/alerts.json` (tail-follows the live log)
- Alternatively polls Wazuh REST API `/alerts` endpoint (fallback / future)
- Filters by severity: only forwards alerts with `rule.level >= severity_threshold` (default 5)
- Formats a human-readable Telegram message per alert
- Sends via Telegram Bot API to a configured `chat_id`
- Deduplicates: tracks sent `id` values to avoid resending on restart (state file, capped at `alerter_state_max_ids`, compacted hourly)
- Optional upper bound `voice_max_level` (one message per alert when Brain is on; ignored with a warning at deploy time if `brain-analyst` is not enabled and active on the host) and `alerter_ignore_rule_ids` (Brain's verify rule)
- Restarts automatically on failure (systemd `Restart=on-failure`)

### 2. Wazuh Dashboard (deferred — not Stage 2)

OpenSearch Dashboards is the official Wazuh UI. It is heavyweight and requires OpenSearch.
Noted here as a future stage item. Stage 2 delivers only the Telegram alerter.

---

## Design Principles

- **Minimal:** one Python script, one systemd unit, one config file
- **Config as code:** all settings live in Ansible vars and templates; no manual edits on hosts
- **Resilient:** systemd `Restart=on-failure`; script catches API/network errors and sleeps, never crashes
- **Signal over noise:** `severity_threshold` var controls what gets through; default 5

---

## Stack

| Component | Detail |
|---|---|
| Language | Python 3 (already present on manager host) |
| Alert source | `/var/ossec/logs/alerts/alerts.json` (line-buffered JSON, one event per line) |
| Notification sink | Telegram Bot API (`https://api.telegram.org/bot<token>/sendMessage`) |
| Secrets | `telegram_bot_token` and `telegram_chat_id` — passed as Ansible vars (vault-encrypted in production) |
| Service manager | systemd on manager host |
| Deployment | Ansible role `telegram-alerter`, playbook `voice.yml` |
| Target host group | `wazuh_managers` (shared inventory from blackwall-eye) |

---

## Repository Layout

```
blackwall-voice/
├── CLAUDE.md                         # this file
├── README.md                         # operator quick-start
├── STATUS.md                         # current module state
└── ansible/
    ├── ansible.cfg                   # roles_path, inventory pointer → blackwall-eye inventory
    ├── playbooks/
    │   ├── voice.yml                 # deploy telegram-alerter to wazuh_managers
    │   └── verify-voice.yml          # check service running, send test alert
    └── roles/
        └── telegram-alerter/
            ├── defaults/
            │   └── main.yml          # severity_threshold, poll_interval, state_file path
            ├── tasks/
            │   └── main.yml          # install deps, deploy script + unit, enable service
            ├── handlers/
            │   └── main.yml          # restart alerter service
            └── templates/
                ├── alerter.py.j2     # the alerter script (Jinja2 → Python)
                └── alerter.service.j2  # systemd unit file
```

---

## Key Variables (defaults)

| Variable | Default | Description |
|---|---|---|
| `severity_threshold` | `5` | Minimum `rule.level` to forward |
| `poll_interval` | `5` | Seconds between log tail reads |
| `alerter_state_file` | `/var/lib/voice-alerter/sent_ids.txt` | Dedup state |
| `alerter_log_file` | `/var/log/voice-alerter/alerter.log` | Service log |
| `telegram_bot_token` | `""` | Must be set; use Ansible vault |
| `telegram_chat_id` | `""` | Must be set |

---

## Inventory

Voice shares the inventory with blackwall-eye. The `ansible.cfg` in this module points to:

```
inventory = ../tests/inventory/hosts.yml
```

Do NOT duplicate inventory files. If a new environment is needed, add a host file
to `blackwall-eye/tests/inventory/` and reference it from both modules.

---

## Agent Rules

1. **Read STATUS.md before starting any work.** If status is `pending`, nothing has been implemented.
2. **Update STATUS.md after completing each task** — `last_task`, `next_task`, `last_updated`.
3. **Test against the shared Docker test environment** defined in `blackwall-eye/tests/inventory/`.
4. **Keep Voice independently deployable** — no imports or hard path dependencies on blackwall-eye source files.
5. **Secrets via Ansible vault** — never commit `telegram_bot_token` or `telegram_chat_id` in plaintext.
6. **Keep docs in sync** — if playbooks, roles, or CLI commands change, update README.md in the same commit.

---

## Testing Checklist

- [ ] `ansible-playbook playbooks/voice.yml` completes without errors on test host
- [ ] `systemctl is-active voice-alerter` returns `active` on manager host
- [ ] Injecting a level-5+ alert into `alerts.json` produces a Telegram message within `poll_interval` seconds
- [ ] Restarting the service does not resend already-sent alerts (dedup state persists)
- [ ] Alerts below `severity_threshold` are silently dropped
