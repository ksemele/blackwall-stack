# Test Environment: master + host with Wazuh dashboard — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace 3-container test env with a focused 2-container topology (`black-master-00` + `black-host-01`), deploy Wazuh manager + indexer + dashboard all-in-one on the master via Ansible roles, parameterize agent endpoint/secrets so agents can be pointed at any master, and expose the dashboard on `https://localhost:5601`.

**Architecture:** Two Docker containers using existing `ubuntu2204` compose profile. Three Ansible roles deploy Wazuh: `wazuh-indexer` (new), `wazuh-manager` (existing, gets filebeat output added), `wazuh-dashboard` (new). One-shot `wazuh-certificates.yml` playbook generates the TLS bundle. Agent role gets flat overrideable vars for endpoint/enrollment.

**Tech Stack:** Docker Compose, Ansible 2.x, Wazuh 4.14.5 (`wazuh-manager`, `wazuh-indexer`, `wazuh-dashboard` apt packages), Filebeat (Wazuh-shipped variant), OpenSearch security plugin.

---

## Reference

- Spec: `docs/superpowers/specs/2026-05-15-test-env-master-host-dashboard-design.md`
- Wazuh 4.x installation docs (canonical, version-pinned reference for templates):
  - Indexer: <https://documentation.wazuh.com/4.14/installation-guide/wazuh-indexer/index.html>
  - Dashboard: <https://documentation.wazuh.com/4.14/installation-guide/wazuh-dashboard/index.html>
  - Filebeat (manager → indexer): <https://documentation.wazuh.com/4.14/installation-guide/wazuh-server/step-by-step.html#installing-filebeat>
  - Certificates: <https://documentation.wazuh.com/4.14/installation-guide/wazuh-server/step-by-step.html#certificates-deployment>

When a task says "templates the upstream defaults", fetch the current upstream YAML via the URL above and substitute the variables listed in the task. Do not invent config keys.

## File Structure

### Created

| Path | Responsibility |
|---|---|
| `blackwall-eye/ansible/group_vars/wazuh_managers.yml` | Shared indexer + manager-API credentials for master roles |
| `blackwall-eye/ansible/playbooks/wazuh-certificates.yml` | One-shot: generate TLS bundle, fetch to controller |
| `blackwall-eye/ansible/roles/wazuh-indexer/defaults/main.yml` | Indexer role vars |
| `blackwall-eye/ansible/roles/wazuh-indexer/tasks/main.yml` | Indexer install, TLS, security-plugin push |
| `blackwall-eye/ansible/roles/wazuh-indexer/handlers/main.yml` | Indexer service restart |
| `blackwall-eye/ansible/roles/wazuh-indexer/templates/opensearch.yml.j2` | Indexer node config |
| `blackwall-eye/ansible/roles/wazuh-indexer/templates/internal_users.yml.j2` | Hashed credentials for security plugin |
| `blackwall-eye/ansible/roles/wazuh-dashboard/defaults/main.yml` | Dashboard role vars |
| `blackwall-eye/ansible/roles/wazuh-dashboard/tasks/main.yml` | Dashboard install, TLS, embedded Wazuh app config |
| `blackwall-eye/ansible/roles/wazuh-dashboard/handlers/main.yml` | Dashboard service restart |
| `blackwall-eye/ansible/roles/wazuh-dashboard/templates/opensearch_dashboards.yml.j2` | Dashboard process config |
| `blackwall-eye/ansible/roles/wazuh-dashboard/templates/wazuh.yml.j2` | Embedded Wazuh app config (points at manager API) |
| `blackwall-eye/ansible/roles/wazuh-manager/templates/filebeat.yml.j2` | Filebeat output to local indexer |
| `tests/.wazuh-certs/.gitkeep` | Marker so cert dir exists, contents gitignored |

### Modified

| Path | Change |
|---|---|
| `tests/docker/docker-compose.yml` | Replace black-00/01/02 with black-master-00 (ports 2200, 5601) + black-host-01 (port 2201); rename future-distro stubs |
| `tests/docker/ssh_config` | Rename aliases black-00/01/02 → black-master-00/black-host-01 |
| `tests/inventory/hosts.yml` | Reduce to two hosts with new names; drop hardcoded telegram secrets (replace with `!vault` placeholders or remove and leave commented note) |
| `blackwall-eye/ansible/roles/wazuh-agent/defaults/main.yml` | Add manager_port, manager_protocol, enrollment_host, enrollment_port, enrollment_password |
| `blackwall-eye/ansible/roles/wazuh-agent/templates/ossec.conf.j2` | Reference new vars; conditional `<authorization_pass_path>` |
| `blackwall-eye/ansible/roles/wazuh-agent/tasks/main.yml` | Add task to write `/var/ossec/etc/authd.pass` when password set |
| `blackwall-eye/ansible/roles/wazuh-manager/tasks/main.yml` | Add filebeat install + template + service start |
| `blackwall-eye/ansible/roles/wazuh-manager/handlers/main.yml` | Add Restart filebeat handler |
| `blackwall-eye/ansible/playbooks/all.yml` | Chain certificates + security-node + monitored-host |
| `blackwall-eye/ansible/playbooks/security-node.yml` | Apply 3 roles in order: indexer → manager → dashboard |
| `blackwall-eye/ansible/playbooks/verify-agents.yml` | Update fail_msg to mention black-host-01 |
| `blackwall-eye/ansible/playbooks/verify-api.yml` | No code change; mention new dashboard creds in usage comment |
| `tests/README.md` | Full rewrite per spec section "Documentation" |
| `README.md` | Add "Test environment" section |
| `.gitignore` (root) | Add `tests/.wazuh-certs/*` exception for `.gitkeep` |
| `blackwall-eye/STATUS.md` | Update last_task / next_task / last_updated |

### Deleted

| Path | Reason |
|---|---|
| `tests/inventory/hosts.single.yml` | Single-host scenario covered by `--limit black-master-00` |

## Testing strategy

Ansible roles are tested by running the playbook against the live Docker test env and asserting on observable state (services running, files present with correct content, HTTPS endpoints responding, agents registered). Existing `verify-*.yml` playbooks are the test harnesses; this plan adds inline `command` / `uri` checks within tasks where a verify playbook doesn't exist yet.

Each major task includes a "Test" step with the exact command and expected output. Tasks that touch templates also include a "Run idempotency check" — re-running the play should show `changed=0`.

---

## Phase 1 — Container topology

### Task 1: Rewrite docker-compose.yml for 2-container topology

**Files:**
- Modify: `tests/docker/docker-compose.yml` (full rewrite of services block)

- [ ] **Step 1: Replace the services block**

Open `tests/docker/docker-compose.yml`. Keep the file header comment, networks block, and `x-black-node` anchor unchanged. Replace the entire `services:` block (everything from line 42 to end of file) with:

```yaml
services:

  # ── Ubuntu 22.04 ───────────────────────────────────────────────────────────
  # Profile: ubuntu2204
  # Base:    geerlingguy/docker-ubuntu2204-ansible (systemd + Python pre-installed)
  # Ports:
  #   black-master-00:  2200 (SSH), 5601 (Wazuh dashboard, container :443)
  #   black-host-01:    2201 (SSH)

  black-master-00:
    <<: *black-node
    profiles: [ubuntu2204]
    build:
      context: ./images/ubuntu2204
    container_name: black-master-00
    hostname: black-master-00
    ports:
      - "2200:22"
      - "5601:443"
    networks:
      blackwall-test:
        ipv4_address: 172.28.0.10

  black-host-01:
    <<: *black-node
    profiles: [ubuntu2204]
    build:
      context: ./images/ubuntu2204
    container_name: black-host-01
    hostname: black-host-01
    ports:
      - "2201:22"
    networks:
      blackwall-test:
        ipv4_address: 172.28.0.11

  # ── Debian 12 (future) ────────────────────────────────────────────────────
  # Profile: debian12 — uncomment when tests/docker/images/debian12/Dockerfile exists.
  # Ports: 2210 (SSH), 5611 (dashboard) for master; 2211 (SSH) for host.
  #
  # black-master-00-debian:
  #   <<: *black-node
  #   profiles: [debian12]
  #   build:
  #     context: ./images/debian12
  #   container_name: black-master-00-debian
  #   hostname: black-master-00-debian
  #   ports:
  #     - "2210:22"
  #     - "5611:443"
  #   networks:
  #     blackwall-test:
  #       ipv4_address: 172.28.0.20
  #
  # black-host-01-debian:
  #   <<: *black-node
  #   profiles: [debian12]
  #   build:
  #     context: ./images/debian12
  #   container_name: black-host-01-debian
  #   hostname: black-host-01-debian
  #   ports:
  #     - "2211:22"
  #   networks:
  #     blackwall-test:
  #       ipv4_address: 172.28.0.21

  # ── CentOS Stream 9 (future) ──────────────────────────────────────────────
  # Profile: centos9 — same shape as debian12, ports 2220/5621 (master), 2221 (host).
  # Add when tests/docker/images/centos9/Dockerfile exists.

  # ── Alpine: not supported (OpenRC, no systemd) ─────────────────────────────
```

- [ ] **Step 2: Validate compose file**

Run: `docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 config --quiet`
Expected: exits 0 with no output.

- [ ] **Step 3: Bring up to confirm containers start**

Run from repo root: `docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build`
Expected: two containers created (`black-master-00`, `black-host-01`), both reach status "Started".

Verify: `docker ps --format '{{.Names}}' | sort`
Expected output:
```
black-host-01
black-master-00
```

- [ ] **Step 4: Tear down**

Run: `docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v`
Expected: both containers + the `blackwall-test` network removed.

- [ ] **Step 5: Commit**

```bash
git add tests/docker/docker-compose.yml
git commit -m "test(env): rewrite compose for black-master-00 + black-host-01 topology"
```

---

### Task 2: Update ssh_config and inventory

**Files:**
- Modify: `tests/docker/ssh_config`
- Modify: `tests/inventory/hosts.yml`
- Delete: `tests/inventory/hosts.single.yml`

- [ ] **Step 1: Read current ssh_config**

Run: `cat tests/docker/ssh_config`

- [ ] **Step 2: Rewrite ssh_config**

Replace contents with:

```
# SSH aliases for the Blackwall test environment.
# Source from repo root: ssh -F tests/docker/ssh_config black-master-00
# Or append to ~/.ssh/config: cat tests/docker/ssh_config >> ~/.ssh/config

Host black-master-00
    HostName localhost
    Port 2200
    User root
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    LogLevel ERROR

Host black-host-01
    HostName localhost
    Port 2201
    User root
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    LogLevel ERROR
```

- [ ] **Step 3: Rewrite hosts.yml**

Replace contents of `tests/inventory/hosts.yml` with:

```yaml
all:
  vars:
    ansible_user: root
    ansible_password: blackwall
    ansible_ssh_common_args: >-
      -o StrictHostKeyChecking=no
      -o UserKnownHostsFile=/dev/null
    ansible_python_interpreter: /usr/bin/python3
    # NOTE: telegram_bot_token / telegram_chat_id previously here were real
    # secrets committed to git. Removed. blackwall-voice should source these
    # from ansible-vault or env vars — tracked as a separate follow-up.

  children:
    ubuntu2204:
      children:
        wazuh_managers:
          hosts:
            black-master-00:
              ansible_host: localhost
              ansible_port: 2200
        wazuh_agents:
          hosts:
            black-host-01:
              ansible_host: localhost
              ansible_port: 2201

    # Future distro groups — match port ranges in docker-compose.yml comments.
    # debian12: ...
    # centos9: ...
```

NOTE: `wazuh_agents:vars.wazuh_manager_host` is intentionally removed. The default lives in the role (`wazuh-agent/defaults/main.yml`, set in Task 11).

- [ ] **Step 4: Delete the single-host inventory file**

Run: `git rm tests/inventory/hosts.single.yml`
Expected: file removed and staged for deletion.

- [ ] **Step 5: Verify ssh works (sanity check, requires containers up)**

Run: `docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build`
Wait ~10 seconds for sshd to start.
Run: `ssh -F tests/docker/ssh_config black-master-00 hostname`

If the ssh command fails with "Permission denied" the most likely cause is the test image's root password being out of sync — check `tests/docker/images/ubuntu2204/Dockerfile`. Expected output:
```
black-master-00
```

Same for `black-host-01`:
```
ssh -F tests/docker/ssh_config black-host-01 hostname
black-host-01
```

- [ ] **Step 6: Verify ansible can reach both hosts**

From `blackwall-eye/`:
```bash
ansible all -m wait_for_connection -T 30
```
Expected: both hosts return `SUCCESS`.

- [ ] **Step 7: Commit**

```bash
git add tests/docker/ssh_config tests/inventory/hosts.yml
git rm tests/inventory/hosts.single.yml
git commit -m "test(env): rename hosts to black-master-00 / black-host-01, drop single-host inventory and committed telegram secrets"
```

---

## Phase 2 — Certificates

### Task 3: wazuh-certificates playbook

**Files:**
- Create: `blackwall-eye/ansible/playbooks/wazuh-certificates.yml`
- Modify: `.gitignore` (root)
- Create: `tests/.wazuh-certs/.gitkeep`

- [ ] **Step 1: Create the playbook**

Create `blackwall-eye/ansible/playbooks/wazuh-certificates.yml` with:

```yaml
---
# wazuh-certificates.yml — Generate TLS bundle for Wazuh indexer/dashboard/filebeat.
#
# Usage (from blackwall-eye/):
#   ansible-playbook ansible/playbooks/wazuh-certificates.yml
#
# Idempotent: skips generation if ../tests/.wazuh-certs/wazuh-certificates.tar exists
# on the controller. To rotate, delete that file and re-run.

- name: Generate Wazuh certificate bundle on the master, fetch to controller
  hosts: wazuh_managers
  become: true
  gather_facts: false
  run_once: true

  vars:
    wazuh_certs_local_dir: "{{ playbook_dir }}/../../../tests/.wazuh-certs"
    wazuh_certs_remote_dir: "/root/wazuh-certs"
    wazuh_certs_tool_url: "https://packages.wazuh.com/4.14/wazuh-certs-tool.sh"
    wazuh_certs_config_url: "https://packages.wazuh.com/4.14/config.yml"

  tasks:
    - name: Check if bundle already exists on controller
      stat:
        path: "{{ wazuh_certs_local_dir }}/wazuh-certificates.tar"
      register: _bundle_local
      delegate_to: localhost
      become: false

    - name: Stop here if bundle exists (idempotency)
      meta: end_play
      when: _bundle_local.stat.exists

    - name: Ensure remote work dir exists
      file:
        path: "{{ wazuh_certs_remote_dir }}"
        state: directory
        mode: "0700"

    - name: Download wazuh-certs-tool.sh
      get_url:
        url: "{{ wazuh_certs_tool_url }}"
        dest: "{{ wazuh_certs_remote_dir }}/wazuh-certs-tool.sh"
        mode: "0700"

    - name: Download config.yml template
      get_url:
        url: "{{ wazuh_certs_config_url }}"
        dest: "{{ wazuh_certs_remote_dir }}/config.yml"
        mode: "0600"

    - name: Customize config.yml for single-node all-in-one (master only)
      copy:
        dest: "{{ wazuh_certs_remote_dir }}/config.yml"
        mode: "0600"
        content: |
          nodes:
            indexer:
              - name: black-master-00
                ip: 172.28.0.10
            server:
              - name: black-master-00
                ip: 172.28.0.10
            dashboard:
              - name: black-master-00
                ip: 172.28.0.10

    - name: Generate certificate bundle
      command: bash ./wazuh-certs-tool.sh -A
      args:
        chdir: "{{ wazuh_certs_remote_dir }}"
        creates: "{{ wazuh_certs_remote_dir }}/wazuh-certificates"

    - name: Tar the bundle for transport
      command: tar -cf wazuh-certificates.tar wazuh-certificates
      args:
        chdir: "{{ wazuh_certs_remote_dir }}"
        creates: "{{ wazuh_certs_remote_dir }}/wazuh-certificates.tar"

    - name: Ensure local cert dir exists
      file:
        path: "{{ wazuh_certs_local_dir }}"
        state: directory
        mode: "0700"
      delegate_to: localhost
      become: false

    - name: Fetch bundle to controller
      fetch:
        src: "{{ wazuh_certs_remote_dir }}/wazuh-certificates.tar"
        dest: "{{ wazuh_certs_local_dir }}/wazuh-certificates.tar"
        flat: true
```

- [ ] **Step 2: Add cert dir to .gitignore**

Read `.gitignore` (or create at repo root if missing). Append:

```
# Wazuh TLS bundle (generated per environment, do not commit)
tests/.wazuh-certs/*
!tests/.wazuh-certs/.gitkeep
```

- [ ] **Step 3: Create the .gitkeep marker**

Run: `mkdir -p tests/.wazuh-certs && touch tests/.wazuh-certs/.gitkeep`

- [ ] **Step 4: Run the playbook (requires Task 1 containers up)**

From `blackwall-eye/`:
```bash
ansible-playbook ansible/playbooks/wazuh-certificates.yml
```
Expected: completes with `failed=0`; `tests/.wazuh-certs/wazuh-certificates.tar` exists on the controller (~10 KB).

Run: `ls -la tests/.wazuh-certs/`
Expected: shows `.gitkeep` and `wazuh-certificates.tar`.

- [ ] **Step 5: Idempotency check**

Run the playbook again. Expected: every task reports `ok`, no `changed`. The `meta: end_play` should fire after the first stat task.

- [ ] **Step 6: Commit**

```bash
git add blackwall-eye/ansible/playbooks/wazuh-certificates.yml \
        .gitignore tests/.wazuh-certs/.gitkeep
git commit -m "feat(eye): wazuh-certificates playbook — one-shot TLS bundle for all-in-one master"
```

---

## Phase 3 — Wazuh indexer role

### Task 4: wazuh-indexer role skeleton + defaults

**Files:**
- Create: `blackwall-eye/ansible/roles/wazuh-indexer/defaults/main.yml`
- Create: `blackwall-eye/ansible/roles/wazuh-indexer/handlers/main.yml`
- Create: `blackwall-eye/ansible/roles/wazuh-indexer/tasks/main.yml` (empty for now)

- [ ] **Step 1: Create defaults/main.yml**

```yaml
---
wazuh_version: "4.14.5"
wazuh_apt_key_url: "https://packages.wazuh.com/key/GPG-KEY-WAZUH"
wazuh_apt_repo: "deb https://packages.wazuh.com/4.x/apt/ stable main"

# Network
wazuh_indexer_node_name: "{{ inventory_hostname }}"
wazuh_indexer_network_host: "172.28.0.10"
wazuh_indexer_http_port: 9200

# Certificates (paths on the controller — fetched by wazuh-certificates.yml)
wazuh_certs_local_dir: "{{ playbook_dir }}/../../../tests/.wazuh-certs"

# Credentials (override in group_vars/wazuh_managers.yml)
wazuh_indexer_admin_user: "admin"
wazuh_indexer_admin_password: "Blackwall1!"
```

- [ ] **Step 2: Create handlers/main.yml**

```yaml
---
- name: Restart wazuh-indexer
  systemd:
    name: wazuh-indexer
    state: restarted
```

- [ ] **Step 3: Create empty tasks/main.yml**

```yaml
---
# Tasks populated incrementally in following tasks.
```

- [ ] **Step 4: Commit**

```bash
git add blackwall-eye/ansible/roles/wazuh-indexer/
git commit -m "feat(eye): wazuh-indexer role skeleton (defaults + handlers)"
```

---

### Task 5: wazuh-indexer install task

**Files:**
- Modify: `blackwall-eye/ansible/roles/wazuh-indexer/tasks/main.yml`

- [ ] **Step 1: Add install tasks**

Replace contents with:

```yaml
---
- name: Wait for dpkg lock to be released
  shell: while fuser /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock >/dev/null 2>&1; do sleep 2; done
  changed_when: false

- name: Install prerequisites (gnupg, curl, debconf, adduser)
  apt:
    name:
      - gnupg
      - curl
      - debconf
      - adduser
    state: present
    update_cache: true

- name: Add Wazuh GPG key
  apt_key:
    url: "{{ wazuh_apt_key_url }}"
    state: present

- name: Add Wazuh apt repository
  apt_repository:
    repo: "{{ wazuh_apt_repo }}"
    state: present
    filename: wazuh

- name: Install wazuh-indexer package
  apt:
    name: "wazuh-indexer={{ wazuh_version }}-*"
    state: present
    update_cache: true
```

- [ ] **Step 2: Wire role into a test playbook (temporary)**

Edit `blackwall-eye/ansible/playbooks/security-node.yml` to:

```yaml
---
- name: Configure security node (Wazuh manager + indexer + dashboard)
  hosts: wazuh_managers
  become: true
  roles:
    - role: wazuh-indexer
      tags: [wazuh-indexer]
    - role: wazuh-manager
      tags: [wazuh-manager]
```

- [ ] **Step 3: Run with --tags wazuh-indexer (requires fresh containers)**

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
cd blackwall-eye
ansible all -m wait_for_connection -T 60
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-indexer
```
Expected: `failed=0`. The package is installed but the service is not started yet (no opensearch.yml or certs).

Verify: `ansible black-master-00 -m command -a "dpkg -l wazuh-indexer" --become`
Expected: package status `ii` (installed) with version starting `4.14.5`.

- [ ] **Step 4: Commit**

```bash
git add blackwall-eye/ansible/roles/wazuh-indexer/tasks/main.yml \
        blackwall-eye/ansible/playbooks/security-node.yml
git commit -m "feat(eye): wazuh-indexer install task + wire into security-node playbook"
```

---

### Task 6: wazuh-indexer opensearch.yml template + certificates

**Files:**
- Create: `blackwall-eye/ansible/roles/wazuh-indexer/templates/opensearch.yml.j2`
- Modify: `blackwall-eye/ansible/roles/wazuh-indexer/tasks/main.yml`

- [ ] **Step 1: Create opensearch.yml.j2**

This template should mirror the upstream Wazuh 4.14 single-node config. Verify the current upstream version at the Indexer URL in the Reference section before committing. Initial content:

```yaml
network.host: "{{ wazuh_indexer_network_host }}"
node.name: "{{ wazuh_indexer_node_name }}"
cluster.name: "wazuh-cluster"
cluster.initial_master_nodes:
  - "{{ wazuh_indexer_node_name }}"
node.max_local_storage_nodes: "3"
path.data: /var/lib/wazuh-indexer
path.logs: /var/log/wazuh-indexer

bootstrap.memory_lock: true

plugins.security.ssl.http.pemcert_filepath: certs/{{ wazuh_indexer_node_name }}.pem
plugins.security.ssl.http.pemkey_filepath: certs/{{ wazuh_indexer_node_name }}-key.pem
plugins.security.ssl.http.pemtrustedcas_filepath: certs/root-ca.pem
plugins.security.ssl.http.enabled: true
plugins.security.ssl.transport.pemcert_filepath: certs/{{ wazuh_indexer_node_name }}.pem
plugins.security.ssl.transport.pemkey_filepath: certs/{{ wazuh_indexer_node_name }}-key.pem
plugins.security.ssl.transport.pemtrustedcas_filepath: certs/root-ca.pem
plugins.security.ssl.transport.resolve_hostname: false
plugins.security.ssl.transport.enforce_hostname_verification: false
plugins.security.authcz.admin_dn:
  - "CN=admin,OU=Wazuh,O=Wazuh,L=California,C=US"
plugins.security.check_snapshot_restore_write_privileges: true
plugins.security.enable_snapshot_restore_privilege: true
plugins.security.nodes_dn:
  - "CN={{ wazuh_indexer_node_name }},OU=Wazuh,O=Wazuh,L=California,C=US"
plugins.security.restapi.roles_enabled:
  - "all_access"
  - "security_rest_api_access"
plugins.security.system_indices.enabled: true
plugins.security.system_indices.indices:
  - ".opendistro-alerting-config"
  - ".opendistro-alerting-alert*"
  - ".opendistro-anomaly-results*"
  - ".opendistro-anomaly-detector*"
  - ".opendistro-anomaly-checkpoints"
  - ".opendistro-anomaly-detection-state"
  - ".opendistro-reports-*"
  - ".opendistro-notifications-*"
  - ".opendistro-notebooks"
  - ".opensearch-observability"
  - ".opendistro-asynchronous-search-response*"
  - ".replication-metadata-store"
```

- [ ] **Step 2: Add cert + template tasks**

Append to `tasks/main.yml`:

```yaml
- name: Unpack cert bundle on master
  unarchive:
    src: "{{ wazuh_certs_local_dir }}/wazuh-certificates.tar"
    dest: /tmp/
    remote_src: false
  changed_when: false

- name: Ensure /etc/wazuh-indexer/certs directory exists
  file:
    path: /etc/wazuh-indexer/certs
    state: directory
    owner: wazuh-indexer
    group: wazuh-indexer
    mode: "0500"

- name: Copy indexer node certificate
  copy:
    src: "/tmp/wazuh-certificates/{{ item.src }}"
    dest: "/etc/wazuh-indexer/certs/{{ item.dest }}"
    owner: wazuh-indexer
    group: wazuh-indexer
    mode: "0400"
    remote_src: true
  loop:
    - { src: "root-ca.pem",                              dest: "root-ca.pem" }
    - { src: "{{ wazuh_indexer_node_name }}.pem",        dest: "{{ wazuh_indexer_node_name }}.pem" }
    - { src: "{{ wazuh_indexer_node_name }}-key.pem",    dest: "{{ wazuh_indexer_node_name }}-key.pem" }
    - { src: "admin.pem",                                dest: "admin.pem" }
    - { src: "admin-key.pem",                            dest: "admin-key.pem" }
  notify: Restart wazuh-indexer

- name: Deploy opensearch.yml
  template:
    src: opensearch.yml.j2
    dest: /etc/wazuh-indexer/opensearch.yml
    owner: wazuh-indexer
    group: wazuh-indexer
    mode: "0640"
  notify: Restart wazuh-indexer

- name: Enable and start wazuh-indexer service
  systemd:
    name: wazuh-indexer
    enabled: true
    state: started
    daemon_reload: true

- name: Wait for indexer HTTPS port (9200) to accept connections
  wait_for:
    host: 127.0.0.1
    port: 9200
    timeout: 120
```

- [ ] **Step 3: Run the role**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-indexer
```
Expected: `failed=0`. Indexer service active; port 9200 listening.

Verify:
```bash
ansible black-master-00 -m command -a "systemctl is-active wazuh-indexer" --become
```
Expected: `active`.

```bash
ansible black-master-00 -m shell -a "curl -sk -u admin:admin https://127.0.0.1:9200/_cluster/health" --become
```
Expected: JSON output. Note: at this point the security plugin still uses *default* `admin:admin` credentials — they are pushed to `Blackwall1!` in Task 7.

- [ ] **Step 4: Idempotency check**

Re-run the same playbook. Expected: `changed=0`.

- [ ] **Step 5: Commit**

```bash
git add blackwall-eye/ansible/roles/wazuh-indexer/
git commit -m "feat(eye): wazuh-indexer config + cert deployment + service start"
```

---

### Task 7: wazuh-indexer internal_users + securityadmin push

**Files:**
- Create: `blackwall-eye/ansible/roles/wazuh-indexer/templates/internal_users.yml.j2`
- Create: `blackwall-eye/ansible/group_vars/wazuh_managers.yml`
- Modify: `blackwall-eye/ansible/roles/wazuh-indexer/tasks/main.yml`

- [ ] **Step 1: Create group_vars file**

Create `blackwall-eye/ansible/group_vars/wazuh_managers.yml`:

```yaml
---
# Shared credentials for the all-in-one master.
# Indexer / dashboard / filebeat (one OpenSearch admin account):
wazuh_indexer_admin_user: "admin"
wazuh_indexer_admin_password: "Blackwall1!"

# Manager REST API (separate user system, on the manager process):
wazuh_manager_api_user: "wazuh"
wazuh_manager_api_password: "wazuh"
```

NOTE: these are test-env defaults. For real deployments override in
a vaulted file under group_vars/ matching your inventory groups.

- [ ] **Step 2: Create internal_users.yml.j2**

```yaml
---
# Wazuh / OpenSearch security internal users config.
# Passwords are bcrypt hashes (templated at deploy time).
_meta:
  type: "internalusers"
  config_version: 2

{{ wazuh_indexer_admin_user }}:
  hash: "{{ wazuh_indexer_admin_password | password_hash('bcrypt') }}"
  reserved: true
  backend_roles:
    - "admin"
  description: "Wazuh admin user"

kibanaserver:
  hash: "{{ wazuh_indexer_admin_password | password_hash('bcrypt') }}"
  reserved: true
  description: "Dashboard service user (uses same secret as admin in test env)"
```

NOTE: Production deployments split these into separate accounts. The test env uses a single password for both to keep verification simple.

- [ ] **Step 3: Append securityadmin push tasks**

Append to `tasks/main.yml`:

```yaml
- name: Deploy internal_users.yml
  template:
    src: internal_users.yml.j2
    dest: /etc/wazuh-indexer/opensearch-security/internal_users.yml
    owner: wazuh-indexer
    group: wazuh-indexer
    mode: "0640"
  register: _internal_users

- name: Push security config via securityadmin.sh
  command: >
    bash /usr/share/wazuh-indexer/plugins/opensearch-security/tools/securityadmin.sh
    -cd /etc/wazuh-indexer/opensearch-security/
    -nhnv
    -cacert /etc/wazuh-indexer/certs/root-ca.pem
    -cert /etc/wazuh-indexer/certs/admin.pem
    -key /etc/wazuh-indexer/certs/admin-key.pem
    -p 9200
    -icl
  environment:
    JAVA_HOME: /usr/share/wazuh-indexer/jdk
  when: _internal_users.changed
  changed_when: true
```

- [ ] **Step 4: Run the role**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-indexer
```
Expected: `internal_users.yml` task `changed`; `securityadmin.sh` task runs and reports "Done with success".

Verify with new password:
```bash
ansible black-master-00 -m shell -a "curl -sk -u admin:Blackwall1! https://127.0.0.1:9200/_cluster/health" --become
```
Expected: JSON output with `"status":"green"` or `"yellow"`.

Verify old password no longer works:
```bash
ansible black-master-00 -m shell -a "curl -sk -u admin:admin https://127.0.0.1:9200/_cluster/health" --become
```
Expected: `Unauthorized` / 401.

- [ ] **Step 5: Idempotency check**

Re-run. Expected: `changed=0` (the `when: _internal_users.changed` guard prevents re-pushing).

- [ ] **Step 6: Commit**

```bash
git add blackwall-eye/ansible/roles/wazuh-indexer/templates/internal_users.yml.j2 \
        blackwall-eye/ansible/roles/wazuh-indexer/tasks/main.yml \
        blackwall-eye/ansible/group_vars/wazuh_managers.yml
git commit -m "feat(eye): wazuh-indexer internal_users + securityadmin push"
```

---

## Phase 4 — Manager filebeat output

### Task 8: Wazuh manager → indexer via filebeat

**Files:**
- Create: `blackwall-eye/ansible/roles/wazuh-manager/templates/filebeat.yml.j2`
- Modify: `blackwall-eye/ansible/roles/wazuh-manager/tasks/main.yml`
- Modify: `blackwall-eye/ansible/roles/wazuh-manager/handlers/main.yml`

- [ ] **Step 1: Create filebeat.yml.j2**

```yaml
# Filebeat config — manager ships alerts to the local Wazuh indexer.
filebeat.modules:
  - module: wazuh
    alerts:
      enabled: true
    archives:
      enabled: false

setup.template.json.enabled: true
setup.template.json.path: '/etc/filebeat/wazuh-template.json'
setup.template.json.name: 'wazuh'
setup.template.overwrite: true
setup.ilm.enabled: false

output.elasticsearch:
  hosts: ["https://172.28.0.10:9200"]
  protocol: https
  username: "{{ wazuh_indexer_admin_user }}"
  password: "{{ wazuh_indexer_admin_password }}"
  ssl.certificate_authorities:
    - /etc/filebeat/certs/root-ca.pem
  ssl.certificate: "/etc/filebeat/certs/black-master-00.pem"
  ssl.key: "/etc/filebeat/certs/black-master-00-key.pem"
```

- [ ] **Step 2: Add filebeat install + cert + template tasks to wazuh-manager**

Append to `blackwall-eye/ansible/roles/wazuh-manager/tasks/main.yml`:

```yaml
- name: Install filebeat (Wazuh-shipped variant)
  apt:
    name: "filebeat={{ wazuh_filebeat_version | default('7.10.2') }}"
    state: present
    update_cache: true

- name: Download Wazuh filebeat module for v4.x
  get_url:
    url: "https://packages.wazuh.com/4.x/filebeat/wazuh-filebeat-0.4.tar.gz"
    dest: /tmp/wazuh-filebeat-0.4.tar.gz
    mode: "0644"

- name: Unpack Wazuh filebeat module
  unarchive:
    src: /tmp/wazuh-filebeat-0.4.tar.gz
    dest: /usr/share/filebeat/module/
    remote_src: true
    creates: /usr/share/filebeat/module/wazuh

- name: Download Wazuh filebeat template
  get_url:
    url: "https://raw.githubusercontent.com/wazuh/wazuh/v{{ wazuh_version }}/extensions/elasticsearch/7.x/wazuh-template.json"
    dest: /etc/filebeat/wazuh-template.json
    mode: "0644"

- name: Ensure /etc/filebeat/certs dir exists
  file:
    path: /etc/filebeat/certs
    state: directory
    owner: root
    group: root
    mode: "0500"

- name: Copy filebeat TLS certs (reuse master node bundle)
  copy:
    src: "/tmp/wazuh-certificates/{{ item.src }}"
    dest: "/etc/filebeat/certs/{{ item.dest }}"
    owner: root
    group: root
    mode: "0400"
    remote_src: true
  loop:
    - { src: "root-ca.pem",            dest: "root-ca.pem" }
    - { src: "black-master-00.pem",    dest: "black-master-00.pem" }
    - { src: "black-master-00-key.pem", dest: "black-master-00-key.pem" }
  notify: Restart filebeat

- name: Deploy filebeat.yml
  template:
    src: filebeat.yml.j2
    dest: /etc/filebeat/filebeat.yml
    owner: root
    group: root
    mode: "0640"
  notify: Restart filebeat

- name: Enable and start filebeat
  systemd:
    name: filebeat
    enabled: true
    state: started
    daemon_reload: true
```

- [ ] **Step 3: Add Restart filebeat handler**

Append to `blackwall-eye/ansible/roles/wazuh-manager/handlers/main.yml`:

```yaml
- name: Restart filebeat
  systemd:
    name: filebeat
    state: restarted
```

- [ ] **Step 4: Run security-node playbook with both tags**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/security-node.yml --tags "wazuh-indexer,wazuh-manager"
```
Expected: `failed=0`. Filebeat service active.

Verify filebeat ships alerts:
```bash
ansible black-master-00 -m command -a "filebeat test output" --become
```
Expected: lines starting with `parse url... OK`, `connection... OK`, `TLS... OK`, `talk to server... OK`.

- [ ] **Step 5: Commit**

```bash
git add blackwall-eye/ansible/roles/wazuh-manager/
git commit -m "feat(eye): wazuh-manager — install filebeat, ship alerts to indexer"
```

---

## Phase 5 — Wazuh dashboard role

### Task 9: wazuh-dashboard role skeleton + install

**Files:**
- Create: `blackwall-eye/ansible/roles/wazuh-dashboard/defaults/main.yml`
- Create: `blackwall-eye/ansible/roles/wazuh-dashboard/handlers/main.yml`
- Create: `blackwall-eye/ansible/roles/wazuh-dashboard/tasks/main.yml`

- [ ] **Step 1: Create defaults/main.yml**

```yaml
---
wazuh_version: "4.14.5"
wazuh_apt_key_url: "https://packages.wazuh.com/key/GPG-KEY-WAZUH"
wazuh_apt_repo: "deb https://packages.wazuh.com/4.x/apt/ stable main"

wazuh_dashboard_node_name: "{{ inventory_hostname }}"
wazuh_dashboard_listen_host: "0.0.0.0"
wazuh_dashboard_listen_port: 443

wazuh_indexer_url: "https://172.28.0.10:9200"
wazuh_manager_api_url: "https://172.28.0.10:55000"

# Credentials default to those in group_vars/wazuh_managers.yml.
# Listed here so the role works standalone if invoked without that group_vars.
wazuh_indexer_admin_user: "admin"
wazuh_indexer_admin_password: "Blackwall1!"
wazuh_manager_api_user: "wazuh"
wazuh_manager_api_password: "wazuh"

wazuh_certs_local_dir: "{{ playbook_dir }}/../../../tests/.wazuh-certs"
```

- [ ] **Step 2: Create handlers/main.yml**

```yaml
---
- name: Restart wazuh-dashboard
  systemd:
    name: wazuh-dashboard
    state: restarted
```

- [ ] **Step 3: Create tasks/main.yml with install + certs**

```yaml
---
- name: Wait for dpkg lock to be released
  shell: while fuser /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock >/dev/null 2>&1; do sleep 2; done
  changed_when: false

- name: Install wazuh-dashboard package
  apt:
    name: "wazuh-dashboard={{ wazuh_version }}-*"
    state: present
    update_cache: true

- name: Ensure /etc/wazuh-dashboard/certs dir exists
  file:
    path: /etc/wazuh-dashboard/certs
    state: directory
    owner: wazuh-dashboard
    group: wazuh-dashboard
    mode: "0500"

- name: Copy dashboard TLS certs from bundle
  copy:
    src: "/tmp/wazuh-certificates/{{ item.src }}"
    dest: "/etc/wazuh-dashboard/certs/{{ item.dest }}"
    owner: wazuh-dashboard
    group: wazuh-dashboard
    mode: "0400"
    remote_src: true
  loop:
    - { src: "root-ca.pem",                              dest: "root-ca.pem" }
    - { src: "{{ wazuh_dashboard_node_name }}.pem",      dest: "{{ wazuh_dashboard_node_name }}.pem" }
    - { src: "{{ wazuh_dashboard_node_name }}-key.pem",  dest: "{{ wazuh_dashboard_node_name }}-key.pem" }
  notify: Restart wazuh-dashboard
```

- [ ] **Step 4: Wire role into security-node playbook**

Edit `blackwall-eye/ansible/playbooks/security-node.yml`:

```yaml
---
- name: Configure security node (Wazuh manager + indexer + dashboard)
  hosts: wazuh_managers
  become: true
  roles:
    - role: wazuh-indexer
      tags: [wazuh-indexer]
    - role: wazuh-manager
      tags: [wazuh-manager]
    - role: wazuh-dashboard
      tags: [wazuh-dashboard]
```

- [ ] **Step 5: Run with --tags wazuh-dashboard**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-dashboard
```
Expected: `failed=0`. Package installed; certs copied. Service not started yet (no opensearch_dashboards.yml).

- [ ] **Step 6: Commit**

```bash
git add blackwall-eye/ansible/roles/wazuh-dashboard/ \
        blackwall-eye/ansible/playbooks/security-node.yml
git commit -m "feat(eye): wazuh-dashboard role skeleton (install + certs)"
```

---

### Task 10: wazuh-dashboard config + service start

**Files:**
- Create: `blackwall-eye/ansible/roles/wazuh-dashboard/templates/opensearch_dashboards.yml.j2`
- Create: `blackwall-eye/ansible/roles/wazuh-dashboard/templates/wazuh.yml.j2`
- Modify: `blackwall-eye/ansible/roles/wazuh-dashboard/tasks/main.yml`

- [ ] **Step 1: Create opensearch_dashboards.yml.j2**

```yaml
server.host: "{{ wazuh_dashboard_listen_host }}"
server.port: {{ wazuh_dashboard_listen_port }}
opensearch.hosts: ["{{ wazuh_indexer_url }}"]
opensearch.ssl.verificationMode: certificate
opensearch.username: "{{ wazuh_indexer_admin_user }}"
opensearch.password: "{{ wazuh_indexer_admin_password }}"
opensearch.requestHeadersWhitelist: ["securitytenant","Authorization"]

opensearch_security.multitenancy.enabled: false
opensearch_security.readonly_mode.roles: ["kibana_read_only"]

server.ssl.enabled: true
server.ssl.certificate: /etc/wazuh-dashboard/certs/{{ wazuh_dashboard_node_name }}.pem
server.ssl.key: /etc/wazuh-dashboard/certs/{{ wazuh_dashboard_node_name }}-key.pem
opensearch.ssl.certificateAuthorities: ["/etc/wazuh-dashboard/certs/root-ca.pem"]

uiSettings.overrides.defaultRoute: /app/wz-home
```

- [ ] **Step 2: Create wazuh.yml.j2 (embedded Wazuh app config)**

```yaml
hosts:
  - default:
      url: "{{ wazuh_manager_api_url | regex_replace(':[0-9]+$', '') }}"
      port: {{ wazuh_manager_api_url | regex_search(':[0-9]+$') | regex_replace(':', '') | default('55000') }}
      username: "{{ wazuh_manager_api_user }}"
      password: "{{ wazuh_manager_api_password }}"
      run_as: false
```

- [ ] **Step 3: Append config + service tasks to tasks/main.yml**

```yaml
- name: Deploy opensearch_dashboards.yml
  template:
    src: opensearch_dashboards.yml.j2
    dest: /etc/wazuh-dashboard/opensearch_dashboards.yml
    owner: wazuh-dashboard
    group: wazuh-dashboard
    mode: "0640"
  notify: Restart wazuh-dashboard

- name: Ensure embedded Wazuh app config dir exists
  file:
    path: /usr/share/wazuh-dashboard/data/wazuh/config
    state: directory
    owner: wazuh-dashboard
    group: wazuh-dashboard
    mode: "0750"

- name: Deploy embedded Wazuh app config (wazuh.yml)
  template:
    src: wazuh.yml.j2
    dest: /usr/share/wazuh-dashboard/data/wazuh/config/wazuh.yml
    owner: wazuh-dashboard
    group: wazuh-dashboard
    mode: "0640"
  notify: Restart wazuh-dashboard

- name: Enable and start wazuh-dashboard service
  systemd:
    name: wazuh-dashboard
    enabled: true
    state: started
    daemon_reload: true

- name: Wait for dashboard HTTPS port (443) to accept connections
  wait_for:
    host: 127.0.0.1
    port: 443
    timeout: 180
```

- [ ] **Step 4: Run the role**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-dashboard
```
Expected: `failed=0`. Dashboard service active; port 443 listening.

Verify HTTPS responds:
```bash
ansible black-master-00 -m shell -a "curl -sk https://127.0.0.1/ -o /dev/null -w '%{http_code}'" --become
```
Expected: `302` (redirect to login).

Verify from macOS host (containers must be up):
```bash
curl -sk https://localhost:5601/ -o /dev/null -w '%{http_code}\n'
```
Expected: `302`.

- [ ] **Step 5: Browser verification (manual)**

Open `https://localhost:5601` in a browser. Click through the self-signed cert warning. Log in with `admin / Blackwall1!`.
Expected: Wazuh dashboard home page renders. No agents listed yet (Phase 6 deploys the agent).

- [ ] **Step 6: Idempotency check**

Re-run. Expected: `changed=0`.

- [ ] **Step 7: Commit**

```bash
git add blackwall-eye/ansible/roles/wazuh-dashboard/
git commit -m "feat(eye): wazuh-dashboard config + service start on :443"
```

---

## Phase 6 — Agent endpoint parameterization

### Task 11: wazuh-agent defaults + template

**Files:**
- Modify: `blackwall-eye/ansible/roles/wazuh-agent/defaults/main.yml`
- Modify: `blackwall-eye/ansible/roles/wazuh-agent/templates/ossec.conf.j2`

- [ ] **Step 1: Update defaults/main.yml**

Replace contents with:

```yaml
---
wazuh_version: "4.14.5"
wazuh_apt_key_url: "https://packages.wazuh.com/key/GPG-KEY-WAZUH"
wazuh_apt_repo: "deb https://packages.wazuh.com/4.x/apt/ stable main"

# Agent identity
wazuh_agent_name: "{{ inventory_hostname }}"
wazuh_agent_groups: "default"

# Master endpoint (override per-target)
wazuh_manager_host: "172.28.0.10"
wazuh_manager_port: 1514
wazuh_manager_protocol: "tcp"

# Enrollment (authd)
wazuh_enrollment_host: "{{ wazuh_manager_host }}"
wazuh_enrollment_port: 1515
wazuh_enrollment_password: ""
```

- [ ] **Step 2: Update ossec.conf.j2**

Replace the `<client>` block (lines 3-18) of `blackwall-eye/ansible/roles/wazuh-agent/templates/ossec.conf.j2` with:

```jinja2
  <client>
    <server>
      <address>{{ wazuh_manager_host }}</address>
      <port>{{ wazuh_manager_port }}</port>
      <protocol>{{ wazuh_manager_protocol }}</protocol>
    </server>
    <crypto_method>aes</crypto_method>
    <notify_time>10</notify_time>
    <time-reconnect>60</time-reconnect>
    <auto_restart>yes</auto_restart>
    <enrollment>
      <enabled>yes</enabled>
      <manager_address>{{ wazuh_enrollment_host }}</manager_address>
      <port>{{ wazuh_enrollment_port }}</port>
      <agent_name>{{ wazuh_agent_name }}</agent_name>
      <groups>{{ wazuh_agent_groups }}</groups>
{% if wazuh_enrollment_password | length > 0 %}
      <authorization_pass_path>/var/ossec/etc/authd.pass</authorization_pass_path>
{% endif %}
    </enrollment>
  </client>
```

- [ ] **Step 3: Render to verify template syntax (no live run yet)**

```bash
cd blackwall-eye
ansible black-host-01 -m template -a "src=../blackwall-eye/ansible/roles/wazuh-agent/templates/ossec.conf.j2 dest=/tmp/ossec.conf.rendered" --check --diff
```
Expected: shows the rendered config with `<port>1514</port>` etc. in the diff.

- [ ] **Step 4: Commit**

```bash
git add blackwall-eye/ansible/roles/wazuh-agent/defaults/main.yml \
        blackwall-eye/ansible/roles/wazuh-agent/templates/ossec.conf.j2
git commit -m "feat(eye): wazuh-agent — parameterized endpoint and enrollment vars"
```

---

### Task 12: Conditional authd.pass write

**Files:**
- Modify: `blackwall-eye/ansible/roles/wazuh-agent/tasks/main.yml`

- [ ] **Step 1: Insert authd.pass task before service start**

In `blackwall-eye/ansible/roles/wazuh-agent/tasks/main.yml`, between the "Deploy ossec.conf from template" task and the "Enable and start wazuh-agent service" task, insert:

```yaml
- name: Write authd.pass when enrollment password set
  copy:
    content: "{{ wazuh_enrollment_password }}"
    dest: /var/ossec/etc/authd.pass
    owner: root
    group: wazuh
    mode: "0640"
  when: wazuh_enrollment_password | length > 0
  notify: Restart wazuh-agent

- name: Remove authd.pass when enrollment password unset
  file:
    path: /var/ossec/etc/authd.pass
    state: absent
  when: wazuh_enrollment_password | length == 0
  notify: Restart wazuh-agent
```

- [ ] **Step 2: Run agent playbook (default — no password)**

Note: the *manager* must be deployed first (Phases 3-5) for enrollment to succeed.

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/monitored-host.yml
```
Expected: `failed=0`. `authd.pass` should NOT exist on the host.

Verify:
```bash
ansible black-host-01 -m stat -a "path=/var/ossec/etc/authd.pass" --become | grep '"exists":'
```
Expected: `"exists": false`.

- [ ] **Step 3: Run with override (simulated password)**

```bash
ansible-playbook ansible/playbooks/monitored-host.yml \
  -e wazuh_enrollment_password=hunter2 \
  --tags wazuh-agent
```
Note: this will likely break enrollment (master doesn't have authd password configured), so we only check the file is written then revert.

Verify file written:
```bash
ansible black-host-01 -m shell -a "cat /var/ossec/etc/authd.pass" --become
```
Expected: `hunter2`.

Revert:
```bash
ansible-playbook ansible/playbooks/monitored-host.yml --tags wazuh-agent
```
Verify file removed:
```bash
ansible black-host-01 -m stat -a "path=/var/ossec/etc/authd.pass" --become | grep '"exists":'
```
Expected: `"exists": false`.

- [ ] **Step 4: Commit**

```bash
git add blackwall-eye/ansible/roles/wazuh-agent/tasks/main.yml
git commit -m "feat(eye): wazuh-agent — write authd.pass conditionally on enrollment_password"
```

---

## Phase 7 — Orchestration + verify

### Task 13: Wire wazuh-certificates into all.yml

**Files:**
- Modify: `blackwall-eye/ansible/playbooks/all.yml`

- [ ] **Step 1: Update all.yml**

```yaml
---
- import_playbook: wazuh-certificates.yml
- import_playbook: security-node.yml
- import_playbook: monitored-host.yml
```

- [ ] **Step 2: Full end-to-end from clean state**

```bash
# Tear down
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
rm -rf tests/.wazuh-certs/*
touch tests/.wazuh-certs/.gitkeep

# Bring up
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build

# Deploy
cd blackwall-eye
ansible all -m wait_for_connection -T 60
ansible-playbook ansible/playbooks/all.yml
```
Expected: `failed=0` across all hosts; play recap shows manager, indexer, dashboard, filebeat, agent all configured.

Allow ~30s for the agent to enrol with the manager, then verify:
```bash
ansible-playbook ansible/playbooks/verify-agents.yml
```
Expected: at least one agent (`black-host-01`) listed as `Active`.

- [ ] **Step 3: Commit**

```bash
git add blackwall-eye/ansible/playbooks/all.yml
git commit -m "feat(eye): wire wazuh-certificates into all.yml for one-command bring-up"
```

---

### Task 14: Update verify-agents.yml messages

**Files:**
- Modify: `blackwall-eye/ansible/playbooks/verify-agents.yml`

- [ ] **Step 1: Update fail_msg**

In `blackwall-eye/ansible/playbooks/verify-agents.yml`, line 27-30:

```yaml
        fail_msg: >
          No external agents are in 'Active' state. Make sure wazuh-agent is installed and
          running on black-host-01 and that it can reach the manager on port 1514/tcp
          and 1515/tcp.
```

- [ ] **Step 2: Run**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/verify-agents.yml
```
Expected: success_msg printed.

- [ ] **Step 3: Commit**

```bash
git add blackwall-eye/ansible/playbooks/verify-agents.yml
git commit -m "test(eye): update verify-agents fail_msg for new host name"
```

---

## Phase 8 — Documentation

### Task 15: Rewrite tests/README.md

**Files:**
- Modify: `tests/README.md`

- [ ] **Step 1: Replace contents**

Full rewrite. The file becomes:

````markdown
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

To change credentials, edit `blackwall-eye/ansible/group_vars/wazuh_managers.yml` and re-run `ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-indexer`.

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

These are the same commands used during development to validate each piece independently. Run any subset to debug or sanity-check after a change. All `ansible` commands are run from `blackwall-eye/`; `docker` and `curl` from the repo root.

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

# From blackwall-eye/
ansible all -m wait_for_connection -T 30
# Expect: SUCCESS for both hosts
```

### Wazuh indexer
```bash
# Service active?
ansible black-master-00 -m command -a "systemctl is-active wazuh-indexer" --become
# Expect: active

# Cluster health (uses configured admin password)
ansible black-master-00 -m shell -a \
  "curl -sk -u admin:Blackwall1! https://127.0.0.1:9200/_cluster/health" --become
# Expect: JSON with "status":"green" or "yellow"

# Default credentials no longer accepted
ansible black-master-00 -m shell -a \
  "curl -sk -u admin:admin https://127.0.0.1:9200/_cluster/health" --become
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

# In-container HTTPS reachability
ansible black-master-00 -m shell -a \
  "curl -sk https://127.0.0.1/ -o /dev/null -w '%{http_code}'" --become
# Expect: 302

# From macOS host
curl -sk https://localhost:5601/ -o /dev/null -w '%{http_code}\n'
# Expect: 302
```

### Wazuh agent
```bash
ansible black-host-01 -m command -a "systemctl status wazuh-agent" --become

ansible black-host-01 -m command -a "tail -20 /var/ossec/logs/ossec.log" --become

# Registered + Active agents via manager CLI
ansible wazuh_managers -m command -a "/var/ossec/bin/agent_control -l" --become
# Expect: at least one Active entry for black-host-01
```

### Agent endpoint override (point at a different master)
```bash
ansible-playbook ansible/playbooks/monitored-host.yml \
  -e wazuh_manager_host=192.0.2.1 \
  -e wazuh_enrollment_password=hunter2 \
  --tags wazuh-agent

# Verify config was rewritten
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
# Expect: 'ii' status for all four, version 4.14.5-*
```

### Full clean-room acceptance
```bash
# Tear down + clear certs
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
rm -rf tests/.wazuh-certs/wazuh-certificates*

# Bring up + deploy from scratch
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
cd blackwall-eye
ansible all -m wait_for_connection -T 60
ansible-playbook ansible/playbooks/all.yml

# Wait ~30s for agent enrolment, then:
ansible-playbook ansible/playbooks/verify-agents.yml

# Browser check
open https://localhost:5601    # admin / Blackwall1!
```

---

## 7. Service status (ad-hoc)

```bash
# Manager
ansible wazuh_managers -m command -a "systemctl status wazuh-manager" --become

# Indexer
ansible wazuh_managers -m command -a "systemctl status wazuh-indexer" --become

# Dashboard
ansible wazuh_managers -m command -a "systemctl status wazuh-dashboard" --become

# Filebeat
ansible wazuh_managers -m command -a "systemctl status filebeat" --become

# Agent
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

## 12. Known limitations

**auditd:** kernel audit netlink is not accessible inside Docker. The role installs rules correctly, but the service won't start in containers (detected via `/.dockerenv`). On real hosts auditd starts normally.

**Dashboard cert:** self-signed; browsers show "Not Secure". Acceptable for `localhost` only. Never expose port 5601 beyond `localhost`.

---

## 13. File layout

```
blackwall/                                  # repo root
├── tests/
│   ├── docker/
│   │   ├── docker-compose.yml
│   │   ├── images/ubuntu2204/Dockerfile
│   │   └── ssh_config
│   ├── inventory/
│   │   └── hosts.yml
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
````

- [ ] **Step 2: Commit**

```bash
git add tests/README.md
git commit -m "docs(tests): rewrite README for 2-container topology + dashboard + agent overrides"
```

---

### Task 16: Update root README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add "Test environment" section**

Read the current `README.md` first to find where to insert the section (typically after the modules table). Add this block:

````markdown
## Test environment

A 2-container Docker setup for development:

| Container | Role |
|---|---|
| `black-master-00` | Wazuh manager + indexer + dashboard |
| `black-host-01`   | Wazuh agent (and target for other module roles) |

```bash
# Up
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
cd blackwall-eye && ansible-playbook ansible/playbooks/all.yml

# Dashboard: https://localhost:5601   (admin / Blackwall1!)

# Down
cd .. && docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
```

To point agents at a real Wazuh master:

```bash
ansible-playbook ansible/playbooks/monitored-host.yml \
  -i inventory/prod.yml \
  -e wazuh_manager_host=wazuh.prod.example.com \
  -e wazuh_enrollment_password=<secret>
```

Full docs: [tests/README.md](tests/README.md).
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs(root): add test environment quick-start section"
```

---

### Task 17: Update STATUS.md

**Files:**
- Modify: `blackwall-eye/STATUS.md`

- [ ] **Step 1: Update front-matter**

In `blackwall-eye/STATUS.md`, update the front-matter to:

```yaml
---
module: blackwall-eye
status: in_progress
stage: 1
last_updated: 2026-05-15
last_task: "Test env redesign: 2-container topology (black-master-00 + black-host-01), Wazuh all-in-one master (manager + indexer + dashboard via Ansible roles, TLS via wazuh-certs-tool), dashboard on https://localhost:5601, agent endpoint/enrollment fully parameterized for any master."
next_task: "Stage 1 complete — mark done or add Suricata/Zeek for Stage 2 network layer"
blockers: []
---
```

Also append a bullet to the "Что сделано" body section:

```markdown
- 2026-05-15: переработана тестовая среда — `black-master-00` (manager + indexer + dashboard) + `black-host-01` (agent). Добавлены роли `wazuh-indexer`, `wazuh-dashboard`. Manager научен шипить алерты в indexer через filebeat. Сертификаты генерируются разово через `wazuh-certificates.yml`. Дашборд доступен на `https://localhost:5601` (admin / Blackwall1!). Агент полностью параметризован: `wazuh_manager_host`/`port`/`protocol`/`enrollment_*` — позволяет указать любого мастера через `-e` или group_vars. Spec: `docs/superpowers/specs/2026-05-15-test-env-master-host-dashboard-design.md`. Plan: `docs/superpowers/plans/2026-05-15-test-env-master-host-dashboard.md`.
```

- [ ] **Step 2: Commit**

```bash
git add blackwall-eye/STATUS.md
git commit -m "docs(eye): update STATUS for 2-container test env + dashboard work"
```

---

## Phase 9 — Acceptance

### Task 18: End-to-end acceptance against the spec

This task validates each of the 8 acceptance criteria from the spec. No code changes; only test commands.

- [ ] **Step 1: Clean slate**

```bash
cd /Users/akrugliak/coding/blackwall
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
rm -rf tests/.wazuh-certs/wazuh-certificates*
```

- [ ] **Step 2: Criterion 1 — `docker compose up` yields exactly two containers**

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
docker ps --format '{{.Names}}' | sort
```
Expected:
```
black-host-01
black-master-00
```

- [ ] **Step 3: Criterion 2 — `all.yml` completes with `failed=0`**

```bash
cd blackwall-eye
ansible all -m wait_for_connection -T 60
ansible-playbook ansible/playbooks/all.yml
```
Expected: PLAY RECAP shows `failed=0` for both hosts.

- [ ] **Step 4: Criterion 3 — dashboard login**

```bash
curl -sk https://localhost:5601/ -o /dev/null -w '%{http_code}\n'
```
Expected: `302`.

Open browser to `https://localhost:5601`, log in `admin / Blackwall1!`.
Expected: home page renders.

- [ ] **Step 5: Criterion 4 — agent listed Active in dashboard**

In dashboard: navigate to Agents → Endpoint Summary.
Expected: `black-host-01` listed with status `Active`.

- [ ] **Step 6: Criterion 5 — verify-agents.yml passes**

```bash
ansible-playbook ansible/playbooks/verify-agents.yml
```
Expected: assert succeeds.

- [ ] **Step 7: Criterion 6 — agent override rewrites config**

```bash
ansible-playbook ansible/playbooks/monitored-host.yml \
  -e wazuh_manager_host=192.0.2.1 \
  -e wazuh_enrollment_password=hunter2 \
  --tags wazuh-agent

ansible black-host-01 -m shell -a "grep -E '<address>|<authorization_pass_path>' /var/ossec/etc/ossec.conf" --become
ansible black-host-01 -m shell -a "cat /var/ossec/etc/authd.pass" --become
```
Expected: `<address>192.0.2.1</address>` present; `<authorization_pass_path>` present; `authd.pass` contains `hunter2`.

Revert:
```bash
ansible-playbook ansible/playbooks/monitored-host.yml --tags wazuh-agent
```

- [ ] **Step 8: Criterion 7 — tear down then re-up cleanly**

```bash
cd ..
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
docker ps --format '{{.Names}}' | sort
```
Expected: both containers come up.

- [ ] **Step 9: Criterion 8 — no references to old names remain**

```bash
cd /Users/akrugliak/coding/blackwall
grep -rn --include='*.md' --include='*.yml' --include='*.j2' \
  -E 'black-0[012]\b|hosts\.single\.yml' \
  . | grep -v '^./docs/superpowers/' | grep -v '^./blackwall-eye/STATUS.md'
```
Expected: no matches (the docs/superpowers/ exclusion is OK — old spec docs naturally mention the previous names; STATUS.md historical entries are also OK).

If any matches surface in tests/, blackwall-eye/ansible/, or README.md, fix them and re-run.

- [ ] **Step 10: Final report**

If all 8 criteria pass, this plan is complete. Hand off to the user with a one-line status summary.

No commit for this task (verification only).

---

## Self-review

**Spec coverage:**
- Container topology → Task 1
- Cert playbook → Task 3
- wazuh-indexer role → Tasks 4–7
- wazuh-manager filebeat wiring → Task 8
- wazuh-dashboard role → Tasks 9–10
- Agent endpoint/secrets vars → Tasks 11–12
- group_vars/wazuh_managers.yml → Task 7
- Playbook orchestration (all.yml, security-node.yml) → Tasks 5, 9, 13
- Inventory + ssh_config update → Task 2
- Dashboard access (port 5601, creds) → Tasks 1, 7, 10, 15
- Real-host deployment example → Task 15
- Documentation (root + tests) → Tasks 15, 16
- STATUS.md update → Task 17
- All 8 acceptance criteria → Task 18

**Placeholder scan:** no TBD/TODO; every step has actual code or commands. Filebeat templates and opensearch.yml referenced upstream URL — engineer told to verify version-specific syntax before committing.

**Type consistency:** variable names consistent across tasks (`wazuh_indexer_admin_password`, `wazuh_manager_api_password`, `wazuh_manager_host`, `wazuh_enrollment_password`). Role names consistent (`wazuh-indexer`, `wazuh-manager`, `wazuh-dashboard`, `wazuh-agent`). Container names consistent (`black-master-00`, `black-host-01`).
