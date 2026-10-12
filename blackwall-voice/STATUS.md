---
module: blackwall-voice
status: in_progress
stage: 2
last_updated: 2026-08-31
last_task: "Роль node-exporter + Prometheus/Grafana как compose-сервисы стенда; метрики трёх хостов доезжают, дашборд Blackwall hosts провижинится из репозитория"
next_task: "Настроить реальный Telegram bot token + chat_id через ansible-vault и протестировать end-to-end доставку алертов"
blockers: []
---

## Что сделано

### Stage 2 — метрики хостов

- **`ansible/roles/node-exporter/`** — ставит `prometheus-node-exporter` из apt
  (`node_exporter_install_method: apt`), с фолбэком на tarball с GitHub.
  Слушает `0.0.0.0:9100`, systemd enabled + started.
- **`ansible/playbooks/metrics.yml`** — раскатка на всю группу `all`.
- **`ansible/playbooks/verify-metrics.yml`** — юнит active + `/metrics` отдаёт
  `node_load1` и `node_memory_MemTotal_bytes`.
- **Prometheus + Grafana — compose-сервисы стенда** (`black-metrics-00`,
  `black-grafana-00`), конфиги в `tests/docker/prometheus/` и
  `tests/docker/grafana/`: scrape-конфиг, datasource и дашборд
  «Blackwall hosts» провижинятся из репозитория, ручных шагов нет.
- Проверено на живом стенде: `metrics.yml` ok=7 на трёх хостах, Prometheus
  скрейпит `black-master-00` и `black-host-01`, `node_load1` возвращает данные.

### Stage 2 — Telegram Alerter

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
- Сервис `voice-alerter` активен на `black-master-00` (Docker test env; на момент теста назывался `black-00`)
- Алертер корректно читает реальные Wazuh-алерты из `alerts.json`
- Синтетические алерты (level=10) подхватываются и логируются
- Telegram 404 ожидаем (токен в hosts.yml невалиден)

## Открытые вопросы

- `telegram_bot_token` и `telegram_chat_id`: на стенде берутся из env (`BW_TELEGRAM_*` в `.envrc`, direnv); из истории git вычищены 2026-10-12. Для прода — ansible-vault.
- State file (`sent_ids.txt`) растёт неограниченно — в будущем нужна ротация/TTL.

## Решения

- Wazuh не отдаёт Prometheus-метрики нативно — экспортер для него не делали.
  «Поступление метрик» проверяется по node_exporter, здоровье агентов — через
  Wazuh API (`tests/bin/bw-check wazuh`).
- Prometheus/Grafana живут в compose стенда, а не в ansible-ролях: это
  инфраструктура тестовой среды, а не то, что раскатывается на прод-хосты.
  node_exporter — наоборот, роль: он ставится на реальные хосты.
- `bw-check voice` даёт SKIP, если алертер не раскатан: `bw-lab provision all`
  намеренно не включает voice — ему нужен настоящий Telegram-токен.
- Дедупликация: `id` → `sent_ids.txt` (персистентно между рестартами)
- Inode rotation detection: сервис переоткрывает файл при замене (линейный файл vs log rotate)
- Логирование: Python → stdout → systemd `StandardOutput=append:` (один путь, без дублирования)
- Dashboard (OpenSearch) отложен — не входит в Stage 2
