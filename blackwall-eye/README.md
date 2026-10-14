# blackwall-eye — Eye

Host monitoring core for the Blackwall security platform. Stage 1: Wazuh manager + agents + auditd on 1–3 hosts.

## Stack

- Wazuh Manager + Agent (HIDS, FIM, log collection)
- auditd (kernel-level syscall audit)
- Ansible IaC — no manual steps
- Target OS: Ubuntu 22.04 LTS / Debian 12 (primary), Rocky Linux 9 (secondary)

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

The roles install Wazuh from `wazuh_apt_repo` (default: the public
`https://packages.wazuh.com/4.x/apt/`) at the version pinned in `wazuh_version`.
For a fleet, point it at a company-wide package mirror instead — one variable,
no role changes:

```yaml
wazuh_apt_repo: "deb https://apt.internal.example/wazuh/4.14.7 stable main"
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

The `nftables` role renders `/etc/nftables.d/blackwall-base.nft` and owns
`/etc/nftables.conf`.

```bash
ansible-playbook ansible/playbooks/nftables.yml
ansible-playbook ansible/playbooks/verify-nftables.yml
```

| Variable | Default | Purpose |
|---|---|---|
| `nft_enabled` | `true` | set to `false` to skip the host entirely |
| `nft_wan_if` | `eth0` | external interface |
| `nft_public_tcp` | `[22]` | TCP ports open to everyone |
| `nft_restricted` | `[]` | list of `{port, sources[]}` — port reachable only from those sources |
| `nft_forward_policy` | `accept` | policy of the `forward` chain |
| `nft_log_drops` | `true` | rate-limited log line on drops |

The `input` chain is always `policy drop` with a fixed scaffolding of
`iif lo accept`, `ct state established,related accept`, `ct state invalid drop`
and full ICMP, so a host cannot lock itself out through this role.

Two things to know before changing the port lists:

- **The role never runs a global `flush ruleset`.** Each file under
  `/etc/nftables.d/` replaces only its own table. A global flush destroys tables
  the role does not own — inside Docker it wipes the NAT rules of the embedded
  resolver `127.0.0.11`, killing DNS in the container while the ruleset itself
  still looks correct.
- **On a Wazuh manager, 9200 and 9300 must stay open.** filebeat and the
  dashboard reach the indexer over the interface address, not over loopback, so
  closing those ports stops alert delivery while every service stays `active`.

Ports for the test environment live in `tests/inventory/group_vars/`.

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
