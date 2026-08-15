---
module: blackwall-scan
status: in_progress
stage: 1
last_updated: 2026-08-15
last_task: "Первый скан разобран; origin example.com (203.0.113.10) закрыт — см. hosts/example.com/"
next_task: "Отсеять false positives naabu (см. заметки); закрыть findings по 198.51.100.20; time-diff storage (ClickHouse) в phase 2"
blockers: []
---

## Что это

Внешний сканер собственного периметра (external attack surface self-assessment).
Смотрит на инфру снаружи глазами атакующего. Полный план и обоснование:
`docs/blackwall-skin-plan.md`.

Кодовое имя в плане было "Skin" — по решению владельца модуль называется
**blackwall-scan**.

## Что сделано

- 2026-08-15: Phase 1 MVP.
  - `Dockerfile` — multi-stage: builder (golang) собирает pd-тулзы
    (subfinder, dnsx, asnmap, naabu, httpx, tlsx, nuclei), runtime (debian-slim)
    с nmap/masscan/libpcap/jq/dnsutils.
  - `scan.sh` — пайплайн: домены→subfinder, +прямые IP → dnsx resolve →
    naabu (connect-scan, top-ports) → httpx (tech/title/status/tls) →
    tlsx (SAN/expiry/шифры) → nuclei (опционально: exposures/misconfig/takeover).
    Вывод JSONL + summary в `output/<timestamp>/`.
  - `run.sh` — хост-хелпер: build + run с монтированием targets/ и output/.
  - `targets.txt` — example.com, 203.0.113.10, 198.51.100.20.

## Известные факты по целям (первичная разведка)

- `example.com` → **Cloudflare** (104.16.0.1 / 172.64.0.1), NS *.ns.cloudflare.com.
  Скан домена бьёт по CF-edge, не по origin. Origin — вероятно прямые IP ниже.
- `203.0.113.10` — без PTR.
- `198.51.100.20` → `198-51-100-20.example.net` (hosting provider).

## Разбор первого скана (2026-08-15)

Закрыто:

- **203.0.113.10 — origin example.com в обход Cloudflare.** Подтверждено
  (httpx: 200, nginx/1.31.1, CF Origin Cert `*.example.com`). Закрыто whitelist'ом
  префиксов CF; заодно хост переведён с firewalld на nftables.
  Конфиги и разбор: `hosts/example.com/`.

Ложные срабатывания — **naabu отдал одинаковый набор портов
`53/80/443/5060/8080` для всех пяти целей**, включая CF-edge:

- httpx подтвердил на 203.0.113.10 только 443;
- firewalld на самом хосте разрешал лишь 22/9090/9999 + DNAT на 443 —
  ни 53, ни 5060, ни 8080 там не было;
- порт 80 действительно закрыт: CF отдавал `523 Origin Unreachable`.

Вывод: connect-scan через домашний NAT/ISP даёт ACK на закрытые порты.
**Доверять `ports.jsonl` без подтверждения httpx/tlsx нельзя** — это меняет
приоритет задачи «второй vantage point» с «для полноты» на «для достоверности».

Открыто:

- `198.51.100.20` (example.net): 8080 «<app>» и 443 → 502.
  По решению владельца пока не трогаем.
- Cockpit на 9090 и порт 9999 у example.com открыты всему интернету —
  вход в систему с root-правами, риск выше открытого origin.
- IP origin уже утёк в CT-логи; firewall закрывает доступ, но не факт утечки.
  Полное решение — ротация IP или Cloudflare Tunnel.

## Заметки / TODO

- **Второй vantage point (внешний VPS) — поднять в приоритете:** нужен не для
  полноты, а чтобы отсекать false positives домашнего ISP (см. разбор выше).
- Сверять `ports.jsonl` с `http.jsonl`/`tls.jsonl` перед выводами; порт без
  подтверждения httpx/tlsx считать неподтверждённым.
- Версии pd-тулзов пока `@latest` — запинить для воспроизводимости (config-as-code).
- naabu использует connect-scan (`-s c`) — не нужен NET_RAW cap; SYN быстрее, но
  требует `--cap-add=NET_ADMIN,NET_RAW`.
- Запуск с ноута = один vantage point (домашний ISP). Для полноты — позже
  добавить внешний VPS (Hetzner/OVH) как в плане.
- IPv6 пока не сканируется — добавить.
