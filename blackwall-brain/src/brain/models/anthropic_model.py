"""Anthropic Messages API adapter."""
import time

import anthropic

from brain.models import ModelError, ModelResult, ModelSettings


class AnthropicModel:
    def __init__(self, settings: ModelSettings, client=None):
        self.s = settings
        self.name = f"anthropic/{settings.model}"
        # Retries belong to brain.analysis, not the SDK.
        self.client = client or anthropic.Anthropic(
            api_key=settings.api_key, base_url=settings.base_url or None,
            timeout=settings.timeout, max_retries=0)

    def complete(self, system: str, user: str, schema: dict | None = None) -> ModelResult:
        output_config = {}
        if self.s.options.get("effort"):
            output_config["effort"] = self.s.options["effort"]
        if schema is not None:
            output_config["format"] = {"type": "json_schema", "schema": schema}
        kwargs = {
            "model": self.s.model,
            "max_tokens": self.s.max_tokens,
            "system": system,
            "messages": [{"role": "user", "content": user}],
        }
        if output_config:
            kwargs["output_config"] = output_config
        if self.s.options.get("workspace_id"):
            # For keys not scoped to a workspace.
            kwargs["extra_headers"] = {"anthropic-workspace-id": self.s.options["workspace_id"]}

        start = time.monotonic()
        try:
            resp = self.client.messages.create(**kwargs)
        except anthropic.APIError as exc:
            raise ModelError(f"{type(exc).__name__}: {exc}"[:200]) from exc
        latency = time.monotonic() - start

        # Thinking blocks come first on thinking models; keep only the text.
        text = "".join(b.text for b in resp.content if b.type == "text")
        if not text:
            raise ModelError(f"no text in the answer (stop_reason: {resp.stop_reason})")
        return ModelResult(text, resp.usage.input_tokens, resp.usage.output_tokens, latency)
