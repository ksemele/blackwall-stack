# blackwall-eye — Eye

Host monitoring core for the Blackwall security platform. Stage 1: Wazuh manager + agents + auditd on 1–3 hosts.

## Stack

- Wazuh Manager + Agent (HIDS, FIM, log collection)
- auditd (kernel-level syscall audit)
- Ansible IaC — no manual steps
- Target OS: Ubuntu 22.04 LTS / Debian 12 (primary), Rocky Linux 9 (secondary): agent, auditd and the whole server stack (indexer, manager + filebeat, dashboard); on RHEL the agent reads system logs from journald

## Test Environment

Start containers from the **repo root**:

```bash
tests/bin/bw-lab up --no-provision   # or: bw-lab up, to provision straight away
```

Wait for SSH before running Ansible:

```bash
cd blackwall-eye
ansible all -m wait_for_connection
```

See [tests/README.md](../tests/README.md) for full environment documentation (topology, multi-distro, teardown).

## Deploy

Run from `blackwall-eye/`:

```bash
# Full stack: master (black-master-00) + agent (black-host-01)
ansible-playbook ansible/playbooks/all.yml

# Manager only
ansible-playbook ansible/playbooks/security-node.yml

# Agents only
ansible-playbook ansible/playbooks/monitored-host.yml
```

### Package source (production)

The `wazuh-repo` role (a meta dependency of all four Wazuh roles) owns the package
source: apt via `wazuh_apt_repo_url` (default: the public
`https://packages.wazuh.com/4.x/apt/`), yum via `wazuh_yum_repo_url`, signed by
`wazuh_gpg_key_url`, at the version pinned in `wazuh_version`.
For a fleet, point it at a company-wide package mirror instead — no role
changes. For apt set `wazuh_apt_repo_url` (plus `wazuh_gpg_key_url` if the mirror
re-signs, since `signed-by` pins that key); for yum set `wazuh_yum_repo_url`
(plus the key):

```yaml
wazuh_apt_repo_url: "https://apt.internal.example/wazuh/4.14.7"
wazuh_gpg_key_url: "https://apt.internal.example/wazuh/key.gpg"  # if re-signed
```

The old `wazuh_apt_repo` / `wazuh_apt_key_url` names are rejected by an assert.

A versioned internal mirror (aptly/pulp snapshots, Artifactory or Nexus apt
remotes, or apt-cacher-ng for a plain cache) gives:

- **reproducible installs** — the exact `.deb` that was tested, even after
  upstream moves on or pulls a version (we already hit a v4.14.5 asset
  disappearing upstream);
- **supply-chain control** — packages are vetted once at the mirror, hosts
  never talk to the internet directly, egress to `packages.wazuh.com` can be
  closed;
- **speed** — ~1.5 GB per manager node from the LAN instead of the internet.

The Docker lab already runs a small version of this: see
[tests/README.md — apt cache](../tests/README.md#apt-cache).

### Version pinning

Wazuh packages (`wazuh-agent`, `-manager`, `-indexer`, `-dashboard`, `filebeat`;
list in `wazuh_pinned_packages`) change version only through Ansible: apt hosts
hold them (`apt-mark hold`), the RedHat yum repo `exclude`s them, so
`apt upgrade`, unattended-upgrades, `dnf upgrade` and dnf-automatic skip them
(an agent newer than the manager is unsupported). To change the version, edit
`wazuh_version` in the `wazuh-repo` role defaults and re-run the playbooks; the
roles install with `allow_change_held_packages` / `disable_excludes`.

### Server stack on RHEL, loopback-only (sentinel)

The server roles bind through variables, so a host can keep every management
endpoint on loopback and be reached over an SSH tunnel:

| Variable | Main lab stack | Loopback-only host |
|---|---|---|
| `wazuh_indexer_network_host` | `172.28.0.10` | `127.0.0.1` |
| `wazuh_api_host` | `0.0.0.0` | `127.0.0.1` |
| `wazuh_dashboard_listen_host` / `_port` | `0.0.0.0` / `443` | `127.0.0.1` / `5601` |
| `wazuh_indexer_url`, `wazuh_manager_api_url` | `https://172.28.0.10:…` | `https://127.0.0.1:…` |
| `wazuh_cert_ip` (certificate SAN) | `172.28.0.10` | `127.0.0.1` |
| `wazuh_indexer_heap` | package default | e.g. `1g` |

Certificates are generated per server host (`wazuh-certificates.yml`, node name =
inventory hostname, bundle in `wazuh_certs_local_dir`). The lab node
`black-sentinel-03` (`ansible/playbooks/sentinel.yml`, group vars in
`tests/inventory/group_vars/sentinel.yml`) runs this layout on Rocky 9 next to a
podman nginx on 443, with `nftables` (`nft_podman`, forward policy drop) and
`cf-wall`. Dashboard access: `ssh -L 5601:127.0.0.1:5601 <host>` then
https://localhost:5601.

Measured footprint (lab sentinel, idle ~10 min, indexer heap 1g): container
2.2 GiB total; indexer (java) 1.75 GiB RSS, manager processes ~460 MiB,
dashboard ~170 MiB, filebeat ~15 MiB; CPU ~4 %. Plan ~2.5 GiB of RAM for the
stack on top of whatever else the host runs.

### Login sessions snapshot

The `wazuh-agent` role makes every agent run `last --time-format iso -w -n 50`
every `wazuh_sessions_frequency` seconds (default 60; disable with
`wazuh_sessions_enabled: false`). The `wazuh-manager` role installs
`/var/ossec/etc/rules/blackwall_rules.xml` with rule **100100** (level 3, group
`blackwall_sessions`, `check_diff`): the output is stored as an alert only when
it changes, i.e. on each login or logout. The `blackwall` TUI reads the newest
snapshot for its Sessions tab.

`last` reads wtmp, which only records sessions with a PTY: non-interactive
`ssh host cmd`, scp and Ansible runs do not appear in the snapshot or in the
Sessions tab.

The manager config now also declares the standard `<ruleset>` explicitly;
before, `GET /rules` failed with error 1106.

### Firewall drop events

Agents (and the manager host, agent 000) run `/usr/local/sbin/blackwall-fw-export`
(from the `nftables` role) as a Wazuh `command` localfile with alias
`blackwall-fw`, every `wazuh_fw_events_interval` seconds (default 60, lab 10;
disable with `wazuh_fw_events_enabled: false`). The `wazuh-manager` role deploys
decoders (`etc/decoders/blackwall_decoders.xml`, children of the stock `ossec`
decoder, fields `fw_kind`, `srcip`, `fw_new`, `fw_total`, `fw_burst`,
`fw_suppressed`) and rules in group `blackwall_fw`:

| Rule | Level | Fires on |
|---|---|---|
| 100200 | 0 | base, any `blackwall-fw` line (below-threshold input/forward drops stop here) |
| 100201 | 10 | `kind=origin_direct`: direct access to origin bypassing Cloudflare |
| 100202 | 8 | `kind=input_drop` with `burst=yes`: possible scan |
| 100203 | 6 | `kind=forward_drop` with `burst=yes` |
| 100204 | 3 | `kind=summary`: exporter output was capped, `fw_suppressed` sources dropped |
| 100205 | 3 | exporter error line (`blackwall-fw-export: ...` on stderr, field `fw_error`) |

What is counted: packets dropped by the final policy path (input and forward
`policy drop`, and cf-wall's origin-direct drop). Invalid-state and anti-spoof
drops are not counted. Counters live in the kernel sets, so they reset on every
firewall reload (the exporter then reports `new = total`), and a source is not
counted at all once the set reaches its size cap (silent, no error).
End-to-end proof: `bw-check fw-events` (opt-in).

`burst=yes` is decided by the exporter from `nft_track_scan_threshold`, so rules
need no numeric comparison. A burst is per export interval: packets from one
source between two runs. Changing `wazuh_fw_events_interval` therefore changes
what counts as a burst (the lab's 10 s is stricter than production's 60 s: the
same scan rate trips the threshold less often at 10 s).

On a public host expect regular 100201/100202 alerts from internet scanners.
The exporter's per-source cooldown (`nft_track_report_cooldown`, default 3600 s)
bounds this to one alert per source per kind per hour; drops from a source in
cooldown accumulate silently and the next report carries `new` = packets since
its last report. Exceptions: after a firewall reload or the weekly cf-wall refresh
a returning source may be reported immediately (state is rebuilt from the sets),
and drops accumulated during cooldown are discarded if the source goes quiet
until its set entry times out. To tune: raise `nft_track_scan_threshold` (fewer scans count),
raise `nft_track_report_cooldown` (fewer repeats; `0` reports every growth),
or raise Voice's `severity_threshold` above 8 to keep only origin-direct (10)
alerts in Telegram. The first run after deploy or state loss is silent
(baseline). Set `BLACKWALL_FW_DEBUG=1` to see the "baseline recorded" message on
stderr when running the exporter by hand.

## Verify

```bash
# Agent registration (checks Active status)
ansible-playbook ansible/playbooks/verify-agents.yml

# auditd rules and config (service check skipped in Docker — expected)
ansible-playbook ansible/playbooks/verify-auditd.yml
```

## Firewall (nftables)

The `nftables` role renders `/etc/nftables.d/blackwall-base.nft`, owns the main
config (`nft_conf_path`, which includes `/etc/nftables.d/*.nft`) and installs a
`nftables.service` drop-in. Debian and RedHat families are supported (the lab
has `black-host-01` on Ubuntu and `black-rocky-02` on Rocky Linux 9).

```bash
ansible-playbook ansible/playbooks/nftables.yml
ansible-playbook ansible/playbooks/verify-nftables.yml
```

The policy, port and no-`flush` checks run on every host; the reload/stop and
container-DNS part runs only in the lab (`bw_lab: true`).

| Variable | Default | Purpose |
|---|---|---|
| `nft_enabled` | `true` | set to `false` to skip the host entirely |
| `nft_wan_if` | `eth0` | external interface |
| `nft_public_tcp` | `[22]` | TCP ports open to everyone |
| `nft_restricted` | `[]` | list of `{port, sources[], name?}` — port reachable only from those sources; the set is named `name` (default `src_<port>`) |
| `nft_forward_policy` | `accept` | policy of the `forward` chain; `drop` adds a rate-limited log line and a counter |
| `nft_log_drops` | `true` | rate-limited log line on drops |
| `nft_track_drops` | `true` | per-source accounting of dropped IPv4 packets: dynamic sets `dropped_input` (and `dropped_forward` when `nft_forward_policy: drop`) with per-element counters, updated just before the final counter. Inspect: `nft list set inet blackwall_base dropped_input`; reset on every reload |
| `nft_track_timeout` | `1h` | idle timeout of an entry in those sets |
| `nft_track_size` | `65536` | maximum number of tracked sources per set |
| `nft_track_scan_threshold` | `20` | packets from one source between two `blackwall-fw-export` runs (one export interval) that set `burst=yes`; rendered to `/etc/blackwall/fw-export.conf` |
| `nft_track_max_lines` | `200` | cap on lines per exporter run (highest `new` first); the rest becomes one `kind=summary suppressed=<n>` line |
| `nft_track_report_cooldown` | `3600` | seconds between two reports of the same (kind, source); growth in between accumulates silently, `0` reports every growth (lab) |
| `nft_podman` | `{}` | `{net, if}` of a podman host: anti-spoofing on the WAN, container DNS to the host, `ct status dnat accept` and container egress in `forward` |
| `nft_conf_path` | per OS family | `/etc/nftables.conf` (Debian) or `/etc/sysconfig/nftables.conf` (RedHat) |

The `input` chain is always `policy drop` with a fixed scaffolding of
`iif lo accept`, `ct state established,related accept`, `ct state invalid drop`
and full ICMP, so a host cannot lock itself out through this role.

Two things to know before changing the port lists:

- **The role never runs a global `flush ruleset`.** Each file under
  `/etc/nftables.d/` replaces only its own table. A global flush destroys tables
  the role does not own — inside Docker it wipes the NAT rules of the embedded
  resolver `127.0.0.11`, killing DNS in the container while the ruleset itself
  still looks correct.
- **`systemctl reload/stop nftables` never flush either.** The stock unit runs
  `flush ruleset` on both; the role's drop-in
  (`/etc/systemd/system/nftables.service.d/blackwall.conf`) re-reads the config
  on reload and deletes only `blackwall_base` / `blackwall_wall` on stop.
- **On a Wazuh manager, 9200 and 9300 must stay open.** filebeat and the
  dashboard reach the indexer over the interface address, not over loopback, so
  closing those ports stops alert delivery while every service stays `active`.

Ports for the test environment live in `tests/inventory/group_vars/`.

### Cloudflare origin filter (`cf-wall`)

For a site behind Cloudflare, the `cf-wall` role lets only Cloudflare prefixes
(and `cfwall_admin_allow`) reach `cfwall_ports` on the WAN interface. It installs
`blackwall-cf-refresh` and a weekly timer; the script generates
`/etc/nftables.d/blackwall-wall.nft`. Details, variables and rollback:
[ansible/roles/cf-wall/README.md](ansible/roles/cf-wall/README.md).

```bash
ansible-playbook ansible/playbooks/cf-wall.yml        # group cf_wall
ansible-playbook ansible/playbooks/verify-cf-wall.yml
```

### Real hosts: values stay local

Host-specific values (ports, allowed sources, podman subnet, admin IPs) go into
a git-ignored inventory, e.g. `hosts/prod.yml` — the repository keeps only the
roles. Before applying to a host whose ruleset was hand-made, prove the role
renders the same table:

```bash
tools/nft-equiv hosts/prod.yml <host> hosts/<host>/nftables/blackwall-base.nft
```

It renders the role template for that host, loads both files into a throwaway
container and diffs nft's own listing (exit 0 = equal). Needs Docker.

### Firewall event exporter

The role installs `/usr/local/sbin/blackwall-fw-export` (python3, stdlib only;
state in `/var/lib/blackwall-fw/state.json`). It reads the per-source sets
`dropped_input`, `dropped_forward` and the cf-wall `origin_direct` and prints
one line per source whose packet count grew since the previous run (a counter
that went down after a reload counts as `new = total`); nothing is printed when
nothing changed or a table/set is absent:

```
blackwall-fw: kind=<input_drop|forward_drop|origin_direct> src=<ip> new=<n> total=<n> burst=<yes|no>
```

`burst=yes` when `new >= nft_track_scan_threshold`. The first run (state missing or corrupt) only records a baseline and prints nothing; state is saved before printing (if that fails, nothing is printed); each (kind, src) is reported at most once per `nft_track_report_cooldown`; output is capped by `nft_track_max_lines` (origin_direct first, then input/forward bursts, then the rest, highest `new` first). Overrides for tests:
`BLACKWALL_FW_NFT_JSON_DIR`, `BLACKWALL_FW_STATE_DIR`,
`BLACKWALL_FW_SCAN_THRESHOLD`, `BLACKWALL_FW_MAX_LINES`, `BLACKWALL_FW_REPORT_COOLDOWN`, `BLACKWALL_FW_NOW`, `BLACKWALL_FW_CONF`. Unit test (no nft needed):
`tools/test-fw-export`.

## Verification playbooks

| Playbook | Asserts |
|---|---|
| `verify-agents.yml` | agents registered on the manager |
| `verify-api.yml` | manager REST API answers |
| `verify-auditd.yml` | auditd rules and config |
| `verify-nftables.yml` | `input` is `policy drop`, expected ports open |
| `verify-versions.yml` | expected packages installed, prints versions |
| `verify-ports.yml` | expected listeners inside the hosts |

`tests/bin/bw-check` wraps all of these — see [tests/README.md](../tests/README.md).

## Tools

### Agent status

The Python `bw-status` script is gone; use the `blackwall` TUI
([tools/blackwall](../tools/blackwall/README.md)):

```bash
blackwall --context lab           # interactive, k9s-style
blackwall --context lab --once    # print agents and exit
```

## Docs

- `CLAUDE.md` — full module context, design decisions, agent rules
- `STATUS.md` — current implementation state
- `docs/ARCHITECTURE.md` — system architecture and deployment model
