# hosts/fehu.dev — origin cloaking

Закрывает прямой доступ к origin `93.115.20.111` мимо Cloudflare: на TCP/80 и
TCP/443 пускаются только префиксы CF, остальное дропается.

Конфиги ручные (хост пока не под Ansible). Про перевод в Ansible — в конце.

## Как устроен хост

Из `nft list ruleset` на самом хосте:

| Таблица | Кто управляет | Трогаем? |
|---|---|---|
| `ip nat`, `ip filter` | netavark (podman), помечены "do not touch" | нет |
| `inet firewalld` | firewalld, зона `public` | нет |
| `inet blackwall_wall` | этот каталог | да |

Что открыто наружу:

- **22, 9090, 9999** — через firewalld (`filter_IN_public_allow`)
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

## Установка

```bash
# 1. Файлы
sudo install -D -m 0755 bin/blackwall-cf-refresh /usr/local/sbin/blackwall-cf-refresh
sudo install -D -m 0644 nftables/blackwall-wall.nft /etc/nftables.d/blackwall-wall.nft
sudo install -D -m 0644 blackwall-cf-refresh.default /etc/default/blackwall-cf-refresh
sudo install -D -m 0644 systemd/blackwall-wall.service        /etc/systemd/system/blackwall-wall.service
sudo install -D -m 0644 systemd/blackwall-cf-refresh.service  /etc/systemd/system/blackwall-cf-refresh.service
sudo install -D -m 0644 systemd/blackwall-cf-refresh.timer    /etc/systemd/system/blackwall-cf-refresh.timer

# 2. ОБЯЗАТЕЛЬНО: вписать свой IP в ADMIN_ALLOW, иначе прямой доступ к
#    origin для деплоя/отладки пропадёт вместе с чужим
sudo vi /etc/default/blackwall-cf-refresh

# 3. Проверить внешний интерфейс (в правилах зашит eth0)
ip -o -4 route show to default | awk '{print $5}'

# 4. Сухой прогон — ничего не применяет
sudo /usr/local/sbin/blackwall-cf-refresh --dry-run

# 5. Применить
sudo systemctl enable --now blackwall-wall.service
sudo systemctl enable --now blackwall-cf-refresh.timer
```

### Первое применение — со страховкой

Правила трогают только 80/443, SSH залочить нельзя. Но если ошибёшься с
`WAN_IF` или `ADMIN_ALLOW`, можно потерять сайт. Автооткат на 5 минут:

```bash
# в отдельной сессии, ДО применения
sudo sh -c 'sleep 300 && nft delete table inet blackwall_wall' &
# применяем, проверяем; если всё хорошо:
sudo kill %1
```

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

```bash
sudo systemctl disable --now blackwall-wall.service blackwall-cf-refresh.timer
sudo nft delete table inet blackwall_wall
```

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

- **Cockpit на 9090 и порт 9999 открыты всему интернету** (из
  `filter_IN_public_allow`). Cockpit — веб-панель управления хостом с
  root-доступом. По масштабу это серьёзнее, чем открытый origin: origin за
  прямым IP отдаёт тот же сайт, а 9090 — это вход в систему. Стоит закрыть
  до своих IP через firewalld:
  `firewall-cmd --permanent --zone=public --remove-port=9090/tcp`
  плюс rich-rule с `source address=`. Здесь не сделано — вне рамок задачи и
  требует твоего решения по списку адресов.
- **Второй хост 192.227.212.28** (ravencraft.dev) — по твоему решению не трогаем.
- **Ротация IP origin.** Firewall закрывает доступ, но сам IP уже утёк в
  CT-логи и historical DNS — он останется известным навсегда. Полное решение —
  сменить IP origin (или уйти на Cloudflare Tunnel, где входящих портов нет
  вообще) и с первого дня держать его за whitelist'ом.

## Про Ansible

Бэкпортить текущие таблицы в Ansible **не нужно** — там нечего версионировать:

- `ip nat` / `ip filter` пересоздаются netavark при каждом старте контейнера.
  Управлять ими из Ansible — воевать с podman на каждом рестарте.
- `inet firewalld` — это вывод firewalld, а не конфиг. Версионировать надо
  источник: `/etc/firewalld/zones/public.xml`, то есть список сервисов и
  портов зоны.

Что реально стоит завести в роли, когда дойдут руки:

| Ansible-ресурс | Чем управляет |
|---|---|
| `ansible.posix.firewalld` | порты зоны public (22/9090/9999) + ограничение по source |
| `containers.podman.podman_container` / quadlet | контейнер nginx и публикация 443 |
| files + systemd (этот каталог) | таблица `blackwall_wall` и таймер обновления CF |

Разложить это лучше в `blackwall-eye/ansible/roles/` — по правилу изоляции
модулей из корневого `CLAUDE.md`. Пока хост ручной, каталог живёт здесь.
