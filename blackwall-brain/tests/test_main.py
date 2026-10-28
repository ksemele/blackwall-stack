import json
import logging

import httpx
import pytest

from brain import main as brain_main
from brain.analysis import PROMPT_VERSION
from brain.config import load
from brain.indexer import IndexerError
from brain.models import ModelError, ModelResult
from brain.state import State
from brain.telegram import send
from tests.test_config import REQUIRED


class Model:
    name = "fake/m"

    def __init__(self, answer):
        self.answer = answer

    def complete(self, system, user, schema=None):
        if isinstance(self.answer, Exception):
            raise self.answer
        return ModelResult(self.answer, 3, 2, 0.01)


@pytest.fixture
def settings(tmp_path):
    return load({**REQUIRED, "BRAIN_STATE_FILE": str(tmp_path / "state.json")})


def test_handle_sends_verdict_message(alert, verdict_json, settings, caplog):
    sent = []
    caplog.set_level(logging.INFO)
    brain_main.handle("doc1", alert, settings=settings, model=Model(verdict_json),
                      send=lambda *a: sent.append(a) or True, sleep=lambda s: None)
    token, chat, text = sent[0]
    assert (token, chat) == ("t", "1") and text.startswith("🔴 <b>HIGH</b>")
    assert "doc1" in caplog.text and PROMPT_VERSION in caplog.text


def test_handle_falls_back_on_analysis_error(alert, settings):
    sent = []
    brain_main.handle("doc1", alert, settings=settings, model=Model(ModelError("down")),
                      send=lambda *a: sent.append(a) or True, sleep=lambda s: None)
    assert "AI analysis unavailable: down" in sent[0][2]


def test_handle_logs_drop_when_telegram_fails(alert, verdict_json, settings, caplog):
    brain_main.handle("doc1", alert, settings=settings, model=Model(verdict_json),
                      send=lambda *a: False, sleep=lambda s: None)
    assert "dropped" in caplog.text


class Index:
    def __init__(self, *results):
        self.results = list(results)

    def search(self, query):
        r = self.results.pop(0) if self.results else []
        if isinstance(r, Exception):
            raise r
        return r


def hit(i, alert, ts):
    return (i, {**alert, "timestamp": ts})


def test_run_marks_and_saves_after_each_alert(alert, settings, monkeypatch):
    monkeypatch.setattr(brain_main, "handle", lambda doc_id, a, **kw: None)
    index = Index([hit("a", alert, "2026-10-28T14:00:00.000+0000"),
                   hit("b", alert, "2026-10-28T14:00:01.000+0000")])
    brain_main.run(settings, model=None, indexer=index, sleep=lambda s: None, iterations=1)
    st = State.load(settings.state_file, 100)
    assert st.seen == ["a", "b"] and st.cursor == "2026-10-28T14:00:01.000+0000"


def test_run_survives_unexpected_error_in_handle(alert, settings, monkeypatch, caplog):
    calls, sent = [], []

    def handle(doc_id, a, **kw):
        calls.append(doc_id)
        if doc_id == "a":
            raise KeyError("title")

    monkeypatch.setattr(brain_main, "handle", handle)
    monkeypatch.setattr(brain_main.telegram, "send", lambda *a, **k: sent.append(a) or True)
    index = Index([hit("a", alert, "2026-10-28T14:00:00.000+0000"),
                   hit("b", alert, "2026-10-28T14:00:01.000+0000")])
    brain_main.run(settings, model=None, indexer=index, sleep=lambda s: None, iterations=1)
    assert calls == ["a", "b"]
    assert State.load(settings.state_file, 100).seen == ["a", "b"]
    assert "internal error" in sent[0][2] and "KeyError" in caplog.text


def test_run_survives_state_save_failure(alert, settings, monkeypatch, caplog):
    handled = []
    monkeypatch.setattr(brain_main, "handle", lambda doc_id, a, **kw: handled.append(doc_id))

    def boom(self, path):
        raise OSError("disk full")

    monkeypatch.setattr(State, "save", boom)
    index = Index([hit("a", alert, "2026-10-28T14:00:00.000+0000"),
                   hit("b", alert, "2026-10-28T14:00:01.000+0000")])
    brain_main.run(settings, model=None, indexer=index, sleep=lambda s: None, iterations=1)
    assert handled == ["a", "b"] and "disk full" in caplog.text


def test_run_survives_indexer_error(alert, settings, monkeypatch):
    handled = []
    monkeypatch.setattr(brain_main, "handle", lambda doc_id, a, **kw: handled.append(doc_id))
    index = Index(IndexerError("HTTP 401"), [hit("a", alert, "2026-10-28T14:00:00.000+0000")])
    brain_main.run(settings, model=None, indexer=index, sleep=lambda s: None, iterations=2)
    assert handled == ["a"]


def test_main_exits_2_on_bad_config(monkeypatch, capsys):
    monkeypatch.setattr("os.environ", {})
    with pytest.raises(SystemExit) as exc:
        brain_main.main()
    assert exc.value.code == 2
    assert "configuration error: BRAIN_" in capsys.readouterr().err


def test_telegram_sends_html():
    body = {}

    def handler(req):
        body.update(json.loads(req.content))
        return httpx.Response(200, json={"ok": True})

    assert send("tok", "42", "<b>x</b>", client=httpx.Client(transport=httpx.MockTransport(handler)),
                sleep=lambda s: None)
    assert body == {"chat_id": "42", "text": "<b>x</b>", "parse_mode": "HTML",
                    "disable_web_page_preview": True}


def test_telegram_429_waits_retry_after_and_does_not_count():
    answers = [httpx.Response(429, json={"ok": False, "parameters": {"retry_after": 7}}),
               httpx.Response(200, json={"ok": True})]
    slept = []
    ok = send("tok", "42", "x", sleep=slept.append,
              client=httpx.Client(transport=httpx.MockTransport(lambda r: answers.pop(0))))
    assert ok is True and slept == [7]


def test_telegram_429_retry_after_is_capped():
    answers = [httpx.Response(429, json={"parameters": {"retry_after": 999}}),
               httpx.Response(200, json={"ok": True})]
    slept = []
    send("tok", "42", "x", sleep=slept.append,
         client=httpx.Client(transport=httpx.MockTransport(lambda r: answers.pop(0))))
    assert slept == [60]


def test_telegram_400_is_not_retried():
    calls = []

    def handler(req):
        calls.append(1)
        return httpx.Response(400, json={"ok": False, "description": "can't parse entities"})

    ok = send("tok", "42", "x", sleep=lambda s: None,
              client=httpx.Client(transport=httpx.MockTransport(handler)))
    assert ok is False and len(calls) == 1


def test_telegram_retries_then_false():
    calls = []

    def handler(req):
        calls.append(1)
        return httpx.Response(500, text="oops")

    slept = []
    ok = send("tok", "42", "x", client=httpx.Client(transport=httpx.MockTransport(handler)),
              sleep=slept.append)
    assert ok is False and len(calls) == 3 and slept == [2, 4]


def test_setup_logging_quiets_http_client_per_request_lines():
    brain_main.setup_logging()
    assert logging.getLogger("httpx").getEffectiveLevel() >= logging.WARNING


def test_telegram_endless_429_gives_up():
    slept = []
    ok = send("tok", "42", "x", sleep=slept.append, client=httpx.Client(
        transport=httpx.MockTransport(lambda r: httpx.Response(429, json={"parameters": {"retry_after": 1}}))))
    assert ok is False and len(slept) <= 10
