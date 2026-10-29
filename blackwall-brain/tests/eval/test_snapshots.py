import json

import pytest

from brain.eval import snapshots
from brain.eval.scenarios import Expected, Scenario, Step
from brain.eval.snapshots import EVAL_SET_VERSION, Case, load_cases, write_snapshot

EXP = Expected("554", "true_positive", "high", "persistence", "new cron job")
SCEN = Scenario("cron-curl-sh", "d", "black-host-01", [Step("black-host-01", "root", "x")],
                [], [EXP])


def case(doc_id="d1", log="file /etc/cron.d/x added"):
    return Case("cron-curl-sh", doc_id, {"rule": {"id": "554"}, "full_log": log}, {}, EXP)


def test_roundtrip(tmp_path):
    write_snapshot(str(tmp_path), SCEN, [case("d1"), case("d2")])
    got = load_cases(str(tmp_path))
    assert [c.doc_id for c in got] == ["d1", "d2"] and got[0] == case("d1")


def test_marker_in_alert_rejected(tmp_path):
    with pytest.raises(ValueError, match="marker"):
        write_snapshot(str(tmp_path), SCEN, [case(log="echo Synthetic alert")])
    assert list(tmp_path.iterdir()) == []


def test_version_mismatch_rejected(tmp_path):
    path = write_snapshot(str(tmp_path), SCEN, [case()])
    data = json.load(open(path))
    data["eval_set_version"] = "old"
    json.dump(data, open(path, "w"))
    with pytest.raises(ValueError, match="eval_set_version"):
        load_cases(str(tmp_path))


def test_stable_output(tmp_path, monkeypatch):
    monkeypatch.setattr(snapshots, "_now", lambda: "2026-10-29T10:00:00Z")
    a = open(write_snapshot(str(tmp_path / "a"), SCEN, [case()])).read()
    b = open(write_snapshot(str(tmp_path / "b"), SCEN, [case()])).read()
    assert a == b and EVAL_SET_VERSION in a


@pytest.mark.parametrize("leak", ["/Users/someone/x", "8.8.8.8", "10.0.0.5"])
def test_host_data_leak_rejected(tmp_path, leak):
    with pytest.raises(ValueError, match="leak"):
        write_snapshot(str(tmp_path), SCEN, [case(log=f"seen {leak}")])


def test_lab_and_documentation_addresses_allowed(tmp_path):
    write_snapshot(str(tmp_path), SCEN,
                   [case(log="from 172.28.0.10 to 203.0.113.66 and 198.51.100.23, 127.0.0.1")])
