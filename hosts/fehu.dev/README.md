# hosts/fehu.dev — origin cloaking

Закрывает прямой доступ к origin `93.115.20.111` мимо Cloudflare: на TCP/80 и
TCP/443 пускаются только префиксы CF, остальное дропается.

Конфиги ручные (хост пока не под Ansible). Про перевод в Ansible — в конце.

## Как устроен хост

Из `nft list ruleset` на самом хосте:

| Таблица | Кто управляет | Трогаем? |
|---|---|---|
| `ip nat`, `ip filter` | netavark (podman), помечены "do not touch" | нет |
| `inet firewalld` | firewalld — **снимается**, см. MIGRATION.md | заменяется |
| `inet blackwall_base` | этот каталог, замена firewalld | да |
| `inet blackwall_wall` | этот каталог, фильтр Cloudflare | да |

Что открыто наружу:

- **22** — SSH отовсюду; **9999** — zuzia, только с адресов сервиса
  (9090/cockpit убран: никто не слушал)
- **443** — мимо firewalld, через DNAT podman на контейнер `10.88.0.31:443`
- **80** — закрыт (нет ни в DNAT, ни в allow-цепочке)

## Почему hook prerouting, а не input

nginx работает в контейнере, netavark вешает DNAT в prerouting:

```
chain NETAVARK-HOSTPORT-DNAT { tcp dport 443 ... }
chain NETAVARK-DN-...        { tcp dport 443 ... dnat to 10.88.0.31:443 }
```

После DNAT адрес назначения — контейнер, пакет уходит в **forward**, а не в
input. Правила на `hook input` по нему не отработают. Проверено на стенде,
воспроизводящем эту топологию (клиент → хост с DNAT → бэкенд-контейнер):

| Конструкция | Трафик | Счётчик drop |
|---|---|---|
| `hook input priority -10` | проходит | `packets 0` — правило его не видит |
| `hook prerouting priority -150` | блокируется | `packets 3` |
| `prerouting`, источник в `cf_v4` | проходит | — |

Раскладка приоритетов в prerouting:

```
-200  conntrack       <- раньше него ct state недоступен
-150  blackwall_wall  <- мы
-100  dstnat          <- netavark DNAT
 -90  firewalld nat_PREROUTING
```

## Почему это не делается через firewalld

В `filter_FORWARD` у firewalld стоит `ct status dnat accept` — весь DNAT-трафик
проскакивает мимо зон. Поэтому портов 80/443 нет в `filter_IN_public_allow`, а
443 снаружи при этом открыт. Rich-rule в зоне `public` эту дыру не закроет.

## Состав

| Файл | Назначение |
|---|---|
| `nftables/blackwall-base.nft` | основной firewall хоста, замена firewalld |
| `nftables/blackwall-wall.nft` | фильтр Cloudflare на 80/443 (prerouting) |
| `nftables/sysconfig-nftables.conf` | что грузит `nftables.service` (`/etc/sysconfig/nftables.conf`) |
| `systemd/nftables.service.d/blackwall.conf` | drop-in: убирает `flush ruleset` из reload/stop |
| `bin/blackwall-cf-refresh` | обновление префиксов CF в nft и nginx `real_ip` |
| `systemd/blackwall-cf-refresh.{service,timer}` | еженедельный запуск обновления |
| `nginx/cf-realip.conf` | снапшот `set_real_ip_from` |

Обе таблицы грузит `nftables.service` через `include`. Отдельного юнита под
`blackwall_wall` нет — он был бы вторым владельцем той же таблицы.

## Установка

Хост сейчас под firewalld, поэтому установка = миграция. Пошаговая процедура
со страховкой и откатом: **[MIGRATION.md](MIGRATION.md)**.

Перед началом обязательно вписать свой IP в `ADMIN_ALLOW`
(`/etc/default/blackwall-cf-refresh`) — иначе прямой доступ к origin для
деплоя и отладки пропадёт вместе с чужим.

## Проверка

```bash
# счётчики должны расти на accept, а не только на drop
sudo nft list chain inet blackwall_wall prerouting

# сайт через Cloudflare — должен работать
curl -sI https://fehu.dev | head -1

# прямо на origin с чужого адреса — должно виснуть в таймаут
curl -sI --max-time 5 --resolve fehu.dev:443:93.115.20.111 https://fehu.dev

# кто ломится напрямую
sudo journalctl -k -g 'blackwall-wall' -n 50
```

`origin-direct` в логах — это уже готовый источник событий для blackwall-eye.

## Откат

Только фильтр Cloudflare (базовый firewall остаётся):

```bash
sudo nft delete table inet blackwall_wall
```

Полный откат на firewalld — в [MIGRATION.md](MIGRATION.md).

## Чего нельзя делать

- **Не убирать `ct status dnat accept` из forward** в `blackwall-base.nft`.
  Проверено: без него 443 через DNAT сразу перестаёт отвечать.
- **Не ставить `nftables.service` без drop-in'а** `nftables.service.d/blackwall.conf`.
  Штатные `ExecReload`/`ExecStop` делают `flush ruleset` и сносят таблицы
  netavark вместе с нашими — сеть контейнеров умрёт до `systemctl restart podman`.
- **Не заводить один и тот же `define` в обоих .nft-файлах.** Они включаются
  вместе, и `nft` падает с `redefinition of symbol`, откатывая всю транзакцию.
  Поэтому переменные названы `BASE_WAN_IF` и `WALL_WAN_IF`.

## real_ip

После включения whitelist'а весь трафик приходит с адресов CF. Без `real_ip` в
логах nginx, в `limit_req`/`limit_conn` и во всём, что видит `$remote_addr`,
клиентом будет Cloudflare — ломается и разбор инцидентов, и rate-limit.

nginx здесь в контейнере, поэтому `nginx/cf-realip.conf` нужно класть в
каталог, смонтированный в контейнер как `/etc/nginx/conf.d`. Путь и имя
контейнера — в `/etc/default/blackwall-cf-refresh` (`REALIP_OUT`,
`NGINX_CONTAINER`). Пока `NGINX_CONTAINER` пуст, скрипт этот шаг пропускает.

Найти монтирования:

```bash
podman ps --format '{{.Names}}\t{{.Ports}}'
podman inspect <container> --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{"\n"}}{{end}}'
```

## Что осталось незакрытым

- **Порт 9999 (zuzia) зависит от внешнего списка адресов.** Сервис публикует
  свои IP; при их смене мониторинг молча перестанет собирать данные, пока не
  обновишь set `zuzia_v4`. Автообновления, как у префиксов Cloudflare, здесь нет.
- **SSH/22 открыт отовсюду** — по твоему решению, вход только по ключу.
  Стоит убедиться, что это подкреплено конфигом: `PasswordAuthentication no`
  и `PermitRootLogin prohibit-password` в `/etc/ssh/sshd_config`.
- **Второй хост 192.227.212.28** (ravencraft.dev) — по твоему решению не трогаем.
- **Ротация IP origin.** Firewall закрывает доступ, но сам IP уже утёк в
  CT-логи и historical DNS — он останется известным навсегда. Полное решение —
  сменить IP origin (или уйти на Cloudflare Tunnel, где входящих портов нет
  вообще) и с первого дня держать его за whitelist'ом.

## Про Ansible

Бэкпортить существующие таблицы **не нужно** — там нечего версионировать:
`ip nat`/`ip filter` пересоздаёт netavark при каждом старте контейнера, а
`inet firewalld` — это вывод firewalld, а не конфиг.

После миграции вопрос упрощается: firewalld больше нет, и весь периметр
описан файлами из этого каталога. Роль сводится к их раскладке:

| Ansible-ресурс | Чем управляет |
|---|---|
| `ansible.builtin.template` | `blackwall-base.nft` (WAN_IF, PODMAN_NET, список портов — переменные) |
| `ansible.builtin.copy` + `systemd` | drop-in, таймер, `blackwall-cf-refresh` |
| `containers.podman.podman_container` / quadlet | контейнер nginx и публикация 443 |

`blackwall-wall.nft` в Ansible не нужен — его генерирует `blackwall-cf-refresh`
на хосте; версионируется шаблон и скрипт, а не результат.

Разложить это лучше в `blackwall-eye/ansible/roles/` — по правилу изоляции
модулей из корневого `CLAUDE.md`. Пока хост ручной, каталог живёт здесь.
