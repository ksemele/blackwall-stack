from datetime import datetime, timezone
from types import SimpleNamespace as NS

from brain.eval import capture
from brain.eval.capture import alerts_query, capture_scenario
from brain.eval.scenarios import Expected, Scenario, Step

NOW = datetime(2026, 10, 29, 10, 0, 0, tzinfo=timezone.utc)


def scen(wait_s=30, expected=("554",)):
    return Scenario(
        "cron-curl-sh", "d", "black-host-01",
        steps=[Step("black-host-01", "root", "step1"), Step("black-host-01", "root", "step2")],
        cleanup=[Step("black-host-01", "root", "cleanup")],
        expected=[Expected(r, "true_positive", "high", "persistence", "why") for r in expected],
        wait_s=wait_s)


class Runner:
    def __init__(self, fail_on=None):
        self.ran, self.fail_on = [], fail_on

    def __call__(self, host, user, command):
        self.ran.append(command)
        return NS(returncode=1 if command == self.fail_on else 0, stdout="", stderr="boom")


class Index:
    """Returns `hits` from the n-th search on; filters by the query's rule ids."""

    def __init__(self, hits, from_call=1):
        self.hits, self.from_call, self.calls = hits, from_call, 0

    def search(self, query):
        self.calls += 1
        if self.calls < self.from_call:
            return []
        filters = query["query"]["bool"]["filter"]
        ids = next((f["terms"]["rule.id"] for f in filters if "terms" in f), None)
        return [h for h in self.hits if ids is None or h[1]["rule"]["id"] in ids]


class Clock:
    def __init__(self):
        self.t = 0.0

    def __call__(self):
        return self.t

    def sleep(self, s):
        self.t += s


def hit(i, rule):
    return (i, {"rule": {"id": rule}, "timestamp": "2026-10-29T10:00:05.000+0000",
                "full_log": "x"})


def run(s, index, runner=None, clock=None):
    clock = clock or Clock()
    return capture_scenario(s, index, run=runner or Runner(), now=lambda: NOW,
                            clock=clock, sleep=clock.sleep, poll_s=5)


def test_alerts_query_shape():
    q = alerts_query("black-host-01", "2026-10-29T10:00:00.000Z", ["554", "550"])
    f = q["query"]["bool"]["filter"]
    assert {"term": {"agent.name": "black-host-01"}} in f
    assert {"range": {"timestamp": {"gte": "2026-10-29T10:00:00.000Z"}}} in f
    assert {"terms": {"rule.id": ["554", "550"]}} in f
    assert q["sort"] == [{"timestamp": {"order": "asc"}}] and q["size"] == 500
    assert not any("terms" in x for x in alerts_query("a", "s", [])["query"]["bool"]["filter"])


def test_runs_steps_then_cleanup():
    runner = Runner()
    r = run(scen(), Index([hit("a", "554")]), runner)
    assert runner.ran == ["step1", "step2", "cleanup"]
    assert r.ok and [c.doc_id for c in r.cases] == ["a"] and r.cases[0].expected.rule_id == "554"


def test_cleanup_runs_after_failed_step():
    runner = Runner(fail_on="step1")
    r = run(scen(), Index([hit("a", "554")]), runner)
    assert runner.ran == ["step1", "cleanup"]
    assert not r.ok and "step1" in r.error


def test_waits_until_expected_rules_seen():
    index = Index([hit("a", "554")], from_call=3)
    r = run(scen(wait_s=60), index)
    assert r.ok and index.calls >= 3


def test_missing_rule_fails_scenario():
    clock = Clock()
    r = run(scen(wait_s=20, expected=("554", "550")), Index([hit("a", "554")]), clock=clock)
    assert not r.ok and r.missing == ["550"] and clock.t >= 20


def test_unlabelled_alerts_counted_not_cased():
    r = run(scen(), Index([hit("a", "554"), hit("b", "5501"), hit("c", "100100")]))
    assert r.ok and [c.doc_id for c in r.cases] == ["a"] and r.unlabelled == 2


def test_main_exit_1_and_no_snapshot_on_failure(tmp_path, monkeypatch):
    (tmp_path / "s").mkdir()
    (tmp_path / "s" / "a.yml").write_text(
        "name: a\ndescription: d\nagent: black-host-01\nwait_s: 0\n"
        "steps: [{host: black-host-01, run: 'true'}]\n"
        "expected: [{rule_id: '554', verdict: true_positive, severity: high,"
        " category: persistence, why: w}]\n")
    monkeypatch.setattr(capture, "docker_exec", Runner())
    monkeypatch.setattr(capture, "IndexerClient", lambda *a, **k: Index([]))
    monkeypatch.setattr(capture.time, "sleep", lambda s: None)
    out = tmp_path / "out"
    rc = capture.main(["--scenarios", str(tmp_path / "s"), "--out", str(out), "--poll", "0"])
    assert rc == 1 and not out.exists()


def test_cleanup_runs_after_alerts_are_collected_then_settles():
    events = []

    class LoggingIndex(Index):
        def search(self, query):
            events.append("search")
            return super().search(query)

    class LoggingRunner(Runner):
        def __call__(self, host, user, command):
            events.append(command)
            return super().__call__(host, user, command)

    clock = Clock()
    r = capture_scenario(scen(), LoggingIndex([hit("a", "554")]), run=LoggingRunner(),
                         now=lambda: NOW, clock=clock, sleep=clock.sleep, poll_s=5, settle_s=20)
    assert r.ok
    assert events.index("cleanup") > max(i for i, e in enumerate(events) if e == "search")
    assert clock.t >= 20


def test_setup_runs_before_window_opens():
    times, events = [], []
    clock = Clock()

    class R(Runner):
        def __call__(self, host, user, command):
            events.append(command)
            return super().__call__(host, user, command)

    s = scen()
    s.setup = [Step("black-host-01", "root", "setup1")]

    def now():
        times.append(len(events))
        return NOW

    capture_scenario(s, Index([hit("a", "554")]), run=R(), now=now, clock=clock,
                     sleep=clock.sleep, poll_s=5, settle_s=10)
    assert events[0] == "setup1" and times[0] == 1   # window opened after setup


def test_match_filters_same_rule_noise():
    s = scen()
    s.expected[0].match = {"syscheck.path": "/etc/cron.d/sysupdate"}
    noise = (("n", {"rule": {"id": "554"}, "syscheck": {"path": "/etc/cron.d/other"},
                    "timestamp": "t", "full_log": "x"}))
    real = (("r", {"rule": {"id": "554"}, "syscheck": {"path": "/etc/cron.d/sysupdate"},
                   "timestamp": "t", "full_log": "x"}))
    r = run(s, Index([noise, real]))
    assert r.ok and [c.doc_id for c in r.cases] == ["r"] and r.unlabelled == 1


def test_match_not_met_fails():
    s = scen(wait_s=10)
    s.expected[0].match = {"full_log": "www-data"}
    r = run(s, Index([hit("a", "554")]))
    assert not r.ok and r.missing == ["554"]
