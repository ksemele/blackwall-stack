---
module: blackwall-scan
status: in_progress
stage: 1
last_updated: 2026-08-15
last_task: "Phase 1 MVP: Docker container with pd-recon pipeline (subfinder→dnsx→naabu→httpx→tlsx→nuclei), host runner, targets example.com + 2 direct IPs"
next_task: "Build image, run first scan against own infra, review findings; add time-diff storage (ClickHouse) in phase 2"
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

## Заметки / TODO

- Версии pd-тулзов пока `@latest` — запинить для воспроизводимости (config-as-code).
- naabu использует connect-scan (`-s c`) — не нужен NET_RAW cap; SYN быстрее, но
  требует `--cap-add=NET_ADMIN,NET_RAW`.
- Запуск с ноута = один vantage point (домашний ISP). Для полноты — позже
  добавить внешний VPS (Hetzner/OVH) как в плане.
- IPv6 пока не сканируется — добавить.
