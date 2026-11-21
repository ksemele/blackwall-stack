"""Unit check: blackwall-scan confirm pass (loopback only — no lab, no network).

Mirrors ksemele/blackwall-stack#26: behind some NAT/ISP setups a middlebox
answers SYNs for closed ports, so a connect scan reports them open while the
connection never carries a byte. That is exactly a silent acceptor, so the
check feeds confirm.py real services (HTTP, greeting banner, raw echo) plus
silent acceptors plus a truly closed port, and asserts the split: real pairs
confirmed, fake ones unconfirmed, both machine-readable JSONL.
"""
import importlib.machinery, importlib.util, json, os, socket, sys, tempfile, threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

sys.dont_write_bytecode = True  # no __pycache__ next to the module's files

root = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
path = os.path.join(root, "blackwall-scan/confirm.py")
loader = importlib.machinery.SourceFileLoader("scan_confirm", path)
spec = importlib.util.spec_from_loader("scan_confirm", loader)
m = importlib.util.module_from_spec(spec); loader.exec_module(m)

def bind_server(payload=None, read_first=False):
    """Listen on a free loopback port. payload=None: accept forever, stay silent
    (the middlebox fake); otherwise read (optionally) then answer and close."""
    srv = socket.socket()
    srv.bind(("127.0.0.1", 0)); srv.listen(8)
    port = srv.getsockname()[1]
    held = []  # silent connections must stay open, not be closed by us
    def loop():
        while True:
            c, _ = srv.accept()
            if payload is None:
                held.append(c)
                continue
            try:
                if read_first:
                    c.settimeout(4); c.recv(4096)
                c.sendall(payload)
            except OSError:
                pass
            c.close()
    threading.Thread(target=loop, daemon=True).start()
    return srv, port

class Ok(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    def do_GET(self):
        self.send_response(200); self.end_headers(); self.wfile.write(b"confirmed\n")
    def log_message(self, *a):
        pass

httpd = ThreadingHTTPServer(("127.0.0.1", 0), Ok)
threading.Thread(target=httpd.serve_forever, daemon=True).start()
HTTP_PORT = httpd.server_address[1]
_, BANNER_PORT = bind_server(b"SSH-2.0-BlackwallTest\r\n")
_, ECHO_PORT = bind_server(b"echo\r\n", read_first=True)   # raw service: answers only after our GET
SILENT1 = bind_server()[1]   # the faked closed port: connect OK, zero bytes
SILENT2 = bind_server()[1]
_, LONG_PORT = bind_server(b"X" * 300 + b"\r\n")

closed = socket.socket(); closed.bind(("127.0.0.1", 0)); CLOSED_PORT = closed.getsockname()[1]; closed.close()
pairs = {"http": HTTP_PORT, "banner": BANNER_PORT, "echo": ECHO_PORT,
         "silent1": SILENT1, "silent2": SILENT2, "long": LONG_PORT, "closed": CLOSED_PORT}

# 1. the whole pass: real services confirmed, fake and closed ports dropped
tmp = tempfile.mkdtemp()
hp, confp, unconfp = (os.path.join(tmp, n) for n in ("hostports.txt", "ports.confirmed.jsonl", "ports.unconfirmed.jsonl"))
open(hp, "w").write("".join("127.0.0.1:%d\n" % p for p in pairs.values()))
assert m.main(["confirm.py", hp, confp, unconfp, "1"]) == 0
confirmed = [json.loads(l) for l in open(confp)]
unconfirmed = [json.loads(l) for l in open(unconfp)]
by_port = {r["port"]: r for r in confirmed + unconfirmed}
assert len(confirmed) == 4 and len(unconfirmed) == 3, (confirmed, unconfirmed)
assert by_port[pairs["http"]]["method"] == "http" and by_port[pairs["http"]]["evidence"] == "HTTP/1.1 200 OK", by_port[pairs["http"]]
assert by_port[pairs["banner"]]["method"] == "banner" and by_port[pairs["banner"]]["evidence"] == "SSH-2.0-BlackwallTest", by_port[pairs["banner"]]
assert by_port[pairs["echo"]]["method"] == "reply", by_port[pairs["echo"]]
for k in ("silent1", "silent2"):
    assert by_port[pairs[k]]["reason"] == "no-reply", by_port[pairs[k]]  # the issue's false positive
assert by_port[pairs["closed"]]["reason"] == "connect-refused", by_port[pairs["closed"]]
for r in confirmed + unconfirmed:
    assert (r in confirmed) == bool(r["confirmed"]), r
    assert r["host"] == "127.0.0.1" and isinstance(r["port"], int), r

# 2. evidence stays one safe line, capped at MAX_EVIDENCE bytes
assert by_port[pairs["long"]]["method"] == "banner"
assert len(by_port[pairs["long"]]["evidence"]) == m.MAX_EVIDENCE
assert "\n" not in by_port[pairs["long"]]["evidence"] and "\r" not in by_port[pairs["long"]]["evidence"]

# 3. probe_pair reasons and JSONL shape, used directly
r = m.probe_pair("127.0.0.1", pairs["closed"], 1)
assert r == {"host": "127.0.0.1", "port": pairs["closed"], "confirmed": False, "reason": "connect-refused"}, r
for line in open(confp):
    json.loads(line)  # machine-readable, one record per line

# 4. input parsing: blanks and malformed lines skipped, clean pairs kept
bad = os.path.join(tmp, "bad.txt")
open(bad, "w").write("127.0.0.1:%d\n\n# not a pair\nno-colon\n127.0.0.1:port\n127.0.0.1:%d\n" % (pairs["closed"], pairs["banner"]))
assert m.parse_pairs(bad) == [("127.0.0.1", pairs["closed"]), ("127.0.0.1", pairs["banner"])]

# 5. an empty input (naabu found nothing) yields empty outputs, not an error
empty = os.path.join(tmp, "empty.txt"); open(empty, "w").write("")
assert m.main(["confirm.py", empty, os.path.join(tmp, "c2"), os.path.join(tmp, "u2"), "1"]) == 0
assert open(os.path.join(tmp, "c2")).read() == "" and open(os.path.join(tmp, "u2")).read() == ""
assert m.main(["confirm.py", hp]) == 2  # missing args -> usage error

print("SCAN CONFIRM OK")
