---
module: blackwall-voice
status: in_progress
stage: 2
last_updated: 2026-05-09
last_task: "Создан модуль blackwall-voice: CLAUDE.md, README.md, структура директорий"
next_task: "Создать Ansible роль telegram-alerter: читает /var/ossec/logs/alerts/alerts.json, фильтрует по severity, отправляет в Telegram"
blockers: []
---

## Что сделано

- `CLAUDE.md` — полный контекст модуля: назначение, стек, layout, переменные, правила агента
- `README.md` — краткое описание для оператора: деплой одной командой, конфигурация
- `STATUS.md` — инициализирован, статус обновлён на `in_progress`

## Открытые вопросы

- `telegram_bot_token` и `telegram_chat_id` будут переданы оператором перед деплоем (или через Ansible vault)
- Источник алертов: `/var/ossec/logs/alerts/alerts.json` (tail-follow) — предпочтителен перед REST API, т.к. не требует аутентификации и работает локально на manager host

## Решения

- Файл дедупликации хранится в `/var/lib/voice-alerter/sent_ids.txt` — персистентен между рестартами сервиса
- Dashboard (OpenSearch) отложен на следующий этап — не входит в Stage 2
- Инвентарь shared с blackwall-eye: `../blackwall-eye/tests/inventory/hosts.yml`
