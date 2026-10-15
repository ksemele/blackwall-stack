# Fang layer 1: reputation blocklists — design

Date: 2026-10-15
Module: blackwall-fang (first production feature of the module)
Status: approved in chat

## 1. Goal

Drop traffic from sources on public reputation lists (Spamhaus DROP, FireHOL
level1) before it reaches anything on a protected host, without ever cutting
off Cloudflare, the owner's admin addresses, monitoring, or internal networks.
Starts in observe-only mode; every would-be/actual block is a Wazuh event.

## 2. Design

### 2.1 Table and placement

- Own table `inet blackwall_fang` (atomic create/delete/define, like the Eye
  tables), chain `prerouting`, `type filter hook prerouting priority -160` —
  before cf-wall (-150) and netavark DNAT (-100), so host ports and published
  container ports are both covered.
- Rules apply only on the WAN interface (`fang_wan_if`, required — no
  autodetect fallback, same reasoning as cf-wall): `iifname != <wan> accept`
  first.
- Order: allowlist accept -> `ct state established,related accept` ->
  source in `blocklist_static`:
  - `observe`: `update @fang_hits { ip saddr counter }` and continue (no drop);
  - `enforce`: `update @fang_hits { ip saddr counter }` then `drop`.
- Sets: `allow_v4` (interval), `blocklist_static` (interval, auto-merge),
  `fang_hits` (dynamic, timeout 1h, size 65536, per-element counters).
- IPv4 only (IPv6 later).

### 2.2 Refresh script `blackwall-fang-refresh` + daily timer

- Sources (role var `fang_sources`): Spamhaus DROP v4 JSON
  (`https://www.spamhaus.org/drop/drop_v4.json`, field `cidr`; EDROP is merged
  into DROP since 2024) and FireHOL level1 netset. https only in production; a
  test-only override may allow `file://` (lab).
- Normalise to IPv4 CIDRs; drop everything inside private / reserved /
  multicast / loopback / link-local / CGNAT / documentation ranges (FireHOL
  level1 carries 0/8, 10/8, 127/8, 172.16/12, 192.168/16, 224/3 ...);
  reject the update if any prefix is shorter than /8 or the total is below a
  sanity floor (`fang_min_entries`, default 500).
- Allowlist = `fang_allowlist` (owner/admin/monitoring) + Cloudflare v4
  ranges fetched by the script + the host's own addresses. Every blocklist
  entry overlapping the allowlist is removed before loading (allow also wins
  at rule level).
- `nft -c` -> load -> persist to `/etc/nftables.d/blackwall-fang.nft` (loaded
  by Eye's nftables config via the `*.nft` include; the Eye drop-in removes
  only `blackwall_base`/`blackwall_wall` on stop — Fang adds its own drop-in
  line or documents that `systemctl stop nftables` leaves Fang's table).
- Fail-safe: any failure leaves the live table untouched; first install with
  no successful fetch loads an empty blocklist (fail open, not closed).

### 2.3 Events

- Eye's `blackwall-fw-export` gains generic extra sets: every
  `/etc/blackwall/fw-export.d/*.conf` declares `kind=<name> table=<family name>
  set=<set>`; Eye knows nothing about Fang.
- Fang drops `fang.conf` with `kind=blocklist_hit`.
- Wazuh rules (manager role, Eye): 100206 `blocklist_hit` level 5 when the
  exporter reports it (mode is not in the line; the README explains that in
  observe mode this means "would be dropped", in enforce "dropped"). Cooldown
  and baseline semantics are the exporter's.

### 2.4 Lab

- Group `fang` = `black-sentinel-03` (the prod model). Lab sources point at a
  local test list containing `black-rocky-02` (172.28.0.15) plus junk/private
  ranges; `fang_allowlist: [172.28.0.10]` (master).
- `bw-lab provision fang`, opt-in `bw-check fang`:
  - private ranges from the list are not in `blocklist_static`;
  - observe: rocky -> sentinel 443 still reaches the allowed path decision
    (cf-wall then drops it as origin-direct — fine), rocky appears in
    `fang_hits`, a 100206 alert for srcip 172.28.0.15 in the sentinel indexer;
  - enforce: rocky's packets are dropped in prerouting (fang counter grows,
    cf-wall origin_direct does not), master (allowlisted) still gets 200 on
    443;
  - a junk / too-small list leaves the live table unchanged;
  - back to observe at the end.

## 3. Out of scope

Layer 2 (in-kernel attempt bans), Wazuh active response, CrowdSec, IPv6.
