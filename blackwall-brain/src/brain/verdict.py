"""The verdict: Brain's structured answer about one alert (spec section 4).

Same schema for every model backend; validated here, not trusted.
"""
import json
from dataclasses import dataclass

VERDICTS = ("true_positive", "false_positive", "uncertain")
SEVERITIES = ("low", "medium", "high", "critical")
CONFIDENCES = ("high", "medium", "low")
CATEGORIES = (
    "lateral_movement", "privilege_escalation", "persistence", "credential_access",
    "defense_evasion", "discovery", "collection", "exfiltration",
    "command_and_control", "impact", "false_positive", "unknown",
)

_ENUMS = {
    "verdict": VERDICTS,
    "severity": SEVERITIES,
    "category": CATEGORIES,
    "confidence": CONFIDENCES,
}
_TEXT = ("summary", "action")
_FIELDS = ("verdict", "severity", "category", "summary", "action", "confidence", "needs")

VERDICT_SCHEMA = {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "type": "object",
    "properties": {
        **{k: {"type": "string", "enum": list(v)} for k, v in _ENUMS.items()},
        **{k: {"type": "string"} for k in _TEXT},
        "needs": {"type": "array", "items": {"type": "string"}},
    },
    "required": list(_FIELDS),
    "additionalProperties": False,
}


class InvalidVerdict(ValueError):
    """The model's answer is not a valid verdict."""


@dataclass(frozen=True)
class Verdict:
    verdict: str
    severity: str
    category: str
    summary: str
    action: str
    confidence: str
    needs: list[str]


def _extract_object(text: str) -> dict:
    # Models wrap JSON in code fences or prose; take the outermost {...}.
    start, end = text.find("{"), text.rfind("}")
    if start < 0 or end <= start:
        raise InvalidVerdict("no JSON object in the answer")
    try:
        obj = json.loads(text[start:end + 1])
    except json.JSONDecodeError as exc:
        raise InvalidVerdict(f"invalid JSON: {exc.msg}") from exc
    if not isinstance(obj, dict):
        raise InvalidVerdict("answer is not a JSON object")
    return obj


def parse_verdict(text: str) -> Verdict:
    obj = _extract_object(text)
    for name in _FIELDS:
        if name not in obj:
            raise InvalidVerdict(f"missing field: {name}")
    for name, allowed in _ENUMS.items():
        if obj[name] not in allowed:
            raise InvalidVerdict(f"invalid {name}: {obj[name]!r}")
    for name in _TEXT:
        if not isinstance(obj[name], str) or not obj[name].strip():
            raise InvalidVerdict(f"empty or non-string {name}")
    needs = obj["needs"]
    if not isinstance(needs, list) or not all(isinstance(n, str) for n in needs):
        raise InvalidVerdict("needs must be a list of strings")
    return Verdict(**{name: obj[name] for name in _FIELDS})
