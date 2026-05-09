---
module: blackwall-eye
status: in_progress
stage: 1
last_updated: 2026-05-09
last_task: "Отлажено Docker тестовое окружение — Ansible подключается ко всем 3 хостам (black-00/01/02)"
next_task: "Создать Ansible роль wazuh-agent: установка пакетов, базовый ossec.conf, systemd unit"
blockers: []
---

## Что сделано

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

- Target OS: Ubuntu 22.04 LTS / Debian 12 (primary), Rocky Linux 9 (secondary)
- Deployment: Ansible only, без Terraform на первом этапе
- Alerting: Telegram/Slack/webhook (реализуется в blackwall-voice, не здесь)
- Docker test env: geerlingguy/docker-ubuntu2204-ansible как base image (systemd + Python pre-installed)
- Multi-distro: Compose profiles (Option A) — один compose file, дистрибутив выбирается флагом --profile
- Alpine исключён из ролей с systemd-зависимостями (Wazuh, auditd) — Alpine использует OpenRC
- Runtime: OrbStack (Docker Desktop 29.4.1 на macOS 26 не пробрасывает cgroup2 корректно)
- cgroup fix: entrypoint.sh монтирует cgroup2 перед systemd (OrbStack монтирует btrfs на /sys/fs/cgroup)
- apt mirror: ftp.uni-erlangen.de (Canonical IPs 91.189.91.x недоступны из текущей сети)
