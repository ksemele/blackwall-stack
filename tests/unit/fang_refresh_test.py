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
print("FANG REFRESH OK")
