import json

import pytest

from brain.verdict import (
    CATEGORIES, CONFIDENCES, SEVERITIES, VERDICT_SCHEMA, VERDICTS,
    InvalidVerdict, Verdict, parse_verdict,
)

VALID = {
    "verdict": "true_positive",
    "severity": "high",
    "category": "privilege_escalation",
    "summary": "New sudoers drop-in on black-host-01",
    "action": "Check who wrote /etc/sudoers.d/90-app and revert it if unknown",
    "confidence": "medium",
    "needs": ["who administers this host"],
}


def test_parses_valid_object():
    v = parse_verdict(json.dumps(VALID))
    assert v == Verdict(**VALID)


def test_strips_code_fence():
    text = "```json\n" + json.dumps(VALID) + "\n```"
    assert parse_verdict(text).severity == "high"


def test_strips_prose_around_object():
    text = "Here is my answer:\n" + json.dumps(VALID) + "\nThanks"
    assert parse_verdict(text).verdict == "true_positive"


def test_rejects_unknown_enum():
    with pytest.raises(InvalidVerdict, match="severity"):
        parse_verdict(json.dumps({**VALID, "severity": "urgent"}))


def test_rejects_missing_field():
    bad = dict(VALID)
    del bad["action"]
    with pytest.raises(InvalidVerdict, match="action"):
        parse_verdict(json.dumps(bad))


def test_rejects_non_json():
    with pytest.raises(InvalidVerdict):
        parse_verdict("I think this is bad")


def test_needs_must_be_list_of_strings():
    with pytest.raises(InvalidVerdict, match="needs"):
        parse_verdict(json.dumps({**VALID, "needs": "x"}))


def test_rejects_empty_summary():
    with pytest.raises(InvalidVerdict, match="summary"):
        parse_verdict(json.dumps({**VALID, "summary": "  "}))


def test_schema_matches_constants():
    props = VERDICT_SCHEMA["properties"]
    assert props["verdict"]["enum"] == list(VERDICTS)
    assert props["severity"]["enum"] == list(SEVERITIES)
    assert props["category"]["enum"] == list(CATEGORIES)
    assert props["confidence"]["enum"] == list(CONFIDENCES)
    assert sorted(VERDICT_SCHEMA["required"]) == sorted(VALID)
    assert VERDICT_SCHEMA["additionalProperties"] is False
