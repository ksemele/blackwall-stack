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
    since = _iso(now())
    try:
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


def _collect(s: Scenario, indexer, since: str, *, clock, sleep, poll_s) -> ScenarioResult:
    wanted = {e.rule_id: e for e in s.expected}

    deadline = clock() + s.wait_s
    while True:
        hits = indexer.search(alerts_query(s.agent, since, list(wanted)))
        counts = {r: sum(1 for _, a in hits if a["rule"]["id"] == r) for r in wanted}
        missing = [r for r, e in wanted.items() if counts[r] < e.min_count]
        if not missing or clock() >= deadline:
            break
        sleep(poll_s)

    everything = indexer.search(alerts_query(s.agent, since, []))
    cases = [Case(s.name, doc_id, a, {}, wanted[a["rule"]["id"]])
             for doc_id, a in everything if a["rule"]["id"] in wanted]
    unlabelled = sum(1 for _, a in everything if a["rule"]["id"] not in wanted)
    return ScenarioResult(s.name, not missing, cases, missing, unlabelled)


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
