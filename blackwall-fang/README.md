# blackwall-fang — Fang

Active countermeasures for the Blackwall platform. Layer 1 (reputation blocklists) is
implemented; see [STATUS.md](STATUS.md) for the plan and the rules every automatic block must follow.

## Reputation blocklists (`fang-reputation`)

Drops traffic from sources on public reputation lists (Spamhaus DROP, FireHOL level1)
before it reaches host ports or published container ports, without cutting off
Cloudflare, your admin addresses or internal networks.

The table `inet blackwall_fang` hooks **prerouting at priority -160** (before cf-wall at
-150 and netavark DNAT at -100). Only traffic arriving on `fang_wan_if` is judged:
allowlist accept -> established accept -> source in `blocklist_static` updates
`fang_hits` (per-source counters, 1h idle timeout) and, in enforce mode, is dropped.
IPv4 only.

Requires python3 >= 3.7 and the Eye `nftables` role on the host (`/etc/nftables.d/` and its service drop-in).
Fang adds `zz-blackwall-fang.conf` so `systemctl stop nftables` removes its table too.

### Variables

| Variable | Default | Meaning |
|---|---|---|
| `fang_mode` | `observe` | `observe`: count and report only; `enforce`: also drop |
| `fang_wan_if` | `""` | external interface, **required** (no autodetect) |
| `fang_allowlist` | `[]` | IPv4 CIDRs never blocked (owner, admin, monitoring, agents); Cloudflare ranges and the host's own addresses are always added. **Must be non-empty in enforce mode** (the role asserts it): Fang filters ALL WAN ports, SSH included |
| `fang_sources` | Spamhaus DROP v4 JSON, FireHOL level1 | https URLs |
| `fang_cloudflare_url` | `https://www.cloudflare.com/ips-v4` | allowlist source |
| `fang_max_cache_age_days` | `7` | a cache older than this is not trusted: the blocklist is emptied (fail open, exit 4) |
| `fang_min_entries` | `500` | abort when fewer entries survive filtering; each source must also yield at least `min_entries / number of sources` |
| `fang_refresh_calendar` / `fang_refresh_jitter` | `daily` / `1h` | timer schedule |
| `fang_test_mode` | `false` | lab only: allows `file://` sources (and then only `file://`) and `fang_test_blockable`; set it in host_vars, never in a shared group |
| `fang_test_blockable` | `[]` | lab only: single private hosts (/32) exempt from the reserved filter |
| `fang_test_files` | `[]` | lab only: controller files copied to `/etc/blackwall/fang-test/` |

### How it runs

`/usr/local/sbin/blackwall-fang-refresh` (config: `/etc/blackwall/fang.conf`):

1. fetches the sources (https only) and the Cloudflare list;
2. drops private, loopback, link-local, CGNAT, documentation, multicast and reserved
   ranges; rejects the update if any remaining prefix is shorter than /8 or fewer than
   `fang_min_entries` entries remain;
3. carves the allowlist out of the blocklist (a listed range containing an allowed address is split);
4. renders the table for the mode, checks it with `nft -c`, loads it, and only then writes
   `/etc/nftables.d/blackwall-fang.nft` (loaded at boot by the Eye nftables config).

Any failure without a usable cache leaves the live table and the persisted file as they were
(also: Cloudflare list under 10 prefixes, or one source junk/empty next to a good one). Validated
source data and the Cloudflare list are cached in `/var/lib/blackwall-fang/` (0700). If a download
fails, or the fresh data is rejected by the floor/format checks, the table is rebuilt from that
cache with the **current** mode and allowlist (exit 3, a WARNING naming the rejected source, the
playbook prints a warning), so allowlist/mode changes never wait for the internet. A cache older
than `fang_max_cache_age_days` is not trusted: the table is rebuilt with an EMPTY blocklist (fail
open, allowlist and mode unchanged, ERROR, exit 4; the playbook treats it as success with a
warning). Without any cache, a rejected update aborts (exit 1, live table untouched) and the first
run (no table, no persisted file) with no successful fetch loads an empty blocklist (fail open,
exit 2). Exit codes: 0 ok, 1 aborted, 2 fail-open (first run), 3 from cache, 4 cache too old,
empty blocklist. `blackwall-fang-refresh.timer` repeats the refresh
daily (`Persistent`, randomized delay); the role also runs it on install and whenever the script
or config change. Each refresh replaces the table, so `fang_hits` counters restart. In enforce
mode the drop is its own rule, so a full `fang_hits` set cannot let a listed source through.

Events: the role drops `/etc/blackwall/fw-export.d/fang.conf`, so Eye's `blackwall-fw-export`
reports growth of `fang_hits`. The file is re-rendered on a mode change and carries the mode in
the kind: `blocklist_would_drop` in observe (Wazuh rule 100206, level 5), `blocklist_dropped` in
enforce (rule 100208, level 3). Cooldown and baseline are the exporter's; Fang needs no Eye code,
only that file.

Health: the script logs its ERROR and WARNING lines to syslog (identifier
`blackwall-fang-refresh`; stderr goes to the journal as well). Eye's rules turn them into Wazuh
alerts: 100207 (level 7) for ERROR (stale cache -> empty blocklist, aborted refresh), 100209
(level 5) for WARNING (applied from cache, fail-open). The Wazuh manager must read system logs
(journald on the RedHat family, `/var/log/syslog` on Debian; the Eye roles configure both).

Inspect: `nft list table inet blackwall_fang`, `nft list set inet blackwall_fang fang_hits`,
`journalctl -u blackwall-fang-refresh`.

### Observe vs enforce

Start in `observe`: hits are counted in `fang_hits` but nothing is dropped, so the first
days show who would have been blocked. Switch with `fang_mode: enforce` and re-run the playbook
(lab: `tests/bin/bw-lab provision fang -e fang_mode=enforce`). Review the 100206 alerts first:
every source that appears there will be dropped in enforce mode, so add legitimate ones to
`fang_allowlist`.

**Lockout guard.** Fang filters ALL WAN ports, SSH included, and the role refuses enforce with an
empty `fang_allowlist`. Put the owner's admin addresses (the same ones as `cfwall_admin_allow`),
monitoring sources and future agent sources there. Keep the modules isolated by composing the
list in the inventory, not in a role:

```yaml
# inventory group_vars/host_vars
fang_allowlist: "{{ cfwall_admin_allow + ['198.51.100.0/24'] }}"   # + monitoring / agent sources
```

### What 100206 / 100208 mean

Wazuh rule 100206 (level 5, group blocklist) fires in observe mode (`kind=blocklist_would_drop`)
when a source on `blocklist_static` sent packets that reached the WAN hook: "a listed source reached
the host, would be dropped in enforce"; `srcip` is the listed source. Rule 100208 (level 3,
`kind=blocklist_dropped`) is the same event in enforce mode: the source was dropped. Because each
refresh rebuilds the table, counters restart and the exporter reports the new total once per cooldown. Needs the Eye exporter
(`blackwall-fw-export`) and its `/etc/blackwall/fw-export.d/` directory, which must stay
root-owned (the exporter skips drop-ins that are not).

### Layer 2: connection-attempt detectors (observe only)

Independent of the reputation lists (rendered even with an empty blocklist), after the
allowlist, `established,related` and the reputation rules, in the same `prerouting` chain. They
look only at new TCP SYNs, count per source (IPv4) in set-backed `limit rate over` meters and put
a source over the rate into a flagged set (`flags dynamic,timeout`, 1h, counters). **Nothing is
dropped**; enforcement is a separate future spec.

| Detector | Matches | Default | Flagged set | Exporter kind / Eye rule |
|---|---|---|---|---|
| SSH | `tcp dport` in `fang_ssh_ports` (`[22]`) | 6/minute, burst 4 | `ssh_flagged` | `ssh_attempts` / 100210 (level 6) |
| Closed ports | `tcp dport != @open_tcp`, only when `fang_open_tcp` is non-empty | 10/minute, burst 10 | `scan_flagged` | `scan_attempts` / 100211 (level 6) |
| Rate | any new TCP | 120/minute, burst 60 | `rate_flagged` | `rate_attempts` / 100212 (level 5) |

Variables: `fang_attempts_enabled` (default `true`), `fang_ssh_ports`, `fang_open_tcp`,
`fang_{ssh,scan,conn}_rate` (`N/second|minute`), `fang_{ssh,scan,conn}_burst`,
`fang_meter_size`, `fang_flagged_size`. List **every** listening and published port (and agent or
monitoring ports) in `fang_open_tcp`, or legitimate clients of an unlisted port look like
scanners; leave it empty to run without the scan detector. The role renders the exporter drop-ins
`fang-ssh.conf`, `fang-scan.conf`, `fang-rate.conf` in `/etc/blackwall/fw-export.d/` for the
enabled detectors only (and removes the others). Inspect: `nft list set inet blackwall_fang
ssh_flagged` (also `scan_flagged`, `rate_flagged`).

Reading the alerts: rule 100210/100211 (level 6) and 100212 (level 5) carry `srcip` = the flagged
source; the exporter reports growth of that source's flagged-set counter, so repeated alerts mean
the source keeps connecting. Triage with `nft list set inet blackwall_fang ssh_flagged` (entries
live 1h). Tune from real alerts on the target host: a legitimate source -> `fang_allowlist`; a
real service flagged as scan -> add its port to `fang_open_tcp`; chronic noise -> raise
`fang_*_rate` / `fang_*_burst`. Lab check: `bw-check fang` (rocky, host-01 and master against the
sentinel; asserts growth over a baseline taken at the start, since flagged entries persist).

Limits and tuning:

- Legitimate high-rate sources belong in `fang_allowlist`: uptime and monitoring probes, NAT'd
  offices, blackwall-scan's own perimeter recon (it looks like a scanner by design), and the
  operator's own Ansible/SSH source (the controller path: every provisioning run and ssh session
  arrives from it and would otherwise be flagged constantly).
- Every refresh (daily timer, config change) replaces the table, so meters and flagged sets reset:
  a flagged source alerts again once after a refresh. On a first run without any Cloudflare data,
  layer 2 is skipped until the next successful refresh.
- Blind spot: a spoofed-source SYN flood can fill the meter sets (65536 sources); the rule then
  silently stops tracking new sources until entries time out (5m). Observe has nothing to
  protect; enforcement must account for it.
- SYN retransmits to filtered ports count as separate attempts, so one slow client can
  exceed a low rate.
- If the Cloudflare list is not available on a fail-open refresh (exit 2/4) and not cached,
  layer 2 is skipped (WARNING `layer 2 skipped: no Cloudflare list`), otherwise Cloudflare
  edges would be counted.

### Rollback

```bash
nft delete table inet blackwall_fang
systemctl disable --now blackwall-fang-refresh.timer
rm /etc/nftables.d/blackwall-fang.nft
```

Leftover files (harmless without the table and the timer; remove for a clean host):
`/usr/local/sbin/blackwall-fang-refresh`, `/etc/blackwall/fang.conf`, `/etc/blackwall/fang-test/`,
`/etc/blackwall/fw-export.d/fang*.conf`, `/var/lib/blackwall-fang/` (cache), the units
`blackwall-fang-refresh.{service,timer}` and
`/etc/systemd/system/nftables.service.d/zz-blackwall-fang.conf`.

### Run

```bash
cd blackwall-fang
ansible-playbook ansible/playbooks/fang.yml
ansible-playbook ansible/playbooks/verify-fang.yml
```

Lab (black-sentinel-03, group `fang`, test lists in `tests/fang/`): `tests/bin/bw-lab provision fang`
(extra args pass through, e.g. `-e fang_mode=enforce`). End-to-end: `tests/bin/bw-check fang`
(opt-in, ~3-5 min; observe, degraded refresh, enforce, over-blocking proof, junk list, back to observe). Unit test: `python3 tests/unit/fang_refresh_test.py`.

Real-source run (checked on the lab sentinel, observe mode, test mode off): Spamhaus DROP v4
JSON 1680 entries, FireHOL level1 4631 raw / 4618 after the reserved-range filter, 4619 merged
in `blocklist_static` (shortest prefix /12, no private/reserved range, no overlap with
`allow_v4`); `allow_v4` 19 entries (Cloudflare ranges, 172.28.0.10, host addresses); exit 0.
