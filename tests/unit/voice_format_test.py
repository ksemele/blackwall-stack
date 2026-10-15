"""Unit check: Voice's formatter escapes Markdown in dynamic fields.

Renders the alerter template with the role defaults and imports it (no
Telegram, no host). Run with the ansible venv python (needs jinja2, yaml):
    python tests/unit/voice_format_test.py
"""
import sys, importlib.util, tempfile, jinja2, yaml
import os; os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
role="blackwall-voice/ansible/roles/telegram-alerter"
d=yaml.safe_load(open(f"{role}/defaults/main.yml")); ctx={}
for k,v in d.items(): ctx[k]=jinja2.Template(v).render(**ctx) if isinstance(v,str) else v
ctx.update(severity_threshold=10, voice_max_level_effective="9", telegram_bot_token="t", telegram_chat_id="1")
src=jinja2.Template(open(f"{role}/templates/alerter.py.j2").read()).render(**ctx)
t=tempfile.mkdtemp(); open(t+"/m.py","w").write(src)
spec=importlib.util.spec_from_file_location("m",t+"/m.py"); m=importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
msg=m.format_message({"rule":{"level":10,"description":"a_b *c* [d] `e`"},"agent":{"name":"h_1"},"full_log":"x ``` y","timestamp":"2026-01-01T00:00:00"})
print(msg)
assert "a\\_b \\*c\\* \\[d] \\`e\\`" in msg, msg
assert "h\\_1" in msg and msg.count("```") == 2, msg
print("FORMAT OK")
