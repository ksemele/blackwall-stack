"""Vendor-neutral model interface (spec section 7).

Brain's code talks only to `Model`. The adapters in this package are the
only place that knows a vendor's request or response format.
"""
from dataclasses import dataclass, field
from typing import Protocol


@dataclass
class ModelResult:
    text: str
    input_tokens: int
    output_tokens: int
    latency_s: float


class ModelError(Exception):
    """Transport, HTTP, timeout or empty answer; str() is a short reason."""


@dataclass
class ModelSettings:
    backend: str
    model: str
    api_key: str
    base_url: str = ""
    timeout: float = 30
    max_tokens: int = 2048
    options: dict = field(default_factory=dict)


class Model(Protocol):
    name: str

    def complete(self, system: str, user: str, schema: dict | None = None) -> ModelResult: ...


def build_model(s: ModelSettings) -> Model:
    if s.backend == "anthropic":
        from brain.models.anthropic_model import AnthropicModel
        return AnthropicModel(s)
    if s.backend == "openai_compat":
        from brain.models.openai_compat import OpenAICompatModel
        return OpenAICompatModel(s)
    raise ValueError(f"unknown model backend: {s.backend!r} (anthropic, openai_compat)")
