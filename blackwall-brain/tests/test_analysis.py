import json

import pytest

from brain.analysis import (
    PROMPT_VERSION, AnalysisError, alert_context, analyze, build_prompt,
)
from brain.models import ModelError, ModelResult
from brain.verdict import CATEGORIES, SEVERITIES, VERDICT_SCHEMA, VERDICTS


class ScriptedModel:
    """Returns (or raises) the scripted answers in order."""
    name = "fake/model"

    def __init__(self, *answers):
        self.answers, self.calls = list(answers), []

    def complete(self, system, user, schema=None):
        self.calls.append((system, user, schema))
        a = self.answers.pop(0)
        if isinstance(a, Exception):
            raise a
        return ModelResult(a, 10, 5, 0.1)


def test_prompt_version_is_set():
    assert PROMPT_VERSION == "2026-10-28.1"


def test_context_truncates_full_log_to_500(alert):
    alert["full_log"] = "x" * 2000
    assert len(alert_context(alert)["full_log_excerpt"]) == 500


def test_context_fields(alert):
    ctx = alert_context(alert)
    assert ctx["host"] == "black-host-01" and ctx["agent_ip"] == "172.28.0.11"
    assert ctx["rule_id"] == "100120" and ctx["rule_level"] == 10
    assert ctx["rule_groups"] == ["sudo", "audit"]
    assert "Privilege Escalation" in ctx["mitre"]


def test_context_host_fallbacks(alert):
    del alert["agent"]
    assert alert_context(alert)["host"] == "black-master-00"
    del alert["manager"]
    assert alert_context(alert)["host"] == "unknown"
    assert alert_context({})["mitre"] == "none"


def test_prompt_lists_all_enum_values(alert):
    system, user = build_prompt(alert)
    for value in (*SEVERITIES, *VERDICTS, *CATEGORIES):
        assert value in system
    assert json.loads(user) == alert_context(alert)


def test_passes_schema_to_model(alert, verdict_json):
    model = ScriptedModel(verdict_json)
    a = analyze(alert, model, sleep=lambda s: None)
    assert model.calls[0][2] == VERDICT_SCHEMA
    assert a.verdict.severity == "high" and a.attempts == 1 and a.result.input_tokens == 10


def test_fenced_json_succeeds_first_try(alert, verdict_json):
    model = ScriptedModel("```json\n" + verdict_json + "\n```")
    assert analyze(alert, model, sleep=lambda s: None).attempts == 1


def test_retries_model_error_three_times_then_raises(alert):
    slept = []
    model = ScriptedModel(ModelError("timeout"), ModelError("timeout"), ModelError("timeout"))
    with pytest.raises(AnalysisError, match="timeout"):
        analyze(alert, model, sleep=slept.append)
    assert len(model.calls) == 3 and slept == [2, 4]


def test_model_error_then_success(alert, verdict_json):
    model = ScriptedModel(ModelError("503"), verdict_json)
    assert analyze(alert, model, sleep=lambda s: None).attempts == 2


def test_invalid_output_retried_once_then_raises(alert, verdict_json):
    bad = verdict_json.replace('"high"', '"urgent"')
    model = ScriptedModel(bad, bad, verdict_json)
    with pytest.raises(AnalysisError, match="severity"):
        analyze(alert, model, sleep=lambda s: None)
    assert len(model.calls) == 2
