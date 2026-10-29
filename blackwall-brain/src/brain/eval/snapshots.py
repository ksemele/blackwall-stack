"""Snapshots: the captured alerts (and the context each got), with labels.

They let anyone compare models or prompts without the lab.
"""
import getpass
import ipaddress
import json
import os
import re
import socket
from dataclasses import asdict, dataclass
from datetime import datetime, timezone

from brain.eval.scenarios import Expected, Scenario, find_marker

# Bump when scenarios or labels change; results record it.
EVAL_SET_VERSION = "2026-10-29.2"


# Addresses a snapshot may contain: the lab, loopback, documentation ranges.
_ALLOWED_NETS = [ipaddress.ip_network(n) for n in
                 ("172.28.0.0/24", "127.0.0.0/8", "0.0.0.0/32", "192.0.2.0/24",
                  "198.51.100.0/24", "203.0.113.0/24")]
_IPV4 = re.compile(r"(?<![\d.])(\d{1,3}(?:\.\d{1,3}){3})(?![\d.])")


def find_leak(text: str) -> str | None:
    """Host-machine data or a non-lab address in captured text (spec 8.2)."""
    for needle in ("/Users/", f"/home/{getpass.getuser()}/", socket.gethostname()):
        if needle and needle in text:
            return needle
    for m in _IPV4.finditer(text):
        try:
            ip = ipaddress.ip_address(m.group(1))
        except ValueError:
            continue
        if not any(ip in net for net in _ALLOWED_NETS):
            return m.group(1)
    return None


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
    body = json.dumps(data["cases"])
    marker = find_marker(body)
    if marker:
        raise ValueError(f"{scenario.name}: snapshot contains test marker {marker!r}")
    leak = find_leak(body)
    if leak:
        raise ValueError(f"{scenario.name}: snapshot would leak host data or a non-lab "
                         f"address: {leak!r}")
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
