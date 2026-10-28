---
module: blackwall-brain
status: in_progress
stage: 3
last_updated: 2026-10-28
last_task: "Brain v2 step 1: structured verdict, vendor-neutral models, HTML message, container, indexer polling"
next_task: "Brain v2 step 2: eval set and harness (spec 2026-10-28)"
blockers: []
---

## Done

- 2026-10-28: Brain v2 step 1 (spec `docs/superpowers/specs/2026-10-28-brain-v2-eval-design.md`,
  plan `docs/superpowers/plans/2026-10-28-brain-v2-step1.md`). Python package
  `src/brain/` in a container built on the host, run by `brain-analyst.service`
  (`podman run`); polls `wazuh-alerts-*` with a read-only indexer user instead
  of tailing `alerts.json`; structured verdict validated against a schema;
  `anthropic` and `openai_compat` backends; HTML Telegram message with severity,
  verdict (🟡 when uncertain), dashboard link and per-rule links. 91 unit tests.
  Verified in the lab with placeholder credentials: podman 3.4 on
  black-master-00, image build, read-only user, pickup of the synthetic alert
  in ~12 s, 3 model attempts → fallback → 3 Telegram attempts → "dropped"
  logged; restart re-sends nothing; `brain_enabled=false` → SKIP; Voice clamps
  to 9 and passes. **Not verified yet (needs real keys):** a real verdict
  message in Telegram and the dashboard link opening that one alert.

- 2026-10-28: no alerts lost around `alerts.json` rotation (#23): same tail
  loop as Voice (drain the old file, read the new one from its start, hold a
  half-written line, reread a shrunk file). Covered by
  `tests/unit/alerts_tail_test.py`.

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

- **Workload identity federation instead of a static API key.** Short-lived
  tokens exchanged from an OIDC JWT, no long-lived secret on the host. Worth it
  once Brain runs where an OIDC issuer comes for free (Kubernetes, AWS, GCP);
  on a single VPS we would have to run our own issuer. The switch likely means
  moving `brain.py` to the Anthropic Python SDK, which does the token exchange
  and refresh itself (today the module is stdlib-only by design).
- Prod secrets (`claude_api_key`, Telegram) are supplied from the owner's own secret management, not stored in the repo (no ansible-vault).
- The lab API key expires 30 days after 2026-10-14.
