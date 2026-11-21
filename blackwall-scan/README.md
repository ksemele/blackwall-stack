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
             dnsx (resolve) -> naabu (port scan) -> confirm (app-level probe)
                          -> httpx (HTTP/tech/title) -> tlsx (cert SAN/expiry)
                          -> nuclei (opt: exposures)
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
RUN_CONFIRM=0 ./run.sh    # skip the naabu confirmation pass
./run.sh --build          # rebuild the image
```

Targets go in `targets/targets.txt` (one per line, a domain or an IPv4 address).
The file is git-ignored — create it from the template:

```bash
cp targets/targets.txt.example targets/targets.txt
```

Results land in `output/<timestamp>/`:
- `SUMMARY.txt` — human-readable summary
- `ports.jsonl`, `http.jsonl`, `tls.jsonl`, `dns.jsonl`, `nuclei.jsonl` — raw data (JSONL)
- `ports.confirmed.jsonl` / `ports.unconfirmed.jsonl` — the confirmation pass split
  (confirmed pairs carry `method` + `evidence`; unconfirmed carry the `reason`)
- `san_discovered.txt` — subdomains found in certificate SANs (seed for the next run)

## Variables

| Env | Default | Purpose |
|---|---|---|
| `PORTS` | `-top-ports 1000` | naabu port spec (`-p -` = all 65535) |
| `RUN_NUCLEI` | `0` | `1` enables nuclei |
| `NUCLEI_TAGS` | `exposure,misconfig,takeover,default-login` | nuclei tags |
| `RATE` | `1000` | naabu packet rate |
| `RUN_CONFIRM` | `1` | `0` skips the confirmation pass (`ports.jsonl` stays unsplit) |
| `CONFIRM_TIMEOUT` | `3` | per-pair probe timeout, seconds |

## Notes

- **Confirmation pass** (step 4b): some home NAT/ISP middleboxes answer SYNs
  for closed ports, so a connect scan gets phantom open ports (see `STATUS.md`).
  `confirm.py` probes every naabu-reported pair at the application level —
  banner grab, then a plain HTTP GET — and splits them: bytes exchanged →
  `ports.confirmed.jsonl` (`method` + `evidence`), dead connection →
  `ports.unconfirmed.jsonl` (`reason: no-reply`). A plain `nmap -sT --reason`
  was not used: from the same container it performs the same `connect()` as
  naabu and would report the same fakes open. Caveat: a real service that
  neither greets nor answers an HTTP GET also lands in the unconfirmed file —
  check it by hand before trusting the drop.
- naabu uses a **connect scan** (`-s c`) — no privileges needed. For a SYN scan
  (faster), add `--cap-add=NET_ADMIN,NET_RAW` in `run.sh`.
- **A single vantage point** (your ISP). For completeness, later also run it
  from an external VPS.
- **Cloudflare/CDN**: a domain behind CF (e.g. `example.com`) resolves to the edge —
  scanning the domain hits Cloudflare, not the origin. Check the origin by its direct IPs.
- IPv6 is not scanned yet (TODO).
- `output/` is in `.gitignore` — results are not committed.

## Status

See `STATUS.md`. Phase 1 (MVP scanner) is in progress. Next: storage + diffing
over time (ClickHouse), alerts via `blackwall-voice`, inside-out ground-truth diff.
