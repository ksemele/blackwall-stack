"""Scenario files: lab actions plus the alerts they must raise, labelled."""
import os
from dataclasses import dataclass, field

import yaml

from brain.verdict import CATEGORIES, SEVERITIES, VERDICTS

# Strings that tell a model "this is a test" (bw-trigger's names). A scenario
# or snapshot containing one would measure the string, not the triage.
MARKERS = ("bw-trigger", "donotuseinprod", "synthetic")

_SCENARIO_KEYS = {"name", "description", "agent", "wait_s", "steps", "cleanup", "expected"}
_STEP_KEYS = {"host", "user", "run"}
_EXPECTED_KEYS = {"rule_id", "verdict", "severity", "category", "why", "min_count"}


@dataclass
class Step:
    host: str
    user: str
    run: str


@dataclass
class Expected:
    rule_id: str
    verdict: str
    severity: str
    category: str
    why: str
    min_count: int = 1


@dataclass
class Scenario:
    name: str
    description: str
    agent: str
    steps: list[Step]
    cleanup: list[Step] = field(default_factory=list)
    expected: list[Expected] = field(default_factory=list)
    wait_s: int = 90


def find_marker(text: str) -> str | None:
    low = text.lower()
    return next((m for m in MARKERS if m in low), None)


def _check_keys(obj, allowed: set, where: str) -> None:
    if not isinstance(obj, dict):
        raise ValueError(f"{where}: must be a mapping")
    unknown = set(obj) - allowed
    if unknown:
        raise ValueError(f"{where}: unknown keys {sorted(unknown)}")


def _step(obj, where: str) -> Step:
    _check_keys(obj, _STEP_KEYS, where)
    return Step(obj["host"], obj.get("user", "root"), obj["run"])


def _expected(obj, where: str) -> Expected:
    _check_keys(obj, _EXPECTED_KEYS, where)
    e = Expected(str(obj["rule_id"]), obj["verdict"], obj["severity"], obj["category"],
                 obj["why"], int(obj.get("min_count", 1)))
    for name, allowed in (("verdict", VERDICTS), ("severity", SEVERITIES),
                          ("category", CATEGORIES)):
        if getattr(e, name) not in allowed:
            raise ValueError(f"{where}: invalid {name} {getattr(e, name)!r}")
    if e.verdict == "false_positive" and e.severity != "low":
        raise ValueError(f"{where}: a false_positive must have severity low")
    return e


def _load(path: str) -> Scenario:
    text = open(path).read()
    marker = find_marker(text)
    if marker:
        raise ValueError(f"{path}: contains test marker {marker!r}")
    try:
        obj = yaml.safe_load(text)
        _check_keys(obj, _SCENARIO_KEYS, path)
        expected = [_expected(e, f"{path}: expected[{i}]")
                    for i, e in enumerate(obj.get("expected") or [])]
        if not expected:
            raise ValueError(f"{path}: no expected alerts")
        seen = set()
        for e in expected:
            if e.rule_id in seen:
                raise ValueError(f"{path}: rule {e.rule_id} listed twice")
            seen.add(e.rule_id)
        return Scenario(
            name=obj["name"], description=obj["description"], agent=obj["agent"],
            steps=[_step(s, f"{path}: steps[{i}]") for i, s in enumerate(obj["steps"])],
            cleanup=[_step(s, f"{path}: cleanup[{i}]")
                     for i, s in enumerate(obj.get("cleanup") or [])],
            expected=expected, wait_s=int(obj.get("wait_s", 90)))
    except KeyError as exc:
        raise ValueError(f"{path}: missing key {exc}") from exc
    except yaml.YAMLError as exc:
        raise ValueError(f"{path}: invalid YAML: {exc}") from exc


def load_scenarios(directory: str) -> list[Scenario]:
    names = sorted(n for n in os.listdir(directory) if n.endswith((".yml", ".yaml")))
    return [_load(os.path.join(directory, n)) for n in names]
