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

### bw-status — Agent Status Dashboard

Query the Wazuh Manager API and print all agents with their status and last heartbeat.

```bash
# Via SSH tunnel to Docker test manager:
ssh -F tests/docker/ssh_config -L 55000:172.28.0.10:55000 black-master-00 -N &
./blackwall-eye/tools/bw-status --host localhost

# Direct (if Wazuh API port is accessible from your machine):
./blackwall-eye/tools/bw-status --host 172.28.0.10

# Options:
#   --host HOST         Wazuh Manager hostname or IP (default: localhost)
#   --port PORT         Wazuh API port (default: 55000)
#   --user USER         Wazuh API user (default: wazuh)
#   --password PASSWORD Wazuh API password (default: wazuh)
#   --no-color          Disable ANSI color output
```

## Docs

- `CLAUDE.md` — full module context, design decisions, agent rules
- `STATUS.md` — current implementation state
- `docs/ARCHITECTURE.md` — system architecture and deployment model
