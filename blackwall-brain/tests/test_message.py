import pytest

from brain.message import (
    MAX_LEN, alert_link, format_fallback_message, format_verdict_message,
    level_to_severity, rule_links,
)
from brain.verdict import Verdict

DASH = "https://localhost:5601"


def verdict(**kw):
    base = dict(verdict="true_positive", severity="high", category="privilege_escalation",
                summary="New sudoers drop-in", action="Revert it if unknown",
                confidence="medium", needs=[])
    return Verdict(**{**base, **kw})


def test_true_positive_layout(alert):
    msg = format_verdict_message(alert, "doc1", verdict(), "", {})
    assert msg == (
        "🔴 <b>HIGH</b> · <code>black-host-01</code> · true positive\n"
        "New sudoers drop-in\n"
        "Category: privilege_escalation · confidence: medium\n"
        "Action: Revert it if unknown\n"
        "Rule 100120 (level 10): Sudoers file changed · 2026-10-28 14:02 UTC"
    )


@pytest.mark.parametrize("sev,icon", [("critical", "🟥"), ("high", "🔴"),
                                      ("medium", "🟠"), ("low", "🟢")])
def test_severity_icons(alert, sev, icon):
    assert format_verdict_message(alert, "d", verdict(severity=sev), "", {}).startswith(icon)


def test_false_positive_text(alert):
    msg = format_verdict_message(alert, "d", verdict(verdict="false_positive", severity="low"), "", {})
    assert msg.splitlines()[0].endswith("· false positive")


def test_uncertain_is_yellow_even_when_critical(alert):
    first = format_verdict_message(alert, "d", verdict(verdict="uncertain", severity="critical"),
                                   "", {}).splitlines()[0]
    assert first == "🟡 <b>CRITICAL</b> · <code>black-host-01</code> · needs a look"


def test_missing_line_only_with_needs(alert):
    msg = format_verdict_message(alert, "d", verdict(needs=["admin list", "deploy log"]), "", {})
    assert "Missing: admin list; deploy log" in msg.splitlines()
    assert "Missing" not in format_verdict_message(alert, "d", verdict(), "", {})


def test_no_link_line_without_dashboard_or_rule_links(alert):
    assert "🔗" not in format_verdict_message(alert, "d", verdict(), "", {})


def test_link_line_with_dashboard_and_rule_links(alert):
    links = {"100120": [{"title": "Runbook", "url": "https://wiki.example.org/sudo"}]}
    last = format_verdict_message(alert, "d", verdict(), DASH, links).splitlines()[-1]
    assert last.startswith('🔗 <a href="https://localhost:5601/app/')
    assert last.endswith('>Alert in dashboard</a> · <a href="https://wiki.example.org/sudo">Runbook</a>')


def test_rule_links_specific_then_wildcard():
    links = {"*": [{"title": "All", "url": "u2"}], "1": [{"title": "One", "url": "u1"}]}
    assert rule_links(links, "1") == [("One", "u1"), ("All", "u2")]
    assert rule_links(links, "2") == [("All", "u2")]
    assert rule_links({}, "1") == []


def test_html_escaped(alert):
    alert["agent"]["name"] = "<script>&"
    alert["rule"]["description"] = "a<b>"
    msg = format_verdict_message(alert, "d", verdict(summary="x & <y>"), "", {})
    assert "&lt;script&gt;&amp;" in msg and "a&lt;b&gt;" in msg and "x &amp; &lt;y&gt;" in msg
    assert "<script>" not in msg


def test_huge_fields_fit_max_len(alert):
    alert["full_log"] = "z" * 20000
    v = verdict(summary="s" * 5000, action="a" * 5000, needs=["n" * 3000])
    msg = format_verdict_message(alert, "d", v, DASH, {})
    assert len(msg) <= MAX_LEN and msg.startswith("🔴 <b>HIGH</b>")
    fb = format_fallback_message(alert, "d", "boom", DASH, {})
    assert len(fb) <= MAX_LEN and fb.count("<pre>") == fb.count("</pre>") == 1


@pytest.mark.parametrize("level,sev", [(0, "low"), (6, "low"), (7, "medium"), (9, "medium"),
                                       (10, "high"), (12, "high"), (13, "critical"), (15, "critical")])
def test_level_to_severity(level, sev):
    assert level_to_severity(level) == sev


def test_fallback_layout(alert):
    msg = format_fallback_message(alert, "d", "timeout <x>", "", {})
    assert msg == (
        "🔴 <b>HIGH</b> · <code>black-host-01</code> · rule level 10\n"
        "<i>AI analysis unavailable: timeout &lt;x&gt;</i>\n"
        "Rule 100120 (level 10): Sudoers file changed · 2026-10-28 14:02 UTC\n"
        "<pre>type=SYSCALL msg=audit(...): comm=&quot;tee&quot; name=&quot;/etc/sudoers.d/90-app&quot;</pre>"
    )


def test_alert_link_empty_without_base():
    assert alert_link("", "abc", "2026-10-28T14:02:03.123+0000") == ""


def test_alert_link_contains_doc_id_and_time_range():
    url = alert_link(DASH + "/", "AbC-12_x", "2026-10-28T14:02:03.123+0000")
    assert url.startswith("https://localhost:5601/app/data-explorer/discover")
    assert "AbC-12_x" in url and "wazuh-alerts-*" in url
    assert "2026-10-28T13:47:03" in url and "2026-10-28T14:17:03" in url
    assert " " not in url and '"' not in url
