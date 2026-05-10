---
module: blackwall-eye
status: in_progress
stage: 1
last_updated: 2026-05-10
last_task: "Полный тест верификации Stage 1: контейнеры работают, SSH ready, агенты зарегистрированы (Active), auditd полностью развёрнут, алерты генерируются (88 алертов: 52 level 9, 28 level 3, 4 level 8, 4 level 7). Тестовое событие (создание/удаление пользователя) успешно захвачено (rule 5901/5902/5903)."
next_task: "Начать blackwall-voice — Stage 2: стандартизированные алёрты (Telegram/Slack/webhook)"
blockers: []
---

## Что сделано

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
