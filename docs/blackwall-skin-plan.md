# Blackwall — модуль внешнего сканера периметра (codename: **Skin**)

> Attack surface = «кожа» системы: то, чем она соприкасается с внешним миром.
> Модуль смотрит на твою инфру **снаружи контура** глазами атакующего и сравнивает
> с тем, что должно быть открыто (inside-out ground truth).

Статус: **план / pending**. Это проектный документ, не реализация.

---

## 0. Позиционирование

Класс системы — **CESM / Continuous External Attack Surface Monitoring** для
**собственного** периметра (не internet-wide intelligence, как CVE.GLOBAL/Shodan).

Ключевые следствия для нас:
- **Легально без оговорок** — сканируем свои активы.
- **Есть ground truth изнутри**, которого нет ни у одного публичного сканера
  (Terraform state, cloud API, DNS-зоны, k8s Ingress). Главная ценность модуля —
  **дифф между "должно быть открыто" и "реально видно снаружи"**, а не сам скан.
- MVP не требует инфраструктурных вложений: asset-centric подход (домен/ASN →
  discovery → fingerprint → CVE → дифф во времени) даёт ~80% ценности.

**Тематическое имя:** `blackwall-skin` (кожа/поверхность = attack surface).
Альтернатива — `blackwall-mirror` (взгляд на себя снаружи). Финальный выбор за тобой.

---

## 1. Инструментарий (с ссылками)

### 1.1 Discovery — сбор скоупа и активов

| Тул | Назначение | Ссылка |
|---|---|---|
| **subfinder** | Пассивный перебор поддоменов (десятки API) | https://github.com/projectdiscovery/subfinder |
| **OWASP Amass** | Эталон asset discovery + граф связей | https://github.com/owasp-amass/amass |
| **asnmap** | Домен/орг → ASN → CIDR-префиксы | https://github.com/projectdiscovery/asnmap |
| **dnsx** | Быстрый DNS-резолвер, CNAME/A/AAAA, wildcard | https://github.com/projectdiscovery/dnsx |
| **massdns** | Массовый брутфорс поддоменов | https://github.com/blechschmidt/massdns |
| **uncover** | Обёртка над Shodan/Censys/FOFA/Quake API | https://github.com/projectdiscovery/uncover |
| **bgp.tools / RIPEstat** | ASN → префиксы (ручная сверка) | https://bgp.tools · https://stat.ripe.net |

### 1.2 Port / service discovery (L3-L4)

| Тул | Назначение | Ссылка |
|---|---|---|
| **naabu** | Быстрый SYN-портскан, дружит с pd-стеком | https://github.com/projectdiscovery/naabu |
| **masscan** | Stateless-скан на больших скоростях | https://github.com/robertdavidgraham/masscan |
| **ZMap** | Академический internet-wide сканер | https://github.com/zmap/zmap |
| **nmap** | Version detection, UDP, NSE-скрипты | https://github.com/nmap/nmap |

### 1.3 L7 grab / probing

| Тул | Назначение | Ссылка |
|---|---|---|
| **httpx** | HTTP-тулкит: коды, титлы, tech, редиректы | https://github.com/projectdiscovery/httpx |
| **tlsx** | TLS-граббинг: SAN, expiry, шифры, JARM | https://github.com/projectdiscovery/tlsx |
| **testssl.sh** | Глубокий аудит TLS-конфига | https://github.com/testssl/testssl.sh |
| **zgrab2** | Application-layer banner grab (пара к ZMap) | https://github.com/zmap/zgrab2 |

### 1.4 Fingerprinting

| Тул | Назначение | Ссылка |
|---|---|---|
| **wappalyzergo** | Определение технологий по HTTP | https://github.com/projectdiscovery/wappalyzergo |
| **JARM** | Активный TLS-фингерпринт сервера | https://github.com/salesforce/jarm |
| **JA4+** | Семейство сетевых фингерпринтов (JA4/JA4S/…) | https://github.com/FoxIO-LLC/ja4 |
| favicon mmh3-hash | На нём построена половина Shodan-дорков | (реализуется скриптом; см. httpx `-favicon`) |

### 1.5 Уязвимости и мисконфиги

| Тул | Назначение | Ссылка |
|---|---|---|
| **nuclei** + templates | Шаблонный сканер: exposures/, misconfig/, takeovers/, default-logins/ | https://github.com/projectdiscovery/nuclei · https://github.com/projectdiscovery/nuclei-templates |
| **EPSS** | Вероятность эксплуатации CVE (приоритизация) | https://www.first.org/epss/ |
| **CISA KEV** | Каталог реально эксплуатируемых уязвимостей | https://www.cisa.gov/known-exploited-vulnerabilities-catalog |
| **NVD / OSV** | База CVE / open-source уязвимости | https://nvd.nist.gov · https://osv.dev |

### 1.6 Certificate Transparency & takeover

| Тул | Назначение | Ссылка |
|---|---|---|
| **crt.sh** | Публичный поисковик по CT-логам (есть API) | https://crt.sh |
| **certstream** | Real-time поток новых сертификатов | https://github.com/CaliDog/certstream-server |
| **Cert Spotter** | Алерт на неавторизованные серты по твоим доменам | https://sslmate.com/certspotter/ |
| **nuclei takeover-шаблоны** | Detection dangling DNS / subdomain takeover | (в составе nuclei-templates) |

### 1.7 Оркестрация и фреймворки

| Тул | Назначение | Ссылка |
|---|---|---|
| **reNgine** | Готовый recon-фреймворк с web-UI | https://github.com/yogeshojha/rengine |
| **SpiderFoot** | OSINT-автоматизация | https://github.com/smicallef/spiderfoot |
| **OWASP Nettacker** | Автоматизированный recon/скан | https://github.com/OWASP/Nettacker |
| **Axiom** | Распределённый скан по эфемерным VPS | https://github.com/pry0cc/axiom |
| **Faraday** | Агрегатор находок / single pane of glass | https://github.com/infobyte/faraday |

### 1.8 Готовые сервисы (если не строить своё)

| Сервис | Роль | Ссылка |
|---|---|---|
| **Shodan Monitor** | Подписка на изменения по твоим IP/ASN | https://monitor.shodan.io |
| **Censys** (free tier / research) | Данные скана + история | https://search.censys.io |
| **Rapid7 Project Doppler** | Бесплатный external exposure взгляд | https://www.rapid7.com |
| **Intruder.io / Detectify** | Коммерческий EASM для периметра | https://www.intruder.io · https://detectify.com |
| **Microsoft Defender EASM** | Enterprise EASM (бывш. RiskIQ) | https://azure.microsoft.com |

### 1.9 Открытые датасеты (прототип корреляции без своего сканера)

| Источник | Заметка | Ссылка |
|---|---|---|
| **Censys bulk data / research access** | Дневные снапшоты, ~5 лет истории | https://search.censys.io/data |
| **scans.io (Stanford)** | Публичный архив internet-scan датасетов (non-commercial) | https://scans.io |
| Rapid7 Project Sonar | ⚠️ Публичный доступ **закрыт** с фев. 2022 | https://www.rapid7.com/research/project-sonar/ |

---

## 2. План модуля `blackwall-skin`

### 2.1 Основная идея — дифф двух источников

```
inside-out (ground truth)          outside-in (реальность)
что ДОЛЖНО быть открыто             что ВИДНО снаружи
─────────────────────────          ─────────────────────
• Terraform state                  • subfinder/amass → dnsx
• cloud API (SG, GCP Asset Inv.)   • naabu/masscan (все 65535)
• Route53 / Cloud DNS зоны         • httpx / tlsx
• k8s Ingress + LB Services        • nuclei (exposures/misconfig)
             \                    /
              \                  /
               ▼   DIFF ENGINE ▼
   • есть снаружи, нет в ground truth → shadow IT / забытый staging
   • dangling DNS → risk takeover
   • новый порт/поддомен со вчера → drift
   • cert expiry < 14d, сервис 200 без auth
```

Всё из правой колонки, чего нет в левой = **shadow IT, забытый стейдж, dangling
DNS, руками открытый порт**. Это drift detection для периметра.

### 2.2 Архитектура (по слоям)

1. **Scope/seed** — ASN → префиксы (asnmap/RDAP), домены → CT-логи (crt.sh/certstream)
   + passive DNS + massdns-брут. SAN из TLS-сертов **возвращаются в seed** → граф
   расширяет сам себя.
2. **L3/L4 discovery** — naabu/masscan, полные 65535 TCP + `nmap -sU --top-ports 100`.
   Обязательно **IPv4 и IPv6** (частая дыра: firewall на v4, v6 нараспашку).
3. **L7 grab** — httpx / tlsx.
4. **Fingerprinting** — wappalyzergo, favicon-hash, JARM/JA4S, nmap version.
5. **CVE-корреляция** — version→CPE → NVD/OSV, приоритизация EPSS + CISA KEV.
   ⚠️ Главный источник FP — **бэкпортированные патчи**: баннер "nginx 1.18", а
   CVE давно закрыт вендором. Не алертить по баннеру вслепую.
6. **Хранение** — observations в **ClickHouse** (`first_seen`/`last_seen`, дельты).
   Граф связей domain↔IP↔ASN↔cert можно в Postgres рёбрами (Neo4j — оверкилл на старте).
7. **Дифф + алерты** → интеграция с `blackwall-voice`.

### 2.3 Чек-лист «наш же стек» (классика утечек)

Отдельные высокоприоритетные проверки, а не общий nuclei:
- Prometheus `9090`, Alertmanager `9093` — без auth это готовая карта инфры
- Grafana `3000` (anonymous auth), ClickHouse `8123/9000`
- kubelet `10255` (read-only) и `10250`, Docker API `2375`, etcd `2379`
- Redis `6379`, node_exporter `9100`

### 2.4 Алерты, которые реально стоят того

- новый открытый порт, которого не было вчера
- новый A/CNAME или поддомен в CT-логах (быстрее любого брутфорса)
- cert expires < 14d (включая внутренние, что никто не мониторит)
- сервис отвечает 200 без auth на пути из блэклиста
- dangling CNAME → risk subdomain takeover

### 2.5 Интеграция в существующий Blackwall

```
blackwall-skin (CronJob на ВНЕШНЕМ VPS)
   → JSONL observations → ClickHouse
   → diff engine → pushgateway метрики
   → blackwall-voice (Telegram/Slack/webhook алерты)
   → (позже) blackwall-brain: LLM-объяснение "что появилось и почему опасно"
```

Модуль остаётся **независимо деплоимым** (правило изоляции): свой `ansible/`,
свой контейнер с pd-тулзами, свой `STATUS.md`.

### 2.6 Фазы

| Фаза | Что | Критерий готовности |
|---|---|---|
| **0** | Скелет модуля: `STATUS.md (pending)`, `README.md`, `ansible/`, `tools/` | Структура по шаблону eye/voice |
| **1 — MVP asset-centric** | Контейнер pd-стека (subfinder+dnsx+naabu+httpx+tlsx) + запуск с внешнего VPS, вывод JSONL | Один прогон по своему домену даёт список host:port:service |
| **2 — Хранение + дифф** | ClickHouse-схема, загрузка observations, детекция «что появилось/исчезло» | Второй прогон показывает дельту |
| **3 — Алерты** | Правила из 2.4 → pushgateway → blackwall-voice | Тестовый алерт долетает в Telegram |
| **4 — CVE + чек-лист стека** | nuclei (exposures/misconfig/takeover) + чек-лист 2.3 + EPSS/KEV приоритизация | Находки ранжированы, FP-фильтр по бэкпортам |
| **5 — CT-мониторинг** | certstream/crt.sh watcher по своим доменам | Новый серт → поддомен в seed в реальном времени |
| **6 — inside-out дифф** | Подтяжка ground truth (TF state / cloud API / DNS зоны), полный drift-дифф | «Снаружи видно X, в ground truth нет» → алерт |

### 2.7 ClickHouse — набросок схемы (фаза 2)

```sql
CREATE TABLE skin_observations (
    scan_id      UUID,
    ts           DateTime,
    host         String,      -- fqdn или ip
    ip           IPv6,        -- нормализуем v4→v6-mapped
    port         UInt16,
    proto        LowCardinality(String),   -- tcp/udp
    service      LowCardinality(String),
    product      String,
    version      String,
    tls_san      Array(String),
    tls_expiry   Nullable(DateTime),
    http_status  Nullable(UInt16),
    http_title   String,
    tech         Array(String),
    source_vp    LowCardinality(String)    -- vantage point (eu/us/asia)
) ENGINE = MergeTree ORDER BY (host, port, ts);

-- first_seen/last_seen считаются агрегатом; дифф = LEFT ANTI JOIN двух прогонов
```

---

## 3. Операционка и безопасность (не техника — тут проекты и умирают)

### 3.1 Откуда сканировать — scan-friendly хостинг

**Ключевой факт:** abuse-жалоба приходит от цели скана. Цель — ты сам. Жаловаться
некому. Вопрос сводится не к ToS цели, а к автоматике провайдера на **исходящий** трафик.

Для задачи «свой периметр, десятки-сотни хостов, раз в сутки»:

| Провайдер | Заметка |
|---|---|
| **Hetzner** CX22 (~€4) | Дёшево, стабильно |
| **OVH / SoYouStart** (~€4) | Своя сеть, высокий порог реакции, не «серый» |
| **Netcup / Scaleway Stardust** | Бюджетные альтернативы |
| DigitalOcean / Linode | Лояльны к security research (Linode — по тикету) |
| ⚠️ Vultr | «Только с явной авторизацией destination» + авто-suspend |

Правила гигиены:
- **Не сканировать из того же облака/ASN, где стоит цель** — внутренние маршруты
  и SG-исключения дадут ложно-чистую картину.
- Держать разумный rate (`masscan --rate 1000`, не 100k).
- **2-3 vantage point в разных регионах** (EU/US/Asia): Cloudflare-правила,
  geo-блоки и ASN-фильтры дают разную картину; один источник врёт.
- PTR + внятный whois на сканирующем IP → SOC/MDR не потратит день на инцидент.
- Scan-source IP → в документацию и exception-лист IDS, **но НЕ в firewall
  allow-list** (иначе теряется весь смысл).
- Для агрессивного/регулярного full-range — открыть тикет заранее:
  *"continuous external attack surface assessment of infrastructure owned by us,
  target ranges attached"*.

**НЕ надо:** offshore/DMCA-ignored хостинг с рекламой «mass scanning allowed» —
не по этике, а технически: мусорная репутация ASN, их режут Cloudflare/Spamhaus
DROP/AWS WAF, и твой же периметр половину таких префиксов геоблочит →
получаешь **ложно-чистый** скан («порт закрыт» вместо «отфильтрован на подходе»).

### 3.2 Грабли

- **Cloudflare/CDN маскирует origin**: снаружи видишь WAF, а прод торчит голым IP.
  Ищи origin через историч. DNS, SAN сертов, favicon-hash в Censys → закрывай SG
  только на префиксы Cloudflare.
- **IPv6**: сканируй оба стека.
- **Rate limit**: полный masscan по проду в пик — плохо. Обкатай на staging.
- **AWS/GCP** давно не требуют разрешения на тест своих ресурсов, но abuse-политика
  на исходящий скан из их VM есть → отсюда внешний VPS.

### 3.3 Правовая граница

- Пассивный баннер-граббинг ≠ запуск nuclei по чужим хостам (несанкц. доступ,
  ст. 272 УК РФ / CFAA).
- Сканируем **только свой скоуп** или с письменным разрешением.
- Зафиксировать письменную авторизацию даже самому себе — если инфра клиентская
  или в облаке партнёра, это единственное, что отделяет аудит от 272-й.

---

## 4. Дополнительные материалы по теме

### 4.1 Сообщества (Reddit)

- **r/netsec** — технический security-контент, разборы методологий: https://reddit.com/r/netsec
- **r/AskNetsec** — практические вопросы: https://reddit.com/r/AskNetsec
- **r/bugbounty** — recon/attack surface на практике: https://reddit.com/r/bugbounty
- **r/blueteamsec** — защита/детект: https://reddit.com/r/blueteamsec
- **r/cybersecurity** — общее: https://reddit.com/r/cybersecurity

### 4.2 Методология и практика

- **The Bug Hunter's Methodology (TBHM)**, Jason Haddix — референс по recon:
  https://github.com/jhaddix/tbhm (+ активный Discord с ежемес. коллаб-хантами)
- **NahamSec — Resources for Beginner Bug Bounty Hunters**:
  https://github.com/nahamsec/Resources-for-Beginner-Bug-Bounty-Hunters
- **0xpatrik** — глубокие статьи по recon/passive DNS/Sonar:
  https://0xpatrik.com
- **ProjectDiscovery docs** — как собрать пайплайн из их тулов:
  https://docs.projectdiscovery.io

### 4.3 Академические первоисточники

- **ZMap** paper (USENIX Security 2013) — как устроен быстрый скан:
  https://zmap.io/paper.html
- **Censys** paper (CCS 2015 / обновл. 2025) — «карта интернет-хостов»:
  https://censys.io/publications
- **EPSS** (FIRST) — модель приоритизации эксплуатации: https://www.first.org/epss/

### 4.4 Про «форумы в даркнете»

Честно: для защиты **своего** периметра туда лезть не нужно и вредно.
- Вся релевантная экспертиза (EASM, recon, fingerprinting, CVE-корреляция)
  лежит **в открытом доступе** — GitHub, Reddit, конференции, paper'ы выше.
- Даркнет-форумы по этой теме — это криминальный сегмент (доступы, эксплойты,
  «пробив»). Заходить туда = юридический риск (ст. 272/273) + opsec-риск
  (сам станешь целью) при **нулевой** пользе для defensive-задачи.
- Легальный «underground-adjacent», где реально сидят практики recon —
  это **Discord TBHM / NahamSec**, приватные bug-bounty комьюнити и
  профильные Telegram-каналы, а не Tor-форумы.

Если интересует именно *threat intelligence* (что о твоей инфре знают/продают
снаружи) — это делается через легальные CTI-сервисы и leak-мониторинг
(напр. HaveIBeenPwned, LeakIX, Intelligence X), а не через личное присутствие
на криминальных площадках.

---

## 5. Рекомендация по старту

MVP (фазы 0-1) **asset-centric**, без своей скан-инфры на весь IPv4:
`домен/ASN → subfinder+dnsx → naabu → httpx/tlsx → JSONL`, запуск с одного
Hetzner/OVH VPS раз в сутки через CronJob. Это ~80% ценности при околонулевой
инфраструктуре и без войны с abuse-департаментами. Дифф во времени (фаза 2) —
первая по-настоящему полезная фича; с неё начинается реальная защита.
