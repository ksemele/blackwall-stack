# Brain eval set

An open, labelled set of Wazuh alerts and a harness that measures how well
Brain triages them: is it real or noise, how urgent, what kind. Every alert
comes from a scenario run in the Blackwall lab, not from hand-written JSON,
and most attacks have a benign twin that raises the same rule. Two do not:
web-user sudo (5405) and SSH brute force (5712) have twins on neighbouring
rules (5402, 5710), so those two can be called from the rule alone.

Design: [spec §8](../../docs/superpowers/specs/2026-10-28-brain-v2-eval-design.md).

## Layout

```
eval/
├── scenarios/   # *.yml: lab actions + expected alerts + labels (source of truth)
├── snapshots/   # captured alerts + context, one JSON per scenario (committed)
├── results/     # one JSON per run: model, prompt and set versions, scores
└── prices.json  # USD per million tokens, for the cost column
```

## Run it (no lab needed)

```bash
cd blackwall-brain
python3.12 -m venv .venv && .venv/bin/pip install -e '.[dev]'
BRAIN_MODEL_API_KEY=... .venv/bin/python -m brain.eval.run --max-cost 1
```

Options: `--backend anthropic|openai_compat`, `--model`, `--base-url`
(e.g. `http://127.0.0.1:11434/v1` for Ollama), `--options '{"effort": "medium"}'`,
`--threshold 10` (the "at threshold" row: alerts production Brain would see).
It calls a paid API: run it by hand, never in CI. `--max-cost` stops the run
when the estimate passes it (models missing from `prices.json` show cost n/a
and are not capped).

The report lists **missed threats first** (a real threat called noise), then
threats not called (`uncertain` on a real threat), then accuracy per field,
uncertain and invalid-output rates, the data the model asked for most
(`needs`), tokens, cost and latency. The JSON in `results/` has every case.

## Scoring

- `verdict` is the primary field. Exact match is correct; `uncertain` is
  never correct and is counted in its own column. An invalid answer counts as
  wrong in every field.
- Missed threat: expected `true_positive`, got `false_positive`.
  Expected `true_positive`, got `uncertain`: "threat not called", reported
  separately. In Telegram it is a yellow alert, so it is not silent.
- `severity` and `category` are reported; a `false_positive` is expected to
  be `low` with category `false_positive`.
- The labels are the truth of the scenario, even when the alert alone cannot
  tell: an FIM alert for `/etc/sudoers` looks the same whether an attacker or
  an admin edited it. Such labels carry `needs_context: true` (19 of 30), and
  the report shows accuracy on the rest ("decidable without context")
  separately: the headline number has a ceiling well below 100% until step 3
  (context sources) gives the model more than the alert.
- "At threshold" covers what production Brain sees (`rule.level` ≥ 10 in the
  lab). Only SSH brute force reaches it today (n=1), so read it as a gap in
  the set, not a score.

## Capture (needs the lab)

```bash
tests/bin/bw-lab up                                   # from the repo root
cd blackwall-brain
.venv/bin/python -m brain.eval.capture                # all scenarios
.venv/bin/python -m brain.eval.capture --only cron-backup
```

Each scenario runs its steps (`docker exec` into lab containers), waits until
every expected rule has fired (`wait_s`), snapshots those alerts, then runs
its cleanup and waits `--settle` seconds so cleanup's own alerts stay out of
the next scenario. A scenario whose expected alerts do not all appear fails
and writes nothing. Alerts with rules the scenario does not list are counted
as "unlabelled" and left out.

## Add a scenario

```yaml
name: cron-backup
description: A nightly backup job is installed in /etc/cron.d.
agent: black-host-01
wait_s: 90
steps:
  - {host: black-host-01, user: root, run: "echo '30 2 * * * root /usr/local/bin/backup.sh' > /etc/cron.d/nightly-backup"}
cleanup:
  - {host: black-host-01, run: "rm -f /etc/cron.d/nightly-backup"}
expected:
  - rule_id: "554"
    verdict: false_positive
    severity: low
    category: false_positive
    why: A nightly backup job is routine scheduled maintenance.
    match: {syscheck.path: /etc/cron.d/nightly-backup}
    needs_context: true
```

- `setup:` (optional) runs before the alert window opens, so its own alerts
  (creating a user, touching a file) are never labelled.
- `match:` (dotted alert path → substring) picks the alert the label belongs
  to. Same-rule noise in the window (another sudo, a scheduled FIM scan) stays
  unlabelled. The same rule may appear twice with different `match`.
- `needs_context: true` when a careful analyst could not decide from this
  alert alone.

Rules:
- Realistic names. Never test giveaways (`bw-trigger`, `synthetic`,
  `DoNotUseInProd`): the loader and the snapshot writer refuse them, because a
  model that sees them judges the string, not the event.
- Lab hosts and addresses only; "external" addresses from documentation
  ranges (`198.51.100.0/24`, `203.0.113.0/24`). The snapshot writer refuses
  any other address and host-machine paths (`/Users/…`, the host name).
- Add the benign twin together with the attack. Keep at least 40% of the
  labelled alerts `false_positive`.
- `why` says what a human analyst would conclude and why.
- Bump `EVAL_SET_VERSION` in `src/brain/eval/snapshots.py` when scenarios or
  labels change, then re-capture.

## Coverage and what is missing

26 scenarios, 30 labelled alerts, 47% benign. Covered: SSH (brute force,
a typo), accounts (UID 0 backdoor vs deploy user), sudo and su from the web
server account vs an admin's sudo and su typo, log wiping vs rotation,
`/etc/sudoers` and `/etc/sudoers.d`, `sshd_config` (root login, keys in /tmp
vs keep-alive, banner), `/etc/cron.d` (download-and-run, reboot reverse shell
vs backup, certbot), root's `authorized_keys`, password changes.

Lab facts the scenarios work around:
- FIM watches files in place: `useradd` replaces `/etc/passwd` by rename, so
  account scenarios rely on the auth log rules (5901/5902), not on 550.
- Rule 5403 ("first time user executed sudo") fires once per user per
  manager, so it cannot be reproduced; the admin sudo scenario expects 5402.
- Rule 592 (log size reduced) does not repeat for the same file within a
  short time, so log rotation (12) runs well before log wiping (35).
- The `ops` admin is created once and kept; scenarios that need it create it
  in `setup` when missing and set its password.
- No context switch in `run` yet: snapshots carry `context: {}` until step 3.

Not covered yet, because the lab does not raise an alert for it:
package installs (`dpkg.log` is not collected), process execution (auditd
events are not decoded into alerts), firewall rules 100201/100202 (need the
opt-in fw-events setup).

## License

MIT, like the rest of the repository.
