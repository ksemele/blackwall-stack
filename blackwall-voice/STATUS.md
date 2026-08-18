---
module: blackwall-voice
status: in_progress
stage: 2
last_updated: 2026-08-18
last_task: "Реализована Ansible роль telegram-alerter: alerter.py (tail-follow, dedup, inode-rotation), systemd unit, playbooks voice.yml + verify-voice.yml. Задеплоено в Docker, verify прошёл (ok=13, failed=0)"
next_task: "Настроить реальный Telegram bot token + chat_id через ansible-vault и протестировать end-to-end доставку алертов"
blockers: []
---

## Что сделано

### Stage 2 — Telegram Alerter

- **`ansible/roles/telegram-alerter/defaults/main.yml`** — все переменные с defaults:
  `severity_threshold`, `poll_interval`, `alerter_state_file`, `alerter_log_file`,
  `alerter_install_dir`, `telegram_bot_token`, `telegram_chat_id`

- **`ansible/roles/telegram-alerter/tasks/main.yml`** — задачи:
  создание директорий, деплой скрипта + unit, `daemon_reload` + enable + start

- **`ansible/roles/telegram-alerter/handlers/main.yml`** — хендлеры:
  `Reload systemd`, `Restart voice-alerter`

- **`ansible/roles/telegram-alerter/templates/alerter.py.j2`** — Python 3 скрипт:
  - Tail-follow `/var/ossec/logs/alerts/alerts.json` (seek to EOF при старте)
  - Фильтрация по `rule.level >= severity_threshold`
  - Дедупликация по `id` через state file
  - Форматирование Telegram-сообщения (hostname, level, description, timestamp)
  - Отправка через `urllib.request` (без внешних pip-зависимостей)
  - Обнаружение замены файла (inode rotation) — автоматически переоткрывает лог
  - Логирование в stdout (systemd захватывает в `alerter_log_file`)

- **`ansible/roles/telegram-alerter/templates/alerter.service.j2`** — systemd unit:
  `User=root`, `Restart=on-failure`, `RestartSec=10s`, `PYTHONUNBUFFERED=1`

- **`ansible/playbooks/voice.yml`** — деплой `telegram-alerter` на `wazuh_managers`

- **`ansible/playbooks/verify-voice.yml`** — верификация:
  - `systemctl is-active voice-alerter`
  - Инъекция синтетического алерта (level=10) в `alerts.json` через `printf >> `
  - Ожидание `poll_interval + 3` секунд
  - Проверка лога алертера (grep по synthetic ID)
  - Проверка state file (ignore_errors=true при недействительном токене)

### Результаты тестирования

- `ansible-playbook voice.yml` → `ok=9, changed=2, failed=0`
- `ansible-playbook verify-voice.yml` → `ok=13, changed=1, failed=0`
- Сервис `voice-alerter` активен на `black-00` (Docker test env)
- Алертер корректно читает реальные Wazuh-алерты из `alerts.json`
- Синтетические алерты (level=10) подхватываются и логируются
- Telegram 404 ожидаем (токен в hosts.yml невалиден)

## Открытые вопросы

- `telegram_bot_token` и `telegram_chat_id`: реальные значения **удалены** из `tests/inventory/hosts.yml` (были закоммиченными секретами). Нужен источник через ansible-vault/env для e2e-теста доставки.
- State file (`sent_ids.txt`) растёт неограниченно — в будущем нужна ротация/TTL.

## Решения

- Дедупликация: `id` → `sent_ids.txt` (персистентно между рестартами)
- Inode rotation detection: сервис переоткрывает файл при замене (линейный файл vs log rotate)
- Логирование: Python → stdout → systemd `StandardOutput=append:` (один путь, без дублирования)
- Dashboard (OpenSearch) отложен — не входит в Stage 2
