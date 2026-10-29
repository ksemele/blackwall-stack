"""Snapshots: the captured alerts (and the context each got), with labels.

They let anyone compare models or prompts without the lab.
"""
import json
import os
from dataclasses import asdict, dataclass
from datetime import datetime, timezone

from brain.eval.scenarios import Expected, Scenario, find_marker

# Bump when scenarios or labels change; results record it.
EVAL_SET_VERSION = "2026-10-29.1"


@dataclass
class Case:
    scenario: str
    doc_id: str
    alert: dict
    context: dict
    expected: Expected


def _now() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def write_snapshot(directory: str, scenario: Scenario, cases: list[Case]) -> str:
    data = {
        "eval_set_version": EVAL_SET_VERSION,
        "scenario": scenario.name,
        "captured_at": _now(),
        "cases": [{"doc_id": c.doc_id, "alert": c.alert, "context": c.context,
                   "expected": asdict(c.expected)} for c in cases],
    }
    text = json.dumps(data, indent=2, sort_keys=True, ensure_ascii=False) + "\n"
    marker = find_marker(json.dumps(data["cases"]))
    if marker:
        raise ValueError(f"{scenario.name}: snapshot contains test marker {marker!r}")
    os.makedirs(directory, exist_ok=True)
    path = os.path.join(directory, f"{scenario.name}.json")
    with open(path, "w") as f:
        f.write(text)
    return path


def load_cases(directory: str) -> list[Case]:
    cases = []
    for name in sorted(n for n in os.listdir(directory) if n.endswith(".json")):
        path = os.path.join(directory, name)
        data = json.load(open(path))
        if data.get("eval_set_version") != EVAL_SET_VERSION:
            raise ValueError(f"{path}: eval_set_version {data.get('eval_set_version')!r}, "
                             f"expected {EVAL_SET_VERSION!r}; re-capture it")
        for c in data["cases"]:
            cases.append(Case(data["scenario"], c["doc_id"], c["alert"], c["context"],
                              Expected(**c["expected"])))
    return cases
