# Test-env lab tooling — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Provide two bash scripts (`tests/bin/bw-lab`, `tests/bin/bw-check`) and a set of README commands that bring the Blackwall lab up, tear it down and fully verify it — including the new Prometheus/Grafana and nftables.

**Architecture:** Prometheus and Grafana are added as plain lab compose services with configs from the repo. Host metrics come from a new ansible role `node-exporter` (blackwall-voice), the firewall from a new parametrised ansible role `nftables` (blackwall-eye). The scripts are thin wrappers: host facts are gathered by ansible playbooks with `assert`, HTTP endpoints are checked with `curl` straight from the Mac.

**Tech Stack:** bash, Docker Compose, Ansible (core 2.20), nftables, prometheus-node-exporter, Prometheus, Grafana.

**Spec:** `docs/superpowers/specs/2026-08-31-test-env-lab-tooling-design.md`

## Global Constraints

- The development branch is `claude-dev`. Commit granularly, with super-brief messages. **Do not push.**
- All paths in the scripts resolve from the repo root — the scripts work from any directory.
- Docker commands in the docs are given from the repo root, ansible commands from the module directory (each module has its own `ansible.cfg` pointing at `../tests/inventory/`).
- Prometheus/Grafana images are pinned by version. No `:latest`.
- The default compose profile is `ubuntu2204`, overridden with `--profile` or `BW_PROFILE`.
- The test network is `172.28.0.0/24`. New IPs: Prometheus `172.28.0.12`, Grafana `172.28.0.13`.
- Lab secrets (`admin/Blackwall1!`, `root/blackwall`, Grafana `admin/blackwall`) are localhost-only, never published outside.
- Every `bw-check` check prints `PASS` / `FAIL` / `SKIP` and affects the final exit code.
- Updating README/STATUS is part of the task that changes behaviour (the "Keep docs in sync" rule from CLAUDE.md).

---

### Task 1: `node-exporter` role in blackwall-voice

Installs `prometheus-node-exporter` on all lab hosts so Prometheus has something to scrape.

**Files:**
- Create: `blackwall-voice/ansible/roles/node-exporter/defaults/main.yml`
- Create: `blackwall-voice/ansible/roles/node-exporter/tasks/main.yml`
- Create: `blackwall-voice/ansible/roles/node-exporter/tasks/install-apt.yml`
- Create: `blackwall-voice/ansible/roles/node-exporter/tasks/install-tarball.yml`
- Create: `blackwall-voice/ansible/roles/node-exporter/templates/node-exporter.service.j2`
- Create: `blackwall-voice/ansible/roles/node-exporter/handlers/main.yml`
- Create: `blackwall-voice/ansible/playbooks/metrics.yml`
- Test: `blackwall-voice/ansible/playbooks/verify-metrics.yml`

**Interfaces:**
- Consumes: the inventory `tests/inventory/hosts.yml` (groups `wazuh_managers`, `wazuh_agents`).
- Produces: node_exporter listens on `:9100` on every host of group `all`; the systemd unit is called `prometheus-node-exporter` for an `apt` install and `node_exporter` for a tarball. The role exports the fact `node_exporter_service_name` (set_fact) — `verify-metrics.yml` uses it.

- [ ] **Step 1: Write a failing verify playbook**

`blackwall-voice/ansible/playbooks/verify-metrics.yml`:

```yaml
---
- name: Verify node_exporter is up and serving metrics
  hosts: all
  become: true
  gather_facts: false

  tasks:
    - name: Detect installed node_exporter unit
      ansible.builtin.shell: >-
        systemctl list-unit-files --no-legend --no-pager
        'prometheus-node-exporter.service' 'node_exporter.service'
        | awk '{print $1}' | head -1
      register: ne_unit
      changed_when: false

    - name: Assert a node_exporter unit exists
      ansible.builtin.assert:
        that: ne_unit.stdout | length > 0
        fail_msg: "node_exporter unit not installed on {{ inventory_hostname }}"

    - name: Check unit is active
      ansible.builtin.command: "systemctl is-active {{ ne_unit.stdout }}"
      register: ne_active
      changed_when: false
      failed_when: ne_active.stdout != 'active'

    - name: Fetch metrics endpoint
      ansible.builtin.uri:
        url: http://127.0.0.1:9100/metrics
        return_content: true
      register: ne_metrics

    - name: Assert node_ metrics are present
      ansible.builtin.assert:
        that:
          - "'node_load1' in ne_metrics.content"
          - "'node_memory_MemTotal_bytes' in ne_metrics.content"
        fail_msg: "node_exporter answered but without expected node_* metrics"
```

- [ ] **Step 2: Run it and make sure it fails**

```bash
cd blackwall-voice
ansible-playbook ansible/playbooks/verify-metrics.yml
```

Expected: FAIL on "node_exporter unit not installed".

- [ ] **Step 3: Write the role defaults**

`blackwall-voice/ansible/roles/node-exporter/defaults/main.yml`:

```yaml
---
# apt — the prometheus-node-exporter package from universe (fast, but depends on the mirror)
# tarball — the official GitHub release (fallback if the package is missing)
node_exporter_install_method: apt

node_exporter_version: "1.8.2"
node_exporter_listen_address: "0.0.0.0:9100"
node_exporter_bin_dir: /usr/local/bin
node_exporter_user: node-exporter
```

- [ ] **Step 4: Write tasks/main.yml, install-apt.yml, install-tarball.yml**

`tasks/main.yml`:

```yaml
---
- name: Install node_exporter via apt
  ansible.builtin.include_tasks: install-apt.yml
  when: node_exporter_install_method == 'apt'

- name: Install node_exporter from tarball
  ansible.builtin.include_tasks: install-tarball.yml
  when: node_exporter_install_method == 'tarball'

- name: Enable and start node_exporter
  ansible.builtin.systemd:
    name: "{{ node_exporter_service_name }}"
    enabled: true
    state: started
    daemon_reload: true
```

`tasks/install-apt.yml`:

```yaml
---
- name: Install prometheus-node-exporter package
  ansible.builtin.apt:
    name: prometheus-node-exporter
    state: present
    update_cache: true
    cache_valid_time: 3600

- name: Set listen address
  ansible.builtin.copy:
    dest: /etc/default/prometheus-node-exporter
    content: |
      # Managed by Ansible role node-exporter. Do not edit by hand.
      ARGS="--web.listen-address={{ node_exporter_listen_address }}"
    owner: root
    group: root
    mode: "0644"
  notify: Restart node_exporter

- name: Record service name
  ansible.builtin.set_fact:
    node_exporter_service_name: prometheus-node-exporter
```

`tasks/install-tarball.yml`:

```yaml
---
- name: Create node_exporter system user
  ansible.builtin.user:
    name: "{{ node_exporter_user }}"
    system: true
    shell: /usr/sbin/nologin
    create_home: false

- name: Download node_exporter release
  ansible.builtin.unarchive:
    src: "https://github.com/prometheus/node_exporter/releases/download/v{{ node_exporter_version }}/node_exporter-{{ node_exporter_version }}.linux-amd64.tar.gz"
    dest: /tmp
    remote_src: true
    creates: "/tmp/node_exporter-{{ node_exporter_version }}.linux-amd64/node_exporter"

- name: Install binary
  ansible.builtin.copy:
    src: "/tmp/node_exporter-{{ node_exporter_version }}.linux-amd64/node_exporter"
    dest: "{{ node_exporter_bin_dir }}/node_exporter"
    remote_src: true
    owner: root
    group: root
    mode: "0755"
  notify: Restart node_exporter

- name: Install systemd unit
  ansible.builtin.template:
    src: node-exporter.service.j2
    dest: /etc/systemd/system/node_exporter.service
    owner: root
    group: root
    mode: "0644"
  notify: Restart node_exporter

- name: Record service name
  ansible.builtin.set_fact:
    node_exporter_service_name: node_exporter
```

`templates/node-exporter.service.j2`:

```
[Unit]
Description=Prometheus node_exporter (managed by Blackwall)
After=network-online.target
Wants=network-online.target

[Service]
User={{ node_exporter_user }}
Group={{ node_exporter_user }}
ExecStart={{ node_exporter_bin_dir }}/node_exporter --web.listen-address={{ node_exporter_listen_address }}
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
```

`handlers/main.yml`:

```yaml
---
- name: Restart node_exporter
  ansible.builtin.systemd:
    name: "{{ node_exporter_service_name }}"
    state: restarted
    daemon_reload: true
```

- [ ] **Step 5: Write the rollout playbook**

`blackwall-voice/ansible/playbooks/metrics.yml`:

```yaml
---
- name: Deploy node_exporter to all monitored hosts
  hosts: all
  become: true
  gather_facts: true

  roles:
    - role: node-exporter
      tags: [node-exporter, metrics]
```

- [ ] **Step 6: Roll out and verify**

```bash
cd blackwall-voice
ansible-playbook ansible/playbooks/metrics.yml
ansible-playbook ansible/playbooks/verify-metrics.yml
```

Expected: both `failed=0`. If `apt` cannot find `prometheus-node-exporter` — re-run the rollout with `-e node_exporter_install_method=tarball` and make that the default in `defaults/main.yml`.

- [ ] **Step 7: Commit**

```bash
git add blackwall-voice/ansible/roles/node-exporter blackwall-voice/ansible/playbooks/metrics.yml blackwall-voice/ansible/playbooks/verify-metrics.yml
git commit -m "feat(voice): node-exporter role + metrics/verify-metrics playbooks"
```

---

### Task 2: Prometheus and Grafana as compose services

**Files:**
- Modify: `tests/docker/docker-compose.yml` (add two services at the end of the `ubuntu2204` block)
- Create: `tests/docker/prometheus/prometheus.yml`
- Create: `tests/docker/grafana/provisioning/datasources/prometheus.yml`
- Create: `tests/docker/grafana/provisioning/dashboards/dashboards.yml`
- Create: `tests/docker/grafana/dashboards/blackwall-hosts.json`

**Interfaces:**
- Consumes: node_exporter on `:9100` from Task 1; compose network DNS (service names resolve inside the `blackwall-test` network).
- Produces: Prometheus at `http://localhost:9090`, Grafana at `http://localhost:3000` (`admin` / `blackwall`), a datasource named `Prometheus`, job `blackwall-hosts` with three targets.

- [ ] **Step 1: Write prometheus.yml**

Targets are given as compose service names — inside the `blackwall-test` network they resolve via the built-in DNS, so in Grafana panels `instance` reads as `black-host-01:9100` rather than an IP.

```yaml
# tests/docker/prometheus/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    env: blackwall-test

scrape_configs:
  - job_name: prometheus
    static_configs:
      - targets: ["localhost:9090"]

  - job_name: blackwall-hosts
    static_configs:
      - targets:
          - "black-master-00:9100"
          - "black-host-01:9100"
          - "black-dead-98:9100"
        labels:
          env: test
```

`black-dead-98` normally goes `down` after `tests/triggers/broken-agents.yml` — that is expected. `bw-check metrics` (Task 6) requires `up` only from `black-master-00` and `black-host-01`, and prints an informational line for `black-dead-98`.

- [ ] **Step 2: Write the Grafana provisioning**

`tests/docker/grafana/provisioning/datasources/prometheus.yml`:

```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://black-metrics-00:9090
    isDefault: true
    editable: false
```

`tests/docker/grafana/provisioning/dashboards/dashboards.yml`:

```yaml
apiVersion: 1

providers:
  - name: blackwall
    orgId: 1
    folder: Blackwall
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    allowUiUpdates: false
    options:
      path: /var/lib/grafana/dashboards
      foldersFromFilesStructure: false
```

- [ ] **Step 3: Write the dashboard as code**

`tests/docker/grafana/dashboards/blackwall-hosts.json`:

```json
{
  "uid": "blackwall-hosts",
  "title": "Blackwall hosts",
  "tags": ["blackwall"],
  "timezone": "browser",
  "schemaVersion": 39,
  "refresh": "30s",
  "time": { "from": "now-1h", "to": "now" },
  "panels": [
    {
      "type": "stat",
      "title": "Hosts up",
      "gridPos": { "h": 4, "w": 6, "x": 0, "y": 0 },
      "targets": [
        { "refId": "A", "expr": "sum(up{job=\"blackwall-hosts\"})" }
      ]
    },
    {
      "type": "timeseries",
      "title": "Load average (1m)",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 4 },
      "targets": [
        { "refId": "A", "expr": "node_load1", "legendFormat": "{{instance}}" }
      ]
    },
    {
      "type": "timeseries",
      "title": "Memory used %",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 4 },
      "targets": [
        {
          "refId": "A",
          "expr": "100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)",
          "legendFormat": "{{instance}}"
        }
      ]
    }
  ]
}
```

- [ ] **Step 4: Add the services to docker-compose.yml**

Insert before the "Debian 12 (future)" block. The services deliberately don't inherit the `*black-node` anchor: they need no privileged mode, systemd or host cgroup.

```yaml
  # ── Lab monitoring: Prometheus + Grafana ──────────────────────────────────
  # Plain containers (not systemd nodes). Configs are mounted from the repo,
  # no manual steps inside the containers.

  black-metrics-00:
    image: prom/prometheus:v2.55.1
    profiles: [ubuntu2204]
    container_name: black-metrics-00
    hostname: black-metrics-00
    command:
      - --config.file=/etc/prometheus/prometheus.yml
      - --storage.tsdb.retention.time=24h
      - --web.enable-lifecycle
    volumes:
      - ../docker/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    ports:
      - "9090:9090"
    networks:
      blackwall-test:
        ipv4_address: 172.28.0.12
    restart: "no"

  black-grafana-00:
    image: grafana/grafana:11.4.0
    profiles: [ubuntu2204]
    container_name: black-grafana-00
    hostname: black-grafana-00
    environment:
      GF_SECURITY_ADMIN_USER: admin
      GF_SECURITY_ADMIN_PASSWORD: blackwall
      GF_AUTH_ANONYMOUS_ENABLED: "false"
      GF_USERS_ALLOW_SIGN_UP: "false"
    volumes:
      - ../docker/grafana/provisioning:/etc/grafana/provisioning:ro
      - ../docker/grafana/dashboards:/var/lib/grafana/dashboards:ro
    depends_on:
      - black-metrics-00
    ports:
      - "3000:3000"
    networks:
      blackwall-test:
        ipv4_address: 172.28.0.13
    restart: "no"
```

Volume paths are relative to `tests/docker/` (the compose file's directory). If the paths come out wrong in `docker compose config`, replace `../docker/` with `./`.

- [ ] **Step 5: Validate the config and bring it up**

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 config --quiet
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d
```

Expected: exit 0, both containers in `docker ps`.

- [ ] **Step 6: Check the endpoints**

```bash
curl -sf http://localhost:9090/-/ready
# Expect: Prometheus Server is Ready.

curl -sf 'http://localhost:9090/api/v1/targets?state=active' | jq -r '.data.activeTargets[] | "\(.labels.instance) \(.health)"'
# Expect: black-master-00:9100 up / black-host-01:9100 up

curl -sf -u admin:blackwall http://localhost:3000/api/health | jq -r '.database'
# Expect: ok

curl -sf -u admin:blackwall http://localhost:3000/api/datasources | jq -r '.[].name'
# Expect: Prometheus

curl -sf -u admin:blackwall http://localhost:3000/api/dashboards/uid/blackwall-hosts | jq -r '.dashboard.title'
# Expect: Blackwall hosts
```

- [ ] **Step 7: Commit**

```bash
git add tests/docker/docker-compose.yml tests/docker/prometheus tests/docker/grafana
git commit -m "feat(tests): prometheus + grafana as lab compose services"
```

---

### Task 3: `nftables` role in blackwall-eye

**Files:**
- Create: `blackwall-eye/ansible/roles/nftables/defaults/main.yml`
- Create: `blackwall-eye/ansible/roles/nftables/tasks/main.yml`
- Create: `blackwall-eye/ansible/roles/nftables/templates/nftables.conf.j2`
- Create: `blackwall-eye/ansible/roles/nftables/templates/blackwall-base.nft.j2`
- Create: `blackwall-eye/ansible/roles/nftables/handlers/main.yml`
- Create: `blackwall-eye/ansible/group_vars/wazuh_agents.yml`
- Modify: `blackwall-eye/ansible/group_vars/wazuh_managers.yml` (add `nft_public_tcp`)
- Create: `blackwall-eye/ansible/playbooks/nftables.yml`
- Test: `blackwall-eye/ansible/playbooks/verify-nftables.yml`

**Interfaces:**
- Consumes: inventory groups `wazuh_managers` / `wazuh_agents`.
- Produces: table `inet blackwall_base` with `policy drop` on chain `input`; variables `nft_enabled`, `nft_wan_if`, `nft_public_tcp`, `nft_restricted`, `nft_forward_policy`, `nft_log_drops`.

**Critical — ports that must not be closed.** On `black-master-00`, filebeat and the dashboard reach the indexer not via loopback but at the interface address `172.28.0.10`, so packets go through the `input` chain. Closing `9200`/`9300` silently breaks alert delivery to the indexer — services stay `active`, alerts just stop arriving.

| Group | `nft_public_tcp` | Why |
|---|---|---|
| `wazuh_managers` | 22, 443, 1514, 1515, 9200, 9300, 9100, 55000 | ssh, dashboard, agents, enrollment, indexer REST, indexer transport, metrics, REST API |
| `wazuh_agents` | 22, 9100 | ssh, metrics |

- [ ] **Step 1: Write a failing verify playbook**

`blackwall-eye/ansible/playbooks/verify-nftables.yml`:

```yaml
---
- name: Verify nftables ruleset
  hosts: all
  become: true
  gather_facts: false

  tasks:
    - name: Check nft binary is available
      ansible.builtin.command: which nft
      register: nft_bin
      changed_when: false
      failed_when: false

    - name: Skip host when nftables is unavailable
      ansible.builtin.meta: end_host
      when: nft_bin.rc != 0

    - name: Dump ruleset
      ansible.builtin.command: nft list table inet blackwall_base
      register: nft_dump
      changed_when: false

    - name: Assert input policy is drop
      ansible.builtin.assert:
        that: "'hook input priority filter; policy drop;' in nft_dump.stdout"
        fail_msg: "chain input is not policy drop — the firewall does not protect the host"

    - name: Assert every expected port is present
      ansible.builtin.assert:
        that: "(item | string) in (nft_dump.stdout | regex_search('elements = \\{[^}]*\\}') | default('', true))"
        fail_msg: "port {{ item }} is missing from set public_tcp"
      loop: "{{ nft_public_tcp }}"

    - name: Show ruleset
      ansible.builtin.debug:
        var: nft_dump.stdout_lines
```

- [ ] **Step 2: Run it and make sure it fails**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/verify-nftables.yml
```

Expected: FAIL (no `blackwall_base` table), or every host hits `end_host` if `nft` isn't installed at all.

- [ ] **Step 3: Write the defaults**

`blackwall-eye/ansible/roles/nftables/defaults/main.yml`:

```yaml
---
# Disable the role on a host entirely
nft_enabled: true

# External interface. Check: ip -o -4 route show to default | awk '{print $5}'
nft_wan_if: eth0

# TCP ports open to everyone
nft_public_tcp: [22]

# Ports reachable only from the listed sources:
#   nft_restricted:
#     - port: 9100
#       sources: [172.28.0.12]
nft_restricted: []

nft_forward_policy: accept
nft_log_drops: true
```

- [ ] **Step 4: Write the ruleset template**

`blackwall-eye/ansible/roles/nftables/templates/blackwall-base.nft.j2`:

```
#!/usr/sbin/nft -f
#
# Managed by Ansible role blackwall-eye/nftables. Do not edit by hand.
#
# Check:     nft list table inet blackwall_base
# Roll back: nft delete table inet blackwall_base

# Atomic reload idiom: create (if missing), delete, define again.
table inet blackwall_base
delete table inet blackwall_base

table inet blackwall_base {

    set public_tcp {
        type inet_service
        elements = { {{ nft_public_tcp | join(', ') }} }
    }
{% for r in nft_restricted %}

    set src_{{ r.port }} {
        type ipv4_addr
        flags interval
        auto-merge
        elements = { {{ r.sources | join(', ') }} }
    }
{% endfor %}

    chain input {
        type filter hook input priority filter; policy drop;

        # Skeleton: without these rules the host locks itself out.
        iif lo accept
        ct state established,related accept
        ct state invalid drop

        # All of ICMP: without icmpv6, IPv6 breaks (neighbor discovery, PMTU),
        # without icmp, IPv4 path MTU discovery breaks.
        meta l4proto { icmp, ipv6-icmp } accept
        ip6 daddr fe80::/64 udp dport 546 accept

        tcp dport @public_tcp ct state new counter accept
{% for r in nft_restricted %}
        tcp dport {{ r.port }} ip saddr @src_{{ r.port }} ct state new counter accept
{% endfor %}
{% if nft_log_drops %}

        limit rate 5/minute log prefix "[blackwall-base] input-drop " level info
{% endif %}
        counter
    }

    chain forward {
        type filter hook forward priority filter; policy {{ nft_forward_policy }};

        ct state established,related accept
        ct state invalid drop
    }

    chain output {
        type filter hook output priority filter; policy accept;
    }
}
```

`blackwall-eye/ansible/roles/nftables/templates/nftables.conf.j2`:

```
#!/usr/sbin/nft -f
#
# Managed by Ansible role blackwall-eye/nftables. Do not edit by hand.
# The role owns this file entirely; the rules live in /etc/nftables.d/*.nft.

flush ruleset

include "/etc/nftables.d/*.nft"
```

- [ ] **Step 5: Write tasks and handlers**

`blackwall-eye/ansible/roles/nftables/tasks/main.yml`:

```yaml
---
- name: Skip role when disabled
  ansible.builtin.meta: end_host
  when: not nft_enabled

- name: Install nftables
  ansible.builtin.apt:
    name: nftables
    state: present
    update_cache: true
    cache_valid_time: 3600

- name: Create rules directory
  ansible.builtin.file:
    path: /etc/nftables.d
    state: directory
    owner: root
    group: root
    mode: "0755"

- name: Render ruleset to a staging path
  ansible.builtin.template:
    src: blackwall-base.nft.j2
    dest: /etc/nftables.d/.blackwall-base.nft.new
    owner: root
    group: root
    mode: "0600"
  changed_when: false

- name: Syntax-check the rendered ruleset
  ansible.builtin.command: nft -c -f /etc/nftables.d/.blackwall-base.nft.new
  changed_when: false

- name: Install checked ruleset
  ansible.builtin.copy:
    src: /etc/nftables.d/.blackwall-base.nft.new
    dest: /etc/nftables.d/blackwall-base.nft
    remote_src: true
    owner: root
    group: root
    mode: "0600"
  notify: Reload nftables

- name: Remove staging file
  ansible.builtin.file:
    path: /etc/nftables.d/.blackwall-base.nft.new
    state: absent
  changed_when: false

- name: Install nftables.conf
  ansible.builtin.template:
    src: nftables.conf.j2
    dest: /etc/nftables.conf
    owner: root
    group: root
    mode: "0644"
  notify: Reload nftables

- name: Enable and start nftables
  ansible.builtin.systemd:
    name: nftables
    enabled: true
    state: started
```

`blackwall-eye/ansible/roles/nftables/handlers/main.yml`:

```yaml
---
- name: Reload nftables
  ansible.builtin.systemd:
    name: nftables
    state: reloaded
```

- [ ] **Step 6: Set ports per group**

`blackwall-eye/ansible/group_vars/wazuh_agents.yml` (create):

```yaml
---
# 22   — ssh for ansible
# 9100 — node_exporter, scraped by black-metrics-00
nft_public_tcp: [22, 9100]
```

Append to `blackwall-eye/ansible/group_vars/wazuh_managers.yml`:

```yaml
# nftables: closing 9200/9300 silently breaks alert delivery to the indexer —
# filebeat and the dashboard reach it at the interface address, not via loopback.
#   22    ssh
#   443   wazuh-dashboard (published as localhost:5601)
#   1514  events from agents
#   1515  authd enrollment
#   9100  node_exporter
#   9200  indexer REST
#   9300  indexer transport
#   55000 wazuh-manager REST API
nft_public_tcp: [22, 443, 1514, 1515, 9100, 9200, 9300, 55000]
```

- [ ] **Step 7: Write the rollout playbook**

`blackwall-eye/ansible/playbooks/nftables.yml`:

```yaml
---
- name: Apply nftables ruleset
  hosts: all
  become: true
  gather_facts: true

  roles:
    - role: nftables
      tags: [nftables, firewall]
```

- [ ] **Step 8: Roll out and verify**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/nftables.yml
ansible-playbook ansible/playbooks/verify-nftables.yml
ansible all -m ping
```

Expected: both playbooks `failed=0`, `ping` keeps working (SSH not closed).

If `nft` doesn't start in the container (OrbStack kernel without `nf_tables`), record it in `blackwall-eye/STATUS.md`, keep the role as is, and disable the lab rollout with `nft_enabled: false` in `tests/inventory/hosts.yml` — prod hosts still need the role.

After the rollout, always re-check that the stack isn't broken:

```bash
ansible-playbook ansible/playbooks/verify-agents.yml
ansible black-master-00 -m command -a "filebeat test output" --become
```

- [ ] **Step 9: Commit**

```bash
git add blackwall-eye/ansible/roles/nftables blackwall-eye/ansible/group_vars blackwall-eye/ansible/playbooks/nftables.yml blackwall-eye/ansible/playbooks/verify-nftables.yml
git commit -m "feat(eye): nftables role + nftables/verify-nftables playbooks"
```

---

### Task 4: Verify playbooks for package versions and open ports

**Files:**
- Create: `blackwall-eye/ansible/playbooks/verify-versions.yml`
- Create: `blackwall-eye/ansible/playbooks/verify-ports.yml`

**Interfaces:**
- Consumes: groups `wazuh_managers` / `wazuh_agents`, the `nftables` role from Task 3, the `node-exporter` role from Task 1.
- Produces: two playbooks callable from `bw-check versions` and `bw-check ports` (Task 6). Both return non-zero when an assert fails.

- [ ] **Step 1: Write verify-versions.yml**

```yaml
---
- name: Verify installed package versions
  hosts: all
  become: true
  gather_facts: false

  vars:
    expected_packages:
      wazuh_managers:
        - wazuh-manager
        - wazuh-indexer
        - wazuh-dashboard
        - filebeat
        - auditd
        - nftables
      wazuh_agents:
        - wazuh-agent
        - auditd
        - nftables

  tasks:
    - name: Resolve expected package list for this host
      ansible.builtin.set_fact:
        host_packages: >-
          {{ expected_packages['wazuh_managers']
             if 'wazuh_managers' in group_names
             else expected_packages['wazuh_agents'] }}

    - name: Query installed versions
      ansible.builtin.shell: >-
        dpkg-query -W -f='${Package} ${Version} ${Status}\n' {{ item }} 2>/dev/null || true
      register: pkg_query
      changed_when: false
      loop: "{{ host_packages }}"

    - name: Report versions
      ansible.builtin.debug:
        msg: "{{ inventory_hostname }}: {{ item.stdout | default('MISSING') | trim }}"
      loop: "{{ pkg_query.results }}"
      loop_control:
        label: "{{ item.item }}"

    - name: Assert every expected package is installed
      ansible.builtin.assert:
        that: "'install ok installed' in item.stdout"
        fail_msg: "package {{ item.item }} is not installed on {{ inventory_hostname }}"
      loop: "{{ pkg_query.results }}"
      loop_control:
        label: "{{ item.item }}"
```

`prometheus-node-exporter` is deliberately not in this list: with `node_exporter_install_method: tarball` there is no package, only the binary. `verify-metrics.yml` from Task 1 checks it instead.

- [ ] **Step 2: Write verify-ports.yml**

```yaml
---
- name: Verify listening ports
  hosts: all
  become: true
  gather_facts: false

  vars:
    expected_ports:
      wazuh_managers: [22, 443, 1514, 1515, 9100, 9200, 55000]
      wazuh_agents: [22, 9100]

  tasks:
    - name: Resolve expected ports for this host
      ansible.builtin.set_fact:
        host_ports: >-
          {{ expected_ports['wazuh_managers']
             if 'wazuh_managers' in group_names
             else expected_ports['wazuh_agents'] }}

    - name: List TCP listeners
      ansible.builtin.command: ss -tlnpH
      register: listeners
      changed_when: false

    - name: Report listeners
      ansible.builtin.debug:
        var: listeners.stdout_lines

    - name: Assert each expected port has a listener
      ansible.builtin.assert:
        that: "listeners.stdout is search(':' ~ item ~ '\\s')"
        fail_msg: "nothing listens on port {{ item }} on {{ inventory_hostname }}"
      loop: "{{ host_ports }}"
```

`black-dead-98` is stopped in its normal state, so `bw-check ports` (Task 6) limits the run to live hosts with `--limit '!black-dead-98'`.

- [ ] **Step 3: Run both**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/verify-versions.yml
ansible-playbook ansible/playbooks/verify-ports.yml --limit '!black-dead-98'
```

Expected: `failed=0` on both, versions and listeners visible in the output.

- [ ] **Step 4: Commit**

```bash
git add blackwall-eye/ansible/playbooks/verify-versions.yml blackwall-eye/ansible/playbooks/verify-ports.yml
git commit -m "feat(eye): verify playbooks for package versions and open ports"
```

---

### Task 5: `tests/bin/bw-lab`

**Files:**
- Create: `tests/bin/bw-lab`
- Create: `tests/bin/_bw-common.sh`

**Interfaces:**
- Consumes: `tests/docker/docker-compose.yml`, `tests/docker/ssh_config`, playbooks from Task 1 and Task 3, the existing `blackwall-eye/ansible/playbooks/all.yml`.
- Produces: `_bw-common.sh` exports for both scripts: `BW_ROOT` (repo root), `BW_PROFILE`, `BW_COMPOSE` (path to the compose file), functions `bw_die`, `bw_info`, `bw_compose`, `bw_ansible <module> <playbook> [args...]`, `bw_require_docker`. `bw-check` (Task 6) sources the same file.

- [ ] **Step 1: Write the shared module**

`tests/bin/_bw-common.sh` — sourced, never executed directly:

```bash
# shellcheck shell=bash
# Shared variables and functions for bw-lab / bw-check.

BW_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BW_PROFILE="${BW_PROFILE:-ubuntu2204}"
BW_COMPOSE="$BW_ROOT/tests/docker/docker-compose.yml"
BW_SSH_CONFIG="$BW_ROOT/tests/docker/ssh_config"
BW_INVENTORY="$BW_ROOT/tests/inventory/hosts.yml"

if [[ -t 1 ]]; then
  BW_RED=$'\033[31m'; BW_GREEN=$'\033[32m'; BW_YELLOW=$'\033[33m'
  BW_DIM=$'\033[2m'; BW_RESET=$'\033[0m'
else
  BW_RED=''; BW_GREEN=''; BW_YELLOW=''; BW_DIM=''; BW_RESET=''
fi

bw_info() { printf '%s==>%s %s\n' "$BW_DIM" "$BW_RESET" "$*"; }
bw_die()  { printf '%serror:%s %s\n' "$BW_RED" "$BW_RESET" "$*" >&2; exit 1; }

bw_require_docker() {
  command -v docker >/dev/null 2>&1 || bw_die "docker not found in PATH"
  docker info >/dev/null 2>&1 || bw_die "docker daemon is not responding — start OrbStack/Docker Desktop"
}

bw_compose() {
  docker compose -f "$BW_COMPOSE" --profile "$BW_PROFILE" "$@"
}

# bw_ansible <module-dir> <playbook-relative-path> [extra args...]
bw_ansible() {
  local module="$1" playbook="$2"; shift 2
  ( cd "$BW_ROOT/$module" && ansible-playbook -i "$BW_INVENTORY" "$playbook" "$@" )
}
```

- [ ] **Step 2: Write bw-lab**

`tests/bin/bw-lab` with `#!/usr/bin/env bash` and `set -euo pipefail`. Subcommands and what each does:

| Command | Action |
|---|---|
| `up [--no-provision]` | `bw_compose up -d --build` → wait for SSH → provision |
| `down [--keep-data]` | `bw_compose down` (`-v` unless `--keep-data`) |
| `reset` | `down` (with `-v`) + `rm -rf tests/.wazuh-certs/wazuh-certificates*` + `up` |
| `status` | `bw_compose ps`, published ports, Prometheus/Grafana health |
| `provision <target>` | `eye` \| `nftables` \| `metrics` \| `voice` \| `all` |
| `ssh <host> [cmd...]` | `ssh -F "$BW_SSH_CONFIG" "$host" "$@"` |
| `logs <service> [-f]` | `bw_compose logs` |
| `open <target>` | `open` URL: `dashboard`→`https://localhost:5601`, `grafana`→`http://localhost:3000`, `prometheus`→`http://localhost:9090` |

Key implementations:

```bash
cmd_up() {
  local provision=1
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --no-provision) provision=0; shift ;;
      --profile) BW_PROFILE="$2"; shift 2 ;;
      *) bw_die "unknown flag: $1" ;;
    esac
  done

  bw_require_docker
  bw_info "starting containers (profile $BW_PROFILE)"
  bw_compose up -d --build

  bw_info "waiting for SSH on the systemd nodes"
  ( cd "$BW_ROOT/blackwall-eye" && ansible all -i "$BW_INVENTORY" -m wait_for_connection -a 'timeout=90' ) \
    || bw_die "containers are not reachable over SSH"

  if (( provision )); then
    cmd_provision all
  else
    bw_info "provisioning skipped (--no-provision)"
  fi
}

cmd_provision() {
  local target="${1:-all}"
  case "$target" in
    eye)      bw_ansible blackwall-eye   ansible/playbooks/all.yml ;;
    nftables) bw_ansible blackwall-eye   ansible/playbooks/nftables.yml ;;
    metrics)  bw_ansible blackwall-voice ansible/playbooks/metrics.yml ;;
    voice)    bw_ansible blackwall-voice ansible/playbooks/voice.yml ;;
    all)
      bw_ansible blackwall-eye   ansible/playbooks/all.yml
      bw_ansible blackwall-voice ansible/playbooks/metrics.yml
      bw_ansible blackwall-eye   ansible/playbooks/nftables.yml
      ;;
    *) bw_die "unknown target: $target (eye|nftables|metrics|voice|all)" ;;
  esac
}
```

Order in `all` matters: `nftables` goes **last** so the firewall covers services that are already up rather than getting in the way of their install.

`reset` wipes the certificates before `up` — otherwise the indexer comes up with the old bundle:

```bash
cmd_reset() {
  bw_require_docker
  bw_info "tearing down the lab together with volumes"
  bw_compose down -v || true
  rm -rf "$BW_ROOT/tests/.wazuh-certs/wazuh-certificates" \
         "$BW_ROOT/tests/.wazuh-certs/wazuh-certificates.tar"
  cmd_up
}
```

`status` prints both the containers and the monitoring state:

```bash
cmd_status() {
  bw_require_docker
  bw_compose ps
  printf '\n'
  bw_info "monitoring"
  local url
  for url in "http://localhost:9090/-/ready" "http://localhost:3000/api/health"; do
    if curl -sf -m 3 -u admin:blackwall "$url" >/dev/null 2>&1; then
      printf '  %sok%s   %s\n' "$BW_GREEN" "$BW_RESET" "$url"
    else
      printf '  %sdown%s %s\n' "$BW_RED" "$BW_RESET" "$url"
    fi
  done
}
```

Without arguments and with `-h`/`--help` the script prints usage with every subcommand and exits 0 for `--help`, 2 for an unknown command.

- [ ] **Step 3: Make it executable and run shellcheck**

```bash
chmod +x tests/bin/bw-lab
shellcheck tests/bin/bw-lab tests/bin/_bw-common.sh || echo "shellcheck not installed — skipping"
bash -n tests/bin/bw-lab
```

`bash -n` (syntax check) is mandatory, `shellcheck` if installed.

- [ ] **Step 4: Check against the live lab**

```bash
tests/bin/bw-lab --help
tests/bin/bw-lab status
tests/bin/bw-lab ssh black-host-01 hostname     # → black-host-01
tests/bin/bw-lab provision metrics
```

Expected: usage is printed, `status` shows the containers and `ok` for both URLs, ssh answers with the host name.

- [ ] **Step 5: Commit**

```bash
git add tests/bin/bw-lab tests/bin/_bw-common.sh
git commit -m "feat(tests): bw-lab — lab lifecycle"
```

---

### Task 6: `tests/bin/bw-check`

**Files:**
- Create: `tests/bin/bw-check`

**Interfaces:**
- Consumes: `tests/bin/_bw-common.sh` from Task 5; playbooks `verify-versions.yml`, `verify-nftables.yml`, `verify-ports.yml`, `verify-agents.yml`, `verify-api.yml` (blackwall-eye), `verify-metrics.yml`, `verify-voice.yml` (blackwall-voice).
- Produces: an executable script with subcommands `all|hosts|versions|nft|ports|wazuh|metrics|voice`; exit code 0 when nothing FAILed, 1 on any FAIL.

- [ ] **Step 1: Write the result-tracking skeleton**

```bash
#!/usr/bin/env bash
set -uo pipefail   # no -e: checks must run to the end and add up
source "$(dirname "${BASH_SOURCE[0]}")/_bw-common.sh"

BW_PASS=0; BW_FAIL=0; BW_SKIP=0

bw_pass() { BW_PASS=$((BW_PASS+1)); printf '  %sPASS%s %s\n' "$BW_GREEN"  "$BW_RESET" "$*"; }
bw_fail() { BW_FAIL=$((BW_FAIL+1)); printf '  %sFAIL%s %s\n' "$BW_RED"    "$BW_RESET" "$*"; }
bw_skip() { BW_SKIP=$((BW_SKIP+1)); printf '  %sSKIP%s %s\n' "$BW_YELLOW" "$BW_RESET" "$*"; }

# bw_run <description> <command...> — PASS on exit 0, FAIL otherwise.
# Command output is shown only on failure.
bw_run() {
  local desc="$1"; shift
  local out
  if out="$("$@" 2>&1)"; then
    bw_pass "$desc"
  else
    bw_fail "$desc"
    printf '%s\n' "$out" | sed 's/^/      /' | tail -25
  fi
}

bw_summary() {
  printf '\n%s passed, %s failed, %s skipped\n' "$BW_PASS" "$BW_FAIL" "$BW_SKIP"
  (( BW_FAIL == 0 ))
}
```

`set -e` is deliberately off: the point of `bw-check all` is to run every check and show the whole picture, not stop at the first breakage.

- [ ] **Step 2: Write the host, version, nft and port checks**

```bash
check_hosts() {
  bw_info "hosts"
  bw_require_docker
  local expected="black-master-00 black-host-01 black-metrics-00 black-grafana-00"
  local running; running="$(docker ps --format '{{.Names}}')"
  local name
  for name in $expected; do
    if grep -qx "$name" <<<"$running"; then
      bw_pass "container $name is running"
    else
      bw_fail "container $name is not running"
    fi
  done
  bw_run "ansible reaches every systemd node" \
    env ANSIBLE_CONFIG="$BW_ROOT/blackwall-eye/ansible.cfg" \
    ansible all -i "$BW_INVENTORY" --limit '!black-dead-98' -m ping
}

check_versions() {
  bw_info "package versions"
  bw_run "expected packages are installed" \
    bw_ansible blackwall-eye ansible/playbooks/verify-versions.yml --limit '!black-dead-98'
}

check_nft() {
  bw_info "nftables"
  if ! bw_ansible blackwall-eye ansible/playbooks/verify-nftables.yml \
        --limit '!black-dead-98' >/tmp/bw-nft.$$ 2>&1; then
    if grep -q 'nft: command not found\|which nft' /tmp/bw-nft.$$; then
      bw_skip "nftables unavailable in the containers — kernel without nf_tables"
    else
      bw_fail "ruleset does not match expectations"
      sed 's/^/      /' /tmp/bw-nft.$$ | tail -25
    fi
  else
    bw_pass "ruleset applied, expected ports are open"
  fi
  rm -f /tmp/bw-nft.$$
}

check_ports() {
  bw_info "open ports"
  bw_run "expected listeners inside the containers" \
    bw_ansible blackwall-eye ansible/playbooks/verify-ports.yml --limit '!black-dead-98'

  local spec
  for spec in "2200 ssh black-master-00" "2201 ssh black-host-01" \
              "5601 wazuh-dashboard" "9090 prometheus" "3000 grafana"; do
    set -- $spec
    if nc -z -G 2 localhost "$1" 2>/dev/null; then
      bw_pass "published port $1 ($2) reachable from the host"
    else
      bw_fail "published port $1 ($2) unreachable from the host"
    fi
  done
}
```

The `-G` flag of `nc` is the connect timeout in seconds, supported by the macOS `nc`. A Linux controller lacks it; there `nc -z localhost "$1"` works without the flag.

- [ ] **Step 3: Write the wazuh, metrics and voice checks**

```bash
check_wazuh() {
  bw_info "wazuh"
  bw_run "agents are registered" \
    bw_ansible blackwall-eye ansible/playbooks/verify-agents.yml
  bw_run "REST API answers" \
    bw_ansible blackwall-eye ansible/playbooks/verify-api.yml
  if curl -sk -m 5 -o /dev/null -w '%{http_code}' https://localhost:5601/ | grep -q '^30[02]$'; then
    bw_pass "dashboard answers with a redirect at https://localhost:5601"
  else
    bw_fail "dashboard does not answer at https://localhost:5601"
  fi
}

check_metrics() {
  bw_info "metrics"
  bw_run "node_exporter serves metrics on every host" \
    bw_ansible blackwall-voice ansible/playbooks/verify-metrics.yml --limit '!black-dead-98'

  local targets
  if ! targets="$(curl -sf -m 5 'http://localhost:9090/api/v1/targets?state=active')"; then
    bw_fail "prometheus does not answer at localhost:9090"
    return
  fi

  local host
  for host in black-master-00 black-host-01; do
    if jq -e --arg i "$host:9100" \
         '.data.activeTargets[] | select(.labels.instance == $i and .health == "up")' \
         >/dev/null <<<"$targets"; then
      bw_pass "prometheus scrapes $host"
    else
      bw_fail "target $host:9100 is not up"
    fi
  done

  if curl -sf -m 5 --get --data-urlencode 'query=node_load1' \
       http://localhost:9090/api/v1/query | jq -e '.data.result | length > 0' >/dev/null; then
    bw_pass "query node_load1 returns data"
  else
    bw_fail "query node_load1 is empty — metrics are not arriving"
  fi

  if curl -sf -m 5 -u admin:blackwall http://localhost:3000/api/health \
       | jq -e '.database == "ok"' >/dev/null; then
    bw_pass "grafana is healthy"
  else
    bw_fail "grafana does not answer at localhost:3000"
  fi

  if curl -sf -m 5 -u admin:blackwall http://localhost:3000/api/datasources \
       | jq -e 'map(select(.name == "Prometheus")) | length == 1' >/dev/null; then
    bw_pass "Prometheus datasource is provisioned"
  else
    bw_fail "Prometheus datasource is missing in grafana"
  fi
}

check_voice() {
  bw_info "voice"
  bw_run "telegram-alerter is running" \
    bw_ansible blackwall-voice ansible/playbooks/verify-voice.yml
}
```

Dependency: `jq`. If it is missing the script must fail clearly, so add at the start of `check_metrics`: `command -v jq >/dev/null || { bw_skip "jq not installed — prometheus/grafana checks skipped"; return; }`.

- [ ] **Step 4: Write the dispatcher and usage**

```bash
main() {
  case "${1:-all}" in
    all)      check_hosts; check_versions; check_nft; check_ports; check_wazuh; check_metrics; check_voice ;;
    hosts)    check_hosts ;;
    versions) check_versions ;;
    nft)      check_nft ;;
    ports)    check_ports ;;
    wazuh)    check_wazuh ;;
    metrics)  check_metrics ;;
    voice)    check_voice ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
  bw_summary
}

main "$@"
```

- [ ] **Step 5: Make it executable and check the syntax**

```bash
chmod +x tests/bin/bw-check
bash -n tests/bin/bw-check
shellcheck tests/bin/bw-check || echo "shellcheck not installed — skipping"
```

- [ ] **Step 6: Run against the live lab**

```bash
tests/bin/bw-check hosts
tests/bin/bw-check metrics
tests/bin/bw-check all; echo "exit=$?"
```

Expected: `bw-check all` ends with `exit=0`, no `FAIL` in the output. The only acceptable `SKIP` is `nft`, if the container kernel lacks nf_tables.

Also check the opposite case — that a failure really shows up:

```bash
docker stop black-grafana-00
tests/bin/bw-check metrics; echo "exit=$?"   # Expect: FAIL on grafana, exit=1
docker start black-grafana-00
```

- [ ] **Step 7: Commit**

```bash
git add tests/bin/bw-check
git commit -m "feat(tests): bw-check — lab checks"
```

---

### Task 7: Documentation and STATUS

**Files:**
- Modify: `tests/README.md`
- Modify: `blackwall-eye/README.md`
- Modify: `blackwall-voice/README.md`
- Modify: `README.md`
- Modify: `blackwall-eye/STATUS.md`
- Modify: `blackwall-voice/STATUS.md`

**Interfaces:**
- Consumes: the final `bw-lab` and `bw-check` subcommand sets from Task 5 and Task 6.
- Produces: `tests/README.md` as the single entry point to the lab.

- [ ] **Step 1: Restructure tests/README.md**

New structure (**keep** the existing detailed command cookbook, moving it under the heading "Manual mode — component-level debugging"):

1. `## Quick start` — `tests/bin/bw-lab up` + `tests/bin/bw-check all`
2. `## bw-lab — lifecycle` — subcommand table with examples
3. `## bw-check — checks` — subcommand table, what exactly each one checks
4. `## Topology` — update the host table: add `black-metrics-00` (172.28.0.12, port 9090) and `black-grafana-00` (172.28.0.13, port 3000)
5. `## Access` — table: dashboard `https://localhost:5601` (admin/Blackwall1!), Grafana `http://localhost:3000` (admin/blackwall), Prometheus `http://localhost:9090` (no auth)
6. `## Manual mode — component-level debugging` — the whole current section 6a
7. Add the outcome of the nftables spike to `## Known limitations` (worked / didn't work and why)
8. Update `## File layout` — add `tests/bin/`, `tests/docker/prometheus/`, `tests/docker/grafana/`

Quick start block:

```bash
# Bring everything up: containers + wazuh + metrics + firewall
tests/bin/bw-lab up

# Check that everything is alive
tests/bin/bw-check all

# Open the UIs
tests/bin/bw-lab open dashboard     # https://localhost:5601  admin / Blackwall1!
tests/bin/bw-lab open grafana       # http://localhost:3000   admin / blackwall
tests/bin/bw-lab open prometheus    # http://localhost:9090

# Tear down
tests/bin/bw-lab down
```

- [ ] **Step 2: Update the module READMEs and the root one**

- `blackwall-eye/README.md` — a section on the `nftables` role (variables `nft_public_tcp`, `nft_restricted`, `nft_enabled`; the 9200/9300 warning) and on `verify-versions.yml` / `verify-ports.yml` / `verify-nftables.yml`
- `blackwall-voice/README.md` — a section on the `node-exporter` role, the `metrics.yml` / `verify-metrics.yml` playbooks, and that Prometheus/Grafana live in the lab as compose services
- `README.md` (root) — add a line about `tests/bin/bw-lab` and `tests/bin/bw-check` as the entry point to the test environment

- [ ] **Step 3: Update STATUS.md of both modules**

In `blackwall-eye/STATUS.md`: update the frontmatter (`last_updated: 2026-08-31`, `last_task`, `next_task`), add a "Done" entry about the `nftables` role and the verify playbooks, and a follow-up in "Notes": port `hosts/example.com/nftables/*.nft` into the role (the prod config is currently outside config-as-code).

In `blackwall-voice/STATUS.md`: update the frontmatter, add an entry about `node-exporter` and Prometheus/Grafana in the lab.

- [ ] **Step 4: Check that the README commands really work**

Run every command from the quick start block and the subcommand tables verbatim. Any mismatch is a documentation bug (the "Stale docs are bugs" rule from CLAUDE.md).

- [ ] **Step 5: Commit**

```bash
git add tests/README.md blackwall-eye/README.md blackwall-voice/README.md README.md blackwall-eye/STATUS.md blackwall-voice/STATUS.md
git commit -m "docs: bw-lab/bw-check, prometheus+grafana, nftables and node-exporter roles"
```

---

## Final acceptance

On a clean machine with Docker running:

```bash
tests/bin/bw-lab reset
tests/bin/bw-check all
echo "exit=$?"
```

Expected: `exit=0`, every check `PASS` (a `SKIP` for `nft` is acceptable if the container kernel lacks nf_tables), `http://localhost:3000` shows the "Blackwall hosts" dashboard with live metrics from `black-master-00` and `black-host-01`.
