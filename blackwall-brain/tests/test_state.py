import json
import os

import pytest

from brain.state import State


def test_missing_file_starts_empty(tmp_path):
    s = State.load(str(tmp_path / "nope.json"), max_ids=10)
    assert s.cursor is None and s.seen == []


def test_state_roundtrip_and_cap(tmp_path):
    path = str(tmp_path / "state.json")
    s = State.load(path, max_ids=3)
    for i in range(5):
        s.mark(f"id{i}", f"2026-10-28T14:0{i}:00.000+0000")
    s.save(path)
    again = State.load(path, max_ids=3)
    assert again.cursor == "2026-10-28T14:04:00.000+0000"
    assert again.seen == ["id2", "id3", "id4"]
    assert again.is_seen("id4") and not again.is_seen("id0")


def test_cursor_never_moves_back(tmp_path):
    s = State.load(str(tmp_path / "s.json"), max_ids=10)
    s.mark("a", "2026-10-28T14:05:00.000+0000")
    s.mark("b", "2026-10-28T14:04:59.000+0000")  # late arrival
    assert s.cursor == "2026-10-28T14:05:00.000+0000"


@pytest.mark.parametrize("content", ["", "{not json", "[]", '{"cursor": 5, "seen": "x"}'])
def test_state_corrupt_file_starts_empty(tmp_path, caplog, content):
    path = tmp_path / "state.json"
    path.write_text(content)
    s = State.load(str(path), max_ids=10)
    assert s.cursor is None and s.seen == []
    assert "state" in caplog.text.lower()


def test_state_save_is_atomic(tmp_path, monkeypatch):
    path = str(tmp_path / "state.json")
    s = State.load(path, max_ids=10)
    s.mark("a", "2026-10-28T14:00:00.000+0000")
    s.save(path)

    def boom(*a, **k):
        raise OSError("disk full")

    s.mark("b", "2026-10-28T14:01:00.000+0000")
    monkeypatch.setattr(os, "replace", boom)
    with pytest.raises(OSError):
        s.save(path)
    assert json.load(open(path))["seen"] == ["a"]
    assert [p.name for p in tmp_path.iterdir()] == ["state.json"]
