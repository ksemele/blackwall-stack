import json
from datetime import datetime, timedelta, timezone

import httpx
import pytest

from brain.indexer import IndexerClient, IndexerError, build_query, poll_once
from brain.state import State

NOW = datetime(2026, 10, 28, 15, 0, 0, tzinfo=timezone.utc)


def ts(dt):
    return dt.strftime("%Y-%m-%dT%H:%M:%S.") + f"{dt.microsecond // 1000:03d}+0000"


def test_query_shape():
    q = build_query("2026-10-28T14:00:00.000+0000", 10, ["99999"], 200)
    assert q == {
        "size": 200,
        "sort": [{"timestamp": {"order": "asc"}}],
        "query": {"bool": {
            "filter": [
                {"range": {"rule.level": {"gte": 10}}},
                {"range": {"timestamp": {"gte": "2026-10-28T14:00:00.000+0000"}}},
            ],
            "must_not": [{"terms": {"rule.id": ["99999"]}}],
        }},
    }


def test_query_without_ignored_rules_has_no_must_not():
    assert "must_not" not in build_query("x", 7, [], 10)["query"]["bool"]


def client_with(handler):
    return IndexerClient("https://127.0.0.1:9200", "brain-ro", "pw", verify=False,
                         client=httpx.Client(transport=httpx.MockTransport(handler)))


def test_search_returns_id_and_source():
    seen = {}

    def handler(req):
        seen["url"], seen["auth"] = str(req.url), req.headers["authorization"]
        seen["body"] = json.loads(req.content)
        return httpx.Response(200, json={"hits": {"hits": [
            {"_id": "a", "_source": {"rule": {"id": "1"}}}]}})

    out = client_with(handler).search({"size": 1})
    assert out == [("a", {"rule": {"id": "1"}})]
    assert seen["url"] == "https://127.0.0.1:9200/wazuh-alerts-*/_search"
    assert seen["auth"].startswith("Basic ") and seen["body"] == {"size": 1}


def test_search_http_401_raises_indexer_error():
    with pytest.raises(IndexerError, match="401"):
        client_with(lambda r: httpx.Response(401, text="Unauthorized")).search({})


def test_search_timeout_raises_indexer_error():
    def handler(req):
        raise httpx.ConnectTimeout("timed out", request=req)

    with pytest.raises(IndexerError):
        client_with(handler).search({})


class FakeIndex:
    """Applies the query's timestamp range, sort and size to an in-memory list."""

    def __init__(self, docs):
        self.docs, self.queries = docs, []

    def search(self, query):
        self.queries.append(query)
        since = query["query"]["bool"]["filter"][1]["range"]["timestamp"]["gte"]
        since_dt = datetime.strptime(since, "%Y-%m-%dT%H:%M:%S.%f%z")
        hits = [(i, d) for i, d in self.docs
                if datetime.strptime(d["timestamp"], "%Y-%m-%dT%H:%M:%S.%f%z") >= since_dt]
        hits.sort(key=lambda h: h[1]["timestamp"])
        return hits[:query["size"]]


def state(tmp_path):
    return State.load(str(tmp_path / "s.json"), max_ids=10000)


def since_of(q):
    return q["query"]["bool"]["filter"][1]["range"]["timestamp"]["gte"]


def poll(index, st, **kw):
    return poll_once(index, st, min_level=10, ignore_rule_ids=[], now=NOW, **kw)


def test_poll_first_start_uses_backfill_window(tmp_path):
    index = FakeIndex([])
    poll(index, state(tmp_path), max_backfill_s=3600)
    assert since_of(index.queries[0]).startswith("2026-10-28T14:00:00")


def test_poll_after_long_downtime_caps_at_backfill(tmp_path):
    st = state(tmp_path)
    st.mark("old", "2026-10-27T09:00:00.000+0000")
    index = FakeIndex([])
    poll(index, st, max_backfill_s=3600)
    assert since_of(index.queries[0]).startswith("2026-10-28T14:00:00")


def test_poll_overlap_and_dedup(tmp_path):
    st = state(tmp_path)
    cursor = NOW - timedelta(minutes=1)
    st.mark("seen", ts(cursor))
    index = FakeIndex([
        ("seen", {"timestamp": ts(cursor)}),
        ("late", {"timestamp": ts(cursor - timedelta(seconds=5))}),
        ("new", {"timestamp": ts(cursor + timedelta(seconds=1))}),
    ])
    out = poll(index, st, overlap_s=10)
    assert [i for i, _ in out] == ["late", "new"]
    assert not st.is_seen("new")  # the caller marks after handling


def test_poll_burst_over_page_size(tmp_path):
    st = state(tmp_path)
    start = NOW - timedelta(minutes=30)
    docs = [(f"d{i:03d}", {"timestamp": ts(start + timedelta(seconds=i))}) for i in range(500)]
    index = FakeIndex(docs)
    got = []
    for _ in range(20):
        batch = poll(index, st, page_size=200)
        if not batch:
            break
        for doc_id, src in batch:
            got.append(doc_id)
            st.mark(doc_id, src["timestamp"])
    assert got == [d for d, _ in docs]


def test_poll_full_page_of_seen_hits_moves_on(tmp_path):
    st = state(tmp_path)
    start = NOW - timedelta(minutes=10)
    docs = [(f"d{i}", {"timestamp": ts(start + timedelta(milliseconds=i))}) for i in range(30)]
    for d, src in docs[:20]:
        st.mark(d, src["timestamp"])
    out = poll(FakeIndex(docs), st, page_size=20, overlap_s=600)
    assert [d for d, _ in out] == [d for d, _ in docs[20:]]


def test_poll_warns_when_backfill_cap_skips_alerts(tmp_path, caplog):
    st = state(tmp_path)
    st.mark("old", "2026-10-28T12:00:00.000+0000")   # 3 h before NOW
    poll(FakeIndex([]), st, max_backfill_s=3600)
    assert "skipping" in caplog.text and "2026-10-28T12:00" in caplog.text


def test_poll_no_warning_within_backfill(tmp_path, caplog):
    st = state(tmp_path)
    st.mark("recent", "2026-10-28T14:59:00.000+0000")
    poll(FakeIndex([]), st, max_backfill_s=3600)
    assert "skipping" not in caplog.text
