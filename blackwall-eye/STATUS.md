---
module: blackwall-eye
status: in_progress
stage: 1
last_updated: 2026-08-31
last_task: "Роль nftables + verify-плейбуки (versions/ports/nftables); тестовый стенд переведён на tests/bin/bw-lab и bw-check"
next_task: "Портировать hosts/fehu.dev/nftables/*.nft в роль nftables (прод-конфиг пока вне config-as-code); interactive smoke test TUI против живого стенда"
blockers: []
---

## Что сделано

- 2026-08-31: роль `nftables` + проверки стенда.
  - `ansible/roles/nftables/` — параметризованная роль: `nft_enabled`,
    `nft_wan_if`, `nft_public_tcp`, `nft_restricted`, `nft_forward_policy`,
    `nft_log_drops`. Рендерит `/etc/nftables.d/blackwall-base.nft`, владеет
    `/etc/nftables.conf`, перед применением гоняет `nft -c -f`.
  - `ansible/playbooks/nftables.yml`, `verify-nftables.yml` (assert
    `policy drop` + все ожидаемые порты), `verify-versions.yml`, `verify-ports.yml`.
  - Порты по группам живут в `tests/inventory/group_vars/`.
  - Проверено на живом стенде: `policy drop` реально фильтрует, allow-list
    открывает порт, SSH и ansible не теряются.

- 2026-05-16: переработана тестовая среда — `black-master-00` (manager + indexer + dashboard) + `black-host-01` (agent). Добавлены роли `wazuh-indexer`, `wazuh-dashboard`. Manager научен шипить алерты в indexer через filebeat. Сертификаты генерируются разово через `wazuh-certificates.yml`. Дашборд доступен на `https://localhost:5601` (admin / Blackwall1!). Агент полностью параметризован: `wazuh_manager_host`/`port`/`protocol`/`enrollment_*` — позволяет указать любого мастера через `-e` или group_vars. Spec: `docs/superpowers/specs/2026-05-15-test-env-master-host-dashboard-design.md`. Plan: `docs/superpowers/plans/2026-05-15-test-env-master-host-dashboard.md`.

- `ansible/roles/wazuh-manager/` — полная Ansible роль: defaults, tasks, handlers, templates/ossec.conf.j2
  - Устанавливает wazuh-manager из официального apt репозитория (pinned 4.14.5)
  - ossec.conf: authd (enrollment, port 1515), API (port 55000), remote listener (tcp:1514), FIM на /etc/{passwd,shadow,sudoers,ssh/sshd_config}, log collection из auth.log и syslog
  - systemd: enabled + started, handler для restart при изменении конфига
  - Active response и vulnerability detection отключены для Stage 1
- `ansible/playbooks/security-node.yml` — обновлён: подключена роль wazuh-manager для wazuh_managers группы
- `ansible/playbooks/verify-agents.yml` — плейбук для проверки регистрации агентов: `agent_control -l`, assert Active status

- `ansible/roles/wazuh-agent/` — полная Ansible роль: defaults, tasks, handlers, templates/ossec.conf.j2
  - Устанавливает wazuh-agent из официального apt репозитория packages.wazuh.com
  - ossec.conf: подключение к manager (tcp:1514), auto-enrollment, FIM на /etc/{passwd,shadow,sudoers,ssh/sshd_config}, log collection из auth.log и syslog
  - systemd: enabled + started, handler для restart при изменении конфига
- `ansible/playbooks/monitored-host.yml` — плейбук для wazuh_agents группы
- `ansible/ansible.cfg` — roles_path настроен, inventory по умолчанию
- Тест: `ansible-playbook` на black-01/02 — 7/7 tasks ok, 0 failed, сервис active на обоих хостах

- `docs/ARCHITECTURE.md` — полная архитектура, компоненты, detection coverage, deployment model
- Структура репозитория спроектирована (см. ARCHITECTURE.md → Repository Layout)
- `docs/stages/stage-1-test-env.md` — design doc для Docker test env (Options A/B/C сравнены, выбран Option A)
- Docker test env реализован в `tests/`:
  - `tests/docker/images/ubuntu2204/Dockerfile` — geerlingguy base + openssh-server
  - `tests/docker/docker-compose.yml` — multi-distro через Compose profiles
  - `tests/docker/ssh_config` — удобные SSH-алиасы
  - `tests/inventory/hosts.yml` — полная топология (manager + 2 agents)
  - `tests/inventory/hosts.single.yml` — изолированный хост для тестирования роли
  - `tests/README.md` — документация up/down и multi-distro стратегии

## Открытые вопросы

- Версия Wazuh: зафиксировать на 4.x (current stable) или конкретный minor?

## Решения

- **nftables: никакого глобального `flush ruleset`.** Роль пересоздаёт только
  свою таблицу (идиома create/delete/define). Глобальный flush внутри Docker
  сносит NAT-правила встроенного резолвера `127.0.0.11` — DNS в контейнере
  умирает, apt падает с `Temporary failure resolving ...`, при этом сам ruleset
  выглядит корректным. Восстанавливается рестартом контейнера. То же верно для
  podman-хостов (таблицы netavark) — см. `hosts/fehu.dev/`.
- **На менеджере нельзя закрывать 9200/9300.** filebeat и dashboard ходят в
  indexer по адресу интерфейса (`172.28.0.10`), а не через loopback, поэтому
  пакеты идут через цепочку `input`. При закрытии все сервисы остаются `active`,
  а алерты молча перестают доезжать.
- **`group_vars` переехал в `tests/inventory/group_vars/`.** Прежний
  `blackwall-eye/ansible/group_vars/` не загружался вообще: ansible ищет
  group_vars рядом с инвентарём или плейбуком, а этот путь не совпадал ни с
  тем, ни с другим — значения молча брались из defaults ролей.
- **auditd не ставится на менеджер** — роль идёт только из `monitored-host.yml`
  по группе `wazuh_agents`; в `verify-versions.yml` она ожидается только там.

- packages.wazuh.com доступен из контейнеров напрямую (CDN не заблокирован, в отличие от Canonical)
- Порядок задач в роли: gnupg/curl → GPG key → apt repo → install package → template → systemd (template после install, т.к. группа wazuh создаётся пакетом)
- Target OS: Ubuntu 22.04 LTS / Debian 12 (primary), Rocky Linux 9 (secondary)
- Deployment: Ansible only, без Terraform на первом этапе
- Alerting: Telegram/Slack/webhook (реализуется в blackwall-voice, не здесь)
- Docker test env: geerlingguy/docker-ubuntu2204-ansible как base image (systemd + Python pre-installed)
- Multi-distro: Compose profiles (Option A) — один compose file, дистрибутив выбирается флагом --profile
- Alpine исключён из ролей с systemd-зависимостями (Wazuh, auditd) — Alpine использует OpenRC
- Runtime: OrbStack (Docker Desktop 29.4.1 на macOS 26 не пробрасывает cgroup2 корректно)
- cgroup fix: entrypoint.sh монтирует cgroup2 перед systemd (OrbStack монтирует btrfs на /sys/fs/cgroup)
- apt mirror: ftp.uni-erlangen.de (Canonical IPs 91.189.91.x недоступны из текущей сети)
