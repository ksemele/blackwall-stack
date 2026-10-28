# Brain v2: structured verdict, container, eval set, context — design

Date: 2026-10-28
Module: blackwall-brain
Task: C9 in `docs/community-tasks.md` (grew into Brain v2; C10 builds on it)
Status: draft, agreed in chat piece by piece, builder check done; awaiting the owner's approval

Marked **[call]**: decisions made by the author of this document, not by the
owner. They can be overridden. Everything else was decided by the owner.

## 1. Why

Brain's Telegram message today is a wall of text: a header, then Claude's
answer as one line, `CATEGORY: … | CONFIDENCE: … | ACTION: …`. The owner
wants to see at a glance the host, the severity, one line on what happened,
and a link to go further.

C9 asks for an open, labelled set of Wazuh alerts and a harness that
measures how well Brain triages them. Almost nobody publishes this for
LLM-on-Wazuh triage; it is useful to others and draws contributors
(C12 wave 6: r/blueteamsec).

Both need the same thing: Brain answering in **separate fields** instead of
free text. The fields drive the message, and the eval set scores the same
fields, so the harness measures exactly what lands in Telegram.

The system is still being designed, so this document also sets the shape
for later modes (section 9) without building them.

## 2. Goals

1. Brain returns a structured verdict (section 4) and the Telegram message
   is built from it (section 5), with a link to the alert in the Wazuh
   dashboard and optional per-rule links.
2. Brain runs in its own container and reads alerts from the Wazuh indexer
   and data from the Wazuh API over the network, like the TUI (section 6).
3. No lock-in to a model vendor or framework: Brain's code talks to its own
   model interface; adapters for the Anthropic API and for any
   OpenAI-compatible endpoint (Ollama, vLLM, llama.cpp server, LM Studio,
   OpenRouter, …) sit under it (section 7).
4. A reproducible eval set built from lab scenarios, and a harness that
   scores Brain per field, with cost and latency (section 8).
5. Mode 1 context: before the model call, Brain gathers context about the
   alert from pluggable context sources (section 9.1). The eval harness
   shows whether the context helps.

## 3. Non-goals and anti-goals

Not in this work (each is future, see section 10):

- Mode 2 (autonomous, read-only, tool use, several agents) and mode 3
  (autonomous defender). The design only must not block them.
- Push intake of alerts (Wazuh integration calling Brain).
- Grouping several triggering alerts into one message **[call]**.
- Notifiers other than Telegram.
- External context sources (logs of other systems, user inventory).

Anti-goals, a failure even if everything else works:

- Brain code (outside the adapters) imports a vendor SDK or depends on one
  vendor's request or response format.
- Brain gets write access anywhere: Wazuh API and indexer users are
  read-only.
- Real hosts, users, IPs or domains in the eval set. Lab names (`black-*`)
  and lab addresses only.
- An eval set of attacks only: it would hide how Brain handles noise. Benign
  scenarios that still raise alerts are required.
- An eval result published without the model, backend, prompt version, eval
  set version and date next to it.
- A message longer or harder to read than today's.

## 4. The verdict

One JSON object per analysed alert. Same schema for every backend and every
mode.

| Field | Type | Meaning | Scored in eval |
|---|---|---|---|
| `verdict` | `true_positive` / `false_positive` / `uncertain` | Is this real or noise | yes, primary |
| `severity` | `low` / `medium` / `high` / `critical` | How urgent, Brain's own judgement (not `rule.level`) | yes |
| `category` | today's list (`lateral_movement`, `privilege_escalation`, `persistence`, `credential_access`, `defense_evasion`, `discovery`, `collection`, `exfiltration`, `command_and_control`, `impact`, `false_positive`, `unknown`) | MITRE-style class | yes, reported only |
| `summary` | string, one line | What happened, in plain words | no |
| `action` | string, one sentence | What the operator should do | no |
| `confidence` | `high` / `medium` / `low` | Model's confidence | no (reported) |
| `needs` | list of strings, may be empty | Data the model lacked to decide | no (reported) |

- `needs` is the seed for mode 2: there it becomes a request to a central
  brain or an operator. In mode 1 it is shown in the message and counted in
  the eval.
- Brain validates the object against the schema. Invalid JSON or schema:
  one retry, then the fallback message (as today when the model fails).
  This matters for weak local models.
- The schema and the prompt carry a version (`prompt_version`), recorded in
  every eval result.
- `category: false_positive` overlaps with `verdict`. Whether to drop it
  from the category list is left to the builder; the eval treats
  `verdict` as the source of truth.

## 5. The Telegram message

Layout (illustration; exact emoji and wording left to the builder):

```
🔴 HIGH · black-host-01 · true positive
New user added to sudoers outside config management
Category: privilege_escalation · confidence: medium
Action: check who edited /etc/sudoers.d/bw-trigger and revert it if unknown
Missing: who normally administers this host
Rule 100xxx (level 10): <rule description> · 2026-10-28 14:02 UTC
🔗 Alert in dashboard · Runbook
```

- First line: severity, host, verdict. Second line: `summary`.
- `verdict: uncertain` is a **yellow alert** (owner, 2026-10-28): the
  first line is visibly marked (e.g. 🟡 and "needs a look") whatever the
  severity, so an unsure verdict never reads as calm.
- `Missing:` only when `needs` is non-empty.
- **Alert link** **[call: variable names]**: `brain_dashboard_url` (base
  URL of the Wazuh dashboard). The link opens Discover on
  `wazuh-alerts-*` filtered to this alert's `_id`, with a time range around
  its timestamp. The builder finds the exact URL format and verifies the
  link in the lab. Empty `brain_dashboard_url`: no link, nothing breaks.
- **Per-rule links**: `brain_rule_links`, a map from rule ID to a list of
  `{title, url}`. Any rule with an entry gets those links in its message.
  This is how runbooks, wiki pages or tickets get attached later; none exist
  today. The builder may add a `"*"` key for links on every message.
- Fallback (model failed): same layout, first line from `rule.level`
  mapped to a severity, plus `AI analysis unavailable: <reason>` and the
  log excerpt, as today.
- **[call]** Switch Telegram `parse_mode` from legacy `Markdown` to
  `HTML`: escaping only `<`, `>`, `&` avoids the class of bugs fixed on
  2026-10-14 (`privilege_escalation` broke the Markdown parser).

## 6. Runtime: container, intake, state

- Brain is a Python package built into its own container image. Source
  moves from the Jinja template (`roles/brain-analyst/templates/brain.py.j2`)
  to a normal package, `blackwall-brain/src/brain/` **[call]**, with unit
  tests; the role copies it and renders only config. The eval harness
  imports the same package. Any pip
  dependencies are allowed, pinned with hashes **[call]**.
- It talks to the network only: Wazuh indexer (alerts, other events),
  Wazuh API (agents and other manager data), the model endpoint, Telegram.
  It no longer reads `/var/ossec/logs/alerts/alerts.json`. The tail and
  rotation code (#23, 2026-10-28) goes away with the old service.
- **Intake: polling the indexer.** Every `poll_interval` seconds, query
  `wazuh-alerts-*` for alerts with `rule.level >= analysis_threshold`,
  newer than a cursor, sorted by time, skipping `brain_ignore_rule_ids`.
  The TUI's `:alerts` feed does the same kind of query; reuse its approach.
- State in a mounted volume: the cursor (last processed timestamp) and a
  bounded set of processed `_id`s for alerts that share a timestamp or
  arrive late (the indexer can make an alert visible a few seconds after its
  timestamp; the query should overlap the cursor by a short margin and
  dedup by `_id`) **[call]**.
- First start or long downtime: process at most `brain_max_backfill`
  (e.g. 1 hour) of history, not the whole index **[call]**.
- Credentials: a read-only Wazuh API user and an indexer user that can only
  read `wazuh-alerts-*` (plus whatever the context sources need). The lab
  creates them; for production the README documents how. Which role creates
  them in the lab is left to the builder.
- Deployment: the `brain-analyst` Ansible role builds or pulls the image and
  runs it under systemd **[call: a plain systemd unit running `podman run`;
  podman is already used in the stack. Not a quadlet: Ubuntu 22.04, the lab
  manager's OS, ships podman 3.4, and quadlets need 4.4+]**. It removes the old systemd Python service
  if present. `brain_enabled: false` keeps working (stop and disable,
  files stay). Production host: any host that reaches the API and the
  indexer, by default the manager host. Which lab node runs it, and whether
  nested podman works there, is left to the builder (nested podman is
  proven on the Rocky node `black-sentinel-03`).
- Image: built on the target from the role's files for now **[call]**;
  publishing to a registry is future.
- Voice coupling: `voice_max_level` in `tests/inventory/hosts.yml` and the
  Voice role check whether the `brain-analyst` service runs. Keep that
  working with the new unit.
- `bw-lab provision brain` and `bw-check brain` keep working
  (`verify-brain.yml` adapted to the new intake).

## 7. Model interface

- Brain's code calls one interface, roughly: send messages (+ optional JSON
  schema, + optional tools for mode 2), get back text or parsed JSON,
  token usage (input, output) and latency.
- Adapters:
  - `anthropic`: official Anthropic SDK. Brings workload identity
    federation (Plans in `STATUS.md`) for free later.
  - `openai_compat`: official OpenAI SDK with a configurable `base_url`.
    Covers local models (C10) and most hosted providers.
- Selected by config: backend, model, base URL, API key, timeout, max
  tokens, and backend-specific options (e.g. `effort` for Claude) passed
  through. Defaults stay Claude (`claude-haiku-5-5`, `effort: low`).
- Structured output: use the backend's native JSON or schema mode when it
  has one, otherwise prompt for JSON; always validate (section 4).
- Cost: a price table in config (per model, per million tokens in and out)
  used by the eval harness; unknown model means cost "n/a", not an error.
- The adapters are the only place that knows a vendor's format.

## 8. Eval set and harness

### 8.1 Scenarios (the set)

- `blackwall-brain/eval/scenarios/<name>.yml` **[call: layout]**. A
  scenario is an action in the lab plus the expected result:
  - what to run: commands on a lab host, written for the eval (see "No
    test giveaways" below); `bw-trigger` events serve as the list of what
    to cover, not as the commands;
  - which alerts it should raise (rule IDs, agent);
  - expected labels per alert: `verdict`, `severity`, `category`;
  - a short note why (what a human analyst would conclude).
- Alerts come from the lab only: run the scenario, let Wazuh raise the
  alerts. No hand-written alert JSON.
- Size: 30–50 labelled alerts to start, covering the Eye rules the stack
  ships and `bw-trigger`'s 17 events.
- **[call]** The set includes alerts below `analysis_threshold`: Eye rules
  and triggers often fire below the lab's 10, and the set needs the
  coverage. The harness reports the at-or-above-threshold subset (what
  production Brain sees) as its own row next to the full set.
- **No test giveaways** (owner, 2026-10-28): the eval scenarios use their
  own commands with realistic names (a `deploy` user, `/etc/sudoers.d/90-app`,
  an ordinary key comment), never `bw-trigger`'s marked names
  (`bw-trigger-user`, `/etc/sudoers.d/bw-trigger`, `/tmp/.bw-trigger.sh`,
  `DoNotUseInProd`). `bw-trigger` itself stays as it is: other modules'
  checks use it. The builder adds a check that fails capture when a
  snapshot contains a known marker string.
- **Benign scenarios are required** (anti-goal in section 3): routine admin
  work that still raises alerts, e.g. a package update by the admin, a user
  added the way config management does it, a known cron job. Their truth is
  `false_positive` even when the alert alone cannot tell; a low score there
  without context is an honest result, and the mode-1 context should raise
  it.
- The set is reproducible: scenarios and labels are the source of truth;
  anyone with the lab can regenerate the alerts.

### 8.2 Snapshots

- **[call]** After a lab run, the captured alerts **and the context bundle
  each alert got** are saved to `blackwall-brain/eval/snapshots/` and
  committed. Lab data only, so nothing to sanitize beyond checking that no
  host-machine data leaked in (the builder adds a check).
- Snapshots let anyone compare models, backends or prompts without Docker
  and without the lab (cheap for C10 and outside contributors). Context is
  part of the snapshot so offline runs see exactly what the model saw.
- Snapshots carry the eval set version; regenerating them is a deliberate
  step, not a side effect of a run.

### 8.3 Harness

- Scoring rules **[call]**:
  - `verdict`: exact match counts as correct; `uncertain` is never correct
    but is reported as its own column, not mixed with wrong answers.
  - Missed threat: expected `true_positive`, got `false_positive`.
    Expected `true_positive`, got `uncertain` is not a miss: it is counted
    separately as "threat not called" (owner, 2026-10-28). It stays
    acceptable only because the message flags it as a yellow alert
    (section 5).
  - Expected `severity` for a `false_positive` is `low`.
- Two steps **[call: names]**:
  - `capture`: needs the lab up; runs scenarios, waits for the expected
    alerts in the indexer (rule ID + agent + time after the trigger, with a
    timeout), builds the context, writes snapshots. A scenario whose
    expected alert never appears is a failure of the scenario, reported, not
    silently skipped.
  - `run`: offline, over snapshots; calls Brain's real analysis code (same
    prompt, same model interface, same validation as production) with the
    chosen backend and model; context on or off.
- Output: per field accuracy; for `verdict` a confusion table, with
  **missed threats** (expected `true_positive`, got `false_positive`)
  reported first and separately; `uncertain` rate; invalid-output rate;
  `needs` frequency; tokens, cost, latency p50 and p95. Printed as a table
  and written as JSON with the model, backend, prompt version, set version
  and date.
- Runs by hand, not in CI (it calls paid APIs). A `--max-cost` guard
  stops a run when the estimated spend passes it **[call]**; with Haiku
  a 50-alert run costs cents, so no budget question for the first runs. Whether it also gets a
  `bw-check` or Makefile entry is left to the builder.
- `eval/README.md`: scenario format, how to add one, how to run, license
  (MIT, same as the repo **[call]**).
- Results of the first runs go into `blackwall-brain/STATUS.md`.

## 9. Modes

### 9.1 Mode 1: context ahead of the call (built now)

- The unit of analysis is one triggering alert plus a context bundle
  **[call]**. "A batch of events" is delivered as context (related events
  around the alert), not as several alerts in one message.
- Context sources share one small interface: given an alert, return a named
  block of facts, within a time and size budget. A failing source is logged
  and skipped; analysis goes on without it. New sources plug in by config
  plus one module. The same sources become mode 2's tools later.
- First sources **[call: exact set and limits left to the builder]**:
  - agent: from the Wazuh API (name, IP, OS, groups, status, last
    keep-alive);
  - host events: other events from the same agent before the alert time
    (a window back from it), any level, capped. Never events after the alert:
    live Brain cannot see them, and offline snapshots must not look better
    than production **[call]**;
  - rule history: how often this rule fired, on which hosts, recently;
  - static facts: free-form facts about the environment from config.
- Brain sees only host names and IPs and what Wazuh holds, plus the static
  facts the owner puts in config. Nothing else.
- Context is on by default once it shows in the eval that it helps; the
  harness runs with and without it.

### 9.2 Not built now, the design must allow

- **Mode 2: autonomous, read-only.** The model gets tools (the context
  sources, with queries) and decides what to look at, under hard limits:
  allowed sources and queries, steps, tokens, wall time. On a limit it
  returns the verdict so far, marked. Can be several agents, one per
  subsystem or team, that ask a central brain or an operator of the same
  installation for more data or authority (via `needs`).
- **Mode 3: autonomous defender with wider rights.** Defensive actions only
  (blocking, isolation through Fang), with human approval. No hack-back
  (see C6).

What this work does for them: the verdict schema with `needs`, the model
interface with optional tools, context sources as separate parts.

## 10. Future (noted, not planned)

- Push intake: a Wazuh integration posts each alert to Brain; faster than
  polling, needs manager config and an inbound port on Brain.
- On-demand research from the TUI (C14): a TUI plugin sends a host, an
  alert or a batch of alerts to Brain and shows the verdict. Shares the
  inbound API with push intake; the verdict schema (section 4) is the
  answer format.
- Several triggering alerts in one message (incident view).
- External context sources: logs of other systems, user inventory.
- Workload identity federation instead of a static API key (Anthropic SDK).
- Publishing the image to a registry.
- C10: run the harness on one or two local models via `openai_compat` and
  write the comparison into `STATUS.md` and the README.

## 11. Order of work

1. Verdict, model interface, message, container, intake (sections 4–7).
   Brain v2 replaces v1 in the lab; `bw-check brain` passes.
2. Eval set, snapshots, harness (section 8); first results without context.
3. Mode-1 context sources (section 9.1); eval with and without context.

Each step is its own plan and PR, planned one at a time: plan step 1
first; steps 2–3 are planned after step 1 lands. The lab Claude key expires around
2026-11-13; if steps 1–2 are not done by then, a new key is needed for
the Claude runs.
