# Firewall events into Wazuh — design

Date: 2026-10-15
Module: blackwall-eye
Status: approved in chat (approach C)

## 1. Goal

Turn what our own firewall drops into Wazuh alerts:

- **origin-direct** — a non-allowed source hit the Cloudflare-protected ports
  (`cf-wall` prerouting drop): someone bypasses Cloudflare. High severity.
- **input-drop burst** — one source had many packets dropped by the base input
  chain within an export interval: likely a scan. Medium severity.
- Occasional drops: recorded, no alert.

It must work identically in the lab (containers, podman), on the RHEL prod
host, and on Ubuntu, and be testable end to end in the lab.

## 2. Why not kernel log / ulogd / eBPF

- `log prefix` lines are dropped for non-init network namespaces and the lab
  kernel (OrbStack) has no `nf_log_all_netns` — untestable in the lab.
- `log group` + `ulogd2`: no `ulogd2` package for RHEL 9 (not in EPEL).
- eBPF (`skb:kfree_skb` with `NETFILTER_DROP`) can't tell which rule dropped,
  needs BTF/CAP_BPF and per-kernel validation.

Existing `log` statements stay for manual inspection; nothing builds on them.

## 3. Design

### 3.1 Per-source accounting in nftables

- Base table (`nftables` role), when `nft_track_drops` (default `true`): a
  dynamic set `dropped_input` (`ipv4_addr`, `flags dynamic,timeout`,
  `timeout` = `nft_track_timeout` (default 1h), `size` = `nft_track_size`
  (default 65536), per-element counters) updated right before the input
  chain's final drop, so only packets that are actually dropped are counted.
  Same for `dropped_forward` only when `nft_forward_policy: drop`.
- `cf-wall` refresh script: a dynamic set `origin_direct` with per-element
  counters, updated right before the `counter drop` on the filtered ports.
- IPv4 only (the base and wall tables account IPv4 sources; IPv6 later).
- Set definitions survive the atomic table replace with empty contents — a
  reload resets accounting; the exporter tolerates counters going down.

### 3.2 Exporter

`/usr/local/sbin/blackwall-fw-export` (deployed by the `nftables` role):

- reads `nft -j list set inet blackwall_base dropped_input` (and
  `dropped_forward`, and `inet blackwall_wall origin_direct` when present);
- keeps per-set, per-source last packet counts in
  `/var/lib/blackwall-fw/state.json`;
- prints one line per source whose count grew since the last run:
  `blackwall-fw: kind=<origin_direct|input_drop|forward_drop> src=<ip> new=<delta> total=<count>`;
- a counter that went down (reload, timeout re-add) counts as new = current;
- exits 0 with no output when nothing changed; never fails the agent on a
  missing table/set (prints nothing for it).

### 3.3 Wazuh

- Agent `ossec.conf` (and the manager's own, for agent 000) gets, when
  `wazuh_fw_events_enabled` (default `true`):
  `<localfile><log_format>command</log_format><command>/usr/local/sbin/blackwall-fw-export</command><alias>blackwall-fw</alias><frequency>{{ wazuh_fw_events_interval }}</frequency></localfile>`
  (default 60 s; lab 10 s). `command` format: one event per output line.
- Manager: decoder `blackwall-fw` (fields `kind`, `srcip`, `new`, `total`)
  in `etc/decoders/blackwall_decoders.xml`; rules in `blackwall_rules.xml`:
  - 100200 base, level 0, group `blackwall_fw`;
  - 100201 `kind=origin_direct` -> level 10 "Direct access to origin bypassing Cloudflare";
  - 100202 `kind=input_drop` and `new` >= `wazuh_fw_scan_threshold`
    (default 20) -> level 8 "Possible scan: many dropped packets from one source";
  - forward drops -> level 0 unless it is a burst (same threshold) -> level 6.
  Exact rule mechanics (numeric comparison) are decided in the plan; Wazuh
  4.x rules can't compare numbers natively, so the exporter adds a
  `burst=yes` field when `new >= threshold` (threshold passed by the role).
- Voice/Brain forward level >= their thresholds automatically (lab: 10 — only
  origin-direct reaches Telegram).

## 4. Testing (lab)

- `bw-check fw-events` (opt-in, ~1 min):
  - origin-direct: `black-rocky-02` connects to `172.28.0.16:443` (not in
    `cfwall_admin_allow`) -> alert rule 100201 with `srcip` 172.28.0.15 in the
    sentinel's indexer;
  - scan: `black-rocky-02` connects to 30 closed ports on `black-host-01` ->
    alert 100202 for agent `black-host-01`, `srcip` 172.28.0.15 in the main
    indexer;
  - exporter idempotence: a second run without traffic prints nothing.
- Main-stack checks stay green; exporter unit test with a fixture JSON
  (counters up, unchanged, reset) runs without nft.

## 5. Out of scope

IPv6 accounting; active response (blocking repeat offenders — Fang); TUI view
of firewall events.
