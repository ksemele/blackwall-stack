# blackwall-eye — Test Environment

Docker-based test environment for iterating on Ansible roles before deploying to real hosts.
Uses privileged containers with systemd as PID 1 so service management works identically to
production hosts.

## Quick Start

> All Ansible commands run from `blackwall-eye/ansible/` — `ansible.cfg` lives there and sets `roles_path`.
> Docker Compose commands run from the repo root.

### Bring up (Ubuntu 22.04, full topology: manager + 2 agents)

```bash
docker compose -f blackwall-eye/tests/docker/docker-compose.yml \
  --profile ubuntu2204 up -d --build
```

### Bring up (single host, for testing one role in isolation)

```bash
docker compose -f blackwall-eye/tests/docker/docker-compose.yml \
  --profile ubuntu2204 up -d --build black-00
```

### Tear down and remove volumes

```bash
docker compose -f blackwall-eye/tests/docker/docker-compose.yml \
  --profile ubuntu2204 down -v
```

### Wait for SSH to be ready

```bash
cd blackwall-eye/ansible
ansible all -i ../tests/inventory/hosts.yml -m wait_for_connection
```

### Run a playbook

```bash
cd blackwall-eye/ansible

# Full topology (manager + agents)
ansible-playbook -i ../tests/inventory/hosts.yml playbooks/site.yml

# Monitored hosts only (wazuh-agent, auditd)
ansible-playbook -i ../tests/inventory/hosts.yml playbooks/monitored-host.yml

# Single host (isolated role test)
ansible-playbook -i ../tests/inventory/hosts.single.yml playbooks/monitored-host.yml
```

### Test a specific role

```bash
cd blackwall-eye/ansible

# wazuh-agent on all agent hosts
ansible-playbook -i ../tests/inventory/hosts.yml playbooks/monitored-host.yml \
  -e "wazuh_manager_host=black-00" --tags wazuh-agent

# auditd on a single host
ansible-playbook -i ../tests/inventory/hosts.single.yml playbooks/monitored-host.yml \
  --tags auditd

# Dry-run (check mode, no changes applied)
ansible-playbook -i ../tests/inventory/hosts.yml playbooks/monitored-host.yml \
  --check --diff
```

### Verify services on hosts

```bash
cd blackwall-eye/ansible

# Check wazuh-agent status on all agent hosts
ansible wazuh_agents -i ../tests/inventory/hosts.yml \
  -m command -a "systemctl status wazuh-agent" --become

# Check auditd
ansible wazuh_agents -i ../tests/inventory/hosts.yml \
  -m command -a "systemctl status auditd" --become

# Ad-hoc: run any command
ansible black-01 -i ../tests/inventory/hosts.yml \
  -m command -a "wazuh-agent --version" --become
```

## SSH Access

Use the bundled `ssh_config` for convenient manual access:

```bash
ssh -F blackwall-eye/tests/docker/ssh_config black-00
```

Or append to your personal SSH config:

```bash
cat blackwall-eye/tests/docker/ssh_config >> ~/.ssh/config
```

## Multi-Distro Support

The compose file uses [Compose profiles](https://docs.docker.com/compose/profiles/) to
select the target distro. One compose file, distro selected by `--profile` flag at runtime.
Port ranges are reserved per distro to avoid conflicts when switching between them.

| Profile | Base image | systemd | Ports | Status |
|---|---|---|---|---|
| `ubuntu2204` | geerlingguy/docker-ubuntu2204-ansible | yes | 2200–2202 | Active |
| `debian12` | (planned) | yes | 2210–2212 | Not implemented |
| `centos9` | (planned) | yes | 2220–2222 | Not implemented |
| `alpine` | (planned) | **no** — OpenRC | 2230+ | Not implemented |

> **Alpine note:** Alpine uses OpenRC instead of systemd. Roles that require systemd services
> (Wazuh, auditd) **cannot** be tested on Alpine. Alpine is only suitable for roles with no
> service dependencies.

### Adding a new distro

1. Create `tests/docker/images/<distro>/Dockerfile` (use a geerlingguy systemd image as base where available)
2. Uncomment the corresponding service block in `tests/docker/docker-compose.yml`
3. Add host entries to `tests/inventory/hosts.yml`

## Host Naming and Topology

| Container | Default role | SSH port | Static IP |
|---|---|---|---|
| `black-00` | Wazuh Manager / security-node | 2200 | 172.28.0.10 |
| `black-01` | Wazuh Agent / monitored-host | 2201 | 172.28.0.11 |
| `black-02` | Wazuh Agent / monitored-host | 2202 | 172.28.0.12 |

Future distro hosts use port ranges and IP ranges starting at 2210/172.28.0.20 (debian12),
2220/172.28.0.30 (centos9), etc. See `docker-compose.yml` comments for details.

## File Layout

```
tests/
├── docker/
│   ├── images/
│   │   └── ubuntu2204/Dockerfile       # geerlingguy base + openssh-server
│   ├── docker-compose.yml              # all distros via Compose profiles
│   └── ssh_config                      # convenience SSH aliases
└── inventory/
    ├── hosts.yml                       # full topology (manager + 2 agents)
    └── hosts.single.yml                # single-host for isolated role tests
```

## Security Note

The containers use `root` with password `blackwall` and `PermitRootLogin yes`.
This is intentional for local test environments. Do not expose these containers
to any network outside `localhost`.
