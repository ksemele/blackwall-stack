import pathlib
from types import SimpleNamespace as NS

import anthropic
import httpx
import openai
import pytest

from brain.models import ModelError, ModelSettings, build_model
from brain.models.anthropic_model import AnthropicModel
from brain.models.openai_compat import OpenAICompatModel

SCHEMA = {"type": "object", "properties": {"a": {"type": "string"}},
          "required": ["a"], "additionalProperties": False}


class Recorder:
    """Fake SDK endpoint: records kwargs, returns or raises what it was given."""

    def __init__(self, result):
        self.result, self.calls = result, []

    def create(self, **kwargs):
        self.calls.append(kwargs)
        if isinstance(self.result, Exception):
            raise self.result
        return self.result


def anthropic_client(result):
    rec = Recorder(result)
    return NS(messages=rec), rec


def openai_client(result):
    rec = Recorder(result)
    return NS(chat=NS(completions=rec)), rec


def anthropic_reply(*blocks, stop="end_turn"):
    return NS(content=list(blocks), stop_reason=stop,
              usage=NS(input_tokens=11, output_tokens=7))


def settings(backend="anthropic", **kw):
    return ModelSettings(backend=backend, model="m", api_key="k", **kw)


def test_anthropic_joins_text_blocks_skips_thinking():
    client, _ = anthropic_client(anthropic_reply(
        NS(type="thinking", thinking=""), NS(type="text", text='{"a":'),
        NS(type="text", text='"x"}')))
    res = AnthropicModel(settings(), client=client).complete("sys", "user")
    assert res.text == '{"a":"x"}'
    assert (res.input_tokens, res.output_tokens) == (11, 7)
    assert res.latency_s >= 0


def test_anthropic_passes_effort_schema_and_workspace_header():
    client, rec = anthropic_client(anthropic_reply(NS(type="text", text="{}")))
    s = settings(options={"effort": "low", "workspace_id": "wrkspc_1"}, max_tokens=99)
    AnthropicModel(s, client=client).complete("sys", "user", schema=SCHEMA)
    call = rec.calls[0]
    assert call["model"] == "m" and call["max_tokens"] == 99 and call["system"] == "sys"
    assert call["messages"] == [{"role": "user", "content": "user"}]
    assert call["output_config"] == {"effort": "low",
                                     "format": {"type": "json_schema", "schema": SCHEMA}}
    assert call["extra_headers"] == {"anthropic-workspace-id": "wrkspc_1"}


def test_anthropic_without_text_raises_with_stop_reason():
    client, _ = anthropic_client(anthropic_reply(NS(type="thinking", thinking=""),
                                                 stop="max_tokens"))
    with pytest.raises(ModelError, match="max_tokens"):
        AnthropicModel(settings(), client=client).complete("s", "u")


def test_anthropic_sdk_error_becomes_model_error():
    req = httpx.Request("POST", "https://api.example.invalid/v1/messages")
    client, _ = anthropic_client(anthropic.APIConnectionError(request=req))
    with pytest.raises(ModelError):
        AnthropicModel(settings(), client=client).complete("s", "u")


def openai_reply(text, usage=True):
    return NS(choices=[NS(message=NS(content=text))],
              usage=NS(prompt_tokens=5, completion_tokens=3) if usage else None)


def test_openai_schema_mode_sends_json_schema_response_format():
    client, rec = openai_client(openai_reply('{"a":"x"}'))
    res = OpenAICompatModel(settings("openai_compat"), client=client).complete(
        "sys", "user", schema=SCHEMA)
    call = rec.calls[0]
    assert call["messages"] == [{"role": "system", "content": "sys"},
                                {"role": "user", "content": "user"}]
    assert call["response_format"] == {
        "type": "json_schema",
        "json_schema": {"name": "verdict", "schema": SCHEMA, "strict": True}}
    assert res.text == '{"a":"x"}' and (res.input_tokens, res.output_tokens) == (5, 3)


@pytest.mark.parametrize("mode,expected", [("object", {"type": "json_object"}), ("none", None)])
def test_openai_object_and_none_modes(mode, expected):
    client, rec = openai_client(openai_reply("{}"))
    OpenAICompatModel(settings("openai_compat", options={"json_mode": mode}),
                      client=client).complete("s", "u", schema=SCHEMA)
    assert rec.calls[0].get("response_format") == expected


def test_openai_missing_usage_is_zero():
    client, _ = openai_client(openai_reply("{}", usage=False))
    res = OpenAICompatModel(settings("openai_compat"), client=client).complete("s", "u")
    assert (res.input_tokens, res.output_tokens) == (0, 0)


def test_openai_sdk_error_becomes_model_error():
    req = httpx.Request("POST", "http://127.0.0.1:11434/v1/chat/completions")
    client, _ = openai_client(openai.APIConnectionError(request=req))
    with pytest.raises(ModelError):
        OpenAICompatModel(settings("openai_compat"), client=client).complete("s", "u")


def test_build_model_picks_backend():
    assert isinstance(build_model(settings()), AnthropicModel)
    assert isinstance(build_model(settings("openai_compat", base_url="http://127.0.0.1:11434/v1")),
                      OpenAICompatModel)


def test_build_model_unknown_backend_raises():
    with pytest.raises(ValueError, match="gemini"):
        build_model(settings("gemini"))


def test_no_vendor_import_outside_adapters():
    src = pathlib.Path(__file__).parent.parent / "src" / "brain"
    allowed = {"anthropic": "anthropic_model.py", "openai": "openai_compat.py"}
    for path in src.rglob("*.py"):
        text = path.read_text()
        for vendor, owner in allowed.items():
            if f"import {vendor}" in text or f"from {vendor}" in text:
                assert path.name == owner, f"{path} imports {vendor}"
