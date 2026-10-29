import json

import pytest

from brain.analysis import PROMPT_VERSION
from brain.eval import run as eval_run
from brain.eval.scenarios import Expected, Scenario, Step
from brain.eval.snapshots import EVAL_SET_VERSION, Case, write_snapshot
from brain.models import ModelError, ModelResult

VERDICT = json.dumps({"verdict": "true_positive", "severity": "high", "category": "persistence",
                      "summary": "s", "action": "a", "confidence": "high", "needs": []})


class Model:
    name = "fake/m"

    def __init__(self, fail_every=0):
        self.n, self.fail_every = 0, fail_every

    def complete(self, system, user, schema=None):
        self.n += 1
        if self.fail_every and self.n % self.fail_every == 0:
            raise ModelError("down")
        return ModelResult(VERDICT, 1_000_000, 100_000, 0.5)


@pytest.fixture
def snaps(tmp_path):
    exp = Expected("554", "true_positive", "high", "persistence", "why")
    s = Scenario("cron", "d", "black-host-01", [Step("h", "root", "x")], [], [exp])
    cases = [Case("cron", f"d{i}", {"rule": {"id": "554", "level": 5}}, {}, exp) for i in range(3)]
    write_snapshot(str(tmp_path / "snap"), s, cases)
    return tmp_path


def go(tmp_path, monkeypatch, model, *extra):
    monkeypatch.setattr(eval_run, "build_model", lambda s: model)
    monkeypatch.setattr(eval_run.time, "sleep", lambda s: None)
    monkeypatch.setenv("BRAIN_MODEL_API_KEY", "k")
    rc = eval_run.main(["--snapshots", str(tmp_path / "snap"), "--out", str(tmp_path / "res"),
                        "--model", "claude-haiku-5-5", *extra])
    [path] = list((tmp_path / "res").iterdir())
    return rc, json.load(open(path)), path.name


def test_run_writes_result_with_versions(snaps, monkeypatch, capsys):
    rc, res, name = go(snaps, monkeypatch, Model())
    assert rc == 0 and name.endswith("-anthropic-claude-haiku-5-5.json")
    assert res["prompt_version"] == PROMPT_VERSION and res["eval_set_version"] == EVAL_SET_VERSION
    assert res["model"] == "claude-haiku-5-5" and res["backend"] == "anthropic"
    assert res["score"]["verdict"]["accuracy"] == 1 and len(res["per_case"]) == 3
    assert "Missed threats" in capsys.readouterr().out


def test_run_survives_analysis_error(snaps, monkeypatch):
    rc, res, _ = go(snaps, monkeypatch, Model(fail_every=1))
    assert rc == 0 and res["score"]["invalid_rate"] == 1
    assert all(c["error"] for c in res["per_case"])


def test_max_cost_stops_and_marks_partial(snaps, monkeypatch):
    # Haiku 5.5: 1M in = $0.10, 0.1M out = $0.05 → $0.15 per case
    rc, res, _ = go(snaps, monkeypatch, Model(), "--max-cost", "0.2")
    assert res["stopped"] == "max_cost" and len(res["per_case"]) == 2


def test_unknown_price_warns(snaps, monkeypatch, capsys):
    monkeypatch.setattr(eval_run, "build_model", lambda s: Model())
    monkeypatch.setenv("BRAIN_MODEL_API_KEY", "k")
    eval_run.main(["--snapshots", str(snaps / "snap"), "--out", str(snaps / "res"),
                   "--model", "mystery", "--max-cost", "1"])
    assert "not enforced" in capsys.readouterr().out


def test_run_survives_unexpected_backend_exception(snaps, monkeypatch):
    class Broken(Model):
        def complete(self, system, user, schema=None):
            raise TypeError("sdk bug")

    rc, res, _ = go(snaps, monkeypatch, Broken())
    assert rc == 0 and res["score"]["invalid_rate"] == 1
    assert "TypeError" in res["per_case"][0]["error"]
