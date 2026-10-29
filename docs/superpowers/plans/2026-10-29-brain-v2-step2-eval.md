# Brain v2 step 2: eval set and harness — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (owner's choice: inline) to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A reproducible, labelled set of lab scenarios and a harness that scores Brain's verdict per field, with cost and latency, against any model backend.

**Architecture:** Scenarios (YAML: lab actions + expected alerts + labels) live in `blackwall-brain/eval/scenarios/`. `brain.eval.capture` runs them in the Docker lab, waits for the alerts in the indexer and writes snapshots (alert + context bundle) to `blackwall-brain/eval/snapshots/`. `brain.eval.run` replays the snapshots through Brain's real `analysis.analyze`, scores and writes a JSON result. Offline runs need no lab.

**Tech Stack:** Python 3.12, PyYAML (dev/eval extra only), httpx, pytest, Docker lab (`docker exec`).

**Spec:** `docs/superpowers/specs/2026-10-28-brain-v2-eval-design.md` §8 (and §4 for the fields).

## Global Constraints

- Scenarios use realistic names. Never `bw-trigger`'s marked strings: `bw-trigger`, `bw-trigger-user`, `/etc/sudoers.d/bw-trigger`, `/tmp/.bw-trigger.sh`, `DoNotUseInProd`, `synthetic`. Capture fails on a snapshot that contains one (case-insensitive).
- Lab data only: hosts `black-*`, lab range `172.28.0.0/24`, documentation ranges for "external" addresses (`198.51.100.0/24`, `203.0.113.0/24`). No real hosts, IPs, domains.
- Scored fields: `verdict` (primary), `severity`, `category`. Expected `severity` for `false_positive` is `low`.
- `verdict` scoring: exact match is correct; `uncertain` is never correct and gets its own column. Missed threat = expected `true_positive`, got `false_positive` (reported first). Expected `true_positive`, got `uncertain` = "threat not called", counted separately.
- Benign scenarios are required; target ≥ 40% of labelled alerts are `false_positive`.
- Results record model, backend, `PROMPT_VERSION`, eval set version, date.
- The harness calls Brain's production code (`brain.analysis.analyze`, `VERDICT_SCHEMA`, the model adapters), never a copy.
- Runs are manual, never in CI. `--max-cost` (USD) stops a run when the estimate passes it.
- Eval set license: MIT (repo license).

## Review Focus

1. A scenario's expected alert never appears (FIM not watching that path, agent lag) → capture reports that scenario as FAILED with the missing rule IDs and exits non-zero; it never writes a partial snapshot silently (Task 3).
2. Background lab noise in the scenario's time window (PAM sessions, `100100`) → only alerts whose rule ID the scenario lists are snapshotted; others are counted as "unlabelled" in the capture log (Task 3).
3. Two scenarios raising the same rule ID back to back → each scenario's window starts after the previous one ends; no alert is attributed to two scenarios (Task 3).
4. Model returns invalid output or fails on some alerts → those count as `invalid` per field, not crash the run, and appear in the invalid-output rate (Task 4).
5. Unknown model in the price table → cost "n/a", run continues; `--max-cost` then cannot be enforced and says so at start (Task 4).

---

### Task 0: Detection coverage the scenarios need (Eye, needs the owner's yes)

Lab recon 2026-10-29: `bw-trigger all` raised only 5901, 5902, 592, 550, 5403, 5501, 5903, 5502, 5555, 100100, 5715 (all < level 10). FIM realtime covers only `/etc/passwd`, `/etc/shadow`, `/etc/sudoers`, `/etc/ssh/sshd_config`.

- [ ] Ask the owner whether to extend Eye's FIM to `/etc/sudoers.d`, `/etc/cron.d`, `/root/.ssh` (realtime) — a detection change in `blackwall-eye/ansible/roles/wazuh-agent/templates/ossec.conf.j2`, own commit `feat(eye): watch sudoers.d, cron.d and root's ssh keys`. Without it, sudoers.d/cron/authorized_keys scenarios are dropped from the set.

### Task 1: Scenario format and loader

**Files:**
- Create: `blackwall-brain/src/brain/eval/__init__.py`, `blackwall-brain/src/brain/eval/scenarios.py`
- Modify: `blackwall-brain/pyproject.toml` (extra `eval = ["pyyaml"]`; `dev` includes it)
- Test: `blackwall-brain/tests/eval/test_scenarios.py`

**Interfaces:**
- Produces:
  - `@dataclass Step: host: str; user: str; run: str` — `host` is a lab container name, `user` the Unix user inside it (default `root`).
  - `@dataclass Expected: rule_id: str; verdict: str; severity: str; category: str; why: str; min_count: int = 1`
  - `@dataclass Scenario: name: str; description: str; agent: str; steps: list[Step]; cleanup: list[Step]; expected: list[Expected]; wait_s: int = 90`
  - `load_scenarios(directory: str) -> list[Scenario]` — sorted by file name; raises `ValueError(path: reason)` on: unknown keys, enum values outside `VERDICTS`/`SEVERITIES`/`CATEGORIES`, `false_positive` with severity ≠ `low`, duplicate `rule_id` in one scenario, a marker string anywhere in the file.
  - `MARKERS = ("bw-trigger", "donotuseinprod", "synthetic")`

YAML shape (exact keys):

```yaml
name: sudoers-dropin-by-webuser
description: A web shell user grants itself passwordless sudo.
agent: black-host-01
wait_s: 90
steps:
  - {host: black-host-01, user: www-data, run: "…"}
cleanup:
  - {host: black-host-01, run: "rm -f /etc/sudoers.d/90-app"}
expected:
  - rule_id: "550"
    verdict: true_positive
    severity: high
    category: privilege_escalation
    why: www-data has no business editing sudoers.
```

- [ ] **Step 1:** Tests: `test_loads_example`, `test_rejects_unknown_key`, `test_rejects_bad_enum`, `test_false_positive_must_be_low`, `test_rejects_duplicate_rule`, `test_rejects_marker_string` (each `ValueError` names the file and the problem), `test_sorted_by_file_name`.
- [ ] **Step 2:** `.venv/bin/pip install -e '.[dev]' && .venv/bin/pytest tests/eval -q` → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** → PASS; full suite green.
- [ ] **Step 5:** `git commit -m "feat(brain): eval scenario format and loader"`

### Task 2: Snapshot format

**Files:**
- Create: `blackwall-brain/src/brain/eval/snapshots.py`
- Test: `blackwall-brain/tests/eval/test_snapshots.py`

**Interfaces:**
- Produces:
  - `EVAL_SET_VERSION = "2026-10-29.1"` (bump when scenarios or labels change)
  - `@dataclass Case: scenario: str; doc_id: str; alert: dict; context: dict; expected: Expected`
  - `write_snapshot(directory: str, scenario: Scenario, cases: list[Case]) -> str` — one file per scenario, `<scenario.name>.json`: `{"eval_set_version", "scenario", "captured_at", "cases": [{doc_id, alert, context, expected}]}`, keys sorted, 2-space indent. Raises `ValueError` if any marker (case-insensitive) appears in the serialized cases.
  - `load_cases(directory: str) -> list[Case]` — all snapshot files, sorted; raises if a file's `eval_set_version` differs from `EVAL_SET_VERSION`.
- `context` is `{}` in this step (context sources are step 3); the field exists so step 3 snapshots are drop-in.

- [ ] **Step 1:** Tests: `test_roundtrip`, `test_marker_in_alert_rejected` (marker inside `full_log`), `test_version_mismatch_rejected`, `test_stable_output` (same input → byte-identical file).
- [ ] **Step 2–4:** RED → implement → GREEN.
- [ ] **Step 5:** `git commit -m "feat(brain): eval snapshot format"`

### Task 3: Capture in the lab

**Files:**
- Create: `blackwall-brain/src/brain/eval/capture.py`
- Test: `blackwall-brain/tests/eval/test_capture.py`

**Interfaces:**
- Consumes: `IndexerClient` (`brain.indexer`), `Scenario` (Task 1), `write_snapshot`/`Case` (Task 2).
- Produces:
  - `docker_exec(host: str, user: str, command: str) -> subprocess.CompletedProcess` — `docker exec -u <user> <host> bash -lc <command>`, check=False, output captured.
  - `alerts_query(agent: str, since: str, until: str | None, rule_ids: list[str]) -> dict` — `agent.name` term, `timestamp` range, `rule.id` terms, sort asc, size 500.
  - `@dataclass ScenarioResult: name: str; ok: bool; cases: list[Case]; missing: list[str]; unlabelled: int; error: str = ""`
  - `capture_scenario(s: Scenario, indexer, *, run=docker_exec, now=…, sleep=time.sleep, poll_s=5) -> ScenarioResult`
    - `since` = now before the first step; run steps in order (non-zero exit of a step → `ok=False`, `error` set, cleanup still runs); poll the indexer every `poll_s` until each expected `rule_id` has ≥ `min_count` alerts or `wait_s` passes; then a final query for all rule IDs in the window to count unlabelled; always run cleanup.
    - Every alert with an expected `rule_id` becomes a `Case` (labels from that `Expected`).
  - `main(argv)` — `python -m brain.eval.capture [--only NAME…] [--scenarios DIR] [--out DIR]`; indexer from `BRAIN_EVAL_INDEXER_URL` (default `https://127.0.0.1:9200`), `_USER`/`_PASSWORD` (default lab admin), TLS verify off by default; writes a snapshot only for `ok` scenarios; prints one line per scenario (`OK n cases`, `FAILED missing [..]`, `unlabelled k`); exit 1 if any failed. Scenarios run strictly one after another (Review Focus 3).

- [ ] **Step 1:** Tests with a fake `run` and fake indexer: `test_runs_steps_then_cleanup`, `test_cleanup_runs_after_failed_step`, `test_waits_until_expected_rules_seen` (alerts appear on the 3rd poll), `test_missing_rule_fails_scenario` (Review Focus 1), `test_unlabelled_alerts_counted_not_cased` (Review Focus 2), `test_main_exit_1_and_no_snapshot_on_failure`.
- [ ] **Step 2–4:** RED → implement → GREEN.
- [ ] **Step 5:** `git commit -m "feat(brain): capture eval scenarios from the lab"`

### Task 4: Scoring and the run command

**Files:**
- Create: `blackwall-brain/src/brain/eval/score.py`, `blackwall-brain/src/brain/eval/run.py`, `blackwall-brain/eval/prices.json`
- Test: `blackwall-brain/tests/eval/test_score.py`, `blackwall-brain/tests/eval/test_run.py`

**Interfaces:**
- Consumes: `analyze`, `AnalysisError`, `PROMPT_VERSION` (`brain.analysis`); `build_model`, `ModelSettings`; `load_cases`, `EVAL_SET_VERSION`.
- Produces:
  - `@dataclass Outcome: case: Case; verdict: Verdict | None; error: str; input_tokens: int; output_tokens: int; latency_s: float`
  - `score(outcomes: list[Outcome], threshold: int) -> dict` with keys: `n`, `verdict` (`accuracy`, `confusion` {expected → {got → count}} incl. `uncertain` and `invalid`), `missed_threats` (list of `scenario/doc_id`), `threats_not_called` (count), `uncertain_rate`, `invalid_rate`, `severity` and `category` (`accuracy`, invalid counts as wrong), `needs` (frequency of each string, top 10), `tokens` (in/out totals), `latency` (`p50`, `p95`), `subset_at_threshold` (same metrics for `rule.level >= threshold`, `n` may be 0).
  - `estimate_cost(model: str, tokens_in: int, tokens_out: int, prices: dict) -> float | None` — `prices.json`: `{"<model>": {"input_per_mtok": x, "output_per_mtok": y}}`; seed with `claude-haiku-5-5` 0.10/0.50, `claude-sonnet-5-5` 2/10, `claude-opus-5-5` 4/20.
  - `run.main(argv)` — `python -m brain.eval.run [--snapshots DIR] [--backend anthropic|openai_compat] [--model M] [--base-url U] [--options JSON] [--threshold 10] [--max-cost USD] [--out DIR]`; API key from `BRAIN_MODEL_API_KEY` (falls back to `BW_CLAUDE_API_KEY`); prints the report table (missed threats first) and writes `eval/results/<YYYY-MM-DD>-<backend>-<model>.json` with `{model, backend, prompt_version, eval_set_version, date, options, score, per_case: [{scenario, doc_id, expected, got, error}]}`.
  - `AnalysisError` on a case → `Outcome` with `verdict=None`, `error` set (Review Focus 4). Cost unknown → warn once that `--max-cost` is not enforced (Review Focus 5). Estimated spend over `--max-cost` → stop, write a partial result marked `"stopped": "max_cost"`.

- [ ] **Step 1:** Tests: `test_verdict_accuracy_and_confusion`, `test_missed_threat_listed`, `test_uncertain_on_threat_is_not_called_not_missed`, `test_invalid_counts_wrong_everywhere`, `test_subset_at_threshold`, `test_latency_percentiles`, `test_needs_frequency`, `test_cost_known_and_unknown`, `test_run_writes_result_with_versions` (fake model via monkeypatched `build_model`), `test_run_survives_analysis_error`, `test_max_cost_stops_and_marks_partial`.
- [ ] **Step 2–4:** RED → implement → GREEN.
- [ ] **Step 5:** `git commit -m "feat(brain): eval scoring and run command"`

### Task 5: The scenarios

**Files:**
- Create: `blackwall-brain/eval/scenarios/*.yml` (target 16–24 files → 30–50 labelled alerts), `blackwall-brain/eval/README.md`

Lab-driven, not TDD: write a scenario, `python -m brain.eval.capture --only <name>`, adjust until OK. Each scenario's `why` says what a human analyst would conclude. Coverage plan (adjust to what the lab actually raises; record drops in the README):

| Theme | True positive | Benign twin |
|---|---|---|
| New account (5902/5901) | `svc-backup2` created by `www-data` via sudo misuse, added to `sudo` | `deploy` user created by root during provisioning (config-management style home, no sudo) |
| Passwords (5555) | root password changed from a web user's shell | admin rotates own password |
| sudo (5403/5402) | `www-data` runs `sudo cat /etc/shadow` | admin `sudo apt-get update` |
| Log tampering (592) | `/var/log/auth.log` truncated with `: >` by a user shell | logrotate-style rotation (`copytruncate`) |
| sudoers (550) | `/etc/sudoers` edited to add `NOPASSWD: ALL` for `www-data` | `visudo`-style change adding an ops group (documented) |
| sshd_config (550) | `PermitRootLogin yes` + `PasswordAuthentication yes` | `ClientAliveInterval` tuned |
| SSH brute force (5710/5712/5763) | many failed logins from `203.0.113.7` style source (lab container) | two typos then success by a known admin |
| Packages (2902/2904) | `nmap`/`netcat` installed by a non-admin | `apt-get upgrade` of a routine package |
| Firewall (100202/100201, if fw-events is provisioned) | port scan burst from one source | — (monitoring host listed as allowed; skip if not reproducible) |
| Task 0 dependent | cron job `curl … \| sh`, `authorized_keys` added for root | cron job for backups, key added by config management |

- [ ] **Step 1:** Write scenarios; capture each until OK.
- [ ] **Step 2:** `python -m brain.eval.capture` (all) → every scenario OK; snapshots written; count labelled alerts (30–50) and the `false_positive` share (≥ 40%).
- [ ] **Step 3:** `eval/README.md`: what the set is, scenario format, how to add one, capture and run commands, scoring rules (from Global Constraints), dropped themes and why, license MIT, `EVAL_SET_VERSION` bump rule.
- [ ] **Step 4:** `git commit -m "feat(brain): eval scenarios and snapshots"`

### Task 6: First results and docs

**Files:**
- Create: `blackwall-brain/eval/results/<date>-anthropic-claude-haiku-5-5.json`
- Modify: `blackwall-brain/STATUS.md`, `blackwall-brain/README.md` (Eval section), `blackwall-brain/CLAUDE.md` (layout), `CONTRIBUTING.md` (eval commands), `docs/community-tasks.md` (C9 status)

- [ ] **Step 1:** `direnv exec . blackwall-brain/.venv/bin/python -m brain.eval.run --max-cost 1` (default Haiku 5.5, effort low). Expected: report printed, result JSON written.
- [ ] **Step 2:** Same with `--options '{"effort": "medium"}'` for one comparison point.
- [ ] **Step 3:** STATUS.md: results table (verdict accuracy, missed threats, threats not called, uncertain, severity/category accuracy, cost, p50/p95) for both runs, with model, prompt and set versions. C9 box ticked with the date.
- [ ] **Step 4:** `git commit -m "docs(brain): first eval results"`
