import textwrap

import pytest

from brain.eval.scenarios import Expected, Scenario, Step, load_scenarios

GOOD = """
name: sudoers-dropin-by-webuser
description: A web shell user grants itself passwordless sudo.
agent: black-host-01
wait_s: 60
steps:
  - {host: black-host-01, user: www-data, run: "echo hi"}
  - {host: black-host-01, run: "true"}
cleanup:
  - {host: black-host-01, run: "rm -f /etc/sudoers.d/90-app"}
expected:
  - rule_id: "554"
    verdict: true_positive
    severity: high
    category: privilege_escalation
    why: www-data has no business editing sudoers.
"""


def write(tmp_path, name, text):
    (tmp_path / name).write_text(textwrap.dedent(text))


def test_loads_example(tmp_path):
    write(tmp_path, "a.yml", GOOD)
    [s] = load_scenarios(str(tmp_path))
    assert s == Scenario(
        name="sudoers-dropin-by-webuser",
        description="A web shell user grants itself passwordless sudo.",
        agent="black-host-01",
        steps=[Step("black-host-01", "www-data", "echo hi"), Step("black-host-01", "root", "true")],
        cleanup=[Step("black-host-01", "root", "rm -f /etc/sudoers.d/90-app")],
        expected=[Expected("554", "true_positive", "high", "privilege_escalation",
                           "www-data has no business editing sudoers.")],
        wait_s=60,
    )


def test_sorted_by_file_name(tmp_path):
    write(tmp_path, "b.yml", GOOD.replace("sudoers-dropin-by-webuser", "b"))
    write(tmp_path, "a.yml", GOOD.replace("sudoers-dropin-by-webuser", "a"))
    assert [s.name for s in load_scenarios(str(tmp_path))] == ["a", "b"]


@pytest.mark.parametrize("change,problem", [
    (("wait_s: 60", "wait_s: 60\ncolour: red"), "colour"),
    (("severity: high", "severity: urgent"), "severity"),
    (("verdict: true_positive", "verdict: false_positive"), "low"),
    (("echo hi", "touch /etc/sudoers.d/bw-trigger"), "marker"),
    (("echo hi", "echo SYNTHETIC"), "marker"),
])
def test_rejects_bad_files(tmp_path, change, problem):
    write(tmp_path, "bad.yml", GOOD.replace(*change))
    with pytest.raises(ValueError, match=f"bad.yml.*{problem}"):
        load_scenarios(str(tmp_path))


def test_rejects_duplicate_rule(tmp_path):
    dup = GOOD + """
  - rule_id: "554"
    verdict: true_positive
    severity: high
    category: persistence
    why: again
"""
    write(tmp_path, "dup.yml", dup)
    with pytest.raises(ValueError, match="dup.yml.*554"):
        load_scenarios(str(tmp_path))


def test_rejects_missing_expected(tmp_path):
    write(tmp_path, "none.yml", GOOD.split("expected:")[0] + "expected: []\n")
    with pytest.raises(ValueError, match="none.yml.*expected"):
        load_scenarios(str(tmp_path))
