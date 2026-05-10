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
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
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
# Full stack: manager (black-00) + all agents (black-01, black-02)
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

## Tools

### bw-status — Agent Status Dashboard

Query the Wazuh Manager API and print all agents with their status and last heartbeat.

```bash
# Via SSH tunnel to Docker test manager:
ssh -F tests/docker/ssh_config -L 55000:172.28.0.10:55000 black-00 -N &
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
