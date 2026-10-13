# Blackwall — external perimeter scanner module (codename Skin → **blackwall-scan**)

> Attack surface = the system's "skin": the part of it that touches the outside world.
> The module looks at your infrastructure **from outside the perimeter**, through an
> attacker's eyes, and compares it with what is supposed to be open (inside-out ground truth).

Status: **phase 1 (MVP) — implemented** in [\`blackwall-scan/\`](../blackwall-scan/)… Phases 2–6 are ahead; this document stays a plan and a reference.

---

## 0. Positioning

The system class is **CESM / Continuous External Attack Surface Monitoring** for
**our own** perimeter (not internet-wide intelligence like CVE.GLOBAL/Shodan).

Key consequences for us:
- **Legal without caveats** — we scan our own assets.
- **We have inside ground truth** that no public scanner has
  (Terraform state, cloud APIs, DNS zones, k8s Ingress). The module's main value
  is **the diff between "should be open" and "actually visible from outside"**,
  not the scan itself.
- The MVP needs no infrastructure investment: an asset-centric approach
  (domain/ASN → discovery → fingerprint → CVE → diff over time) gives ~80% of the value.

**Thematic name:** the plan used `blackwall-skin` (skin/surface = attack surface).
By the owner's decision the module is named **blackwall-scan** (2026-08).

---

## 1. Tooling (with links)

### 1.1 Discovery — collecting scope and assets

| Tool | Purpose | Link |
|---|---|---|
| **subfinder** | Passive subdomain enumeration (dozens of APIs) | https://github.com/projectdiscovery/subfinder |
| **OWASP Amass** | Reference asset discovery + relationship graph | https://github.com/owasp-amass/amass |
| **asnmap** | Domain/org → ASN → CIDR prefixes | https://github.com/projectdiscovery/asnmap |
| **dnsx** | Fast DNS resolver, CNAME/A/AAAA, wildcard | https://github.com/projectdiscovery/dnsx |
| **massdns** | Mass subdomain brute force | https://github.com/blechschmidt/massdns |
| **uncover** | Wrapper over the Shodan/Censys/FOFA/Quake APIs | https://github.com/projectdiscovery/uncover |
| **bgp.tools / RIPEstat** | ASN → prefixes (manual cross-check) | https://bgp.tools · https://stat.ripe.net |

### 1.2 Port / service discovery (L3-L4)

| Tool | Purpose | Link |
|---|---|---|
| **naabu** | Fast SYN port scan, plays well with the pd stack | https://github.com/projectdiscovery/naabu |
| **masscan** | Stateless scanning at high speeds | https://github.com/robertdavidgraham/masscan |
| **ZMap** | Academic internet-wide scanner | https://github.com/zmap/zmap |
| **nmap** | Version detection, UDP, NSE scripts | https://github.com/nmap/nmap |

### 1.3 L7 grab / probing

| Tool | Purpose | Link |
|---|---|---|
| **httpx** | HTTP toolkit: status codes, titles, tech, redirects | https://github.com/projectdiscovery/httpx |
| **tlsx** | TLS grabbing: SAN, expiry, ciphers, JARM | https://github.com/projectdiscovery/tlsx |
| **testssl.sh** | Deep TLS config audit | https://github.com/testssl/testssl.sh |
| **zgrab2** | Application-layer banner grab (pairs with ZMap) | https://github.com/zmap/zgrab2 |

### 1.4 Fingerprinting

| Tool | Purpose | Link |
|---|---|---|
| **wappalyzergo** | Technology detection over HTTP | https://github.com/projectdiscovery/wappalyzergo |
| **JARM** | Active TLS server fingerprint | https://github.com/salesforce/jarm |
| **JA4+** | Family of network fingerprints (JA4/JA4S/…) | https://github.com/FoxIO-LLC/ja4 |
| favicon mmh3 hash | Half of all Shodan dorks are built on it | (scripted; see httpx `-favicon`) |

### 1.5 Vulnerabilities and misconfigurations

| Tool | Purpose | Link |
|---|---|---|
| **nuclei** + templates | Template scanner: exposures/, misconfig/, takeovers/, default-logins/ | https://github.com/projectdiscovery/nuclei · https://github.com/projectdiscovery/nuclei-templates |
| **EPSS** | Probability of CVE exploitation (prioritisation) | https://www.first.org/epss/ |
| **CISA KEV** | Catalog of actually exploited vulnerabilities | https://www.cisa.gov/known-exploited-vulnerabilities-catalog |
| **NVD / OSV** | CVE database / open-source vulnerabilities | https://nvd.nist.gov · https://osv.dev |

### 1.6 Certificate Transparency & takeover

| Tool | Purpose | Link |
|---|---|---|
| **crt.sh** | Public CT log search engine (has an API) | https://crt.sh |
| **certstream** | Real-time stream of new certificates | https://github.com/CaliDog/certstream-server |
| **Cert Spotter** | Alerts on unauthorized certs for your domains | https://sslmate.com/certspotter/ |
| **nuclei takeover templates** | Detecting dangling DNS / subdomain takeover | (part of nuclei-templates) |

### 1.7 Orchestration and frameworks

| Tool | Purpose | Link |
|---|---|---|
| **reNgine** | Ready-made recon framework with a web UI | https://github.com/yogeshojha/rengine |
| **SpiderFoot** | OSINT automation | https://github.com/smicallef/spiderfoot |
| **OWASP Nettacker** | Automated recon/scanning | https://github.com/OWASP/Nettacker |
| **Axiom** | Distributed scanning over ephemeral VPSes | https://github.com/pry0cc/axiom |
| **Faraday** | Findings aggregator / single pane of glass | https://github.com/infobyte/faraday |

### 1.8 Ready-made services (if not building our own)

| Service | Role | Link |
|---|---|---|
| **Shodan Monitor** | Subscription to changes on your IPs/ASN | https://monitor.shodan.io |
| **Censys** (free tier / research) | Scan data + history | https://search.censys.io |
| **Rapid7 Project Doppler** | Free external exposure view | https://www.rapid7.com |
| **Intruder.io / Detectify** | Commercial EASM for the perimeter | https://www.intruder.io · https://detectify.com |
| **Microsoft Defender EASM** | Enterprise EASM (formerly RiskIQ) | https://azure.microsoft.com |

### 1.9 Open datasets (prototyping correlation without our own scanner)

| Source | Note | Link |
|---|---|---|
| **Censys bulk data / research access** | Daily snapshots, ~5 years of history | https://search.censys.io/data |
| **scans.io (Stanford)** | Public archive of internet-scan datasets (non-commercial) | https://scans.io |
| Rapid7 Project Sonar | ⚠️ Public access **closed** since Feb 2022 | https://www.rapid7.com/research/project-sonar/ |

---

## 2. The `blackwall-scan` module plan

### 2.1 Core idea — diffing two sources

```
inside-out (ground truth)          outside-in (reality)
what SHOULD be open                 what is VISIBLE from outside
─────────────────────────          ─────────────────────
• Terraform state                  • subfinder/amass → dnsx
• cloud API (SG, GCP Asset Inv.)   • naabu/masscan (all 65535)
• Route53 / Cloud DNS zones        • httpx / tlsx
• k8s Ingress + LB Services        • nuclei (exposures/misconfig)
             \                    /
              \                  /
               ▼   DIFF ENGINE ▼
   • visible outside, absent in ground truth → shadow IT / forgotten staging
   • dangling DNS → takeover risk
   • new port/subdomain since yesterday → drift
   • cert expiry < 14d, service returns 200 without auth
```

Everything in the right column that is missing from the left = **shadow IT, a
forgotten staging, dangling DNS, a port opened by hand**. This is drift
detection for the perimeter.

### 2.2 Architecture (by layer)

1. **Scope/seed** — ASN → prefixes (asnmap/RDAP), domains → CT logs (crt.sh/certstream)
   + passive DNS + massdns brute force. SANs from TLS certs **feed back into the
   seed** → the graph grows by itself.
2. **L3/L4 discovery** — naabu/masscan, all 65535 TCP + `nmap -sU --top-ports 100`.
   Always **IPv4 and IPv6** (a common hole: firewall on v4, v6 wide open).
3. **L7 grab** — httpx / tlsx.
4. **Fingerprinting** — wappalyzergo, favicon hash, JARM/JA4S, nmap version.
5. **CVE correlation** — version→CPE → NVD/OSV, prioritised with EPSS + CISA KEV.
   ⚠️ The main source of FPs is **backported patches**: the banner says
   "nginx 1.18", but the vendor fixed the CVE long ago. Don't alert on banners blindly.
6. **Storage** — observations in **ClickHouse** (`first_seen`/`last_seen`, deltas).
   The domain↔IP↔ASN↔cert relationship graph can live in Postgres as edges
   (Neo4j is overkill at the start).
7. **Diff + alerts** → integration with `blackwall-voice`.

### 2.3 "Our own stack" checklist (classic leaks)

Dedicated high-priority checks rather than generic nuclei:
- Prometheus `9090`, Alertmanager `9093` — without auth they are a ready-made map of the infrastructure
- Grafana `3000` (anonymous auth), ClickHouse `8123/9000`
- kubelet `10255` (read-only) and `10250`, Docker API `2375`, etcd `2379`
- Redis `6379`, node_exporter `9100`

### 2.4 Alerts actually worth having

- a new open port that wasn't there yesterday
- a new A/CNAME or subdomain in CT logs (faster than any brute force)
- cert expires < 14d (including internal ones nobody monitors)
- a service answers 200 without auth on a blacklisted path
- dangling CNAME → subdomain takeover risk

### 2.5 Integration into the existing Blackwall

```
blackwall-scan (CronJob on an EXTERNAL VPS)
   → JSONL observations → ClickHouse
   → diff engine → pushgateway metrics
   → blackwall-voice (Telegram/Slack/webhook alerts)
   → (later) blackwall-brain: LLM explanation of "what appeared and why it is dangerous"
```

The module stays **independently deployable** (the isolation rule): its own
`ansible/`, its own container with the pd tools, its own `STATUS.md`.

### 2.6 Phases

| Phase | What | Definition of done |
|---|---|---|
| **0** | Module skeleton: `STATUS.md (pending)`, `README.md`, `ansible/`, `tools/` | Structure follows the eye/voice template |
| **1 — asset-centric MVP** | pd stack container (subfinder+dnsx+naabu+httpx+tlsx) + run from an external VPS, JSONL output | One run over our domain yields a host:port:service list |
| **2 — Storage + diff** | ClickHouse schema, loading observations, detecting "what appeared/disappeared" | The second run shows the delta |
| **3 — Alerts** | Rules from 2.4 → pushgateway → blackwall-voice | A test alert reaches Telegram |
| **4 — CVE + stack checklist** | nuclei (exposures/misconfig/takeover) + checklist 2.3 + EPSS/KEV prioritisation | Findings are ranked, backport FP filter in place |
| **5 — CT monitoring** | certstream/crt.sh watcher for our domains | New cert → subdomain into the seed in real time |
| **6 — inside-out diff** | Pulling ground truth (TF state / cloud API / DNS zones), full drift diff | "X is visible outside but not in ground truth" → alert |

### 2.7 ClickHouse — schema sketch (phase 2)

```sql
CREATE TABLE skin_observations (
    scan_id      UUID,
    ts           DateTime,
    host         String,      -- fqdn or ip
    ip           IPv6,        -- normalise v4→v6-mapped
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

-- first_seen/last_seen are computed by aggregation; diff = LEFT ANTI JOIN of two runs
```

---

## 3. Operations and safety (not technology — this is where projects die)

### 3.1 Where to scan from — scan-friendly hosting

**Key fact:** an abuse complaint comes from the scan target. The target is you.
There is nobody to complain. So the question is not the target's ToS but the
provider's automation on **outgoing** traffic.

For "our own perimeter, tens to hundreds of hosts, once a day":

| Provider | Note |
|---|---|
| **Hetzner** CX22 (~€4) | Cheap, stable |
| **OVH / SoYouStart** (~€4) | Own network, high reaction threshold, not "grey" |
| **Netcup / Scaleway Stardust** | Budget alternatives |
| DigitalOcean / Linode | Tolerant of security research (Linode — via a ticket) |
| ⚠️ Vultr | "Only with explicit authorization from the destination" + auto-suspend |

Hygiene rules:
- **Don't scan from the same cloud/ASN as the target** — internal routes and SG
  exceptions give a falsely clean picture.
- Keep a sane rate (`masscan --rate 1000`, not 100k).
- **2–3 vantage points in different regions** (EU/US/Asia): Cloudflare rules,
  geo-blocks and ASN filters give different pictures; a single source lies.
- PTR + a clear whois on the scanning IP → the SOC/MDR won't waste a day on an incident.
- Scan-source IPs → into the documentation and the IDS exception list, **but NOT
  into the firewall allow-list** (otherwise the whole point is lost).
- For aggressive/regular full-range scans — open a ticket in advance:
  *"continuous external attack surface assessment of infrastructure owned by us,
  target ranges attached"*.

**DON'T:** offshore/DMCA-ignored hosting advertising "mass scanning allowed" —
not on ethical grounds but technical ones: the ASN has a junk reputation,
Cloudflare/Spamhaus DROP/AWS WAF cut it, and your own perimeter geo-blocks half
of those prefixes → you get a **falsely clean** scan ("port closed" instead of
"filtered on the way in").

### 3.2 Pitfalls

- **Cloudflare/CDN hides the origin**: from outside you see the WAF while prod
  sticks out as a bare IP. Find the origin via historical DNS, cert SANs,
  favicon hash in Censys → restrict the SG to Cloudflare prefixes only.
- **IPv6**: scan both stacks.
- **Rate limit**: a full masscan of prod at peak is bad. Try it on staging first.
- **AWS/GCP** have long stopped requiring permission to test your own resources,
  but they do have an abuse policy for outgoing scans from their VMs → hence the external VPS.

### 3.3 The legal line

- Passive banner grabbing ≠ running nuclei against other people's hosts
  (unauthorized access, Art. 272 of the Russian Criminal Code / CFAA).
- Scan **only your own scope** or with written permission.
- Record written authorization even for yourself — if the infrastructure is a
  client's or in a partner's cloud, it is the only thing separating an audit
  from Art. 272.

---

## 4. Further reading

### 4.1 Communities (Reddit)

- **r/netsec** — technical security content, methodology write-ups: https://reddit.com/r/netsec
- **r/AskNetsec** — practical questions: https://reddit.com/r/AskNetsec
- **r/bugbounty** — recon/attack surface in practice: https://reddit.com/r/bugbounty
- **r/blueteamsec** — defence/detection: https://reddit.com/r/blueteamsec
- **r/cybersecurity** — general: https://reddit.com/r/cybersecurity

### 4.2 Methodology and practice

- **The Bug Hunter's Methodology (TBHM)**, Jason Haddix — the recon reference:
  https://github.com/jhaddix/tbhm (+ an active Discord with monthly collab hunts)
- **NahamSec — Resources for Beginner Bug Bounty Hunters**:
  https://github.com/nahamsec/Resources-for-Beginner-Bug-Bounty-Hunters
- **0xpatrik** — in-depth articles on recon/passive DNS/Sonar:
  https://0xpatrik.com
- **ProjectDiscovery docs** — how to build a pipeline from their tools:
  https://docs.projectdiscovery.io

### 4.3 Academic primary sources

- **ZMap** paper (USENIX Security 2013) — how fast scanning works:
  https://zmap.io/paper.html
- **Censys** paper (CCS 2015 / updated 2025) — "a map of internet hosts":
  https://censys.io/publications
- **EPSS** (FIRST) — exploitation prioritisation model: https://www.first.org/epss/

### 4.4 About "darknet forums"

Honestly: to defend **your own** perimeter there is no need to go there, and it is harmful.
- All the relevant expertise (EASM, recon, fingerprinting, CVE correlation) is
  **openly available** — GitHub, Reddit, conferences, the papers above.
- Darknet forums on this topic are the criminal segment (access, exploits,
  "lookups"). Going there = legal risk (Art. 272/273) + opsec risk (you become a
  target yourself) with **zero** benefit for a defensive task.
- The legal "underground-adjacent" places where recon practitioners actually
  hang out are **the TBHM / NahamSec Discords**, private bug bounty communities
  and specialised Telegram channels, not Tor forums.

If you are interested specifically in *threat intelligence* (what is known or
sold about your infrastructure outside), that is done via legal CTI services and
leak monitoring (e.g. HaveIBeenPwned, LeakIX, Intelligence X), not via personal
presence on criminal marketplaces.

---

## 5. Recommendation for getting started

An **asset-centric** MVP (phases 0–1), without our own scanning infrastructure for all of IPv4:
`domain/ASN → subfinder+dnsx → naabu → httpx/tlsx → JSONL`, run from a single
Hetzner/OVH VPS once a day via a CronJob. That is ~80% of the value with
near-zero infrastructure and no war with abuse departments. Diffing over time
(phase 2) is the first truly useful feature; real protection starts there.
