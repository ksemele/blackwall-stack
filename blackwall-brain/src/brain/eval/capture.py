"""Run eval scenarios in the Docker lab and snapshot the alerts they raise.

    python -m brain.eval.capture [--only NAME ...] [--scenarios DIR] [--out DIR]

Needs the lab up (tests/bin/bw-lab up). Scenarios run strictly one after
another; a scenario whose expected alerts do not all appear fails and gets no
snapshot.
"""
import argparse
import os
import subprocess
import sys
import time
from dataclasses import dataclass, field
from datetime import datetime, timezone

from brain.eval.scenarios import Scenario, load_scenarios
from brain.eval.snapshots import Case, write_snapshot
from brain.indexer import IndexerClient, IndexerError

EVAL_DIR = os.path.normpath(os.path.join(os.path.dirname(__file__), "..", "..", "..", "eval"))


def docker_exec(host: str, user: str, command: str) -> subprocess.CompletedProcess:
    return subprocess.run(["docker", "exec", "-u", user, host, "bash", "-lc", command],
                          capture_output=True, text=True, check=False)


def alerts_query(agent: str, since: str, rule_ids: list[str]) -> dict:
    filters: list = [{"term": {"agent.name": agent}},
                     {"range": {"timestamp": {"gte": since}}}]
    if rule_ids:
        filters.append({"terms": {"rule.id": list(rule_ids)}})
    return {"size": 500, "sort": [{"timestamp": {"order": "asc"}}],
            "query": {"bool": {"filter": filters}}}


@dataclass
class ScenarioResult:
    name: str
    ok: bool
    cases: list[Case] = field(default_factory=list)
    missing: list[str] = field(default_factory=list)
    unlabelled: int = 0
    error: str = ""


def _iso(dt: datetime) -> str:
    return dt.strftime("%Y-%m-%dT%H:%M:%S.") + f"{dt.microsecond // 1000:03d}Z"


def capture_scenario(s: Scenario, indexer, *, run=docker_exec,
                     now=lambda: datetime.now(timezone.utc), clock=time.monotonic,
                     sleep=time.sleep, poll_s: float = 5, settle_s: float = 20) -> ScenarioResult:
    try:
        for step in s.setup:
            res = run(step.host, step.user, step.run)
            if res.returncode != 0:
                return ScenarioResult(s.name, False, error=f"setup failed ({res.returncode}): "
                                      f"{step.run[:80]}: {res.stderr[-200:]}")
        if s.setup:
            sleep(settle_s)   # setup's own alerts land before the window opens
        since = _iso(now())
        for step in s.steps:
            res = run(step.host, step.user, step.run)
            if res.returncode != 0:
                return ScenarioResult(s.name, False, error=f"step failed ({res.returncode}): "
                                      f"{step.run[:80]}: {res.stderr[-200:]}")
        return _collect(s, indexer, since, clock=clock, sleep=sleep, poll_s=poll_s)
    finally:
        # Cleanup only after the alerts are in: FIM can miss a file that is
        # created and removed within seconds. Then let cleanup's own alerts
        # land before the next scenario's window opens.
        for step in s.cleanup:
            run(step.host, step.user, step.run)
        sleep(settle_s)


def _assign(s: Scenario, hits: list) -> tuple[list[Case], int]:
    cases, unlabelled = [], 0
    for doc_id, alert in hits:
        e = next((e for e in s.expected if e.matches(alert)), None)
        if e is None:
            unlabelled += 1
        else:
            cases.append(Case(s.name, doc_id, alert, {}, e))
    return cases, unlabelled


def _missing(s: Scenario, cases: list[Case]) -> list[str]:
    return [e.rule_id for e in s.expected
            if sum(1 for c in cases if c.expected is e) < e.min_count]


def _collect(s: Scenario, indexer, since: str, *, clock, sleep, poll_s) -> ScenarioResult:
    rule_ids = sorted({e.rule_id for e in s.expected})
    deadline = clock() + s.wait_s
    while True:
        cases, _ = _assign(s, indexer.search(alerts_query(s.agent, since, rule_ids)))
        missing = _missing(s, cases)
        if not missing or clock() >= deadline:
            break
        sleep(poll_s)
    # Wanted rules and everything else separately, so noise cannot crowd them out.
    cases, _ = _assign(s, indexer.search(alerts_query(s.agent, since, rule_ids)))
    _, unlabelled = _assign(s, indexer.search(alerts_query(s.agent, since, [])))
    return ScenarioResult(s.name, not _missing(s, cases), cases, _missing(s, cases), unlabelled)


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(prog="python -m brain.eval.capture")
    p.add_argument("--scenarios", default=os.path.join(EVAL_DIR, "scenarios"))
    p.add_argument("--out", default=os.path.join(EVAL_DIR, "snapshots"))
    p.add_argument("--only", nargs="*", default=None)
    p.add_argument("--poll", type=float, default=5)
    p.add_argument("--settle", type=float, default=20)
    args = p.parse_args(argv)

    env = os.environ
    indexer = IndexerClient(env.get("BRAIN_EVAL_INDEXER_URL", "https://127.0.0.1:9200"),
                            env.get("BRAIN_EVAL_INDEXER_USER", "admin"),
                            env.get("BRAIN_EVAL_INDEXER_PASSWORD", "Blackwall1!"),
                            verify=env.get("BRAIN_EVAL_INDEXER_VERIFY_TLS", "") == "true")
    scenarios = [s for s in load_scenarios(args.scenarios)
                 if args.only is None or s.name in args.only]
    failed = 0
    for s in scenarios:
        try:
            r = capture_scenario(s, indexer, poll_s=args.poll, settle_s=args.settle,
                                 sleep=time.sleep)
        except IndexerError as exc:
            r = ScenarioResult(s.name, False, error=f"indexer: {exc}")
        if r.ok:
            try:
                write_snapshot(args.out, s, r.cases)
                print(f"OK      {s.name}: {len(r.cases)} cases, unlabelled {r.unlabelled}")
                continue
            except ValueError as exc:
                r.error = str(exc)
        failed += 1
        print(f"FAILED  {s.name}: {r.error or 'missing rules ' + str(r.missing)}"
              f" (unlabelled {r.unlabelled})")
    print(f"{len(scenarios) - failed}/{len(scenarios)} scenarios OK")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
