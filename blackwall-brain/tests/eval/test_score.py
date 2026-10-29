import pytest

from brain.eval.scenarios import Expected
from brain.eval.score import Outcome, estimate_cost, score
from brain.eval.snapshots import Case
from brain.verdict import Verdict


def case(i, verdict="true_positive", severity="high", category="persistence", level=10):
    exp = Expected("554", verdict, severity if verdict != "false_positive" else "low",
                   category, "why")
    return Case(f"s{i}", f"d{i}", {"rule": {"id": "554", "level": level}}, {}, exp)


def got(verdict="true_positive", severity="high", category="persistence", needs=()):
    return Verdict(verdict, severity, category, "s", "a", "high", list(needs))


def out(c, v, latency=1.0, error=""):
    return Outcome(c, v, error, 100, 10, latency)


def test_verdict_accuracy_and_confusion():
    r = score([out(case(1), got()), out(case(2, "false_positive"), got("true_positive")),
               out(case(3, "false_positive"), got("false_positive", "low"))], threshold=10)
    assert r["n"] == 3 and r["verdict"]["accuracy"] == pytest.approx(2 / 3)
    assert r["verdict"]["confusion"]["false_positive"] == {"true_positive": 1, "false_positive": 1}


def test_missed_threat_listed():
    r = score([out(case(1), got("false_positive", "low"))], threshold=10)
    assert r["missed_threats"] == ["s1/d1"] and r["threats_not_called"] == 0


def test_uncertain_on_threat_is_not_called_not_missed():
    r = score([out(case(1), got("uncertain"))], threshold=10)
    assert r["missed_threats"] == [] and r["threats_not_called"] == 1
    assert r["uncertain_rate"] == 1 and r["verdict"]["accuracy"] == 0


def test_invalid_counts_wrong_everywhere():
    r = score([out(case(1), None, error="invalid answer"), out(case(2), got())], threshold=10)
    assert r["invalid_rate"] == 0.5
    assert r["verdict"]["accuracy"] == r["severity"]["accuracy"] == r["category"]["accuracy"] == 0.5
    assert r["verdict"]["confusion"]["true_positive"]["invalid"] == 1


def test_subset_at_threshold():
    r = score([out(case(1, level=12), got()), out(case(2, level=5), got("false_positive", "low"))],
              threshold=10)
    assert r["subset_at_threshold"]["n"] == 1 and r["subset_at_threshold"]["verdict"]["accuracy"] == 1


def test_empty_subset():
    r = score([out(case(1, level=3), got())], threshold=10)
    assert r["subset_at_threshold"]["n"] == 0


def test_latency_percentiles_and_tokens():
    r = score([out(case(i), got(), latency=float(i)) for i in range(1, 21)], threshold=10)
    assert r["latency"]["p50"] == pytest.approx(10.5) and r["latency"]["p95"] == pytest.approx(19.05)
    assert r["tokens"] == {"input": 2000, "output": 200}


def test_needs_frequency():
    r = score([out(case(1), got(needs=["admin list", "deploy log"])),
               out(case(2), got(needs=["admin list"]))], threshold=10)
    assert r["needs"] == [["admin list", 2], ["deploy log", 1]]


def test_cost_known_and_unknown():
    prices = {"m": {"input_per_mtok": 1.0, "output_per_mtok": 5.0}}
    assert estimate_cost("m", 1_000_000, 200_000, prices) == pytest.approx(2.0)
    assert estimate_cost("other", 1, 1, prices) is None
