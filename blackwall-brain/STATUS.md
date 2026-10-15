---
module: blackwall-brain
status: in_progress
stage: 3
last_updated: 2026-10-15
last_task: "Lab deploy with a real key: text-block parsing for thinking models, low effort, real retries, optional workspace id header, strict verify-brain, bw-lab/bw-check brain targets"
next_task: "Noise: one message per alert when Brain is on; voice skips brain verify alerts (secrets are managed outside the repo)"
blockers: []
---

## Done

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

- **Workload identity federation instead of a static API key.** Short-lived
  tokens exchanged from an OIDC JWT, no long-lived secret on the host. Worth it
  once Brain runs where an OIDC issuer comes for free (Kubernetes, AWS, GCP);
  on a single VPS we would have to run our own issuer. The switch likely means
  moving `brain.py` to the Anthropic Python SDK, which does the token exchange
  and refresh itself (today the module is stdlib-only by design).
- Prod secrets (`claude_api_key`, Telegram) are supplied from the owner's own secret management, not stored in the repo (no ansible-vault).
- The lab API key expires 30 days after 2026-10-14.
