# blackwall-scan

External attack surface scanner — looks at your infrastructure **from outside
the perimeter**, through an attacker's eyes. A Blackwall module (see
`../CLAUDE.md`, plan: `../docs/blackwall-skin-plan.md`).

> **Your own scope only.** Scan only assets you own, or with written
> permission. Scanning other people's hosts is unauthorized access
> (Art. 272 of the Russian Criminal Code / CFAA).

## What it does (Phase 1)

A pipeline inside a Docker container:

```
targets.txt ─┬─ domains ─> subfinder (passive subdomains)
             └─ IPs ──────┐
                          v
             dnsx (resolve) -> naabu (port scan) -> httpx (HTTP/tech/title)
                          -> tlsx (cert SAN/expiry) -> nuclei (opt: exposures)
                          -> output/<timestamp>/  (JSONL + SUMMARY.txt)
```

Tools: ProjectDiscovery (subfinder, dnsx, asnmap, naabu, httpx, tlsx, nuclei)
plus nmap/masscan in the runtime image.

## Requirements

- Docker (on macOS — OrbStack or Docker Desktop; the daemon must be running).

## Running

```bash
cd blackwall-scan
./run.sh                  # top 1000 ports, no nuclei
RUN_NUCLEI=1 ./run.sh     # + exposure/misconfig/takeover checks
PORTS="-p -" ./run.sh     # full scan of all 65535 TCP ports (slow)
./run.sh --build          # rebuild the image
```

Targets go in `targets.txt` (one per line, a domain or an IPv4 address).

Results land in `output/<timestamp>/`:
- `SUMMARY.txt` — human-readable summary
- `ports.jsonl`, `http.jsonl`, `tls.jsonl`, `dns.jsonl`, `nuclei.jsonl` — raw data (JSONL)
- `san_discovered.txt` — subdomains found in certificate SANs (seed for the next run)

## Variables

| Env | Default | Purpose |
|---|---|---|
| `PORTS` | `-top-ports 1000` | naabu port spec (`-p -` = all 65535) |
| `RUN_NUCLEI` | `0` | `1` enables nuclei |
| `NUCLEI_TAGS` | `exposure,misconfig,takeover,default-login` | nuclei tags |
| `RATE` | `1000` | naabu packet rate |

## Notes

- naabu uses a **connect scan** (`-s c`) — no privileges needed. For a SYN scan
  (faster), add `--cap-add=NET_ADMIN,NET_RAW` in `run.sh`.
- **A single vantage point** (your ISP). For completeness, later also run it
  from an external VPS.
- **Cloudflare/CDN**: a domain behind CF (e.g. `fehu.dev`) resolves to the edge —
  scanning the domain hits Cloudflare, not the origin. Check the origin by its direct IPs.
- IPv6 is not scanned yet (TODO).
- `output/` is in `.gitignore` — results are not committed.

## Status

See `STATUS.md`. Phase 1 (MVP scanner) is in progress. Next: storage + diffing
over time (ClickHouse), alerts via `blackwall-voice`, inside-out ground-truth diff.
