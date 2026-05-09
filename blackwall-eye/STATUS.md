---
module: blackwall-eye
status: in_progress
stage: 1
last_updated: 2026-05-09
last_task: "Спроектировано Docker-окружение для тестирования Ansible-ролей"
next_task: "Реализовать docker-compose.yml и скрипт up/down для тестового окружения"
blockers: []
---

## Что сделано

- `docs/ARCHITECTURE.md` — полная архитектура, компоненты, detection coverage, deployment model
- Структура репозитория спроектирована (см. ARCHITECTURE.md → Repository Layout)

## Открытые вопросы

- Версия Wazuh: зафиксировать на 4.x (current stable) или конкретный minor?
- Molecule driver: docker или vagrant для тестов Ansible-ролей?

## Решения

- Target OS: Ubuntu 22.04 LTS / Debian 12 (primary), Rocky Linux 9 (secondary)
- Deployment: Ansible only, без Terraform на первом этапе
- Alerting: Telegram/Slack/webhook (реализуется в blackwall-voice, не здесь)
