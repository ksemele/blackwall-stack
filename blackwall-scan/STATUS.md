---
module: blackwall-scan
status: in_progress
stage: 1
last_updated: 2026-08-15
last_task: "First scan analysed; example.com origin (203.0.113.10) closed — see hosts/example.com/"
next_task: "Weed out naabu false positives (see notes); close the findings on 198.51.100.20; time-diff storage (ClickHouse) in phase 2"
blockers: []
---

## What it is

An external scanner of our own perimeter (external attack surface
self-assessment). Looks at the infrastructure from outside, through an
attacker's eyes. Full plan and rationale: `docs/blackwall-skin-plan.md`.

The codename in the plan was "Skin" — by the owner's decision the module is
called **blackwall-scan**.

## Done

- 2026-08-15: Phase 1 MVP.
  - `Dockerfile` — multi-stage: a golang builder compiles the pd tools
    (subfinder, dnsx, asnmap, naabu, httpx, tlsx, nuclei), the debian-slim
    runtime adds nmap/masscan/libpcap/jq/dnsutils.
  - `scan.sh` — pipeline: domains→subfinder, + direct IPs → dnsx resolve →
    naabu (connect scan, top ports) → httpx (tech/title/status/tls) →
    tlsx (SAN/expiry/ciphers) → nuclei (optional: exposures/misconfig/takeover).
    Output: JSONL + summary in `output/<timestamp>/`.
  - `run.sh` — host helper: build + run with targets/ and output/ mounted.
  - `targets/targets.txt` — local only (git-ignored); template in `targets.txt.example`.

## Known facts about the targets (initial recon)

- `example.com` → **Cloudflare** (104.16.0.1 / 172.64.0.1), NS *.ns.cloudflare.com.
  Scanning the domain hits the CF edge, not the origin. The origin is probably
  one of the direct IPs below.
- `203.0.113.10` — no PTR.
- `198.51.100.20` → `198-51-100-20.example.net` (hosting provider).

## First scan analysis (2026-08-15)

Closed:

- **203.0.113.10 — the example.com origin, bypassing Cloudflare.** Confirmed
  (httpx: 200, nginx/1.31.1, CF Origin Cert `*.example.com`). Closed with a
  whitelist of CF prefixes; the host was also moved from firewalld to nftables.
  Configs and write-up: `hosts/example.com/`.

False positives — **naabu reported the same port set `53/80/443/5060/8080`
for all five targets**, including the CF edge:

- httpx confirmed only 443 on 203.0.113.10;
- firewalld on the host itself allowed only 22/9090/9999 + DNAT on 443 —
  neither 53, 5060 nor 8080 were there;
- port 80 really is closed: CF returned `523 Origin Unreachable`.

Conclusion: a connect scan through the home NAT/ISP gets ACKs on closed ports.
**`ports.jsonl` cannot be trusted without httpx/tlsx confirmation** — this
changes the priority of the "second vantage point" task from "for
completeness" to "for correctness".

Open:

- `198.51.100.20` (example.net): 8080 "<app>" and 443 → 502.
  Left alone for now by the owner's decision.
- Cockpit on 9090 and port 9999 on example.com were open to the whole internet —
  a root-level login to the system, a bigger risk than the open origin.
- The origin IP has already leaked into CT logs; the firewall closes access but
  not the fact of the leak. The complete fix is IP rotation or Cloudflare Tunnel.

## Notes / TODO

- **Second vantage point (external VPS) — raise its priority:** needed not for
  completeness but to filter out the home ISP's false positives (see above).
- Cross-check `ports.jsonl` against `http.jsonl`/`tls.jsonl` before drawing
  conclusions; treat a port without httpx/tlsx confirmation as unconfirmed.
- pd tool versions are `@latest` for now — pin them for reproducibility (config as code).
- naabu uses a connect scan (`-s c`) — no NET_RAW cap needed; SYN is faster but
  requires `--cap-add=NET_ADMIN,NET_RAW`.
- Running from a laptop = one vantage point (home ISP). For completeness, add an
  external VPS (Hetzner/OVH) later, as in the plan.
- IPv6 is not scanned yet — add it.
