# Test-env lab tooling — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Дать два bash-скрипта (`tests/bin/bw-lab`, `tests/bin/bw-check`) и набор команд в README, которыми стенд Blackwall поднимается, разбирается и полностью проверяется — включая новые Prometheus/Grafana и nftables.

**Architecture:** Prometheus и Grafana добавляются как обычные compose-сервисы стенда с конфигами из репозитория. Метрики хостов даёт новая ansible-роль `node-exporter` (blackwall-voice), firewall — новая параметризованная ansible-роль `nftables` (blackwall-eye). Скрипты — тонкие обёртки: факты о хостах собираются ansible-плейбуками с `assert`, HTTP-эндпоинты проверяются `curl` прямо с мака.

**Tech Stack:** bash, Docker Compose, Ansible (core 2.20), nftables, prometheus-node-exporter, Prometheus, Grafana.

**Spec:** `docs/superpowers/specs/2026-08-31-test-env-lab-tooling-design.md`

## Global Constraints

- Ветка разработки — `claude-dev`. Коммитить гранулярно, супер-краткими сообщениями. **Не пушить.**
- Все пути в скриптах резолвятся от корня репозитория — скрипты работают из любой директории.
- Docker-команды в документации даются от корня репо, ansible-команды — из директории модуля (у каждого модуля свой `ansible.cfg` на `../tests/inventory/`).
- Образы Prometheus/Grafana пинятся по версии. Никаких `:latest`.
- Профиль compose по умолчанию — `ubuntu2204`, переопределяется `--profile` или `BW_PROFILE`.
- Тестовая сеть — `172.28.0.0/24`. Новые IP: Prometheus `172.28.0.12`, Grafana `172.28.0.13`.
- Секреты стенда (`admin/Blackwall1!`, `root/blackwall`, Grafana `admin/blackwall`) — только для localhost, наружу не публиковать.
- Каждая проверка в `bw-check` печатает `PASS` / `FAIL` / `SKIP` и влияет на итоговый exit code.
- Обновление README/STATUS входит в задачу, которая меняет поведение (правило «Keep docs in sync» из CLAUDE.md).

---

### Task 1: Роль `node-exporter` в blackwall-voice

Ставит `prometheus-node-exporter` на все хосты стенда, чтобы Prometheus было что скрейпить.

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
- Consumes: инвентарь `tests/inventory/hosts.yml` (группы `wazuh_managers`, `wazuh_agents`).
- Produces: node_exporter слушает `:9100` на всех хостах группы `all`; systemd-юнит называется `prometheus-node-exporter` при `apt`-установке и `node_exporter` при tarball. Роль экспортит факт `node_exporter_service_name` (set_fact) — им пользуется `verify-metrics.yml`.

- [ ] **Step 1: Написать падающий verify-плейбук**

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

- [ ] **Step 2: Прогнать и убедиться, что падает**

```bash
cd blackwall-voice
ansible-playbook ansible/playbooks/verify-metrics.yml
```

Ожидаемо: FAIL на «node_exporter unit not installed».

- [ ] **Step 3: Написать defaults роли**

`blackwall-voice/ansible/roles/node-exporter/defaults/main.yml`:

```yaml
---
# apt — пакет prometheus-node-exporter из universe (быстро, но зависит от зеркала)
# tarball — официальный релиз с GitHub (фолбэк, если пакета нет)
node_exporter_install_method: apt

node_exporter_version: "1.8.2"
node_exporter_listen_address: "0.0.0.0:9100"
node_exporter_bin_dir: /usr/local/bin
node_exporter_user: node-exporter
```

- [ ] **Step 4: Написать tasks/main.yml, install-apt.yml, install-tarball.yml**

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

- [ ] **Step 5: Написать плейбук раскатки**

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

- [ ] **Step 6: Раскатать и проверить**

```bash
cd blackwall-voice
ansible-playbook ansible/playbooks/metrics.yml
ansible-playbook ansible/playbooks/verify-metrics.yml
```

Ожидаемо: оба `failed=0`. Если `apt` не находит `prometheus-node-exporter` — перезапустить раскатку с `-e node_exporter_install_method=tarball` и зафиксировать этот дефолт в `defaults/main.yml`.

- [ ] **Step 7: Коммит**

```bash
git add blackwall-voice/ansible/roles/node-exporter blackwall-voice/ansible/playbooks/metrics.yml blackwall-voice/ansible/playbooks/verify-metrics.yml
git commit -m "feat(voice): роль node-exporter + metrics/verify-metrics плейбуки"
```

---

### Task 2: Prometheus и Grafana как compose-сервисы

**Files:**
- Modify: `tests/docker/docker-compose.yml` (добавить два сервиса в конец блока `ubuntu2204`)
- Create: `tests/docker/prometheus/prometheus.yml`
- Create: `tests/docker/grafana/provisioning/datasources/prometheus.yml`
- Create: `tests/docker/grafana/provisioning/dashboards/dashboards.yml`
- Create: `tests/docker/grafana/dashboards/blackwall-hosts.json`

**Interfaces:**
- Consumes: node_exporter на `:9100` из Task 1; DNS compose-сети (имена сервисов резолвятся внутри сети `blackwall-test`).
- Produces: Prometheus на `http://localhost:9090`, Grafana на `http://localhost:3000` (`admin` / `blackwall`), datasource с именем `Prometheus`, job `blackwall-hosts` с тремя целями.

- [ ] **Step 1: Написать prometheus.yml**

Цели задаются именами compose-сервисов — внутри сети `blackwall-test` они резолвятся через встроенный DNS, поэтому в панелях Grafana `instance` читается как `black-host-01:9100`, а не как IP.

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

`black-dead-98` штатно уходит в `down` после `tests/triggers/broken-agents.yml` — это ожидаемо. `bw-check metrics` (Task 6) требует `up` только от `black-master-00` и `black-host-01`, а по `black-dead-98` печатает информационную строку.

- [ ] **Step 2: Написать provisioning Grafana**

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

- [ ] **Step 3: Написать дашборд как код**

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

- [ ] **Step 4: Добавить сервисы в docker-compose.yml**

Вставить перед блоком «Debian 12 (future)». Сервисы намеренно не наследуют якорь `*black-node`: им не нужны privileged, systemd и host cgroup.

```yaml
  # ── Мониторинг стенда: Prometheus + Grafana ───────────────────────────────
  # Обычные контейнеры (не systemd-ноды). Конфиги монтируются из репозитория,
  # ручных шагов внутри контейнеров нет.

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

Пути томов заданы относительно `tests/docker/` (директории compose-файла). Если при `docker compose config` пути разъезжаются — заменить `../docker/` на `./`.

- [ ] **Step 5: Проверить конфиг и поднять**

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 config --quiet
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d
```

Ожидаемо: exit 0, оба контейнера в `docker ps`.

- [ ] **Step 6: Проверить эндпоинты**

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

- [ ] **Step 7: Коммит**

```bash
git add tests/docker/docker-compose.yml tests/docker/prometheus tests/docker/grafana
git commit -m "feat(tests): prometheus + grafana как compose-сервисы стенда"
```

---

### Task 3: Роль `nftables` в blackwall-eye

**Files:**
- Create: `blackwall-eye/ansible/roles/nftables/defaults/main.yml`
- Create: `blackwall-eye/ansible/roles/nftables/tasks/main.yml`
- Create: `blackwall-eye/ansible/roles/nftables/templates/nftables.conf.j2`
- Create: `blackwall-eye/ansible/roles/nftables/templates/blackwall-base.nft.j2`
- Create: `blackwall-eye/ansible/roles/nftables/handlers/main.yml`
- Create: `blackwall-eye/ansible/group_vars/wazuh_agents.yml`
- Modify: `blackwall-eye/ansible/group_vars/wazuh_managers.yml` (добавить `nft_public_tcp`)
- Create: `blackwall-eye/ansible/playbooks/nftables.yml`
- Test: `blackwall-eye/ansible/playbooks/verify-nftables.yml`

**Interfaces:**
- Consumes: группы инвентаря `wazuh_managers` / `wazuh_agents`.
- Produces: таблица `inet blackwall_base` с `policy drop` в chain `input`; переменные `nft_enabled`, `nft_wan_if`, `nft_public_tcp`, `nft_restricted`, `nft_forward_policy`, `nft_log_drops`.

**Критично — какие порты нельзя закрыть.** На `black-master-00` filebeat и dashboard ходят в indexer не через loopback, а по адресу интерфейса `172.28.0.10`, поэтому пакеты проходят цепочку `input`. Закрытие `9200`/`9300` ломает доставку алертов в indexer молча — сервисы остаются `active`, алерты просто не доезжают.

| Группа | `nft_public_tcp` | Почему |
|---|---|---|
| `wazuh_managers` | 22, 443, 1514, 1515, 9200, 9300, 9100, 55000 | ssh, dashboard, агенты, enrollment, indexer REST, indexer transport, метрики, REST API |
| `wazuh_agents` | 22, 9100 | ssh, метрики |

- [ ] **Step 1: Написать падающий verify-плейбук**

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
        fail_msg: "chain input не в policy drop — firewall не защищает хост"

    - name: Assert every expected port is present
      ansible.builtin.assert:
        that: "(item | string) in (nft_dump.stdout | regex_search('elements = \\{[^}]*\\}') | default('', true))"
        fail_msg: "порт {{ item }} отсутствует в set public_tcp"
      loop: "{{ nft_public_tcp }}"

    - name: Show ruleset
      ansible.builtin.debug:
        var: nft_dump.stdout_lines
```

- [ ] **Step 2: Прогнать и убедиться, что падает**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/verify-nftables.yml
```

Ожидаемо: FAIL (таблицы `blackwall_base` нет) либо все хосты `end_host`, если `nft` вообще не установлен.

- [ ] **Step 3: Написать defaults**

`blackwall-eye/ansible/roles/nftables/defaults/main.yml`:

```yaml
---
# Полностью выключить роль на хосте
nft_enabled: true

# Внешний интерфейс. Проверить: ip -o -4 route show to default | awk '{print $5}'
nft_wan_if: eth0

# TCP-порты, открытые отовсюду
nft_public_tcp: [22]

# Порты, доступные только с перечисленных источников:
#   nft_restricted:
#     - port: 9100
#       sources: [172.28.0.12]
nft_restricted: []

nft_forward_policy: accept
nft_log_drops: true
```

- [ ] **Step 4: Написать шаблон ruleset**

`blackwall-eye/ansible/roles/nftables/templates/blackwall-base.nft.j2`:

```
#!/usr/sbin/nft -f
#
# Managed by Ansible role blackwall-eye/nftables. Do not edit by hand.
#
# Проверить:  nft list table inet blackwall_base
# Откатить:   nft delete table inet blackwall_base

# Идиома атомарной перезагрузки: создать (если нет), удалить, определить заново.
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

        # Каркас: без этих правил хост запирает сам себя.
        iif lo accept
        ct state established,related accept
        ct state invalid drop

        # ICMP целиком: без icmpv6 ломается IPv6 (neighbor discovery, PMTU),
        # без icmp — path MTU discovery в IPv4.
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
# Роль владеет этим файлом целиком; правила лежат в /etc/nftables.d/*.nft.

flush ruleset

include "/etc/nftables.d/*.nft"
```

- [ ] **Step 5: Написать tasks и handlers**

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

- [ ] **Step 6: Задать порты по группам**

`blackwall-eye/ansible/group_vars/wazuh_agents.yml` (создать):

```yaml
---
# 22   — ssh для ansible
# 9100 — node_exporter, скрейпит black-metrics-00
nft_public_tcp: [22, 9100]
```

Дописать в `blackwall-eye/ansible/group_vars/wazuh_managers.yml`:

```yaml
# nftables: закрытие 9200/9300 молча ломает доставку алертов в indexer —
# filebeat и dashboard ходят туда по адресу интерфейса, а не через loopback.
#   22    ssh
#   443   wazuh-dashboard (публикуется как localhost:5601)
#   1514  приём событий от агентов
#   1515  authd enrollment
#   9100  node_exporter
#   9200  indexer REST
#   9300  indexer transport
#   55000 wazuh-manager REST API
nft_public_tcp: [22, 443, 1514, 1515, 9100, 9200, 9300, 55000]
```

- [ ] **Step 7: Написать плейбук раскатки**

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

- [ ] **Step 8: Раскатать и проверить**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/nftables.yml
ansible-playbook ansible/playbooks/verify-nftables.yml
ansible all -m ping
```

Ожидаемо: оба плейбука `failed=0`, `ping` продолжает работать (SSH не закрыт).

Если `nft` в контейнере не стартует (ядро OrbStack без `nf_tables`), зафиксировать это в `blackwall-eye/STATUS.md`, оставить роль как есть, а раскатку в стенде отключить через `nft_enabled: false` в `tests/inventory/hosts.yml` — роль всё равно нужна для прод-хостов.

После раскатки обязательно перепроверить, что стек не сломан:

```bash
ansible-playbook ansible/playbooks/verify-agents.yml
ansible black-master-00 -m command -a "filebeat test output" --become
```

- [ ] **Step 9: Коммит**

```bash
git add blackwall-eye/ansible/roles/nftables blackwall-eye/ansible/group_vars blackwall-eye/ansible/playbooks/nftables.yml blackwall-eye/ansible/playbooks/verify-nftables.yml
git commit -m "feat(eye): роль nftables + nftables/verify-nftables плейбуки"
```

---

### Task 4: Verify-плейбуки для версий пакетов и открытых портов

**Files:**
- Create: `blackwall-eye/ansible/playbooks/verify-versions.yml`
- Create: `blackwall-eye/ansible/playbooks/verify-ports.yml`

**Interfaces:**
- Consumes: группы `wazuh_managers` / `wazuh_agents`, роль `nftables` из Task 3, роль `node-exporter` из Task 1.
- Produces: два плейбука, пригодных для вызова из `bw-check versions` и `bw-check ports` (Task 6). Оба возвращают ненулевой код при провале assert.

- [ ] **Step 1: Написать verify-versions.yml**

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
        fail_msg: "пакет {{ item.item }} не установлен на {{ inventory_hostname }}"
      loop: "{{ pkg_query.results }}"
      loop_control:
        label: "{{ item.item }}"
```

`prometheus-node-exporter` намеренно не в этом списке: при `node_exporter_install_method: tarball` пакета нет, а бинарь есть. Его наличие проверяет `verify-metrics.yml` из Task 1.

- [ ] **Step 2: Написать verify-ports.yml**

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
        fail_msg: "на {{ inventory_hostname }} никто не слушает порт {{ item }}"
      loop: "{{ host_ports }}"
```

`black-dead-98` в штатном состоянии остановлен, поэтому `bw-check ports` (Task 6) ограничивает прогон живыми хостами через `--limit '!black-dead-98'`.

- [ ] **Step 3: Прогнать оба**

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/verify-versions.yml
ansible-playbook ansible/playbooks/verify-ports.yml --limit '!black-dead-98'
```

Ожидаемо: `failed=0` на обоих, в выводе видны версии и слушатели.

- [ ] **Step 4: Коммит**

```bash
git add blackwall-eye/ansible/playbooks/verify-versions.yml blackwall-eye/ansible/playbooks/verify-ports.yml
git commit -m "feat(eye): verify-плейбуки версий пакетов и открытых портов"
```

---

### Task 5: `tests/bin/bw-lab`

**Files:**
- Create: `tests/bin/bw-lab`
- Create: `tests/bin/_bw-common.sh`

**Interfaces:**
- Consumes: `tests/docker/docker-compose.yml`, `tests/docker/ssh_config`, плейбуки из Task 1 и Task 3, существующий `blackwall-eye/ansible/playbooks/all.yml`.
- Produces: `_bw-common.sh` экспортирует для обоих скриптов: `BW_ROOT` (корень репо), `BW_PROFILE`, `BW_COMPOSE` (путь к compose-файлу), функции `bw_die`, `bw_info`, `bw_compose`, `bw_ansible <module> <playbook> [args...]`, `bw_require_docker`. `bw-check` (Task 6) подключает тот же файл.

- [ ] **Step 1: Написать общий модуль**

`tests/bin/_bw-common.sh` — подключается через `source`, не исполняется напрямую:

```bash
# shellcheck shell=bash
# Общие переменные и функции для bw-lab / bw-check.

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
  command -v docker >/dev/null 2>&1 || bw_die "docker не найден в PATH"
  docker info >/dev/null 2>&1 || bw_die "docker-демон не отвечает — запусти OrbStack/Docker Desktop"
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

- [ ] **Step 2: Написать bw-lab**

`tests/bin/bw-lab` со `#!/usr/bin/env bash` и `set -euo pipefail`. Подкоманды и то, что каждая делает:

| Команда | Действие |
|---|---|
| `up [--no-provision]` | `bw_compose up -d --build` → ждать SSH → провижининг |
| `down [--keep-data]` | `bw_compose down` (`-v` без `--keep-data`) |
| `reset` | `down` (с `-v`) + `rm -rf tests/.wazuh-certs/wazuh-certificates*` + `up` |
| `status` | `bw_compose ps`, published-порты, health Prometheus/Grafana |
| `provision <target>` | `eye` \| `nftables` \| `metrics` \| `voice` \| `all` |
| `ssh <host> [cmd...]` | `ssh -F "$BW_SSH_CONFIG" "$host" "$@"` |
| `logs <service> [-f]` | `bw_compose logs` |
| `open <target>` | `open` URL: `dashboard`→`https://localhost:5601`, `grafana`→`http://localhost:3000`, `prometheus`→`http://localhost:9090` |

Ключевые реализации:

```bash
cmd_up() {
  local provision=1
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --no-provision) provision=0; shift ;;
      --profile) BW_PROFILE="$2"; shift 2 ;;
      *) bw_die "неизвестный флаг: $1" ;;
    esac
  done

  bw_require_docker
  bw_info "поднимаю контейнеры (профиль $BW_PROFILE)"
  bw_compose up -d --build

  bw_info "жду SSH на systemd-нодах"
  ( cd "$BW_ROOT/blackwall-eye" && ansible all -i "$BW_INVENTORY" -m wait_for_connection -a 'timeout=90' ) \
    || bw_die "контейнеры не отвечают по SSH"

  if (( provision )); then
    cmd_provision all
  else
    bw_info "провижининг пропущен (--no-provision)"
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
    *) bw_die "неизвестная цель: $target (eye|nftables|metrics|voice|all)" ;;
  esac
}
```

Порядок в `all` важен: `nftables` идёт **последним**, чтобы firewall накрывал уже поднятые сервисы, а не мешал их установке.

`reset` перед `up` сносит сертификаты — иначе indexer поднимется со старым бандлом:

```bash
cmd_reset() {
  bw_require_docker
  bw_info "сношу стенд вместе с томами"
  bw_compose down -v || true
  rm -rf "$BW_ROOT/tests/.wazuh-certs/wazuh-certificates" \
         "$BW_ROOT/tests/.wazuh-certs/wazuh-certificates.tar"
  cmd_up
}
```

`status` печатает и контейнеры, и состояние мониторинга:

```bash
cmd_status() {
  bw_require_docker
  bw_compose ps
  printf '\n'
  bw_info "мониторинг"
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

Без аргументов и по `-h`/`--help` скрипт печатает usage со всеми подкомандами и завершается кодом 0 для `--help`, кодом 2 для неизвестной команды.

- [ ] **Step 3: Сделать исполняемым и прогнать shellcheck**

```bash
chmod +x tests/bin/bw-lab
shellcheck tests/bin/bw-lab tests/bin/_bw-common.sh || echo "shellcheck отсутствует — пропускаю"
bash -n tests/bin/bw-lab
```

`bash -n` (проверка синтаксиса) обязательна, `shellcheck` — если установлен.

- [ ] **Step 4: Проверить на живом стенде**

```bash
tests/bin/bw-lab --help
tests/bin/bw-lab status
tests/bin/bw-lab ssh black-host-01 hostname     # → black-host-01
tests/bin/bw-lab provision metrics
```

Ожидаемо: usage печатается, `status` показывает контейнеры и `ok` по обоим URL, ssh отвечает именем хоста.

- [ ] **Step 5: Коммит**

```bash
git add tests/bin/bw-lab tests/bin/_bw-common.sh
git commit -m "feat(tests): bw-lab — жизненный цикл стенда"
```

---

### Task 6: `tests/bin/bw-check`

**Files:**
- Create: `tests/bin/bw-check`

**Interfaces:**
- Consumes: `tests/bin/_bw-common.sh` из Task 5; плейбуки `verify-versions.yml`, `verify-nftables.yml`, `verify-ports.yml`, `verify-agents.yml`, `verify-api.yml` (blackwall-eye), `verify-metrics.yml`, `verify-voice.yml` (blackwall-voice).
- Produces: исполняемый скрипт с подкомандами `all|hosts|versions|nft|ports|wazuh|metrics|voice`; exit code 0 при отсутствии FAIL, 1 при любом FAIL.

- [ ] **Step 1: Написать каркас учёта результатов**

```bash
#!/usr/bin/env bash
set -uo pipefail   # без -e: проверки должны идти до конца и суммироваться
source "$(dirname "${BASH_SOURCE[0]}")/_bw-common.sh"

BW_PASS=0; BW_FAIL=0; BW_SKIP=0

bw_pass() { BW_PASS=$((BW_PASS+1)); printf '  %sPASS%s %s\n' "$BW_GREEN"  "$BW_RESET" "$*"; }
bw_fail() { BW_FAIL=$((BW_FAIL+1)); printf '  %sFAIL%s %s\n' "$BW_RED"    "$BW_RESET" "$*"; }
bw_skip() { BW_SKIP=$((BW_SKIP+1)); printf '  %sSKIP%s %s\n' "$BW_YELLOW" "$BW_RESET" "$*"; }

# bw_run <описание> <команда...> — PASS при нулевом коде, FAIL иначе.
# Вывод команды показывается только при провале.
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

`set -e` намеренно не включается: смысл `bw-check all` — прогнать все проверки и показать полную картину, а не остановиться на первой поломке.

- [ ] **Step 2: Написать проверки хостов, версий, nft, портов**

```bash
check_hosts() {
  bw_info "хосты"
  bw_require_docker
  local expected="black-master-00 black-host-01 black-metrics-00 black-grafana-00"
  local running; running="$(docker ps --format '{{.Names}}')"
  local name
  for name in $expected; do
    if grep -qx "$name" <<<"$running"; then
      bw_pass "контейнер $name запущен"
    else
      bw_fail "контейнер $name не запущен"
    fi
  done
  bw_run "ansible достучался до всех systemd-нод" \
    env ANSIBLE_CONFIG="$BW_ROOT/blackwall-eye/ansible.cfg" \
    ansible all -i "$BW_INVENTORY" --limit '!black-dead-98' -m ping
}

check_versions() {
  bw_info "версии пакетов"
  bw_run "ожидаемые пакеты установлены" \
    bw_ansible blackwall-eye ansible/playbooks/verify-versions.yml --limit '!black-dead-98'
}

check_nft() {
  bw_info "nftables"
  if ! bw_ansible blackwall-eye ansible/playbooks/verify-nftables.yml \
        --limit '!black-dead-98' >/tmp/bw-nft.$$ 2>&1; then
    if grep -q 'nft: command not found\|which nft' /tmp/bw-nft.$$; then
      bw_skip "nftables недоступен в контейнерах — ядро без nf_tables"
    else
      bw_fail "ruleset не соответствует ожиданиям"
      sed 's/^/      /' /tmp/bw-nft.$$ | tail -25
    fi
  else
    bw_pass "ruleset применён, ожидаемые порты открыты"
  fi
  rm -f /tmp/bw-nft.$$
}

check_ports() {
  bw_info "открытые порты"
  bw_run "внутри контейнеров слушают ожидаемые порты" \
    bw_ansible blackwall-eye ansible/playbooks/verify-ports.yml --limit '!black-dead-98'

  local spec
  for spec in "2200 ssh black-master-00" "2201 ssh black-host-01" \
              "5601 wazuh-dashboard" "9090 prometheus" "3000 grafana"; do
    set -- $spec
    if nc -z -G 2 localhost "$1" 2>/dev/null; then
      bw_pass "published-порт $1 ($2) доступен с хоста"
    else
      bw_fail "published-порт $1 ($2) недоступен с хоста"
    fi
  done
}
```

Флаг `-G` у `nc` — таймаут подключения в секундах, поддерживается macOS-версией `nc`. На Linux-контроллере его нет; там `nc -z localhost "$1"` работает без флага.

- [ ] **Step 3: Написать проверки wazuh, metrics, voice**

```bash
check_wazuh() {
  bw_info "wazuh"
  bw_run "агенты зарегистрированы" \
    bw_ansible blackwall-eye ansible/playbooks/verify-agents.yml
  bw_run "REST API отвечает" \
    bw_ansible blackwall-eye ansible/playbooks/verify-api.yml
  if curl -sk -m 5 -o /dev/null -w '%{http_code}' https://localhost:5601/ | grep -q '^30[02]$'; then
    bw_pass "dashboard отвечает редиректом на https://localhost:5601"
  else
    bw_fail "dashboard не отвечает на https://localhost:5601"
  fi
}

check_metrics() {
  bw_info "метрики"
  bw_run "node_exporter отдаёт метрики на всех хостах" \
    bw_ansible blackwall-voice ansible/playbooks/verify-metrics.yml --limit '!black-dead-98'

  local targets
  if ! targets="$(curl -sf -m 5 'http://localhost:9090/api/v1/targets?state=active')"; then
    bw_fail "prometheus не отвечает на localhost:9090"
    return
  fi

  local host
  for host in black-master-00 black-host-01; do
    if jq -e --arg i "$host:9100" \
         '.data.activeTargets[] | select(.labels.instance == $i and .health == "up")' \
         >/dev/null <<<"$targets"; then
      bw_pass "prometheus скрейпит $host"
    else
      bw_fail "цель $host:9100 не в состоянии up"
    fi
  done

  if curl -sf -m 5 --get --data-urlencode 'query=node_load1' \
       http://localhost:9090/api/v1/query | jq -e '.data.result | length > 0' >/dev/null; then
    bw_pass "запрос node_load1 возвращает данные"
  else
    bw_fail "запрос node_load1 пустой — метрики не доезжают"
  fi

  if curl -sf -m 5 -u admin:blackwall http://localhost:3000/api/health \
       | jq -e '.database == "ok"' >/dev/null; then
    bw_pass "grafana здорова"
  else
    bw_fail "grafana не отвечает на localhost:3000"
  fi

  if curl -sf -m 5 -u admin:blackwall http://localhost:3000/api/datasources \
       | jq -e 'map(select(.name == "Prometheus")) | length == 1' >/dev/null; then
    bw_pass "datasource Prometheus прописан"
  else
    bw_fail "datasource Prometheus отсутствует в grafana"
  fi
}

check_voice() {
  bw_info "voice"
  bw_run "telegram-alerter работает" \
    bw_ansible blackwall-voice ansible/playbooks/verify-voice.yml
}
```

Зависимость: `jq`. Если его нет — скрипт должен упасть внятно, поэтому в начале `check_metrics` добавить `command -v jq >/dev/null || { bw_skip "jq не установлен — проверки prometheus/grafana пропущены"; return; }`.

- [ ] **Step 4: Написать диспетчер и usage**

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

- [ ] **Step 5: Сделать исполняемым и проверить синтаксис**

```bash
chmod +x tests/bin/bw-check
bash -n tests/bin/bw-check
shellcheck tests/bin/bw-check || echo "shellcheck отсутствует — пропускаю"
```

- [ ] **Step 6: Прогнать на живом стенде**

```bash
tests/bin/bw-check hosts
tests/bin/bw-check metrics
tests/bin/bw-check all; echo "exit=$?"
```

Ожидаемо: `bw-check all` заканчивается `exit=0`, в выводе нет `FAIL`. Допустимый `SKIP` — только `nft`, если ядро контейнера не поддерживает nf_tables.

Проверить и обратный случай — что провал действительно виден:

```bash
docker stop black-grafana-00
tests/bin/bw-check metrics; echo "exit=$?"   # Expect: FAIL по grafana, exit=1
docker start black-grafana-00
```

- [ ] **Step 7: Коммит**

```bash
git add tests/bin/bw-check
git commit -m "feat(tests): bw-check — проверки стенда"
```

---

### Task 7: Документация и STATUS

**Files:**
- Modify: `tests/README.md`
- Modify: `blackwall-eye/README.md`
- Modify: `blackwall-voice/README.md`
- Modify: `README.md`
- Modify: `blackwall-eye/STATUS.md`
- Modify: `blackwall-voice/STATUS.md`

**Interfaces:**
- Consumes: финальные наборы подкоманд `bw-lab` и `bw-check` из Task 5 и Task 6.
- Produces: `tests/README.md` как единая точка входа в стенд.

- [ ] **Step 1: Перестроить tests/README.md**

Новая структура (существующий подробный кукбук команд **сохранить**, переместив под заголовок «Ручной режим — отладка по компонентам»):

1. `## Быстрый старт` — `tests/bin/bw-lab up` + `tests/bin/bw-check all`
2. `## bw-lab — жизненный цикл` — таблица подкоманд с примерами
3. `## bw-check — проверки` — таблица подкоманд, что именно проверяет каждая
4. `## Топология` — обновить таблицу хостов: добавить `black-metrics-00` (172.28.0.12, порт 9090) и `black-grafana-00` (172.28.0.13, порт 3000)
5. `## Доступы` — таблица: dashboard `https://localhost:5601` (admin/Blackwall1!), Grafana `http://localhost:3000` (admin/blackwall), Prometheus `http://localhost:9090` (без auth)
6. `## Ручной режим — отладка по компонентам` — весь текущий раздел 6a целиком
7. Дописать в `## Известные ограничения` результат спайка по nftables (заработал / не заработал и почему)
8. Обновить `## File layout` — добавить `tests/bin/`, `tests/docker/prometheus/`, `tests/docker/grafana/`

Блок быстрого старта:

```bash
# Поднять всё: контейнеры + wazuh + метрики + firewall
tests/bin/bw-lab up

# Проверить, что всё живо
tests/bin/bw-check all

# Открыть интерфейсы
tests/bin/bw-lab open dashboard     # https://localhost:5601  admin / Blackwall1!
tests/bin/bw-lab open grafana       # http://localhost:3000   admin / blackwall
tests/bin/bw-lab open prometheus    # http://localhost:9090

# Разобрать
tests/bin/bw-lab down
```

- [ ] **Step 2: Обновить README модулей и корневой**

- `blackwall-eye/README.md` — секция про роль `nftables` (переменные `nft_public_tcp`, `nft_restricted`, `nft_enabled`; предупреждение про 9200/9300) и про `verify-versions.yml` / `verify-ports.yml` / `verify-nftables.yml`
- `blackwall-voice/README.md` — секция про роль `node-exporter`, плейбуки `metrics.yml` / `verify-metrics.yml`, и что Prometheus/Grafana живут в стенде как compose-сервисы
- `README.md` (корневой) — в описание репозитория добавить строку про `tests/bin/bw-lab` и `tests/bin/bw-check` как точку входа в тестовый стенд

- [ ] **Step 3: Обновить STATUS.md обоих модулей**

В `blackwall-eye/STATUS.md`: обновить frontmatter (`last_updated: 2026-08-31`, `last_task`, `next_task`), добавить запись в «Что сделано» про роль `nftables` и verify-плейбуки, и follow-up в «Заметки»: портировать `hosts/example.com/nftables/*.nft` в роль (сейчас прод-конфиг вне config-as-code).

В `blackwall-voice/STATUS.md`: обновить frontmatter, добавить запись про `node-exporter` и Prometheus/Grafana в стенде.

- [ ] **Step 4: Проверить, что команды из README реально работают**

Выполнить дословно каждую команду из блока быстрого старта и из таблиц подкоманд. Любое расхождение — баг в документации (правило «Stale docs are bugs» из CLAUDE.md).

- [ ] **Step 5: Коммит**

```bash
git add tests/README.md blackwall-eye/README.md blackwall-voice/README.md README.md blackwall-eye/STATUS.md blackwall-voice/STATUS.md
git commit -m "docs: bw-lab/bw-check, prometheus+grafana, роли nftables и node-exporter"
```

---

## Итоговая приёмка

На чистой машине с запущенным Docker:

```bash
tests/bin/bw-lab reset
tests/bin/bw-check all
echo "exit=$?"
```

Ожидаемо: `exit=0`, все проверки `PASS` (допустим `SKIP` по `nft`, если ядро контейнера не поддерживает nf_tables), `http://localhost:3000` показывает дашборд «Blackwall hosts» с живыми метриками `black-master-00` и `black-host-01`.
