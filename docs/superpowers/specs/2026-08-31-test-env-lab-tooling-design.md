# Test-env lab tooling — design

- **Дата:** 2026-08-31
- **Статус:** approved
- **Модули:** `tests/`, `blackwall-eye`, `blackwall-voice`

## Задача

Дать воспроизводимый способ поднять/разобрать тестовый стенд Blackwall и
проверить его состояние одной командой, а не пачкой ручных `ansible`/`docker`
вызовов из `tests/README.md`.

Три сценария, которые должны закрываться:

1. Запустить и разобрать тестовые контейнеры.
2. Подключиться к ним и проверить: доступность хостов, версии пакетов,
   применённые nftables, открытые порты.
3. Поднять локальные Wazuh + Prometheus + Grafana и убедиться, что агенты
   зарегистрированы, а метрики поступают.

## Решения (зафиксированы владельцем 2026-08-31)

| Вопрос | Решение |
|---|---|
| Форма «двух бинарей» | bash-скрипты в `tests/bin/`, дергают ansible-плейбуки |
| Prometheus / Grafana | обычные compose-сервисы стенда, не ansible-роли |
| nftables | добавить параметризованную ansible-роль |
| Существующий `tools/blackwall` (TUI) | не трогаем |

## Топология после изменений

Профиль `ubuntu2204`, сеть `172.28.0.0/24`:

| Контейнер | Роль | IP | Порт на хосте |
|---|---|---|---|
| `black-master-00` | wazuh manager + indexer + dashboard | 172.28.0.10 | 2200 (ssh), 5601 (dashboard) |
| `black-host-01` | wazuh agent + auditd | 172.28.0.11 | 2201 (ssh) |
| `black-dead-98` | фикстура disconnected | 172.28.0.98 | 2202 (ssh) |
| `black-metrics-00` | **новый** — Prometheus | 172.28.0.12 | 9090 |
| `black-grafana-00` | **новый** — Grafana | 172.28.0.13 | 3000 |

`black-metrics-00` и `black-grafana-00` — обычные контейнеры (не privileged,
без systemd), в тот же профиль `ubuntu2204`, чтобы `--profile` оставался единой
ручкой выбора стенда.

## Компоненты

### 1. Compose-сервисы Prometheus + Grafana

Файлы конфигурации лежат в репозитории и монтируются read-only —
никаких ручных шагов внутри контейнеров.

```
tests/docker/
├── prometheus/
│   └── prometheus.yml
└── grafana/
    └── provisioning/
        ├── datasources/prometheus.yml
        └── dashboards/
            ├── dashboards.yml            # provider
            └── blackwall-hosts.json      # дашборд как код
```

`prometheus.yml` скрейпит:

- job `blackwall-hosts` — `172.28.0.10:9100`, `172.28.0.11:9100`, `172.28.0.98:9100`
  (node_exporter), с `labels: {env: test}`
- job `prometheus` — сам себя (`localhost:9090`), чтобы `bw-check metrics`
  всегда имел хотя бы одну заведомо живую цель

Grafana: анонимный доступ выключен, `admin / blackwall`, datasource Prometheus
(`http://black-metrics-00:9090`) прописан provisioning'ом.

Образы пинятся по версии (config-as-code, воспроизводимость):
`prom/prometheus:v2.55.1`, `grafana/grafana:11.4.0` — точные теги
подтверждаются при реализации, если тег недоступен, берётся ближайший
стабильный и фиксируется в этом файле.

Wazuh **не** отдаёт Prometheus-метрики нативно, экспортер для него в scope не
входит. «Поступление метрик» проверяется по node_exporter. Регистрация агентов
проверяется отдельно, через Wazuh API / `agent_control`.

### 2. Ansible-роль `node-exporter` (blackwall-voice)

`blackwall-voice/ansible/roles/node-exporter/`

- ставит `prometheus-node-exporter` из apt; если пакет недоступен на выбранном
  зеркале — фолбэк на tarball с GitHub (`node_exporter_install_method: apt|tarball`)
- systemd unit enabled + started, слушает `:9100` на всех интерфейсах
- переменные: `node_exporter_version`, `node_exporter_listen_address`,
  `node_exporter_install_method`

Плейбуки:

- `blackwall-voice/ansible/playbooks/metrics.yml` — раскатка на всю группу `all`
- `blackwall-voice/ansible/playbooks/verify-metrics.yml` — сервис active,
  `curl localhost:9100/metrics` отдаёт `node_` метрики

Роль живёт в Voice: Voice — слой наблюдаемости и дашбордов, Eye — детект.

### 3. Ansible-роль `nftables` (blackwall-eye)

`blackwall-eye/ansible/roles/nftables/`

- ставит пакет `nftables`
- рендерит `/etc/nftables.d/blackwall-base.nft` из шаблона
- systemd drop-in подключает `/etc/nftables.d/*.nft`, сервис `nftables` enabled
- **перед применением** прогоняет `nft -c -f <файл>` (dry-run); при ошибке
  синтаксиса таск падает, ruleset не применяется

Переменные:

| Переменная | Дефолт | Смысл |
|---|---|---|
| `nft_enabled` | `true` | полностью выключить роль на хосте |
| `nft_wan_if` | `eth0` | внешний интерфейс |
| `nft_public_tcp` | `[22]` | TCP-порты, открытые отовсюду |
| `nft_restricted` | `[]` | список `{port, sources[]}` — порт только с этих подсетей |
| `nft_log_drops` | `true` | `limit rate 5/minute log prefix ...` на дропах |

Неотключаемый каркас в шаблоне (иначе стенд запирает сам себя):
`iif lo accept`, `ct state established,related accept`, `ct state invalid drop`,
ICMP/ICMPv6 целиком, `policy drop` только в `input`.

Правила стенда (в `group_vars`):

| Группа | `nft_public_tcp` |
|---|---|
| `wazuh_managers` | 22, 443, 1514, 1515, 55000, 9100 |
| `wazuh_agents` | 22, 9100 |

Плейбуки:

- `blackwall-eye/ansible/playbooks/nftables.yml`
- `blackwall-eye/ansible/playbooks/verify-nftables.yml` — `nft list ruleset`,
  assert что каждый порт из `nft_public_tcp` присутствует и что `policy drop`
  стоит на input

Прод-конфиги `hosts/example.com/nftables/*.nft` в этой работе **не трогаются** —
они специфичны для podman/DNAT/Cloudflare. Их портирование в роль — отдельная
задача, записывается как follow-up в `blackwall-eye/STATUS.md`.

**Известный риск.** nftables внутри Docker требует, чтобы ядро VM (OrbStack)
отдавало `nf_tables`, а в образе не конфликтовали iptables-legacy/nft. Это
проверяется спайком в начале реализации. Если не заводится — роль остаётся
(она нужна для прод-хостов), а `bw-check nft` в стенде выдаёт `SKIP` с
объяснением вместо `FAIL`.

### 4. `tests/bin/bw-lab` — жизненный цикл стенда

Тонкая bash-обёртка над `docker compose` и ansible-плейбуками. Все пути
резолвятся от корня репозитория, поэтому скрипт можно звать из любой директории.

```
bw-lab up [--profile <p>] [--no-provision]   compose up --build → ждать SSH → провижининг
bw-lab down [--keep-data]                    compose down (-v, если не --keep-data)
bw-lab reset                                 down -v + снос tests/.wazuh-certs + up
bw-lab status                                контейнеры, published-порты, health Prometheus/Grafana
bw-lab provision <target>                    только ansible: eye|voice|nftables|metrics|all
bw-lab ssh <host> [cmd...]                   ssh -F tests/docker/ssh_config
bw-lab logs <service> [-f]                   docker compose logs
bw-lab open <dashboard|grafana|prometheus>   открыть URL в браузере
```

`bw-lab up` последовательность:

1. `docker compose --profile <p> up -d --build`
2. `ansible all -m wait_for_connection -T 60` (только systemd-ноды)
3. `blackwall-eye`: `all.yml` (сертификаты → security-node → monitored-host)
4. `blackwall-eye`: `nftables.yml`
5. `blackwall-voice`: `metrics.yml`

`--no-provision` останавливается после шага 1 — для случая, когда провижининг
хочется прогнать вручную по кускам.

Профиль по умолчанию `ubuntu2204`; переопределяется `--profile` или
переменной окружения `BW_PROFILE`.

### 5. `tests/bin/bw-check` — проверки

```
bw-check all                 все проверки подряд
bw-check hosts               docker ps + ansible ping/wait_for_connection
bw-check versions            версии пакетов на всех хостах
bw-check nft                 применённый ruleset + assert ожидаемых портов
bw-check ports               ss -tlnp внутри контейнеров + published-порты с хоста
bw-check wazuh               регистрация агентов + REST API
bw-check metrics             Prometheus targets + query; Grafana health + datasource
bw-check voice               telegram-alerter
```

Разделение ответственности:

- **факты о хостах** (versions, nft, ports) — новые ansible-плейбуки с `assert`,
  скрипт только вызывает их и транслирует exit code
- **HTTP-эндпоинты** (Prometheus, Grafana, Wazuh dashboard) — `curl` прямо
  с хоста, ansible там не нужен

Проверяемые пакеты в `versions`: `wazuh-manager`, `wazuh-indexer`,
`wazuh-dashboard`, `wazuh-agent`, `filebeat`, `auditd`, `nftables`,
`prometheus-node-exporter`. Отсутствие пакета на хосте, где он не ожидается, —
не ошибка; ожидания задаются по группам инвентаря.

`metrics` проверяет:

- `GET /api/v1/targets` — все цели job `blackwall-hosts` в состоянии `up`
- `GET /api/v1/query?query=node_load1` — непустой `result`
- Grafana `GET /api/health` → `"database": "ok"`
- Grafana `GET /api/datasources` → есть datasource `Prometheus`

Вывод: по строке на проверку с префиксом `PASS` / `FAIL` / `SKIP`, итоговая
сводка, ненулевой exit code при любом `FAIL` — годится для CI.

### 6. Новые verify-плейбуки

| Файл | Проверяет |
|---|---|
| `blackwall-eye/ansible/playbooks/verify-versions.yml` | `dpkg-query` по списку пакетов, assert по группам |
| `blackwall-eye/ansible/playbooks/verify-nftables.yml` | `nft list ruleset`, assert портов и policy |
| `blackwall-eye/ansible/playbooks/verify-ports.yml` | `ss -tlnp`, assert ожидаемых слушателей |
| `blackwall-voice/ansible/playbooks/verify-metrics.yml` | node_exporter active + отдаёт метрики |

Существующие `verify-agents.yml`, `verify-api.yml`, `verify-auditd.yml`,
`verify-voice.yml` переиспользуются как есть.

## Документация

- `tests/README.md` — перестраивается: сверху сценарии через `bw-lab`/`bw-check`,
  существующий подробный кукбук ручных команд остаётся ниже как раздел
  «ручной режим / отладка» (он полезен при разборе поломок)
- `blackwall-eye/README.md` — роль `nftables`, новые verify-плейбуки
- `blackwall-voice/README.md` — роль `node-exporter`, Prometheus/Grafana в стенде
- корневой `README.md` — упоминание `tests/bin/`
- `blackwall-eye/STATUS.md`, `blackwall-voice/STATUS.md` — состояние и follow-up

## Явно вне scope

- Экспортер метрик Wazuh для Prometheus
- Портирование `hosts/example.com/nftables/*.nft` в новую роль
- Профили `debian12` / `centos9`
- Grafana-алерты и Alertmanager
- Юнит-тесты для bash-скриптов (bats) — проверка через реальный прогон стенда

## Критерии готовности

С нуля, на чистой машине с запущенным Docker:

```bash
tests/bin/bw-lab reset
tests/bin/bw-check all
```

должно закончиться нулевым exit code и всеми `PASS` (допустим `SKIP` для
`nft`, если ядро контейнера не поддерживает nf_tables), а
`http://localhost:3000` показывать дашборд с живыми метриками трёх хостов.
