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

Requires the Eye `nftables` role on the host (`/etc/nftables.d/` and its service drop-in).
Fang adds `zz-blackwall-fang.conf` so `systemctl stop nftables` removes its table too.

### Variables

| Variable | Default | Meaning |
|---|---|---|
| `fang_mode` | `observe` | `observe`: count and report only; `enforce`: also drop |
| `fang_wan_if` | `""` | external interface, **required** (no autodetect) |
| `fang_allowlist` | `[]` | IPv4 CIDRs never blocked (owner, admin, monitoring); Cloudflare ranges and the host's own addresses are always added |
| `fang_sources` | Spamhaus DROP v4 JSON, FireHOL level1 | https URLs |
| `fang_cloudflare_url` | `https://www.cloudflare.com/ips-v4` | allowlist source |
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

Any failure leaves the live table and the persisted file as they were (also: Cloudflare list
under 10 prefixes, or one source junk/empty next to a good one). Validated source data and the
Cloudflare list are cached in `/var/lib/blackwall-fang/` (0700). If a download fails, the table
is rebuilt from that cache with the **current** mode and allowlist (exit 3, the playbook prints a
warning), so allowlist/mode changes never wait for the internet. Without a cache, the first run
(no table, no persisted file) loads an empty blocklist (fail open, exit 2, warning). Exit codes:
0 ok, 1 aborted, 2 fail-open, 3 from cache. `blackwall-fang-refresh.timer` repeats the refresh
daily (`Persistent`, randomized delay); the role also runs it on install and whenever the script
or config change. Each refresh replaces the table, so `fang_hits` counters restart. In enforce
mode the drop is its own rule, so a full `fang_hits` set cannot let a listed source through.

Inspect: `nft list table inet blackwall_fang`, `nft list set inet blackwall_fang fang_hits`,
`journalctl -u blackwall-fang-refresh`.

### Observe vs enforce

Start in `observe`: hits are counted in `fang_hits` but nothing is dropped, so the first
days show who would have been blocked. Switch with `fang_mode: enforce` and re-run the playbook.

### Rollback

```bash
nft delete table inet blackwall_fang
systemctl disable --now blackwall-fang-refresh.timer
rm /etc/nftables.d/blackwall-fang.nft
```

### Run

```bash
cd blackwall-fang
ansible-playbook ansible/playbooks/fang.yml
ansible-playbook ansible/playbooks/verify-fang.yml
```

Lab (black-sentinel-03, group `fang`, test lists in `tests/fang/`): `tests/bin/bw-lab provision fang`
(extra args pass through, e.g. `-e fang_mode=enforce`). Unit test: `python3 tests/unit/fang_refresh_test.py`.
