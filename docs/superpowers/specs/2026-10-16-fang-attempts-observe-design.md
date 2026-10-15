# Fang layer 2: connection-attempt detection (observe only) — design

Date: 2026-10-16
Module: blackwall-fang
Status: approved in chat — observe only; enforcement is a separate future spec
(`2026-10-16-fang-attempts-enforce-design.md`).

## 1. Goal

Flag, in the kernel, sources that make too many new connection attempts to a
protected host, without dropping anything:

- **SSH attempts** — new TCP connections to the SSH port from one source above
  a rate (brute force / credential stuffing / floods).
- **Closed-port scans** — new TCP connections to ports that are not open on
  the host, above a rate (only when the open-port list is configured).
- **High connection rate** — new connections of any kind above a higher rate.

Each flagged source becomes a Wazuh event. Nothing is blocked.

## 2. Design

### 2.1 Placement

Same table `inet blackwall_fang`, same `prerouting` chain, rendered by
`blackwall-fang-refresh` (so the allowlist — admin, monitoring, host
addresses and the Cloudflare ranges — applies; Cloudflare proxies open many
connections and must never be flagged). The new rules sit after
`iifname != WAN accept`, the allowlist accept and
`ct state established,related accept`, and after the reputation rules; they
only look at `ct state new` TCP SYNs. Layer 2 is independent of the
reputation lists: it is rendered even when the blocklist is empty
(fail-open / first run).

### 2.2 Rules (per source, IPv4)

Rate meters are set-backed per-element `limit rate over` expressions
(`add @meter { ip saddr limit rate over R/minute burst B packets }`); a source
over the rate is recorded in a flagged set with a per-element counter and a
timeout. Observe never drops.

| Detector | Match | Default rate | Flagged set |
|---|---|---|---|
| SSH | `tcp dport {{ fang_ssh_ports }}` (default `[22]`) | 6/minute, burst 4 | `ssh_flagged` |
| Closed ports | `tcp dport != @open_tcp` — only when `fang_open_tcp` is non-empty | 10/minute, burst 10 | `scan_flagged` |
| Rate | any new TCP | 120/minute, burst 60 | `rate_flagged` |

Flagged sets: `flags dynamic,timeout`, `timeout 1h`, `size 65536`, counters.
Meter sets: dynamic, short timeout (5m), `size 65536`. All thresholds and
sizes are role variables; `fang_attempts_enabled` (default `true`).

Full-set behaviour: an `update` on a full set only breaks that rule; in
observe there is no drop to protect, so no extra split is needed (the enforce
spec splits the drop out, as layer 1 does).

### 2.3 Events

Fang renders three exporter drop-ins (`/etc/blackwall/fw-export.d/`):
`kind=ssh_attempts`, `kind=scan_attempts`, `kind=rate_attempts`. Eye rules:

| Rule | Kind | Level |
|---|---|---|
| 100210 | ssh_attempts | 6 |
| 100211 | scan_attempts | 6 |
| 100212 | rate_attempts | 5 |

Exporter baseline/cooldown/cap semantics apply (default cooldown 1h per
source and kind).

### 2.4 Lab

- `fang_open_tcp` on the sentinel = its real open ports (22, 443 plus
  1514/1515 for agent sources) so the scan detector is exercised.
- `bw-check fang` (extended): rocky makes 20 quick SSH connection attempts
  and 30 connects to closed ports on the sentinel -> rocky in `ssh_flagged`
  and `scan_flagged`, alerts 100210/100211 newer than the test start;
  black-host-01 makes 2 SSH attempts -> not flagged; master (allowlisted)
  makes 20 SSH attempts -> not flagged; nothing was dropped (SSH from rocky
  still completes a TCP handshake). The existing layer 1 checks stay.

## 3. Out of scope

Dropping (future enforce spec), Wazuh active response (future spec), IPv6,
UDP.
