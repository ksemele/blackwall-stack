# Blackwall — Architecture Overview

## What Is This

Blackwall is an open-source security monitoring toolkit for Linux servers. It combines
battle-tested open-source components into a coherent, opinionated stack with:

- Infrastructure-as-Code deployment (Ansible + optional Terraform)
- Curated detection rules for the most actionable threat scenarios
- Low-overhead host and network visibility
- Structured alerting to Telegram/Slack/webhooks

The goal is not to replace commercial SIEMs but to give any team a production-grade
security monitoring baseline in a day, with full control over every piece.

---

## Design Principles

**Signal over noise.** Start with high-confidence, high-impact detections. ML anomaly
detection comes after the baseline is stable, not before.

**Config as code.** Every component is deployed and configured through version-controlled
automation. No manual UI steps that cannot be reproduced.

**Defense in depth, not vendor lock-in.** Each layer works independently. Adding or
removing a component should not break the others.

**Minimal footprint on monitored hosts.** Agents and sensors must not become a resource
problem on the systems they protect.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        MONITORED HOSTS                          │
│                                                                 │
│  ┌──────────────┐   ┌──────────┐   ┌─────────────────────────┐ │
│  │ Wazuh Agent  │   │  auditd  │   │  systemd / syslog /     │ │
│  │ (FIM + log   │◄──│ (syscall │   │  auth.log / sudo / cron │ │
│  │  collection) │   │  audit)  │   │                         │ │
│  └──────┬───────┘   └──────────┘   └─────────────────────────┘ │
│         │                  events feed into Wazuh agent         │
└─────────┼───────────────────────────────────────────────────────┘
          │ encrypted (TLS 1.3)
          │
┌─────────┼───────────────────────────────────────────────────────┐
│         │             SECURITY NODE                             │
│  ┌──────▼──────────────────────────────────────────────────┐   │
│  │                    Wazuh Manager                        │   │
│  │  - Log aggregation & normalization                      │   │
│  │  - Rule engine (built-in + custom)                      │   │
│  │  - File Integrity Monitoring correlation                │   │
│  │  - Alert routing                                        │   │
│  └──────┬──────────────────────────────────────────────────┘   │
│         │                                                       │
│  ┌──────▼──────────────────────────────────────────────────┐   │
│  │            OpenSearch Indexer + Dashboard               │   │
│  │  - Long-term log storage                                │   │
│  │  - Anomaly Detection (ML, Stage 4)                      │   │
│  │  - Visualization / investigation                        │   │
│  └──────┬──────────────────────────────────────────────────┘   │
│         │                                                       │
│  ┌──────▼──────────────────────────────────────────────────┐   │
│  │                    Alert Router                         │   │
│  │          Telegram / Slack / Email / Webhook             │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    NETWORK SENSOR (optional)                    │
│                  border / SPAN port / TAP                       │
│                                                                 │
│  ┌──────────────┐          ┌───────────────────────────────┐   │
│  │   Suricata   │          │             Zeek              │   │
│  │ (signature   │          │ (DNS / HTTP / TLS / conn      │   │
│  │  IDS/IPS)    │          │  structured logs)             │   │
│  └──────┬───────┘          └──────────────┬────────────────┘   │
│         │                                 │                    │
│         └───────────────┬─────────────────┘                    │
│                         │ events forwarded to Wazuh Manager    │
└─────────────────────────┼───────────────────────────────────────┘
                          │
                    (same security node)
```

---

## Components

### Wazuh
Role: central event collector, rule engine, HIDS, FIM, alert hub.

Deployed as manager + indexer + dashboard on the security node. Agents run on every
monitored host. Ships with hundreds of built-in rules; Blackwall adds a curated layer
of custom rules for higher-signal detections.

### auditd
Role: kernel-level syscall audit on monitored hosts.

Captures `execve`, `sudo`, file permission changes, policy modifications. Feeds into
the Wazuh agent via the audit log. Provides ground truth for process and privilege
activity that syslog alone cannot give.

### Suricata
Role: network IDS/IPS with signature-based detection.

Runs on a network vantage point (border router, SPAN port, dedicated sensor). Detects
known attack patterns in network traffic. Alerts forwarded to Wazuh for correlation
with host events.

### Zeek
Role: network visibility and telemetry.

Produces structured logs per protocol (conn, dns, http, ssl, files, x509). Complements
Suricata signatures with behavioral and statistical analysis. Especially useful for
threat hunting and investigating lateral movement.

### OpenSearch Anomaly Detection
Role: ML-based anomaly detection over indexed event data (Stage 4).

Enabled after the data pipeline is stable and noise is filtered. Builds baselines per
host and alerts on statistical deviations (login time, traffic volume, process frequency).

---

## Detection Coverage

### Stage 1 — Host Events (MVP)
| Scenario | Source |
|---|---|
| SSH brute force (many failures from one IP) | auth.log → Wazuh |
| Successful login after failure sequence | auth.log → Wazuh |
| Privilege escalation via sudo/su | auditd + auth.log |
| Changes to `/etc`, `/usr/local/bin`, `cron`, `sshd_config`, `sudoers` | Wazuh FIM |
| New systemd unit files / services | auditd + FIM |
| Unusual binaries executed | auditd execve |

### Stage 2 — Enriched Detections
| Scenario | Source |
|---|---|
| Login from new IP/ASN | Wazuh GeoIP enrichment |
| Off-hours authentication | Wazuh time-based rules |
| Repeated sudo failures | Wazuh correlation |
| New user/group created | auditd |

### Stage 3 — Network Layer
| Scenario | Source |
|---|---|
| Port scan / host sweep | Suricata |
| Known exploit signatures | Suricata ET rules |
| C2 callbacks via DNS/HTTP | Suricata + Zeek |
| Unusual outbound TLS (unknown certs) | Zeek ssl.log |
| DNS tunneling indicators | Zeek dns.log |
| Network + host event correlation | Wazuh cross-source |

### Stage 4 — Anomaly Detection
| Scenario | Source |
|---|---|
| Unusual login frequency per user/host | OpenSearch AD |
| Traffic volume spikes | OpenSearch AD on Zeek conn.log |
| Rare process execution | OpenSearch AD on auditd |

---

## Deployment Model

```
Role              Minimum spec        Notes
──────────────    ─────────────────   ──────────────────────────────
Security Node     4 vCPU / 8 GB RAM   Wazuh manager + OpenSearch
Monitored Host    any                 Wazuh agent + auditd (< 50 MB RAM)
Network Sensor    2 vCPU / 4 GB RAM   Suricata ± Zeek, needs traffic mirror
```

All deployment is driven by Ansible. The security node and network sensor can run on
the same machine for small environments.

---

## Repository Layout (planned)

```
blackwall/
├── ansible/
│   ├── roles/
│   │   ├── wazuh-manager/      # security node setup
│   │   ├── wazuh-agent/        # per-host agent + auditd config
│   │   ├── suricata/           # network sensor
│   │   └── zeek/               # network telemetry
│   ├── playbooks/
│   │   ├── all.yml            # full stack
│   │   ├── security-node.yml
│   │   └── monitored-host.yml
│   └── inventory/
│       └── example.yml
├── rules/
│   ├── wazuh/                  # custom .xml rule files
│   └── suricata/               # custom .rules files
├── config/
│   ├── auditd/                 # audit.rules
│   ├── wazuh/                  # ossec.conf fragments
│   └── zeek/                   # local.zeek
├── docs/
│   ├── ARCHITECTURE.md         # this file
│   ├── stages/
│   │   ├── stage-1-host.md     # implementation plan: host layer
│   │   ├── stage-2-detects.md  # implementation plan: detection rules
│   │   ├── stage-3-network.md  # implementation plan: network layer
│   │   └── stage-4-anomaly.md  # implementation plan: anomaly detection
│   └── runbooks/               # alert investigation guides
└── tests/
    └── molecule/               # Ansible role tests
```

---

## Roadmap

| Stage | Focus | Key outcome |
|---|---|---|
| 1 | Host layer | Wazuh + auditd on 1–3 hosts, first alerts firing |
| 2 | Detection rules | Curated high-signal ruleset, alert routing to Telegram/Slack |
| 3 | Network layer | Suricata + Zeek feeding into Wazuh, host+network correlation |
| 4 | Anomaly detection | OpenSearch AD baselines, ML detections as second layer |

Each stage produces working, independently testable infrastructure.

---

## Target Platforms

- **OS:** Ubuntu 22.04 LTS, Debian 12 (primary); Rocky Linux 9 (secondary)
- **Deployment:** bare metal and VMs; cloud-agnostic
- **Wazuh version:** 4.x (current stable)
- **Suricata version:** 7.x
- **Zeek version:** 6.x
