# blackwall-scan

External attack surface scanner — смотрит на твою инфру **снаружи контура**
глазами атакующего. Модуль Blackwall (см. `../CLAUDE.md`, план: `../docs/blackwall-skin-plan.md`).

> **Только свой скоуп.** Сканируй лишь активы, которыми владеешь, или с письменным
> разрешением. Скан чужих хостов = несанкц. доступ (ст. 272 УК РФ / CFAA).

## Что делает (Phase 1)

Пайплайн в Docker-контейнере:

```
targets.txt ─┬─ domains ─> subfinder (passive subdomains)
             └─ IPs ──────┐
                          v
             dnsx (resolve) -> naabu (port scan) -> httpx (HTTP/tech/title)
                          -> tlsx (cert SAN/expiry) -> nuclei (opt: exposures)
                          -> output/<timestamp>/  (JSONL + SUMMARY.txt)
```

Инструменты: ProjectDiscovery (subfinder, dnsx, asnmap, naabu, httpx, tlsx, nuclei)
+ nmap/masscan в рантайме.

## Требования

- Docker (на macOS — OrbStack или Docker Desktop; демон должен быть запущен).

## Запуск

```bash
cd blackwall-scan
./run.sh                  # top-1000 портов, без nuclei
RUN_NUCLEI=1 ./run.sh     # + проверки exposure/misconfig/takeover
PORTS="-p -" ./run.sh     # полный скан 65535 TCP (долго)
./run.sh --build          # пересобрать образ
```

Цели — в `targets.txt` (по одной на строку, домен или IPv4).

Результаты — в `output/<timestamp>/`:
- `SUMMARY.txt` — человекочитаемая сводка
- `ports.jsonl`, `http.jsonl`, `tls.jsonl`, `dns.jsonl`, `nuclei.jsonl` — сырьё (JSONL)
- `san_discovered.txt` — поддомены из SAN сертификатов (seed для следующего прогона)

## Переменные

| Env | Default | Назначение |
|---|---|---|
| `PORTS` | `-top-ports 1000` | Спек портов naabu (`-p -` = все 65535) |
| `RUN_NUCLEI` | `0` | `1` — включить nuclei |
| `NUCLEI_TAGS` | `exposure,misconfig,takeover,default-login` | Теги nuclei |
| `RATE` | `1000` | naabu packet rate |

## Заметки

- naabu использует **connect-scan** (`-s c`) — не требует привилегий. Для SYN-скана
  (быстрее) добавь `--cap-add=NET_ADMIN,NET_RAW` в `run.sh`.
- **Один vantage point** (твой ISP). Полноты ради — позже гонять и с внешнего VPS.
- **Cloudflare/CDN**: домен за CF (напр. `example.com`) резолвится в edge — скан домена
  бьёт по Cloudflare, не по origin. Origin проверяй по прямым IP.
- IPv6 пока не сканируется (TODO).
- `output/` в `.gitignore` — результаты не коммитятся.

## Статус

См. `STATUS.md`. Phase 1 (MVP-сканер) — в работе. Дальше: хранение + дифф во
времени (ClickHouse), алерты через `blackwall-voice`, inside-out ground-truth дифф.
