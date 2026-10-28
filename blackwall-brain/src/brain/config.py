"""Settings from BRAIN_* environment variables (rendered by the Ansible role)."""
import json
import os
from collections.abc import Mapping
from dataclasses import dataclass

from brain.models import ModelSettings

_BOOLS = {"true": True, "1": True, "yes": True, "false": False, "0": False, "no": False}


@dataclass
class Settings:
    analysis_threshold: int
    ignore_rule_ids: list[str]
    poll_interval: float
    max_backfill: int
    state_file: str
    state_max_ids: int
    indexer_url: str
    indexer_user: str
    indexer_password: str
    indexer_verify: bool | str     # False, True, or a CA bundle path
    model: ModelSettings
    telegram_bot_token: str
    telegram_chat_id: str
    dashboard_url: str
    rule_links: dict


def load(env: Mapping[str, str] = os.environ) -> Settings:
    def get(name, default=None):
        value = env.get(name, "")
        if value == "":
            if default is None:
                raise ValueError(f"{name} is required")
            return default
        return value

    def typed(name, conv, default):
        raw = env.get(name, "")
        if raw == "":
            return default
        try:
            return conv(raw)
        except (ValueError, TypeError) as exc:
            raise ValueError(f"{name}: invalid value {raw!r}") from exc

    def json_object(name, default):
        value = typed(name, json.loads, default)
        if not isinstance(value, dict):
            raise ValueError(f"{name}: must be a JSON object")
        return value

    def boolean(raw):
        return _BOOLS[raw.strip().lower()]

    def bool_var(name, default):
        try:
            return typed(name, boolean, default)
        except KeyError as exc:
            raise ValueError(f"{name}: expected true or false") from exc

    backend = get("BRAIN_MODEL_BACKEND", "anthropic")
    # Local OpenAI-compatible servers usually need no key.
    api_key = get("BRAIN_MODEL_API_KEY", "" if backend == "openai_compat" else None)
    ca_file = env.get("BRAIN_INDEXER_CA_FILE", "")
    verify = bool_var("BRAIN_INDEXER_VERIFY_TLS", True)

    return Settings(
        analysis_threshold=typed("BRAIN_ANALYSIS_THRESHOLD", int, 7),
        ignore_rule_ids=[i.strip() for i in get("BRAIN_IGNORE_RULE_IDS", "99999").split(",")
                         if i.strip()],
        poll_interval=typed("BRAIN_POLL_INTERVAL", float, 5.0),
        max_backfill=typed("BRAIN_MAX_BACKFILL", int, 3600),
        state_file=get("BRAIN_STATE_FILE", "/var/lib/brain-analyst/state.json"),
        state_max_ids=typed("BRAIN_STATE_MAX_IDS", int, 10000),
        indexer_url=get("BRAIN_INDEXER_URL"),
        indexer_user=get("BRAIN_INDEXER_USER"),
        indexer_password=get("BRAIN_INDEXER_PASSWORD"),
        indexer_verify=ca_file if (ca_file and verify) else verify,
        model=ModelSettings(
            backend=backend,
            model=get("BRAIN_MODEL", "claude-haiku-5-5"),
            api_key=api_key,
            base_url=get("BRAIN_MODEL_BASE_URL", ""),
            timeout=typed("BRAIN_MODEL_TIMEOUT", float, 30.0),
            max_tokens=typed("BRAIN_MODEL_MAX_TOKENS", int, 2048),
            options=json_object("BRAIN_MODEL_OPTIONS", {"effort": "low"}),
        ),
        telegram_bot_token=get("BRAIN_TELEGRAM_BOT_TOKEN"),
        telegram_chat_id=get("BRAIN_TELEGRAM_CHAT_ID"),
        dashboard_url=get("BRAIN_DASHBOARD_URL", ""),
        rule_links=json_object("BRAIN_RULE_LINKS", {}),
    )
