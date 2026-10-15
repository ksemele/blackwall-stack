---
module: blackwall-brain
status: in_progress
stage: 3
last_updated: 2026-10-15
last_task: "Noise: bounded state, ignored Voice verify rule, raw-alert fallback when Claude fails"
next_task: "Trap integration (Stage 4) or workload identity federation (see Plans)"
blockers: []
---

## Done

- 2026-10-15: less noise. State file capped at `brain_state_max_ids` (10000,
  compacted at startup and hourly, atomic). `brain_ignore_rule_ids`
  (`["99999"]`, Voice's verify rule) are skipped. When Claude fails after 3
  attempts the raw alert goes to Telegram marked `AI analysis unavailable:
  <reason>` and is marked processed (Telegram failure: retry, then dropped).
  Voice stops below `analysis_threshold` via `voice_max_level`, so there is one
  message per alert. Verified in the lab with an invalid key (fallback) and a
  real one (single Brain message).

- 2026-10-15: `brain_enabled` toggle (default `true`): `false` stops and
  disables the service, files stay; `bw-lab provision brain -e
  brain_enabled=false`, `bw-check brain` SKIPs while off.
- 2026-10-14: end-to-end verified in the lab — `bw-check brain` PASS, Claude
  (`claude-haiku-5-5`) classified the synthetic alert and the analysis arrived
  in Telegram. Claude's answer is now Markdown-escaped (categories like
  `privilege_escalation` broke Telegram's parser).
- 2026-10-14: ready for end-to-end in the lab.
  - `brain.py`: reads the `text` block (thinking blocks come first on
    `claude-haiku-5-5`), `output_config.effort: low`, `max_tokens: 2048`
    (thinking counts against it), logs `stop_reason` when no text came back.
  - Claude/Telegram failures are retried 3 times; the ID is saved only after
    both succeed, otherwise the drop is logged at ERROR.
  - Optional `claude_workspace_id` (`BW_CLAUDE_WORKSPACE_ID`) sends
    `anthropic-workspace-id` for keys not scoped to a workspace.
  - `verify-brain.yml` waits up to 90 s for the ID in the state file and fails
    when credentials are set but nothing was delivered.
  - `bw-lab provision brain`, `bw-check brain`; lab `analysis_threshold: 10`.
- 2026-05-10: bootstrap — `brain-analyst` role, `brain.yml`, `verify-brain.yml`.

## Plans

- Backlog: log rotation of `alerts.json` loses the unread tail (reopen at EOF); pre-existing.

- **Workload identity federation instead of a static API key.** Short-lived
  tokens exchanged from an OIDC JWT, no long-lived secret on the host. Worth it
  once Brain runs where an OIDC issuer comes for free (Kubernetes, AWS, GCP);
  on a single VPS we would have to run our own issuer. The switch likely means
  moving `brain.py` to the Anthropic Python SDK, which does the token exchange
  and refresh itself (today the module is stdlib-only by design).
- Prod secrets (`claude_api_key`, Telegram) are supplied from the owner's own secret management, not stored in the repo (no ansible-vault).
- The lab API key expires 30 days after 2026-10-14.
