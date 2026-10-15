# blackwall-eye — Eye

Host monitoring core for the Blackwall security platform. Stage 1: Wazuh manager + agents + auditd on 1–3 hosts.

## Stack

- Wazuh Manager + Agent (HIDS, FIM, log collection)
- auditd (kernel-level syscall audit)
- Ansible IaC — no manual steps
- Target OS: Ubuntu 22.04 LTS / Debian 12 (primary), Rocky Linux 9 (secondary; the agent reads system logs from journald)

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
For a fleet, point it at a company-wide package mirror instead — one variable,
no role changes:

```yaml
wazuh_apt_repo_url: "https://apt.internal.example/wazuh/4.14.7"
```

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
