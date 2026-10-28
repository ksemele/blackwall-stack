"""Unit check: Voice and Brain lose no alerts.json lines around rotation.

Renders both templates with the role defaults, replaces handle_line with a
collector and runs tail_alerts against a temp file in a background thread.
Covers what Wazuh and a busy writer do to alerts.json:

- a line written in two parts (newline last) is parsed once, whole;
- rotation (rename + new file): lines appended to the old file just before the
  rename, and lines in the new file written before the tail notices, all
  arrive, in order;
- a file truncated in place is reread from its start.

Run with the ansible venv python (needs jinja2, yaml):
    python tests/unit/alerts_tail_test.py
"""
import importlib.util
import json
import os
import tempfile
import threading
import time

import jinja2
import yaml

os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))

POLL = 0.05

MODULES = {
    "voice": ("blackwall-voice/ansible/roles/telegram-alerter", "alerter.py.j2",
              {"severity_threshold": 10, "voice_max_level_effective": "",
               "telegram_bot_token": "t", "telegram_chat_id": "1"}),
    "brain": ("blackwall-brain/ansible/roles/brain-analyst", "brain.py.j2",
              {"telegram_bot_token": "t", "telegram_chat_id": "1"}),
}


def load(name, role, template, extra, log_path, tmp):
    d = yaml.safe_load(open(f"{role}/defaults/main.yml"))
    ctx = {}
    for k, v in d.items():
        ctx[k] = jinja2.Template(v).render(**ctx) if isinstance(v, str) else v
    ctx.update(extra)
    ctx.update(poll_interval=POLL, alerter_alert_log=log_path, brain_alert_log=log_path,
               alerter_state_file=f"{tmp}/{name}-state", brain_state_file=f"{tmp}/{name}-state")
    src = jinja2.Template(open(f"{role}/templates/{template}").read()).render(**ctx)
    path = f"{tmp}/{name}_mod.py"
    open(path, "w").write(src)
    spec = importlib.util.spec_from_file_location(f"{name}_mod", path)
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


def alert(n):
    return (json.dumps({"id": str(n), "rule": {"id": "5710", "level": 10}}) + "\n").encode()


def wait_for(got, n, what):
    deadline = time.monotonic() + 5
    while time.monotonic() < deadline:
        if len(got) >= n:
            return
        time.sleep(POLL)
    raise AssertionError(f"{what}: expected {n} lines, got {len(got)}: {got}")


def run(name):
    role, template, extra = MODULES[name]
    tmp = tempfile.mkdtemp()
    log_path = f"{tmp}/alerts.json"
    with open(log_path, "wb") as f:
        f.write(alert(0))  # before start: must not be replayed

    m = load(name, role, template, extra, log_path, tmp)
    got = []
    m.handle_line = lambda line, sent_ids: got.append(json.loads(line)["id"])
    threading.Thread(target=m.tail_alerts, args=(set(),), daemon=True).start()
    time.sleep(POLL * 4)  # let it open the file and seek to its end

    w = open(log_path, "ab", buffering=0)

    # 1. plain line, then one written in two parts
    w.write(alert(1))
    half = alert(2)
    w.write(half[:10])
    time.sleep(POLL * 4)  # the tail sees the half line and must hold it
    w.write(half[10:])
    wait_for(got, 2, f"{name}: split line")

    # 2. rotation: last writes land in the old file, the new file fills up
    #    before the tail notices
    w.write(alert(3))
    os.rename(log_path, log_path + ".1")
    w.write(alert(4))  # writer still holds the old file
    w.close()
    w = open(log_path, "ab", buffering=0)
    w.write(alert(5) + alert(6))
    wait_for(got, 6, f"{name}: rotation")

    # 3. truncation in place
    time.sleep(POLL * 4)
    w.truncate(0)
    w.seek(0)
    time.sleep(POLL * 4)
    w.write(alert(7))
    wait_for(got, 7, f"{name}: truncation")
    w.close()

    assert got == ["1", "2", "3", "4", "5", "6", "7"], f"{name}: {got}"
    print(f"{name}: TAIL OK {got}")


for name in MODULES:
    run(name)
