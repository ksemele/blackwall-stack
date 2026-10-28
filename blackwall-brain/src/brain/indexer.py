"""Alert intake: poll wazuh-alerts-* in the Wazuh indexer (OpenSearch)."""
import logging
from datetime import datetime, timedelta, timezone

import httpx

from brain.state import State, parse_ts

log = logging.getLogger(__name__)

MAX_PAGES = 50  # per poll; the rest waits for the next poll


class IndexerError(Exception):
    """Indexer unreachable, TLS failure, timeout or non-200 answer."""


def build_query(since: str, min_level: int, ignore_rule_ids: list[str], size: int) -> dict:
    query: dict = {"filter": [
        {"range": {"rule.level": {"gte": min_level}}},
        {"range": {"timestamp": {"gte": since}}},
    ]}
    if ignore_rule_ids:
        query["must_not"] = [{"terms": {"rule.id": list(ignore_rule_ids)}}]
    return {"size": size, "sort": [{"timestamp": {"order": "asc"}}],
            "query": {"bool": query}}


class IndexerClient:
    def __init__(self, url: str, user: str, password: str, verify: bool | str,
                 client: httpx.Client | None = None, timeout: float = 15):
        self.url = url.rstrip("/")
        self.client = client or httpx.Client(verify=verify, timeout=timeout)
        self.auth = (user, password)

    def search(self, query: dict) -> list[tuple[str, dict]]:
        try:
            resp = self.client.post(f"{self.url}/wazuh-alerts-*/_search", json=query,
                                    auth=self.auth)
        except httpx.HTTPError as exc:
            raise IndexerError(f"{type(exc).__name__}: {exc}") from exc
        if resp.status_code != 200:
            raise IndexerError(f"HTTP {resp.status_code}: {resp.text[:200]}")
        try:
            hits = resp.json()["hits"]["hits"]
            return [(h["_id"], h["_source"]) for h in hits]
        except (ValueError, KeyError, TypeError) as exc:
            raise IndexerError(f"unexpected search response: {exc}") from exc


def _fmt(dt: datetime) -> str:
    dt = dt.astimezone(timezone.utc)
    return dt.strftime("%Y-%m-%dT%H:%M:%S.") + f"{dt.microsecond // 1000:03d}Z"


def poll_once(client, state: State, *, min_level: int, ignore_rule_ids: list[str],
              overlap_s: int = 10, max_backfill_s: int = 3600, page_size: int = 200,
              now: datetime | None = None) -> list[tuple[str, dict]]:
    """New alerts in timestamp order. Does not mark them: the caller marks each
    one after handling it, so a crash re-delivers only the unhandled ones."""
    now = now or datetime.now(timezone.utc)
    floor = now - timedelta(seconds=max_backfill_s)
    since = floor
    if state.cursor is not None:
        # Overlap: the indexer can show an alert a few seconds after its timestamp.
        wanted = parse_ts(state.cursor) - timedelta(seconds=overlap_s)
        if wanted < floor:
            log.warning("skipping alerts from %s to %s: older than brain_max_backfill (%d s)",
                        state.cursor, _fmt(floor), max_backfill_s)
        since = max(wanted, floor)
    since_s = _fmt(since)

    for _ in range(MAX_PAGES):
        hits = client.search(build_query(since_s, min_level, ignore_rule_ids, page_size))
        new = [(i, src) for i, src in hits if not state.is_seen(i)]
        if new or len(hits) < page_size:
            return new
        # A full page of already processed alerts: move past it.
        last = _fmt(parse_ts(hits[-1][1]["timestamp"]))
        if last == since_s:
            log.warning("more than %d processed alerts share timestamp %s", page_size, last)
            return []
        since_s = last
    return []
