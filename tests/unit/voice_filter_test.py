"""Unit check: Voice's alert filtering, dedup and retry in handle_line().

Renders the alerter template with the role defaults and imports it (no
Telegram, no host), then calls handle_line() with synthetic alerts.json lines.
send_telegram, save_sent_id, current_max_level and time.sleep are replaced
with stubs, so nothing touches the network, systemd, a state file or the clock.

Covers the filters the lab only checks end to end:

- a non-JSON line is skipped, not raised;
- a rule in alerter_ignore_rule_ids is dropped, the same level is sent for
  any other rule;
- level < severity_threshold is dropped, level == it is sent;
- while the covering unit is active, level > voice_max_level is dropped
  (Brain sends it) and level == it is sent; while inactive, everything
  >= severity_threshold is sent;
- the same alert ID is sent once, also across a restart (state file reload);
- a send that fails twice is retried and then recorded; a send that always
  fails is dropped after SEND_ATTEMPTS attempts.

Run with the ansible venv python (needs jinja2, yaml):
    python tests/unit/voice_filter_test.py
"""
import importlib.util
import json
import os
import tempfile

import jinja2
import yaml

os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))

ROLE = "blackwall-voice/ansible/roles/telegram-alerter"


def load():
    """Render the role template with its defaults plus a voice_max_level of 9."""
    d = yaml.safe_load(open(f"{ROLE}/defaults/main.yml"))
    ctx = {}
    for k, v in d.items():
        ctx[k] = jinja2.Template(v).render(**ctx) if isinstance(v, str) else v
    # voice_max_level_effective is a set_fact in the role, not a default; the
    # lab sets voice_max_level = Brain's analysis_threshold - 1.
    ctx.update(voice_max_level_effective="9", telegram_bot_token="t", telegram_chat_id="1")
    src = jinja2.Template(open(f"{ROLE}/templates/alerter.py.j2").read()).render(**ctx)
    tmp = tempfile.mkdtemp()
    path = f"{tmp}/alerter_mod.py"
    open(path, "w").write(src)
    spec = importlib.util.spec_from_file_location("alerter_mod", path)
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


m = load()
assert m.SEVERITY_THRESHOLD <= m.MAX_LEVEL, "the test needs threshold <= voice_max_level"

# --- Stubs: record what handle_line did, send nothing, sleep nowhere ---
sent, saved, sleeps, outcomes = [], [], [], []
cover_active = [True]  # current_max_level: MAX_LEVEL only while Brain is active


def send_telegram(text):
    sent.append(text)
    return outcomes.pop(0)


class FakeTime:
    def sleep(self, seconds):
        sleeps.append(seconds)

    def monotonic(self):
        return 0.0


m.send_telegram = send_telegram
m.save_sent_id = lambda alert_id: saved.append(alert_id)
m.current_max_level = lambda: m.MAX_LEVEL if cover_active[0] else None
m.time = FakeTime()


def alert(alert_id, level, rule_id="5710"):
    return json.dumps({
        "id": str(alert_id),
        "timestamp": "2026-01-01T00:00:00",
        "agent": {"name": "black-host-01"},
        "rule": {"id": rule_id, "level": level, "description": "synthetic test alert"},
    })


def run(lines, results, sent_ids=None, active=True):
    """Feed lines through handle_line; reset and return the recorded calls."""
    global sent, saved, sleeps
    sent, saved, sleeps = [], [], []
    outcomes[:] = list(results)
    cover_active[0] = active
    ids = set() if sent_ids is None else sent_ids
    for line in lines:
        m.handle_line(line, ids)
    return sent, ids


threshold, max_level = m.SEVERITY_THRESHOLD, m.MAX_LEVEL

# 1. garbage and a half-written line are skipped, not raised
_, ids = run(["not json at all", '{"id": "1", "rule": {'], [True])
assert not sent, "a non-JSON line must not be sent"

# 2. a rule from alerter_ignore_rule_ids is dropped (as a string and as a number)
ignored = "99998"
assert ignored in m.IGNORE_RULE_IDS, f"{ignored} is no longer an ignored rule id"
_, ids = run([alert(1, threshold, ignored), alert(2, threshold, ignored)], [True, True])
assert not sent, "an ignored rule id must not be sent"
_, ids = run([alert(3, threshold, "5710")], [True])
assert len(sent) == 1, "the same level for another rule must be sent"

# 3. below the threshold is dropped, at it is sent
_, ids = run([alert(4, threshold - 1)], [True])
assert not sent, f"level {threshold - 1} is below the threshold"
_, ids = run([alert(5, threshold)], [True])
assert len(sent) == 1, f"level {threshold} equals the threshold and must be sent"

# 4. the upper bound holds while the covering unit is active, lifted while not
_, ids = run([alert(6, max_level + 1)], [True], active=True)
assert not sent, f"level {max_level + 1} is above voice_max_level {max_level}"
_, ids = run([alert(7, max_level)], [True], active=True)
assert len(sent) == 1, f"level {max_level} is the bound and must be sent"
_, ids = run([alert(8, max_level + 1)], [True], active=False)
assert len(sent) == 1, "with the covering unit inactive Voice sends every level >= threshold"

# 5. the same ID is sent once, in one run and after a restart
line = alert(9, max_level)
sent_ids = set()
_, ids = run([line, line], [True, True], sent_ids)
assert len(sent) == 1, "the same alert ID twice must be sent once"
assert ids is sent_ids and sent_ids == {"9"}, f"sent ID not recorded: {sent_ids}"
_, ids = run([line], [True], sent_ids={"9"})  # state file reloaded after a restart
assert not sent, "an ID already in the state file must not be sent again"

# 6a. two failed sends, then success: retried, and the ID is recorded
_, ids = run([alert(10, max_level)], [False, False, True])
assert len(sent) == 3, f"expected {m.SEND_ATTEMPTS} attempts, got {len(sent)}"
assert sleeps == [m.POLL_INTERVAL] * (m.SEND_ATTEMPTS - 1), f"retry sleeps: {sleeps}"
assert ids == {"10"} and saved == ["10"], f"sent ID not recorded: {ids} / {saved}"
assert not outcomes, "a successful send must not try again"

# 6b. every attempt fails: dropped after SEND_ATTEMPTS, ID not recorded
_, ids = run([alert(11, max_level)], [False] * m.SEND_ATTEMPTS)
assert len(sent) == m.SEND_ATTEMPTS, f"expected exactly {m.SEND_ATTEMPTS} attempts, got {len(sent)}"
assert not ids and not saved, f"a dropped alert must not be recorded as sent: {ids} / {saved}"

print(f"FILTER OK: {threshold}..{max_level}, {m.SEND_ATTEMPTS} attempts, "
      f"ignored rules {sorted(m.IGNORE_RULE_IDS)}")
