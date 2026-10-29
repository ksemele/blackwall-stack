---
module: blackwall-brain
status: in_progress
stage: 3
last_updated: 2026-10-29
last_task: "Brain v2 step 2: eval set (26 scenarios, 30 alerts) and harness; first results"
next_task: "Brain v2 step 3: mode-1 context sources, measured with the eval set (spec 2026-10-28 §9.1)"
blockers: []
---

## Done

- 2026-10-29: Brain v2 step 2, eval set and harness (`eval/`, plan
  `docs/superpowers/plans/2026-10-29-brain-v2-step2-eval.md`). 26 lab
  scenarios (attack + benign twin), 30 labelled alerts, 47% benign; snapshots
  committed; `python -m brain.eval.capture` / `python -m brain.eval.run`.
  Baseline (prompt 2026-10-28.1, set 2026-10-29.2, no context):

  | Model, effort | Verdict (n=30) | Decidable w/o context (n=11) | At threshold (n=1) | Missed threats | Threats not called | Uncertain | Severity | Category | Cost | p50 / p95 |
  |---|---|---|---|---|---|---|---|---|---|---|
  | claude-haiku-5-5, low | 20% | 45% | 0% | 0 | 11 / 16 | 77% | 27% | 50% | $0.0065 | 2.2 / 3.0 s |
  | claude-haiku-5-5, medium | 17% | 45% | 0% | 0 | 12 / 16 | 80% | 33% | 47% | $0.0083 | 2.7 / 4.2 s |

  Reading:
  - 19 of 30 alerts cannot be decided from the alert alone (`needs_context`),
    so the headline verdict has a low ceiling by design. Even on the 11
    decidable ones Brain is right on 45% and answers `uncertain` on the rest.
  - "No missed threats" is not a strength yet: Brain rarely commits at all
    (77–80% uncertain). What it asks for most: who or what changed the file,
    the contents or a diff, change records.
  - The at-threshold row (what production Brain sees, level ≥ 10) has n=1
    (SSH brute force, answered uncertain): the set barely covers it.
  - Effort barely matters: the gap is data, not reasoning. Step 3 (context)
    has to move these numbers; the set should also grow at level ≥ 10.

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
  to 9 and passes. 2026-10-29 with real keys: a real verdict arrived in
  Telegram (Haiku 5.5 accepts effort + JSON schema). The dashboard link is a
  placeholder for now: its URL shape is not verified against the dashboard.
- 2026-10-29: the model's answer is one `<pre>` block (Telegram copy button).

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
