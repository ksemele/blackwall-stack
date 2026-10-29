"""OpenAI-compatible Chat Completions adapter: Ollama, vLLM, llama.cpp server,
LM Studio, OpenRouter and most hosted providers."""
import time

import openai

from brain.models import ModelError, ModelResult, ModelSettings


class OpenAICompatModel:
    def __init__(self, settings: ModelSettings, client=None):
        self.s = settings
        self.name = f"openai_compat/{settings.model}"
        # Local servers ignore the key, but the SDK requires a non-empty one.
        self.client = client or openai.OpenAI(
            api_key=settings.api_key or "unused", base_url=settings.base_url or None,
            timeout=settings.timeout, max_retries=0)

    def _response_format(self, schema: dict | None) -> dict | None:
        mode = self.s.options.get("json_mode", "schema")
        if schema is None or mode == "none":
            return None
        if mode == "object":
            return {"type": "json_object"}
        return {"type": "json_schema",
                "json_schema": {"name": "verdict", "schema": schema, "strict": True}}

    def complete(self, system: str, user: str, schema: dict | None = None) -> ModelResult:
        kwargs = {
            "model": self.s.model,
            "max_tokens": self.s.max_tokens,
            "messages": [{"role": "system", "content": system},
                         {"role": "user", "content": user}],
        }
        fmt = self._response_format(schema)
        if fmt is not None:
            kwargs["response_format"] = fmt

        start = time.monotonic()
        try:
            resp = self.client.chat.completions.create(**kwargs)
        except openai.OpenAIError as exc:
            raise ModelError(f"{type(exc).__name__}: {exc}"[:200]) from exc
        latency = time.monotonic() - start

        text = (resp.choices[0].message.content or "") if resp.choices else ""
        if not text:
            raise ModelError("no text in the answer")
        usage = resp.usage
        return ModelResult(text, getattr(usage, "prompt_tokens", 0) or 0,
                           getattr(usage, "completion_tokens", 0) or 0, latency)
