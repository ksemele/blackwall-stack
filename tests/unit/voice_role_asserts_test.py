"""Unit check: the role's channel/URL/format assert expressions really evaluate.

Regression guard for review round 1 (PR #42): the URL check was written as
`alerter_webhook_url | regex('^https?://')`, but Ansible has no `regex`
filter — `regex`/`match` are tests. The whole assert task then died on
every deploy (even Telegram-only) with "No filter named 'regex'", and plain
Jinja-only tests could not see it. repo-check has no ansible lint, so this
test evaluates the actual tasks from tasks/main.yml.

Primary path (ansible-playbook on PATH, as on the check server): run the
three assert tasks verbatim in a localhost playbook (`-c local`) over the
five documented var combinations; check pass/fail per combination, the
expected fail_msg on failures, and that the URL never appears in output
(`no_log`).

Fallback (no ansible installed): evaluate the extracted `that` expressions in
Jinja with Ansible's `regex`/`match` tests registered (start-anchored,
re.match) and no `regex` filter — reproducing exactly the filter/test
resolution failure the bug caused.

Run with the ansible venv python (needs jinja2, yaml):
    python tests/unit/voice_role_asserts_test.py
"""
import os
import re
import shutil
import subprocess
import tempfile

import jinja2
import yaml

os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))

ROLE = "blackwall-voice/ansible/roles/telegram-alerter"
URL = "https://hooks.example.com/services/T0000000/B0000000/XXXXXXXXXXXXXXXX"

# The three guard tasks, by name — the test follows them if the expressions change.
WANTED = ["Require an alert channel", "Validate the webhook URL", "Validate the webhook format"]

COMBOS = [
    # (vars, expected task outcomes per WANTED task, tag)
    ({"telegram_bot_token": "123456:FAKE-BOT-TOKEN", "telegram_chat_id": "42",
      "alerter_webhook_url": "", "alerter_webhook_format": "generic"},
     (True, True, True), "telegram-only (the ordinary deploy)"),
    ({"telegram_bot_token": "", "telegram_chat_id": "",
      "alerter_webhook_url": URL, "alerter_webhook_format": "generic"},
     (True, True, True), "valid webhook only"),
    ({"telegram_bot_token": "", "telegram_chat_id": "",
      "alerter_webhook_url": "ftp://hooks.example.com/services/X", "alerter_webhook_format": "generic"},
     (True, False, True), "URL scheme refused"),
    ({"telegram_bot_token": "123456:FAKE-BOT-TOKEN", "telegram_chat_id": "42",
      "alerter_webhook_url": URL, "alerter_webhook_format": "xml"},
     (True, True, False), "format refused"),
    ({"telegram_bot_token": "", "telegram_chat_id": "",
      "alerter_webhook_url": "", "alerter_webhook_format": "generic"},
     (False, True, True), "no channel"),
]

EXPECTED_MSG = {1: "must start with http:// or https://",
                2: "must be 'generic' or 'slack'",
                0: "No alert channel configured"}


def load_assert_tasks():
    tasks = yaml.safe_load(open(f"{ROLE}/tasks/main.yml"))
    selected = [t for t in tasks if isinstance(t, dict) and t.get("name") in WANTED]
    assert [t["name"] for t in selected] == WANTED, [t.get("name") for t in selected]
    for t in selected:
        assert t["ansible.builtin.assert"].get("quiet") is True
    assert selected[1].get("no_log") is True, "URL assert must keep no_log: true"
    return selected


def run_with_ansible(selected, tmp):
    ansible_playbook = shutil.which("ansible-playbook")
    for vars_, expected, tag in COMBOS:
        play = [{"name": "voice role asserts", "hosts": "localhost",
                 "gather_facts": False, "vars": vars_, "tasks": selected}]
        path = f"{tmp}/voice_asserts.yml"
        with open(path, "w") as f:
            yaml.safe_dump(play, f)
        env = dict(os.environ, ANSIBLE_LOCALHOST_WARNING="false", ANSIBLE_NOCOLOR="1")
        proc = subprocess.run(
            [ansible_playbook, "-c", "local", "-i", "localhost,", path],
            capture_output=True, text=True, timeout=180, env=env)
        out = proc.stdout + proc.stderr
        recap = re.search(r"failed=(\d+)", out)
        assert recap, f"[{tag}] no PLAY RECAP in output:\n{out}"
        failed = int(recap.group(1))
        assert (failed == 0) == all(expected), f"[{tag}] failed={failed}, expected {expected}:\n{out}"
        for i, want_pass in enumerate(expected):
            if not want_pass:
                msg = EXPECTED_MSG[i]
                assert msg in out, f"[{tag}] fail_msg for '{WANTED[i]}' not shown:\n{out}"
        url = vars_["alerter_webhook_url"]
        token = vars_["telegram_bot_token"]
        if url:
            assert url not in out, f"[{tag}] webhook URL leaked into output"
        if token:
            assert token not in out, f"[{tag}] bot token leaked into output"
    return "ansible-core"


def run_with_jinja(selected, tmp):
    # Mirror ansible-core's resolution: `regex`/`match` exist as tests
    # (start-anchored, re.match), `regex` does NOT exist as a filter —
    # `x | regex(...)` must blow up at template compile, like in the role bug.
    env = jinja2.Environment()
    anchored = lambda v, p: bool(re.match(p, str(v)))  # noqa: E731
    env.tests["match"] = anchored
    env.tests["regex"] = anchored
    for vars_, expected, tag in COMBOS:
        for i, task in enumerate(selected):
            for expr in task["ansible.builtin.assert"]["that"]:
                try:
                    out = env.from_string("{{ (1 if (" + expr + ") else 0) }}").render(**vars_)
                    value = out.strip() == "1"
                except jinja2.TemplateError as exc:
                    raise AssertionError(f"[{tag}] '{WANTED[i]}' does not compile: {exc}") from exc
                assert value == expected[i], (
                    f"[{tag}] '{WANTED[i]}' -> {value}, expected {expected[i]}: {expr!r}")
    return "jinja fallback"


def main():
    tmp = tempfile.mkdtemp()
    selected = load_assert_tasks()
    engine = run_with_ansible(selected, tmp) if shutil.which("ansible-playbook") \
        else run_with_jinja(selected, tmp)
    print(f"ROLE ASSERTS OK ({engine})")


if __name__ == "__main__":
    main()
