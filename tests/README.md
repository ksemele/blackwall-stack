# Blackwall — Test Environment

Two-container Docker environment for all Blackwall modules:
- `black-master-00` — Wazuh manager + indexer + dashboard (all-in-one)
- `black-host-01`   — Wazuh agent + auditd target

Containers are privileged with systemd as PID 1, so service management
works identically to production hosts.

> **Docker commands:** run from **repo root** (`blackwall/`)
> **Ansible commands:** run from the **module directory** (`blackwall-eye/`, etc.)
> Each module has `ansible.cfg` pointing at `../tests/inventory/`.

---

## 1. Quick start

```bash
# Up
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
cd blackwall-eye
ansible all -m wait_for_connection -T 60
ansible-playbook ansible/playbooks/all.yml

# Dashboard
open https://localhost:5601                # admin / Blackwall1!

# Down
cd ..
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
```

`all.yml` runs three sub-playbooks: certificates → security-node (indexer + manager + dashboard) → monitored-host (agent + auditd).

---

## 2. Connect to the Wazuh dashboard

| | |
|---|---|
| URL | `https://localhost:5601` |
| Username | `admin` |
| Password | `Blackwall1!` |
| Cert | self-signed; click through the browser warning |

To change credentials, edit `blackwall-eye/ansible/group_vars/wazuh_managers.yml` and re-run `ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-indexer,wazuh-dashboard`.

---

## 3. Point agents at a different master

The `wazuh-agent` role reads endpoint and enrollment params from variables;
the test-env defaults (in `wazuh-agent/defaults/main.yml`) point at the local
master `172.28.0.10`. Override per run:

```bash
ansible-playbook ansible/playbooks/monitored-host.yml \
  -e wazuh_manager_host=wazuh.prod.example.com \
  -e wazuh_enrollment_password=<secret>
```

For durable multi-host overrides, write your own inventory file:

```yaml
# inventory/prod.yml
all:
  vars:
    ansible_user: deploy
    ansible_ssh_private_key_file: ~/.ssh/id_ed25519
  children:
    wazuh_agents:
      vars:
        wazuh_manager_host: wazuh.prod.example.com
        wazuh_enrollment_password: "{{ lookup('community.general.passwordstore', 'wazuh/enrollment') }}"
      hosts:
        web-01.prod.example.com:
        web-02.prod.example.com:
```

```bash
ansible-playbook -i inventory/prod.yml ansible/playbooks/monitored-host.yml
```

Available variables (full list in `roles/wazuh-agent/defaults/main.yml`):

| Variable | Default | Purpose |
|---|---|---|
| `wazuh_manager_host` | `172.28.0.10` | Manager IP or hostname |
| `wazuh_manager_port` | `1514` | Manager listener port (TCP) |
| `wazuh_manager_protocol` | `tcp` | `tcp` or `udp` |
| `wazuh_enrollment_host` | `wazuh_manager_host` | Enrollment server (usually same) |
| `wazuh_enrollment_port` | `1515` | authd port |
| `wazuh_enrollment_password` | `""` | Shared secret; empty = no password |
| `wazuh_agent_name` | `inventory_hostname` | Agent name registered with manager |
| `wazuh_agent_groups` | `default` | Comma-separated Wazuh agent groups |

---

## 4. Run a playbook on a specific host

```bash
# Manager only
ansible-playbook ansible/playbooks/security-node.yml --limit black-master-00

# Agent only
ansible-playbook ansible/playbooks/monitored-host.yml --limit black-host-01
```

---

## 5. Run a single role

```bash
# wazuh-indexer
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-indexer

# wazuh-manager
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-manager

# wazuh-dashboard
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-dashboard

# wazuh-agent on the lone host
ansible-playbook ansible/playbooks/monitored-host.yml --tags wazuh-agent

# auditd on the lone host
ansible-playbook ansible/playbooks/monitored-host.yml --tags auditd
```

---

## 6. Verify (high-level)

```bash
# Agent registration
ansible-playbook ansible/playbooks/verify-agents.yml

# Live agent status via REST API
ansible-playbook ansible/playbooks/verify-api.yml

# auditd rules and config (service check skipped in Docker)
ansible-playbook ansible/playbooks/verify-auditd.yml
```

---

## 6a. Verify (per-component cookbook)

Run any subset of these to debug a specific piece. All `ansible` commands run from `blackwall-eye/`; `docker` and `curl` from the repo root.

### Containers
```bash
docker ps --format '{{.Names}}' | sort
# Expect:
#   black-host-01
#   black-master-00

docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 config --quiet
# Expect: exit 0, no output
```

### SSH / Ansible reachability
```bash
ssh -F tests/docker/ssh_config black-master-00 hostname    # → black-master-00
ssh -F tests/docker/ssh_config black-host-01 hostname      # → black-host-01

ansible all -m wait_for_connection -T 30
# Expect: SUCCESS for both hosts
```

### Wazuh indexer
```bash
ansible black-master-00 -m command -a "systemctl is-active wazuh-indexer" --become
# Expect: active

ansible black-master-00 -m shell -a \
  "curl -sk -u admin:Blackwall1! https://172.28.0.10:9200/_cluster/health" --become
# Expect: JSON with "status":"green" or "yellow"

ansible black-master-00 -m shell -a \
  "curl -sk -u admin:admin https://172.28.0.10:9200/_cluster/health" --become
# Expect: Unauthorized / 401
```

### Wazuh manager + filebeat
```bash
ansible black-master-00 -m command -a "systemctl is-active wazuh-manager" --become
# Expect: active

ansible black-master-00 -m command -a "systemctl is-active filebeat" --become
# Expect: active

ansible black-master-00 -m command -a "filebeat test output" --become
# Expect: lines ending in "OK" for connection / TLS / talk to server
```

### Wazuh dashboard
```bash
ansible black-master-00 -m command -a "systemctl is-active wazuh-dashboard" --become
# Expect: active

ansible black-master-00 -m shell -a \
  "curl -sk https://127.0.0.1/ -o /dev/null -w '%{http_code}'" --become
# Expect: 302

curl -sk https://localhost:5601/ -o /dev/null -w '%{http_code}\n'
# Expect: 302
```

### Wazuh agent
```bash
ansible black-host-01 -m command -a "systemctl status wazuh-agent" --become

ansible black-host-01 -m command -a "tail -20 /var/ossec/logs/ossec.log" --become

ansible wazuh_managers -m command -a "/var/ossec/bin/agent_control -l" --become
# Expect: at least one Active entry for black-host-01
```

### Agent endpoint override (point at a different master)
```bash
ansible-playbook ansible/playbooks/monitored-host.yml \
  -e wazuh_manager_host=192.0.2.1 \
  -e wazuh_enrollment_password=hunter2 \
  --tags wazuh-agent

ansible black-host-01 -m shell -a \
  "grep -E '<address>|<authorization_pass_path>' /var/ossec/etc/ossec.conf" --become
# Expect: <address>192.0.2.1</address> and <authorization_pass_path>... lines

ansible black-host-01 -m shell -a "cat /var/ossec/etc/authd.pass" --become
# Expect: hunter2

# Revert
ansible-playbook ansible/playbooks/monitored-host.yml --tags wazuh-agent
ansible black-host-01 -m stat -a "path=/var/ossec/etc/authd.pass" --become | grep '"exists":'
# Expect: "exists": false
```

### Version + build
```bash
ansible all -m command -a "/var/ossec/bin/wazuh-control info" --become 2>/dev/null || true

ansible black-master-00 -m command -a "dpkg -l wazuh-manager wazuh-indexer wazuh-dashboard filebeat" --become
# Expect: 'ii' status for all four
```

### Full clean-room acceptance
```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
rm -rf tests/.wazuh-certs/wazuh-certificates*

docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
cd blackwall-eye
ansible all -m wait_for_connection -T 60
ansible-playbook ansible/playbooks/all.yml

ansible-playbook ansible/playbooks/verify-agents.yml

open https://localhost:5601    # admin / Blackwall1!
```

---

## 6b. Trigger synthetic alerts (`bw-trigger`)

Drive Wazuh detections on `black-host-01` to verify the stack is wired end-to-end and to see real alerts in the dashboard.

```bash
tests/triggers/bw-trigger list           # show available events
tests/triggers/bw-trigger user           # fire one
tests/triggers/bw-trigger all            # fire everything (~60s)
tests/triggers/bw-trigger clean          # undo persistent side effects
```

Events range from harmless (`useradd`/`userdel`, FIM touches) to attacker-flavoured (web-shell drop, sudoers tampering, log clearing). Full event list and Wazuh-rule expectations: [tests/triggers/README.md](triggers/README.md).

Live tail the manager while triggering:

```bash
ansible black-master-00 -m shell -a 'tail -f /var/ossec/logs/alerts/alerts.json' --become
```

---

## 6c. Broken agent fixtures (`broken-agents.yml`)

Two static fixtures test the TUI's per-status row colouring:

| Agent | Status | Mechanism |
|---|---|---|
| `black-ghost-99` | `never_connected` | Registered on manager; no container ever created |
| `black-dead-98`  | `disconnected`    | Container starts, agent registers, then container is stopped |

**Prerequisites:** bring the env up and provision all hosts including `black-dead-98` first:

```bash
# 1. Start containers
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build

# 2. Provision everything (manager + both agents)
cd blackwall-eye
ansible-playbook -i ../tests/inventory/hosts.yml ansible/playbooks/all.yml
ansible-playbook -i ../tests/inventory/hosts.yml ansible/playbooks/monitored-host.yml --limit black-dead-98
cd ..

# 3. Register ghost + stop dead
ansible-playbook -i tests/inventory/hosts.yml tests/triggers/broken-agents.yml
```

**Verify** on the manager:

```bash
ansible wazuh_managers -i tests/inventory/hosts.yml \
  -m command -a "/var/ossec/bin/agent_control -l" --become
# Expect:
#   black-host-01   Active
#   black-ghost-99  Never connected
#   black-dead-98   Disconnected
```

The TUI renders each status with a distinct colour:

| Status | Colour |
|---|---|
| `active` | default |
| `pending` | yellow |
| `disconnected` | red |
| `never_connected` | dim grey |

---

## 7. Service status (ad-hoc)

```bash
ansible wazuh_managers -m command -a "systemctl status wazuh-manager" --become
ansible wazuh_managers -m command -a "systemctl status wazuh-indexer" --become
ansible wazuh_managers -m command -a "systemctl status wazuh-dashboard" --become
ansible wazuh_managers -m command -a "systemctl status filebeat" --become
ansible wazuh_agents -m command -a "systemctl status wazuh-agent" --become
```

---

## 8. SSH access

```bash
ssh -F tests/docker/ssh_config black-master-00
ssh -F tests/docker/ssh_config black-host-01
```

Or append to your personal SSH config:

```bash
cat tests/docker/ssh_config >> ~/.ssh/config
```

---

## 9. Host topology

| Container | Role | SSH port | Dashboard port | Static IP |
|---|---|---|---|---|
| `black-master-00` | manager + indexer + dashboard | `2200→22` | `5601→443` | `172.28.0.10` |
| `black-host-01`   | agent + auditd target | `2201→22` | — | `172.28.0.11` |
| `black-dead-98`   | agent fixture — disconnected | `2202→22` | — | `172.28.0.98` |
| `black-ghost-99`  | ghost fixture — never_connected | no container | — | `172.28.0.99` |

`black-ghost-99` is a manager-side registration only (no container). It is registered via the `broken-agents.yml` playbook and will always appear as `never_connected` in Wazuh.

`black-dead-98` is a real container that registers as a Wazuh agent; the `broken-agents.yml` playbook then stops the container so it transitions to `disconnected`.

---

## 10. Certificates

`tests/.wazuh-certs/wazuh-certificates.tar` is generated by
`wazuh-certificates.yml` and consumed by indexer, manager (filebeat),
and dashboard roles. It's per-environment and gitignored.

To rotate certs:

```bash
rm -rf tests/.wazuh-certs/wazuh-certificates*
ansible-playbook ansible/playbooks/wazuh-certificates.yml
# then re-run security-node.yml
```

---

## 11. Multi-distro support

The compose file uses [Compose profiles](https://docs.docker.com/compose/profiles/) to select a distro.

| Profile | Base image | systemd | SSH ports | Dashboard | Status |
|---|---|---|---|---|---|
| `ubuntu2204` | geerlingguy/docker-ubuntu2204-ansible | yes | 2200, 2201 | 5601 | Active |
| `debian12` | (planned) | yes | 2210, 2211 | 5611 | Not implemented |
| `centos9` | (planned) | yes | 2220, 2221 | 5621 | Not implemented |
| `alpine` | n/a — OpenRC | **no** | n/a | n/a | Not supported |

To add a new distro:

1. Create `tests/docker/images/<distro>/Dockerfile`.
2. Uncomment the corresponding service block in `tests/docker/docker-compose.yml`.
3. Add host entries to `tests/inventory/hosts.yml`.

---

## 12. Controller prerequisites

`password_hash('bcrypt')` filter (used by `wazuh-indexer` role) needs `passlib` on the Ansible controller. For pipx-installed Ansible:

```bash
pipx inject ansible 'passlib' 'bcrypt<5.0.0'
```

(bcrypt 5.0 has a known incompatibility with passlib 1.7.4; pin until upstream fixes.)

---

## 13. Known limitations

**auditd:** kernel audit netlink is not accessible inside Docker. The role installs rules correctly, but the service won't start in containers (detected via `/.dockerenv`). On real hosts auditd starts normally.

**Dashboard cert:** self-signed; browsers show "Not Secure". Acceptable for `localhost` only. Never expose port 5601 beyond `localhost`.

**Indexer/dashboard re-template:** Because bcrypt salts are random, the `internal_users.yml` template renders a new hash every run, so the indexer security config is re-pushed on every `security-node.yml` execution. Functionally idempotent (same password verifies) but not byte-idempotent. Acceptable.

---

## 14. File layout

```
blackwall/                                  # repo root
├── tests/
│   ├── docker/
│   │   ├── docker-compose.yml
│   │   ├── images/ubuntu2204/Dockerfile
│   │   └── ssh_config
│   ├── inventory/
│   │   └── hosts.yml
│   ├── triggers/
│   │   ├── bw-trigger                      # synthetic alert events
│   │   └── broken-agents.yml               # registers ghost + stops dead
│   └── .wazuh-certs/                       # gitignored TLS bundle
└── blackwall-eye/
    ├── ansible.cfg
    └── ansible/
        ├── group_vars/wazuh_managers.yml
        ├── playbooks/
        │   ├── all.yml
        │   ├── wazuh-certificates.yml
        │   ├── security-node.yml           # indexer + manager + dashboard
        │   ├── monitored-host.yml          # agent + auditd
        │   ├── verify-agents.yml
        │   ├── verify-api.yml
        │   └── verify-auditd.yml
        └── roles/
            ├── wazuh-indexer/
            ├── wazuh-manager/              # includes filebeat output
            ├── wazuh-dashboard/
            ├── wazuh-agent/
            └── auditd/
```

---

## Security note

Containers run `root:blackwall` over SSH with `PermitRootLogin yes`. Indexer/dashboard credentials default to `admin / Blackwall1!`. **This is intentional for the local test environment. Do not expose any of these containers or ports beyond `localhost`.**
