"""Scoring: how Brain's verdicts compare with the labels (spec section 8.3)."""
from collections import Counter
from dataclasses import dataclass

from brain.eval.snapshots import Case
from brain.verdict import Verdict


@dataclass
class Outcome:
    case: Case
    verdict: Verdict | None     # None: the model gave no valid verdict
    error: str
    input_tokens: int
    output_tokens: int
    latency_s: float


def _percentile(values: list[float], q: float) -> float | None:
    if not values:
        return None
    v = sorted(values)
    pos = (len(v) - 1) * q
    lo, hi = int(pos), min(int(pos) + 1, len(v) - 1)
    return v[lo] + (v[hi] - v[lo]) * (pos - lo)


def _field_accuracy(outcomes: list[Outcome], name: str) -> dict:
    right = sum(1 for o in outcomes
                if o.verdict and getattr(o.verdict, name) == getattr(o.case.expected, name))
    return {"accuracy": right / len(outcomes) if outcomes else None}


def _core(outcomes: list[Outcome]) -> dict:
    n = len(outcomes)
    confusion: dict = {}
    for o in outcomes:
        got = o.verdict.verdict if o.verdict else "invalid"
        row = confusion.setdefault(o.case.expected.verdict, {})
        row[got] = row.get(got, 0) + 1
    verdict = _field_accuracy(outcomes, "verdict")
    verdict["confusion"] = confusion
    threats = [o for o in outcomes if o.case.expected.verdict == "true_positive"]
    return {
        "n": n,
        "verdict": verdict,
        # The worst error: a real threat called noise.
        "missed_threats": [f"{o.case.scenario}/{o.case.doc_id}" for o in threats
                           if o.verdict and o.verdict.verdict == "false_positive"],
        "threats_not_called": sum(1 for o in threats
                                  if o.verdict and o.verdict.verdict == "uncertain"),
        "uncertain_rate": (sum(1 for o in outcomes if o.verdict and o.verdict.verdict == "uncertain")
                           / n if n else None),
        "invalid_rate": sum(1 for o in outcomes if o.verdict is None) / n if n else None,
        "severity": _field_accuracy(outcomes, "severity"),
        "category": _field_accuracy(outcomes, "category"),
    }


def score(outcomes: list[Outcome], threshold: int) -> dict:
    result = _core(outcomes)
    needs = Counter(n for o in outcomes if o.verdict for n in o.verdict.needs)
    result["needs"] = [[k, v] for k, v in needs.most_common(10)]
    result["tokens"] = {"input": sum(o.input_tokens for o in outcomes),
                        "output": sum(o.output_tokens for o in outcomes)}
    lat = [o.latency_s for o in outcomes]
    result["latency"] = {"p50": _percentile(lat, 0.5), "p95": _percentile(lat, 0.95)}
    # What production Brain actually sees: rule.level >= analysis_threshold.
    # Cases where the alert alone is enough to decide (no context needed).
    result["subset_without_context_need"] = _core(
        [o for o in outcomes if not o.case.expected.needs_context])
    result["subset_at_threshold"] = _core(
        [o for o in outcomes if int(o.case.alert.get("rule", {}).get("level", 0)) >= threshold])
    return result


def estimate_cost(model: str, tokens_in: int, tokens_out: int, prices: dict) -> float | None:
    p = prices.get(model)
    if p is None:
        return None
    return (tokens_in * p["input_per_mtok"] + tokens_out * p["output_per_mtok"]) / 1_000_000
