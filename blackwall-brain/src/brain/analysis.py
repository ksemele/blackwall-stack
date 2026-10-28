"""Prompt and retries: one alert in, one validated verdict out."""
import json
import logging
import time
from dataclasses import dataclass

from brain.models import Model, ModelError, ModelResult
from brain.verdict import (
    CATEGORIES, CONFIDENCES, SEVERITIES, VERDICT_SCHEMA, VERDICTS,
    InvalidVerdict, Verdict, parse_verdict,
)

log = logging.getLogger(__name__)

# Bump when the prompt or the verdict schema changes; eval results record it.
PROMPT_VERSION = "2026-10-28.1"

MODEL_ATTEMPTS = 3      # transport/HTTP failures
INVALID_ATTEMPTS = 2    # answers that fail verdict validation
BACKOFF_S = (2, 4)

SYSTEM_PROMPT = f"""You are a security analyst triaging one Wazuh SIEM alert.
Answer with one JSON object only, no other text, with these fields:

- verdict: one of {", ".join(VERDICTS)}. Is this real malicious or unwanted
  activity (true_positive) or noise (false_positive)? Use uncertain rather
  than guess when the alert alone cannot tell.
- severity: one of {", ".join(SEVERITIES)}. How urgently a human should look,
  in your judgement (not the rule level).
- category: one of {", ".join(CATEGORIES)}.
- summary: one short line, what happened in plain words.
- action: one short sentence, what the operator should do next.
- confidence: one of {", ".join(CONFIDENCES)}.
- needs: list of short strings naming the data you lacked to decide; an empty
  list if nothing was missing.
"""


class AnalysisError(Exception):
    """The model gave no valid verdict; str() goes into the fallback message."""


@dataclass
class Analysis:
    verdict: Verdict
    result: ModelResult
    attempts: int


def alert_context(alert: dict) -> dict:
    rule = alert.get("rule", {})
    agent = alert.get("agent", {})
    mitre = rule.get("mitre") or {}
    tactics, techniques = mitre.get("tactic", []), mitre.get("technique", [])
    mitre_info = "none"
    if tactics or techniques:
        mitre_info = f"Tactics: {', '.join(tactics)}; Techniques: {', '.join(techniques)}"
    return {
        "host": agent.get("name") or alert.get("manager", {}).get("name") or "unknown",
        "agent_ip": agent.get("ip", ""),
        "rule_id": rule.get("id", ""),
        "rule_level": rule.get("level", ""),
        "rule_description": rule.get("description", ""),
        "rule_groups": rule.get("groups", []),
        "timestamp": alert.get("timestamp", ""),
        "mitre": mitre_info,
        "full_log_excerpt": alert.get("full_log", "")[:500],
    }


def build_prompt(alert: dict) -> tuple[str, str]:
    return SYSTEM_PROMPT, json.dumps(alert_context(alert), indent=2)


def analyze(alert: dict, model: Model, sleep=time.sleep) -> Analysis:
    system, user = build_prompt(alert)
    model_failures = invalid = 0
    attempts = 0
    while True:
        attempts += 1
        try:
            result = model.complete(system, user, schema=VERDICT_SCHEMA)
        except ModelError as exc:
            model_failures += 1
            log.warning("model call failed (%d/%d): %s", model_failures, MODEL_ATTEMPTS, exc)
            if model_failures >= MODEL_ATTEMPTS:
                raise AnalysisError(str(exc)) from exc
            sleep(BACKOFF_S[model_failures - 1])
            continue
        try:
            return Analysis(parse_verdict(result.text), result, attempts)
        except InvalidVerdict as exc:
            invalid += 1
            log.warning("invalid verdict (%d/%d): %s", invalid, INVALID_ATTEMPTS, exc)
            if invalid >= INVALID_ATTEMPTS:
                raise AnalysisError(f"invalid answer: {exc}") from exc
