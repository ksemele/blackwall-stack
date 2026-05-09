# blackwall-voice — Voice

Notification layer for the Blackwall security platform. Consumes Wazuh alerts and
delivers them to configured channels. Stage 2 delivers a Telegram alerter running
as a systemd service on the Wazuh manager host.

## Stack

- Python 3 + Telegram Bot API
- Wazuh alert log: `/var/ossec/logs/alerts/alerts.json`
- Ansible role `telegram-alerter` deploys to `wazuh_managers` group

## Configure

Set these vars before deploying (use Ansible vault for secrets):

```yaml
telegram_bot_token: "your-bot-token"
telegram_chat_id:   "your-chat-id"
severity_threshold: 5   # forward alerts with rule.level >= this value
```

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

Sends a test alert through the pipeline and confirms the Telegram message is received.

## Docs

- `CLAUDE.md` — full module context, design decisions, agent rules
- `STATUS.md` — current implementation state
