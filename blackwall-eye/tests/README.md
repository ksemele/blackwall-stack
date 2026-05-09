# blackwall-eye — Test Environment

Docker-based test environment for iterating on Ansible roles before deploying to real hosts.
Uses privileged containers with systemd as PID 1 so service management works identically to production hosts.

> **Working directory: `blackwall-eye/`** — run all commands from there.
> `ansible.cfg` in this directory sets inventory and roles path automatically.

---

## 1. Containers: up / down

### Full topology (manager + 2 agents)

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
```

### Single container (isolated role test — black-00 only)

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build black-00
```

### Tear down

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
```

### Wait for SSH to be ready

Run after `up` before any Ansible commands:

```bash
ansible all -m wait_for_connection
```

---

## 2. Full deployment — all hosts

Deploys manager first, then all agents. One command for the complete blackwall-eye stack.

```bash
ansible-playbook ansible/playbooks/all.yml
```

---

## 3. Run a playbook on a specific host

```bash
# Manager only (black-00)
ansible-playbook ansible/playbooks/security-node.yml --limit black-00

# One agent only (e.g. black-01)
ansible-playbook ansible/playbooks/monitored-host.yml --limit black-01
```

---

## 4. Run a single role

Uses `--tags` to apply only one role. Combine with `--limit` to target one host.

```bash
# wazuh-agent role — all agent hosts
ansible-playbook ansible/playbooks/monitored-host.yml --tags wazuh-agent

# wazuh-agent role — single host
ansible-playbook ansible/playbooks/monitored-host.yml --tags wazuh-agent --limit black-01

# wazuh-manager role — manager
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-manager

# auditd role — all agent hosts
ansible-playbook ansible/playbooks/monitored-host.yml --tags auditd

# auditd role — single host
ansible-playbook ansible/playbooks/monitored-host.yml --tags auditd --limit black-01
```

**Single-container test** (only `black-00` is running, test agent role on it):

```bash
ansible-playbook -i tests/inventory/hosts.single.yml ansible/playbooks/monitored-host.yml
```

---

## 5. Verify deployments

```bash
# Agent registration
ansible-playbook ansible/playbooks/verify-agents.yml

# auditd rules and config (note: service check skipped in Docker)
ansible-playbook ansible/playbooks/verify-auditd.yml
```

**verify-agents.yml successful output:**
- `TASK [Show agent list]` — prints connected agents table
- `TASK [Assert at least one agent is connected (Active status)]` — `ok: [black-00]` with success message

**verify-agents.yml failure output:**
- `TASK [Assert...]` — `FAILED!` with message: `No external agents are in 'Active' state. Make sure wazuh-agent is installed and running...`

**verify-auditd.yml successful output:**
- Package, rules file, rule keys, log_format — all `ok`
- `TASK [Assert auditd is active]` — skipped in Docker (expected), asserts on real hosts

---

## 6. Check service status

> No `-i` needed — `ansible.cfg` sets the default inventory.

### Wazuh Manager

```bash
# Service status
ansible wazuh_managers -m command -a "systemctl status wazuh-manager" --become

# List registered agents
ansible wazuh_managers -m command -a "/var/ossec/bin/agent_control -l" --become
```

### Wazuh Agent

```bash
# Service status — all agents
ansible wazuh_agents -m command -a "systemctl status wazuh-agent" --become

# Service status — single agent
ansible black-01 -m command -a "systemctl status wazuh-agent" --become

# Check agent log (last 20 lines)
ansible black-01 -m command -a "tail -20 /var/ossec/logs/ossec.log" --become
```

### Version info

```bash
ansible all -m command -a "/var/ossec/bin/wazuh-control info" --become
```

---

## SSH Access

```bash
ssh -F tests/docker/ssh_config black-00
```

Or append to your personal SSH config:

```bash
cat tests/docker/ssh_config >> ~/.ssh/config
```

---

## Multi-Distro Support

The compose file uses [Compose profiles](https://docs.docker.com/compose/profiles/) to select the target distro.

| Profile | Base image | systemd | Ports | Status |
|---|---|---|---|---|
| `ubuntu2204` | geerlingguy/docker-ubuntu2204-ansible | yes | 2200–2202 | Active |
| `debian12` | (planned) | yes | 2210–2212 | Not implemented |
| `centos9` | (planned) | yes | 2220–2222 | Not implemented |
| `alpine` | (planned) | **no** — OpenRC | 2230+ | Not implemented |

> **Alpine note:** Alpine uses OpenRC instead of systemd. Roles that require systemd services (Wazuh, auditd) **cannot** be tested on Alpine.

### Adding a new distro

1. Create `tests/docker/images/<distro>/Dockerfile`
2. Uncomment the corresponding service block in `tests/docker/docker-compose.yml`
3. Add host entries to `tests/inventory/hosts.yml`

---

## Host Topology

| Container | Role | SSH port | Static IP |
|---|---|---|---|
| `black-00` | Wazuh Manager (security-node) | 2200 | 172.28.0.10 |
| `black-01` | Wazuh Agent (monitored-host) | 2201 | 172.28.0.11 |
| `black-02` | Wazuh Agent (monitored-host) | 2202 | 172.28.0.12 |

---

## File Layout

```
blackwall-eye/
├── ansible.cfg                             # ansible config (roles path, default inventory)
├── ansible/
│   ├── playbooks/
│   │   ├── all.yml                        # full stack: manager + agents
│   │   ├── security-node.yml               # wazuh-manager role
│   │   ├── monitored-host.yml              # wazuh-agent + auditd roles
│   │   ├── verify-agents.yml               # registration test
│   │   └── verify-auditd.yml               # auditd rules and config test
│   └── roles/
│       ├── wazuh-manager/
│       ├── wazuh-agent/
│       └── auditd/
└── tests/
    ├── docker/
    │   ├── docker-compose.yml
    │   ├── images/ubuntu2204/Dockerfile
    │   └── ssh_config
    └── inventory/
        ├── hosts.yml                       # full topology (manager + 2 agents)
        └── hosts.single.yml                # single-host for isolated role tests
```

## Known Limitations

**auditd:** The kernel audit subsystem (netlink socket) is not accessible inside Docker containers — it belongs to the host kernel exclusively. The auditd role installs the package and deploys rules correctly, but the service will not start in test containers. The role detects `/.dockerenv` and skips the failure — expect `ignore_errors` warnings on the systemd task. On real hosts auditd starts normally.

---

## Security Note

The containers use `root` with password `blackwall` and `PermitRootLogin yes`.
This is intentional for local test environments. Do not expose these containers to any network outside `localhost`.
