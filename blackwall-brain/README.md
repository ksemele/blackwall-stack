# blackwall-brain — Brain

AI analysis layer for the Blackwall security platform. Tail-follows Wazuh alerts,
calls Claude API to classify threats, and sends enriched analysis to Telegram.
Stage 3 delivers the brain-analyst service running alongside Voice on the Wazuh manager host.

## Stack

- Python 3 stdlib (no pip deps) + Anthropic Claude API
- Wazuh alert log: `/var/ossec/logs/alerts/alerts.json`
- Telegram Bot API (same bot as blackwall-voice)
- Ansible role `brain-analyst` deploys to `wazuh_managers` group

## Test Environment

Shared with blackwall-eye and blackwall-voice. Start containers from the **repo root**:

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
```

Wait for SSH before running Ansible:

```bash
cd blackwall-brain
ansible all -m wait_for_connection
```

See [tests/README.md](../tests/README.md) for full environment documentation.

## Configure

On the test environment the secrets (claude_api_key, telegram_bot_token, telegram_chat_id)
are read from env vars set in `.envrc` — see [tests/README.md](../tests/README.md#secrets-telegram-claude-api).
For other targets, set these vars before deploying (use Ansible vault for secrets in production):

```yaml
claude_api_key:     "your-anthropic-api-key"
telegram_bot_token: "your-bot-token"
telegram_chat_id:   "your-chat-id"
analysis_threshold: 7   # analyze alerts with rule.level >= this value
claude_model:       "claude-haiku-5-5"
claude_effort:      "low"
max_tokens:         2048   # thinking counts against it
```

## Deploy

```bash
cd blackwall-brain
ansible-playbook ansible/playbooks/brain.yml \
  -e claude_api_key=<key> \
  -e telegram_bot_token=<token> \
  -e telegram_chat_id=<chat_id>
```

## Verify

```bash
ansible-playbook ansible/playbooks/verify-brain.yml
```

Injects a synthetic level-9 alert and confirms brain-analyst picks it up.
With placeholder credentials: Claude API failure is logged gracefully (expected behavior).
With real credentials: full pipeline executes and Telegram message is received.

## Docs

- `CLAUDE.md` — full module context, design decisions, agent rules
- `STATUS.md` — current implementation state
