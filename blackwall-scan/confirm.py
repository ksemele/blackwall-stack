#!/usr/bin/env python3
"""blackwall-scan confirmation pass — verify naabu's open host:port pairs.

A TCP connect scan behind some home NAT/ISP setups gets SYN-ACKs for *closed*
ports: a middlebox answers on the host's behalf, naabu reports the pair open,
but the connection is dead — it never carries a single byte in either
direction. Real services talk: they greet first (SSH, SMTP, ...) or answer a
request. So the discriminator is application-level byte exchange, not the
handshake (nmap -sT from the same container performs the same connect() as
naabu and would report the same fake ports open).

For every "host:port" line of the input file:
  1. TCP connect (a refused/timed-out pair was not open even for naabu);
  2. read whatever the service sends first (banner grab);
  3. if it stays silent, send a plain HTTP GET and read the reply
     (httpx already fires a full HTTP request at every pair, so this adds
     nothing new to the target).

Pairs that exchanged bytes are confirmed -> ports.confirmed.jsonl
(method + evidence). Pairs whose connection stayed dead are unconfirmed ->
ports.unconfirmed.jsonl (reason). The raw naabu output is left untouched.

Usage: confirm.py <hostports-file> <confirmed-out> <unconfirmed-out> [timeout_s]
"""
import json
import socket
import sys
from concurrent.futures import ThreadPoolExecutor

MAX_WORKERS = 32
MAX_EVIDENCE = 120          # bytes of a reply kept as evidence
MAX_REPLY = 4096            # bytes read before giving up

def _clean(data):
    """First reply line, truncated, safe for one JSONL line."""
    line = data.split(b"\r\n", 1)[0].split(b"\n", 1)[0][:MAX_EVIDENCE]
    return line.decode("utf-8", "replace").strip()

def _read_some(sock):
    """Read up to MAX_REPLY bytes with the socket deadline; b'' when silent."""
    data = b""
    try:
        while len(data) < MAX_REPLY:
            chunk = sock.recv(2048)
            if not chunk:
                break
            data += chunk
            if b"\n" in data:
                break       # a line is enough to confirm
    except (TimeoutError, OSError):
        pass
    return data

def probe_pair(host, port, timeout):
    """Probe one host:port pair, return its record (confirmed or not)."""
    base = {"host": host, "port": port}
    try:
        sock = socket.create_connection((host, port), timeout=timeout)
    except TimeoutError:
        return {**base, "confirmed": False, "reason": "connect-timeout"}
    except ConnectionRefusedError:
        return {**base, "confirmed": False, "reason": "connect-refused"}
    except OSError as e:
        return {**base, "confirmed": False,
                "reason": "connect-error: %s" % (e.strerror or e)}
    try:
        sock.settimeout(timeout)
        reply = _read_some(sock)
        greeted = bool(reply)
        if not greeted:
            try:
                sock.sendall(("GET / HTTP/1.1\r\nHost: %s:%d\r\nConnection: close\r\n"
                              "User-Agent: blackwall-scan/1.0 (confirm pass)\r\n\r\n"
                              % (host, port)).encode())
            except OSError:
                return {**base, "confirmed": False, "reason": "send-failed"}
            reply = _read_some(sock)
        if not reply:
            # connect OK but the connection is dead: the middlebox fake
            return {**base, "confirmed": False, "reason": "no-reply"}
        text = _clean(reply)
        method = "banner" if greeted else ("http" if text.startswith("HTTP/") else "reply")
        return {**base, "confirmed": True, "method": method, "evidence": text}
    finally:
        sock.close()

def parse_pairs(path):
    """Read 'host:port' lines; skip blanks and malformed ones."""
    pairs = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            host, sep, port = line.rpartition(":")
            if not host or not sep:
                print("confirm: skipping malformed line %r" % line, file=sys.stderr)
                continue
            try:
                pairs.append((host, int(port)))
            except ValueError:
                print("confirm: skipping malformed line %r" % line, file=sys.stderr)
    return pairs

def main(argv):
    if len(argv) < 4:
        print(__doc__, file=sys.stderr)
        return 2
    timeout = 3.0
    if len(argv) > 4:
        try:
            timeout = max(0.1, min(float(argv[4]), 30.0))
        except ValueError:
            print("confirm: bad timeout %r, using 3" % argv[4], file=sys.stderr)
    pairs = parse_pairs(argv[1])
    if pairs:
        with ThreadPoolExecutor(max_workers=min(MAX_WORKERS, len(pairs))) as ex:
            results = list(ex.map(lambda hp: probe_pair(hp[0], hp[1], timeout), pairs))
    else:
        results = []
    confirmed = [r for r in results if r["confirmed"]]
    unconfirmed = [r for r in results if not r["confirmed"]]
    with open(argv[2], "w") as c, open(argv[3], "w") as u:
        for r in confirmed:
            c.write(json.dumps(r, ensure_ascii=False) + "\n")
        for r in unconfirmed:
            u.write(json.dumps(r, ensure_ascii=False) + "\n")
    print("confirm: %d confirmed, %d unconfirmed" % (len(confirmed), len(unconfirmed)))
    return 0

if __name__ == "__main__":
    sys.exit(main(sys.argv))
