# Миграция firewalld → nftables на fehu.dev

Замена работающего периметра на живом хосте. Читать целиком до начала.

Исходное состояние:

```
firewalld.service   enabled, active (running)
nftables.service    disabled, inactive (dead)
```

Целевое: `nftables.service` основной, firewalld выключен и замаскирован.

## Что должно сохраниться

| Что | Откуда сейчас | Куда переезжает |
|---|---|---|
| 22 SSH отовсюду | `filter_IN_public_allow` | `blackwall_base` input |
| 9090 cockpit | `filter_IN_public_allow` | `blackwall_base` input |
| 9999 | `filter_IN_public_allow` | `blackwall_base` input |
| 443 → контейнер | DNAT netavark + `ct status dnat accept` | `blackwall_base` forward |
| контейнеры наружу | `filter_FWD_public_allow { oifname eth0 }` | `blackwall_base` forward |
| ICMP/ICMPv6 | `meta l4proto { icmp, ipv6-icmp } accept` | `blackwall_base` input |
| DHCPv6 | `ip6 daddr fe80::/64 udp dport 546` | `blackwall_base` input |
| фильтр CF на 80/443 | — (не было) | `blackwall_wall` prerouting |

Проверено на стенде (две сети, DNAT в бэкенд-контейнер):

```
22/9090/9999 -> OPEN     8080, 53 -> blocked
443 не-CF    -> blocked  443 из cf_v4 -> OPEN
контейнер -> интернет: OPEN
```

## Две мины, на которые уже наступили

**1. `ct status dnat accept` — несущее правило.** Проверено удалением: без него
443 через DNAT сразу `blocked`. Не убирать из forward.

**2. Штатный `nftables.service` делает `flush ruleset`:**

```
ExecReload=/sbin/nft 'flush ruleset; include "/etc/sysconfig/nftables.conf";'
ExecStop=/sbin/nft flush ruleset
```

`flush ruleset` сносит **все** таблицы, включая netavark'овские. На стенде это
воспроизведено: после штатного reload таблица podman исчезает и 443 становится
`blocked`, при этом SSH остаётся жив (он в нашей таблице). Поэтому ставится
drop-in `systemd/nftables.service.d/blackwall.conf`, ограничивающий службу
собственными таблицами. **Без drop-in'а миграцию не делать.**

## Порядок

Нужны две SSH-сессии: рабочая и запасная (не закрывать до конца).

```bash
# --- 1. Файлы ---
sudo install -d -m 0755 /etc/nftables.d
sudo install -m 0644 nftables/blackwall-base.nft /etc/nftables.d/blackwall-base.nft
sudo install -m 0644 nftables/blackwall-wall.nft /etc/nftables.d/blackwall-wall.nft
sudo install -m 0644 nftables/sysconfig-nftables.conf /etc/sysconfig/nftables.conf
sudo install -D -m 0644 systemd/nftables.service.d/blackwall.conf \
    /etc/systemd/system/nftables.service.d/blackwall.conf
sudo install -D -m 0755 bin/blackwall-cf-refresh /usr/local/sbin/blackwall-cf-refresh
sudo install -D -m 0644 blackwall-cf-refresh.default /etc/default/blackwall-cf-refresh
sudo install -m 0644 systemd/blackwall-cf-refresh.service /etc/systemd/system/
sudo install -m 0644 systemd/blackwall-cf-refresh.timer   /etc/systemd/system/
sudo systemctl daemon-reload

# --- 2. Сверить переменные с реальностью ---
ip -o -4 route show to default | awk '{print $5}'   # -> BASE_WAN_IF / WALL_WAN_IF
podman network inspect podman -f '{{range .Subnets}}{{.Subnet}}{{end}}'  # -> PODMAN_NET
sudo vi /etc/nftables.d/blackwall-base.nft
sudo vi /etc/default/blackwall-cf-refresh            # ADMIN_ALLOW — свой IP!

# --- 3. Синтаксис, ничего не применяя ---
sudo nft -c -f /etc/nftables.d/blackwall-base.nft
sudo nft -c -f /etc/nftables.d/blackwall-wall.nft

# --- 4. Страховка: автооткат через 10 минут ---
# systemd-run, а не `sleep &` — переживёт обрыв SSH-сессии.
sudo systemd-run --on-active=600 --unit=fw-rollback /bin/sh -c '
  nft delete table inet blackwall_base 2>/dev/null
  nft delete table inet blackwall_wall 2>/dev/null
  systemctl unmask firewalld
  systemctl start firewalld'

# --- 5. Применить ПОВЕРХ работающего firewalld ---
# Обе таблицы сосуществуют: наши правила разрешают ровно то же, что зона public.
sudo nft -f /etc/sysconfig/nftables.conf
sudo nft list tables      # ждём: ip nat, ip filter, inet firewalld, inet blackwall_base, inet blackwall_wall
```

**Стоп. Проверить из ЗАПАСНОЙ сессии — новое SSH-подключение, не существующее:**

```bash
ssh root@93.115.20.111 true && echo "SSH жив"
curl -sI https://fehu.dev | head -1          # сайт через CF
curl -sI --max-time 5 https://93.115.20.111  # прямо на origin -> должно виснуть
```

Если что-то не так — `sudo nft delete table inet blackwall_base` и разбираться.
Если всё хорошо:

```bash
# --- 6. Снять firewalld ---
sudo systemctl disable --now firewalld
sudo systemctl mask firewalld
sudo nft list tables      # inet firewalld должен исчезнуть

# --- 7. Снова проверить из запасной сессии (пункт выше) ---

# --- 8. Включить nftables как основной ---
sudo systemctl enable --now nftables
sudo systemctl enable --now blackwall-cf-refresh.timer

# --- 9. Снять страховку ---
sudo systemctl stop fw-rollback.timer 2>/dev/null
sudo systemctl reset-failed fw-rollback.service 2>/dev/null
```

## Проверка после ребута

Единственное, что по-настоящему подтверждает миграцию:

```bash
sudo reboot
# ... ждём ...
ssh root@93.115.20.111 'nft list tables; systemctl is-active nftables podman'
curl -sI https://fehu.dev | head -1
```

Ждём все пять таблиц: `ip nat`, `ip filter` (пересоздал podman),
`inet blackwall_base`, `inet blackwall_wall`.

## Откат

```bash
sudo systemctl disable --now nftables
sudo nft delete table inet blackwall_wall
sudo nft delete table inet blackwall_base
sudo systemctl unmask firewalld
sudo systemctl enable --now firewalld
sudo systemctl restart podman   # netavark перевесит свои правила
```

## Осталось на подумать

**9090 (cockpit) и 9999 открыты всему интернету.** Перенесены как есть, чтобы
миграция ничего не меняла по доступу. Но cockpit — веб-панель управления хостом
с root-доступом; по риску это больше, чем открытый origin. Ограничить своими
адресами — правка одной строки в `blackwall_base`:

```nft
tcp dport { 9090, 9999 } ip saddr @admin_v4 accept
tcp dport 22 accept
```

`admin_v4` при этом надо перенести из `blackwall_wall` в `blackwall_base` или
завести второй такой же set. SSH остаётся открытым отовсюду — вход по ключу.
