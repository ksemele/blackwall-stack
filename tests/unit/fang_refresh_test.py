"""Unit check: blackwall-fang-refresh filtering and rendering (no network, no nft).

    python3 tests/unit/fang_refresh_test.py
"""
import importlib.machinery, importlib.util, ipaddress, os

root = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
path = os.path.join(root, "blackwall-fang/ansible/roles/fang-reputation/files/blackwall-fang-refresh")
loader = importlib.machinery.SourceFileLoader("fang_refresh", path)
spec = importlib.util.spec_from_loader("fang_refresh", loader)
m = importlib.util.module_from_spec(spec); loader.exec_module(m)
N = ipaddress.ip_network
S = lambda nets: sorted(str(n) for n in nets)

PUBLIC = "".join("%d.%d.%d.0/24\n" % (a, a + 1, a + 2) for a in range(20, 30))
FIREHOL = "# firehol\n0.0.0.0/8\n10.0.0.0/8\n127.0.0.0/8\n172.16.0.0/12\n192.168.0.0/16\n224.0.0.0/3\n100.64.0.0/10\n" + PUBLIC
DROP = '{"cidr":"5.5.5.0/24","sblid":"SBL1"}\n{"cidr":"6.6.6.0/24"}\n{"type":"metadata","copyright":"x"}\nnot json\n'
JDROP = DROP
CF = "173.245.48.0/20\n103.21.244.0/22\n" + "".join("%d.0.0.0/20\n" % i for i in range(100, 108))

# 1. private/reserved removed (incl. 224.0.0.0/3 despite being /3), parsers for both formats
block, allow, _ = m.compute([FIREHOL, DROP], CF, [], [], 1)
b = set(S(block))
assert {"5.5.5.0/24", "6.6.6.0/24", "20.21.22.0/24"} <= b, b
for bad in ("0.0.0.0/8", "10.0.0.0/8", "127.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
            "224.0.0.0/3", "100.64.0.0/10"):
    assert bad not in b, bad
assert all(not n.overlaps(r) for n in block for r in m.RESERVED), "reserved space leaked"

# 2. allowlist, Cloudflare and host addresses are carved out
big = "5.0.0.0/16\n7.7.7.0/24\n9.9.9.0/24\n" + PUBLIC
block, allow, _ = m.compute([big], "7.7.7.0/24\n" + CF, ["5.0.1.0/24"], ["9.9.9.9"], 1)
assert not any(n.overlaps(N("5.0.1.0/24")) for n in block)
assert not any(n.overlaps(N("7.7.7.0/24")) for n in block)
assert not any(n.overlaps(N("9.9.9.9/32")) for n in block)
assert N("5.0.0.0/24") in block and N("5.0.2.0/23") in block, S(block)
assert {"5.0.1.0/24", "7.7.7.0/24", "9.9.9.9/32"} <= set(S(allow))

# 3. /0-/7 rejected (only when not entirely reserved)
for bad in ("0.0.0.0/0", "8.0.0.0/7", "64.0.0.0/2"):
    try:
        m.compute([bad + "\n" + PUBLIC], CF, [], [], 1)
        raise SystemExit("accepted " + bad)
    except m.Abort as e:
        assert "shorter" in str(e), e

# 4. sanity floor
try:
    m.compute([DROP], CF, [], [], 500); raise SystemExit("floor not enforced")
except m.Abort as e:
    assert "minimum 500" in str(e), e

# 5. test-only exemption lets one private host through, nothing else
block, _, _ = m.compute(["172.28.0.15/32\n172.28.1.0/24\n10.0.0.0/8\n" + PUBLIC], CF, [], [], 1,
                        [N("172.28.0.15/32")])
assert N("172.28.0.15/32") in block and not any(n.overlaps(N("172.28.1.0/24")) for n in block)
assert not any(n.overlaps(N("10.0.0.0/8")) for n in block)

# 6. observe vs enforce rendering
block, allow, _ = m.compute([DROP + PUBLIC], CF, ["203.0.113.7"], [], 1)
obs = m.render("eth0", "observe", allow, block)
enf = m.render("eth0", "enforce", allow, block)
assert "priority -160" in obs and 'iifname != "eth0" accept' in obs
UPD = "ip saddr @blocklist_static update @fang_hits { ip saddr counter }\n"
DROP = "ip saddr @blocklist_static counter drop\n"
assert UPD in obs and "drop" not in obs.split("chain prerouting")[1]
assert UPD in enf and DROP in enf and enf.index(UPD) < enf.index(DROP)  # drop is its own rule
assert obs.index("@allow_v4 accept") < obs.index("established,related") < obs.index("@blocklist_static")
assert "5.5.5.0/24" in obs and "173.245.48.0/20" in obs
try:
    m.render("eth0", "bogus", allow, block); raise SystemExit("bad mode accepted")
except m.Abort:
    pass

# 7. file:// refused without the test flag; plain http refused
for u in ("file:///etc/passwd", "http://example.com/x"):
    try:
        m.fetch(u, False); raise SystemExit("fetched " + u)
    except m.Abort:
        pass
# 8. Cloudflare floor, per-source floor, extra reserved blocks, IPv6 allowlist skipped
try:
    m.compute([PUBLIC], "173.245.48.0/20\n", [], [], 1); raise SystemExit("CF floor not enforced")
except m.Abort as e:
    assert "Cloudflare" in str(e), e
try:
    m.compute([PUBLIC, "garbage\n"], CF, [], [], 4); raise SystemExit("junk source accepted")
except m.Abort as e:
    assert "source #2" in str(e), e
block, _, _ = m.compute([PUBLIC + "192.31.196.0/24\n192.52.193.0/24\n192.175.48.0/24\n"], CF,
                        ["2001:db8::/32"], [], 1)
assert not any(n.overlaps(N(x)) for n in block for x in ("192.31.196.0/24", "192.52.193.0/24", "192.175.48.0/24"))

# 9. cache roundtrip (0700 dir, atomic)
import tempfile
m.STATE_DIR = tempfile.mkdtemp() + "/st"
assert m.cache_load("k") is None
m.cache_save("k", "abc"); assert m.cache_load("k") == "abc"
assert oct(os.stat(m.STATE_DIR).st_mode & 0o777) == "0o700"
assert m.cache_age("nope") is None and 0 <= m.cache_age("k") < 5

# 10. main(): cache fallback, rejected fresh data, stale cache (stubbed I/O)
import types, time
tmp = tempfile.mkdtemp()
confp = os.path.join(tmp, "fang.conf")
def write_conf(extra=""):
    open(confp, "w").write('MODE=enforce\nWAN_IF=eth0\nSOURCES="u1 u2"\nCF_URL=cfu\nALLOWLIST="203.0.113.7"\n'
                           "MIN_ENTRIES=5\nTEST_MODE=1\nRULES_OUT=%s/out.nft\n%s" % (tmp, extra))
m.CONF = confp
applied = []
m.apply = lambda text, out, dry: applied.append(text)
m.nft = lambda *a: types.SimpleNamespace(returncode=0, stdout="", stderr="")
m.host_addresses = lambda: []
m.table_loaded = lambda: True
m.log = lambda msg: logs.append(msg)
m.os.geteuid = lambda: 0
real_exists = os.path.exists
m.os.path.exists = lambda p: True if p.startswith("/sys/class/net/") else real_exists(p)
m.STATE_DIR = tempfile.mkdtemp() + "/st2"
logs = []
DATA = {"u1": PUBLIC, "u2": JDROP, "cfu": CF}
def fetcher(data):
    def f(url, allow_file):
        if data[url] is None:
            raise m.FetchError("failed to download %s" % url)
        return data[url]
    return f
def run(data):
    del applied[:]; del logs[:]
    m.fetch = fetcher(data)
    try:
        return m.main([])
    except m.Abort as e:
        logs.append("ABORT " + str(e)); return 1
# no cache + junk fresh data -> abort, nothing applied
write_conf()
assert run({"u1": PUBLIC, "u2": "garbage\n", "cfu": CF}) == 1 and not applied, logs
assert any("u2" in l for l in logs), logs
# good run fills the cache
assert run(DATA) == 0 and len(applied) == 1 and "5.5.5.0/24" in applied[0]
# rejected fresh data -> rebuilt from cache, exit 3, WARNING names the source
assert run({"u1": PUBLIC, "u2": "garbage\n", "cfu": CF}) == 3, logs
assert len(applied) == 1 and "5.5.5.0/24" in applied[0] and "counter drop" in applied[0], logs
assert any(l.startswith("WARNING") and "u2" in l and "rejected" in l for l in logs), logs
# download failure -> from cache, with the CURRENT mode and allowlist
write_conf(); open(confp, "a").write("")
conf_txt = open(confp).read().replace("MODE=enforce", "MODE=observe").replace("203.0.113.7", "5.5.5.9")
open(confp, "w").write(conf_txt)
assert run({"u1": None, "u2": None, "cfu": None}) == 3, logs
assert "counter drop" not in applied[0] and "5.5.5.9" in applied[0], logs
# cache older than MAX_CACHE_AGE_DAYS -> empty blocklist, exit 4, ERROR, mode kept
old = time.time() - 9 * 86400
for k in ("u1", "u2", "cf:cfu"):
    os.utime(m._cache_name(k), (old, old))
assert run({"u1": None, "u2": None, "cfu": None}) == 4, logs
assert len(applied) == 1 and "blocklist: 0" in applied[0] and "5.5.5.0/24" not in applied[0], logs
assert any(l.startswith("ERROR") and "EMPTY" in l for l in logs), logs
# a rejected fresh update with only a stale cache -> also empty, exit 4
assert run({"u1": PUBLIC, "u2": "garbage\n", "cfu": CF}) == 4, logs

# 11. layer 2 (observe only): detectors, ordering, never a drop of their own
import re
A = m.attempts_from_conf({"ATTEMPTS": "1", "OPEN_TCP": "22 443 1514"})
assert m.attempts_from_conf({}) is None and m.attempts_from_conf({"ATTEMPTS": "0"}) is None
block, allow, _ = m.compute([JDROP + PUBLIC], CF, ["203.0.113.7"], [], 1)
for mode in ("observe", "enforce"):
    for blk in (block, []):
        t = m.render("eth0", mode, allow, blk, "", A)
        ch = t.split("chain prerouting")[1]
        # the only drop is the layer 1 enforce rule
        assert ch.count("drop") == (1 if mode == "enforce" else 0), (mode, ch)
        l2 = ch.split("# Layer 2")[1]
        assert "drop" not in l2 and "reject" not in l2
        assert ch.index("@allow_v4 accept") < ch.index("established,related") < ch.index("# Layer 2")
        assert ch.index("@blocklist_static update") < ch.index("# Layer 2")  # after the reputation rules
        assert ch.rindex("counter drop") < ch.index("# Layer 2") if mode == "enforce" else True
        rules = [l.strip() for l in l2.splitlines() if l.strip().startswith("ct state new")]
        assert len(rules) == 3, rules
        for r in rules:  # new TCP SYNs only, one update per rule
            assert r.startswith("ct state new tcp flags & (syn | ack) == syn"), r
            assert r.count("update @") == 1 and r.count(" add @") == 1, r
        assert "tcp dport { 22 } add @ssh_meter { ip saddr limit rate over 6/minute burst 4 packets } update @ssh_flagged" in rules[0]
        assert "tcp dport != @open_tcp add @scan_meter { ip saddr limit rate over 10/minute burst 10 packets } update @scan_flagged" in rules[1]
        assert "add @rate_meter { ip saddr limit rate over 120/minute burst 60 packets } update @rate_flagged" in rules[2]
        assert "elements = { 22, 443, 1514 }" in t
        for d in ("ssh", "scan", "rate"):
            assert re.search(r"set %s_meter \{[^}]*flags dynamic,timeout[^}]*timeout 5m[^}]*size 65536" % d, t), d
            assert re.search(r"set %s_flagged \{[^}]*flags dynamic,timeout[^}]*timeout 1h[^}]*size 65536[^}]*counter" % d, t), d
# layer 2 renders with an empty blocklist (fail-open / first run)
assert "update @ssh_flagged" in m.render("eth0", "observe", allow, [], "", A)
# without fang_open_tcp: no scan detector, no open_tcp set
B = m.attempts_from_conf({"ATTEMPTS": "1", "OPEN_TCP": ""})
t = m.render("eth0", "observe", allow, block, "", B)
assert "scan" not in t and "open_tcp" not in t and "@ssh_flagged" in t and "@rate_flagged" in t
# disabled (or layer 1 call as before): nothing of layer 2
for t in (m.render("eth0", "observe", allow, block), m.render("eth0", "observe", allow, block, "", None)):
    assert "_flagged" not in t and "_meter" not in t and "Layer 2" not in t
# custom ports/rates, and bad values never reach nft syntax
C = m.attempts_from_conf({"ATTEMPTS": "1", "SSH_PORTS": "2222 22", "SSH_RATE": "3/second", "SSH_BURST": "2"})
assert "tcp dport { 22, 2222 } add @ssh_meter { ip saddr limit rate over 3/second burst 2 packets }" in m.render("eth0", "observe", allow, block, "", C)
for bad in ({"SSH_RATE": "6/min; flush ruleset"}, {"SSH_PORTS": "22 x"}, {"OPEN_TCP": "70000"},
            {"SSH_BURST": "0"}, {"METER_SIZE": "abc"}):
    try:
        m.attempts_from_conf(dict(bad, ATTEMPTS="1")); raise SystemExit("accepted %r" % bad)
    except m.Abort:
        pass
# main() passes ATTEMPTS through, also on the degraded paths (cache, fail-open empty)
write_conf("ATTEMPTS=1\nOPEN_TCP=\"22 443\"\n")
assert run(DATA) == 0 and "update @ssh_flagged" in applied[0] and "elements = { 22, 443 }" in applied[0], logs
assert run({"u1": None, "u2": None, "cfu": None}) == 3 and "update @scan_flagged" in applied[0], logs
for k in ("u1", "u2", "cf:cfu"):
    os.utime(m._cache_name(k), (old, old))
assert run({"u1": None, "u2": None, "cfu": None}) == 4 and "update @rate_flagged" in applied[0], logs
print("FANG REFRESH OK")
