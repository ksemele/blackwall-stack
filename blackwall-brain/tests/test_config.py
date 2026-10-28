import pytest

from brain.config import load

REQUIRED = {
    "BRAIN_INDEXER_URL": "https://127.0.0.1:9200",
    "BRAIN_INDEXER_USER": "brain-ro",
    "BRAIN_INDEXER_PASSWORD": "pw",
    "BRAIN_MODEL_API_KEY": "k",
    "BRAIN_TELEGRAM_BOT_TOKEN": "t",
    "BRAIN_TELEGRAM_CHAT_ID": "1",
}


def test_config_defaults():
    s = load(REQUIRED)
    assert s.analysis_threshold == 7 and s.ignore_rule_ids == ["99999"]
    assert s.poll_interval == 5 and s.max_backfill == 3600
    assert s.state_file == "/var/lib/brain-analyst/state.json" and s.state_max_ids == 10000
    assert s.indexer_verify is True
    assert s.model.backend == "anthropic" and s.model.model == "claude-haiku-5-5"
    assert s.model.timeout == 30 and s.model.max_tokens == 2048
    assert s.model.options == {"effort": "low"}
    assert s.dashboard_url == "" and s.rule_links == {}


def test_config_overrides():
    s = load({**REQUIRED, "BRAIN_ANALYSIS_THRESHOLD": "10", "BRAIN_IGNORE_RULE_IDS": "1, 2,",
              "BRAIN_INDEXER_VERIFY_TLS": "false", "BRAIN_MODEL_BACKEND": "openai_compat",
              "BRAIN_MODEL_OPTIONS": '{"json_mode": "object"}',
              "BRAIN_RULE_LINKS": '{"1": [{"title": "R", "url": "u"}]}'})
    assert s.analysis_threshold == 10 and s.ignore_rule_ids == ["1", "2"]
    assert s.indexer_verify is False and s.model.backend == "openai_compat"
    assert s.model.options == {"json_mode": "object"}
    assert s.rule_links == {"1": [{"title": "R", "url": "u"}]}


def test_ca_file_is_used_for_verify():
    assert load({**REQUIRED, "BRAIN_INDEXER_CA_FILE": "/ca.pem"}).indexer_verify == "/ca.pem"


def test_config_required_named():
    env = dict(REQUIRED)
    del env["BRAIN_INDEXER_URL"]
    with pytest.raises(ValueError, match="BRAIN_INDEXER_URL"):
        load(env)


def test_api_key_optional_for_openai_compat():
    env = {k: v for k, v in REQUIRED.items() if k != "BRAIN_MODEL_API_KEY"}
    assert load({**env, "BRAIN_MODEL_BACKEND": "openai_compat"}).model.api_key == ""
    with pytest.raises(ValueError, match="BRAIN_MODEL_API_KEY"):
        load(env)


@pytest.mark.parametrize("name,value", [("BRAIN_MODEL_OPTIONS", "{bad"),
                                        ("BRAIN_RULE_LINKS", "[]"),
                                        ("BRAIN_ANALYSIS_THRESHOLD", "ten"),
                                        ("BRAIN_INDEXER_VERIFY_TLS", "maybe")])
def test_config_bad_value_named(name, value):
    with pytest.raises(ValueError, match=name):
        load({**REQUIRED, name: value})
