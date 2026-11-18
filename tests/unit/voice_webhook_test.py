"""Unit check: Voice's webhook payload builders and secret redaction.

Renders the alerter template with the role defaults (webhook vars set to a
placeholder URL) and imports it (no network, no host). Checks:

- `generic` payload is the documented JSON object: level, rule_id,
  description, agent, timestamp, log — the excerpt truncated to 300 chars;
- `slack` payload is {"text": ...} with the alert rendered as mrkdwn,
  chosen by alerter_webhook_format;
- the logging filter keeps the webhook URL and the bot token out of every
  log line, including exception text (a webhook URL is a bearer secret);
- no channel configured -> deliver() reports failure (ID not recorded).

Run with the ansible venv python (needs jinja2, yaml):
    python tests/unit/voice_webhook_test.py
"""
import importlib.util
import io
import logging
import os
import tempfile

import jinja2
import yaml

os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))

ROLE = "blackwall-voice/ansible/roles/telegram-alerter"
URL = "https://hooks.example.com/services/T0000000/B0000000/XXXXXXXXXXXXXXXX"
TOKEN = "123456:BOT-PH-SECRET"

ALERT = {
    "id": "42",
    "timestamp": "2026-11-18T12:34:56.789012+00:00",
    "rule": {"id": "5710", "level": 10, "description": "First time user login"},
    "agent": {"name": "black-host-01"},
    "manager": {"name": "black-master-00"},
    "full_log": "x" * 400,
}


def load_alerter(tmp, webhook_format="generic"):
    d = yaml.safe_load(open(f"{ROLE}/defaults/main.yml"))
    ctx = {}
    for k, v in d.items():
        ctx[k] = jinja2.Template(v).render(**ctx) if isinstance(v, str) else v
    ctx.update(severity_threshold=5, voice_max_level_effective="",
               telegram_bot_token=TOKEN, telegram_chat_id="1",
               alerter_webhook_url=URL, alerter_webhook_format=webhook_format,
               alerter_state_file=f"{tmp}/state", alerter_log_file=f"{tmp}/log")
    src = jinja2.Template(open(f"{ROLE}/templates/alerter.py.j2").read()).render(**ctx)
    path = f"{tmp}/alerter_{webhook_format}.py"
    open(path, "w").write(src)
    spec = importlib.util.spec_from_file_location(f"alerter_{webhook_format}", path)
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


def main():
    tmp = tempfile.mkdtemp()
    m = load_alerter(tmp)

    # --- generic payload: pure builder, documented shape ---
    g = m.format_webhook_generic(ALERT)
    assert g == {
        "level": 10,
        "rule_id": "5710",
        "description": "First time user login",
        "agent": "black-host-01",
        "timestamp": "2026-11-18T12:34:56.789012+00:00",
        "log": "x" * 300,  # excerpt capped at WEBHOOK_LOG_CHARS
    }, g
    assert m.build_webhook_payload(ALERT) == g
    # agent falls back to the manager name (agentless alerts)
    assert m.format_webhook_generic({"rule": {}})["agent"] == "unknown"
    empty = m.format_webhook_generic({})
    assert empty["level"] == 0 and empty["log"] == ""

    # --- slack payload: {"text": ...} chosen by alerter_webhook_format ---
    ms = load_alerter(tmp, webhook_format="slack")
    s = ms.build_webhook_payload(ALERT)
    assert list(s.keys()) == ["text"] and isinstance(s["text"], str)
    assert "*BLACKWALL ALERT* level 10" in s["text"]
    assert "Host: `black-host-01`" in s["text"]
    assert "Rule: First time user login" in s["text"]
    assert "Time: 2026-11-18 12:34:56" in s["text"]
    assert s["text"].count("```") == 2 and "x" * 300 in s["text"]

    # --- secret redaction: neither URL nor token may reach the log ---
    buf = io.StringIO()
    m.log.addHandler(logging.StreamHandler(buf))
    m.log.error("Webhook send failed: %s", ValueError(f"unknown url type: {URL}"))
    try:
        raise ValueError(f"URL can't contain control characters: {URL}")
    except ValueError:
        m.log.exception("Error handling alert line")
    out = buf.getvalue()
    assert URL not in out and TOKEN not in out, out
    assert "<redacted>" in out and "Webhook send failed" in out, out

    # --- no channel configured -> deliver() False, ID not recorded ---
    m0 = load_alerter(tmp, webhook_format="generic")
    # Re-render with empty secrets: the deploy assert normally prevents this,
    # the runtime guard must keep deliver() honest anyway.
    m0.TELEGRAM_ON = False
    m0.WEBHOOK_ON = False
    sent = set()
    m0.handle_line('{"id":"99","rule":{"id":"5710","level":10}}', sent)
    assert "99" not in sent and m0.load_sent_ids() == set()

    # --- two channels: ID recorded only when both accept ---
    ok = {"telegram": False, "webhook": False}
    m0.TELEGRAM_ON = True
    m0.WEBHOOK_ON = True
    m0.SEND_ATTEMPTS = 1
    m0.send_telegram = lambda text: ok["telegram"]
    m0.send_webhook = lambda payload: ok["webhook"]
    sent = set()
    m0.handle_line('{"id":"100","rule":{"id":"5710","level":10}}', sent)
    assert "100" not in sent  # telegram accepted? no: both fail -> not sent
    ok["telegram"] = True
    m0.handle_line('{"id":"101","rule":{"id":"5710","level":10}}', sent)
    assert "101" not in sent  # one channel accepted, the other failed -> not "sent"
    ok["webhook"] = True
    m0.handle_line('{"id":"102","rule":{"id":"5710","level":10}}', sent)
    assert "102" in sent  # every configured channel accepted -> recorded
    assert "102" in open(f"{tmp}/state").read()

    print("WEBHOOK OK")


if __name__ == "__main__":
    main()
