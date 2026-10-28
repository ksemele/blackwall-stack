"""Intake state: the cursor and a bounded list of processed indexer _ids."""
import json
import logging
import os
import tempfile
from dataclasses import dataclass, field
from datetime import datetime

log = logging.getLogger(__name__)


def parse_ts(ts: str) -> datetime:
    """Wazuh alert timestamp, e.g. 2026-10-28T14:02:03.123+0000."""
    return datetime.strptime(ts, "%Y-%m-%dT%H:%M:%S.%f%z")


@dataclass
class State:
    max_ids: int
    cursor: str | None = None   # raw timestamp of the newest processed alert
    seen: list[str] = field(default_factory=list)

    def __post_init__(self):
        self._seen_set = set(self.seen)

    @classmethod
    def load(cls, path: str, max_ids: int) -> "State":
        try:
            with open(path) as f:
                data = json.load(f)
            cursor, seen = data["cursor"], data["seen"]
            if cursor is not None:
                parse_ts(cursor)
            if not isinstance(seen, list) or not all(isinstance(i, str) for i in seen):
                raise ValueError("seen is not a list of strings")
        except FileNotFoundError:
            return cls(max_ids)
        except (OSError, ValueError, KeyError, TypeError) as exc:
            log.warning("state file %s unreadable (%s); starting from the backfill window",
                        path, exc)
            return cls(max_ids)
        return cls(max_ids, cursor, seen[-max_ids:])

    def is_seen(self, doc_id: str) -> bool:
        return doc_id in self._seen_set

    def mark(self, doc_id: str, timestamp: str) -> None:
        if doc_id not in self._seen_set:
            self.seen.append(doc_id)
            self._seen_set.add(doc_id)
            if len(self.seen) > self.max_ids:
                for old in self.seen[:-self.max_ids]:
                    self._seen_set.discard(old)
                del self.seen[:-self.max_ids]
        try:
            when = parse_ts(timestamp)
        except (TypeError, ValueError):
            log.warning("alert %s has an unparsable timestamp %r; cursor kept", doc_id, timestamp)
            return
        # Late alerts must not move the cursor back.
        if self.cursor is None or when > parse_ts(self.cursor):
            self.cursor = timestamp

    def save(self, path: str) -> None:
        directory = os.path.dirname(path) or "."
        fd, tmp = tempfile.mkstemp(dir=directory, prefix=".state-")
        try:
            with os.fdopen(fd, "w") as f:
                json.dump({"cursor": self.cursor, "seen": self.seen}, f)
            os.replace(tmp, path)
        except BaseException:
            os.unlink(tmp)
            raise
