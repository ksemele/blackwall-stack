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
claude_workspace_id: ""  # only if the key is not scoped to a workspace (wrkspc_...)
telegram_bot_token: "your-bot-token"
telegram_chat_id:   "your-chat-id"
analysis_threshold: 7   # analyze alerts with rule.level >= this value
claude_model:       "claude-haiku-5-5"
claude_effort:      "low"
max_tokens:         2048   # thinking counts against it
```

| Variable | Default | Purpose |
|---|---|---|
| `brain_ignore_rule_ids` | `["99999"]` | `rule.id`s never analyzed (Voice's verify rule) |
| `claude_api_url` / `claude_timeout` | Anthropic messages URL / `30` | endpoint and per-request timeout (override in tests) |
| `brain_state_max_ids` | `10000` | processed IDs kept in the state file |

The state file keeps only the newest `brain_state_max_ids` IDs: compacted at
startup and at most once an hour (atomic rewrite), the in-memory set follows.
Older IDs are useless, the analyst seeks to the end of the alert log on start.

## One message per alert

Brain's Telegram message replaces Voice's for alerts at or above
`analysis_threshold`: Voice is capped with `voice_max_level = analysis_threshold - 1`
(see [blackwall-voice/README.md](../blackwall-voice/README.md#one-message-per-alert-with-brain);
the lab inventory derives it from `brain_enabled`). The roles stay independent.

If Claude fails for an alert after 3 attempts, Brain does not drop it: it sends
the raw alert to Telegram with the marker `AI analysis unavailable: <reason>`
(e.g. `Claude API HTTP 401`, `Claude API timeout/unreachable`) and marks it processed. If Telegram itself fails,
the send is retried 3 times and then logged as dropped.

## Deploy

```bash
cd blackwall-brain
ansible-playbook ansible/playbooks/brain.yml \
  -e claude_api_key=<key> \
  -e telegram_bot_token=<token> \
  -e telegram_chat_id=<chat_id>
```

## Turn AI analysis on/off

`brain_enabled` (default `true`) switches the analyst without uninstalling it:
`false` stops and disables the service, the script, config and state stay in
place. With Brain off Voice must forward everything again: Voice lifts its
`voice_max_level` by itself, at deploy time and at runtime (it checks the
`brain-analyst` unit every 15 s, so a crash or stop is covered too, apart from
a window of up to 15 s plus the restart delay); in the lab
`bw-lab provision brain ...` re-deploys Voice for you. Both roles must run on
the same host.

```bash
ansible-playbook ansible/playbooks/brain.yml -e brain_enabled=false   # off
ansible-playbook ansible/playbooks/brain.yml                          # on again
```

In the lab: `tests/bin/bw-lab provision brain -e brain_enabled=false` (re-deploys Voice too); while it
is off, `bw-check brain` reports SKIP. Set it in the inventory to make it stick.
Enabling requires `claude_api_key`, `telegram_bot_token` and `telegram_chat_id`
(the role fails up front otherwise — in the lab run through `direnv exec .`);
disabling works without them.

## Verify

```bash
ansible-playbook ansible/playbooks/verify-brain.yml
```

Injects a synthetic level-10 alert (rule 99998, ignored by Voice) and confirms brain-analyst picks it up.
With placeholder credentials: Claude API failure is logged gracefully (expected behavior).
With real credentials: full pipeline executes and Telegram message is received.

## Docs

- `CLAUDE.md` — full module context, design decisions, agent rules
- `STATUS.md` — current implementation state
