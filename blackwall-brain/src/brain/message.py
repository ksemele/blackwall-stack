"""Telegram message (HTML parse mode) built from a verdict (spec section 5)."""
from datetime import datetime, timedelta, timezone
from html import escape
from urllib.parse import quote

from brain.verdict import Verdict

MAX_LEN = 4096  # Telegram's limit for one message

ICONS = {"critical": "🟥", "high": "🔴", "medium": "🟠", "low": "🟢"}
UNCERTAIN_ICON = "🟡"
VERDICT_TEXT = {"true_positive": "true positive", "false_positive": "false positive",
                "uncertain": "needs a look"}
LINK_WINDOW = timedelta(minutes=15)
# Free-text limits tried in order until the message fits MAX_LEN.
TEXT_LIMITS = (None, 1000, 400, 150, 60)


def level_to_severity(level: int) -> str:
    if level >= 13:
        return "critical"
    if level >= 10:
        return "high"
    if level >= 7:
        return "medium"
    return "low"


def _parse_ts(ts: str) -> datetime | None:
    try:
        return datetime.strptime(ts, "%Y-%m-%dT%H:%M:%S.%f%z").astimezone(timezone.utc)
    except (TypeError, ValueError):
        return None


def _iso(dt: datetime) -> str:
    return dt.strftime("%Y-%m-%dT%H:%M:%S.000Z")


def alert_link(dashboard_url: str, doc_id: str, timestamp: str) -> str:
    """Discover on wazuh-alerts-*, filtered to one document, ±15 min around it."""
    if not dashboard_url:
        return ""
    when = _parse_ts(timestamp) or datetime.now(timezone.utc)
    # Rison: strings in single quotes, ' and ! escaped with !
    rid = doc_id.replace("!", "!!").replace("'", "!'")
    state = (
        f"_g=(time:(from:'{_iso(when - LINK_WINDOW)}',to:'{_iso(when + LINK_WINDOW)}'))"
        "&_a=(metadata:(indexPattern:'wazuh-alerts-*',view:discover))"
        f"&_q=(filters:!(),query:(language:kuery,query:'_id:\"{rid}\"'))"
    )
    return (dashboard_url.rstrip("/") + "/app/data-explorer/discover#?"
            + quote(state, safe="=&():,!'*-_.~"))


def rule_links(links: dict, rule_id: str) -> list[tuple[str, str]]:
    out = []
    for key in (rule_id, "*"):
        out += [(e["title"], e["url"]) for e in links.get(key, [])]
    return out


def _cut(text: str, limit: int | None) -> str:
    if limit is None or len(text) <= limit:
        return text
    return text[:limit - 1] + "…"


def _host(alert: dict) -> str:
    return (alert.get("agent", {}).get("name") or alert.get("manager", {}).get("name")
            or "unknown")


def _rule_line(alert: dict, limit: int | None) -> str:
    rule = alert.get("rule", {})
    when = _parse_ts(alert.get("timestamp", ""))
    ts = when.strftime("%Y-%m-%d %H:%M UTC") if when else escape(alert.get("timestamp", "?"))
    return (f"Rule {escape(str(rule.get('id', '?')))} (level {escape(str(rule.get('level', '?')))}): "
            f"{escape(_cut(rule.get('description', ''), limit))} · {ts}")


def _links_line(alert: dict, doc_id: str, dashboard_url: str, links: dict) -> list[str]:
    parts = []
    url = alert_link(dashboard_url, doc_id, alert.get("timestamp", ""))
    if url:
        parts.append(f'<a href="{escape(url, quote=True)}">Alert in dashboard</a>')
    for title, u in rule_links(links, str(alert.get("rule", {}).get("id", ""))):
        parts.append(f'<a href="{escape(u, quote=True)}">{escape(title)}</a>')
    return ["🔗 " + " · ".join(parts)] if parts else []


def _fit(render) -> str:
    for limit in TEXT_LIMITS:
        msg = render(limit)
        if len(msg) <= MAX_LEN:
            return msg
    return msg


def format_verdict_message(alert: dict, doc_id: str, v: Verdict, dashboard_url: str,
                           links: dict) -> str:
    icon = UNCERTAIN_ICON if v.verdict == "uncertain" else ICONS[v.severity]

    def render(limit):
        lines = [
            f"{icon} <b>{v.severity.upper()}</b> · <code>{escape(_cut(_host(alert), limit))}</code>"
            f" · {VERDICT_TEXT[v.verdict]}",
            escape(_cut(v.summary, limit)),
            f"Category: {v.category} · confidence: {v.confidence}",
            f"Action: {escape(_cut(v.action, limit))}",
        ]
        if v.needs:
            lines.append("Missing: " + escape(_cut("; ".join(v.needs), limit)))
        lines.append(_rule_line(alert, limit))
        lines += _links_line(alert, doc_id, dashboard_url, links)
        return "\n".join(lines)

    return _fit(render)


def format_fallback_message(alert: dict, doc_id: str, reason: str, dashboard_url: str,
                            links: dict) -> str:
    level = alert.get("rule", {}).get("level", 0)
    sev = level_to_severity(level if isinstance(level, int) else 0)

    def render(limit):
        lines = [
            f"{ICONS[sev]} <b>{sev.upper()}</b> · <code>{escape(_cut(_host(alert), limit))}</code>"
            f" · rule level {escape(str(level))}",
            f"<i>AI analysis unavailable: {escape(_cut(reason, limit))}</i>",
            _rule_line(alert, limit),
            f"<pre>{escape(_cut(alert.get('full_log', ''), min(300, limit or 300)))}</pre>",
        ]
        lines += _links_line(alert, doc_id, dashboard_url, links)
        return "\n".join(lines)

    return _fit(render)
