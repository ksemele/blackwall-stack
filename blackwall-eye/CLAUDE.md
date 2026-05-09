# blackwall-eye — Module Context

**Codename:** Eye
**Purpose:** Host monitoring core. First line of perception.
**Scope:** Collects, normalizes, and correlates host-level security events.
Does NOT handle alerting UI, AI analysis, honeypots, or active response —
those belong to later modules.

## Stack

| Component | Role |
|---|---|
| Wazuh Manager | Central event collector, rule engine, HIDS, FIM, alert hub |
| Wazuh Agent | Per-host agent: log collection, FIM, auditd integration |
| auditd | Kernel-level syscall audit (execve, sudo, file changes) |
| Suricata | Network IDS/IPS, signature-based detection |
| Zeek | Network telemetry: structured logs per protocol |
| OpenSearch | Long-term storage, anomaly detection (Stage 4) |

Deployment: Ansible IaC. No manual UI steps.
Target OS: Ubuntu 22.04 LTS / Debian 12 (primary), Rocky Linux 9 (secondary).

## Stage Breakdown

| Stage | Delivers |
|---|---|
| 1 | Wazuh manager + agent + auditd on 1–3 hosts. First alerts firing. |
| 2 | Enriched detections: GeoIP, time-based rules, correlation. |
| 3 | Suricata + Zeek network layer, host+network correlation. |
| 4 | OpenSearch Anomaly Detection baselines. |

## Design Principles

- **Signal over noise.** High-confidence detections before broad coverage.
- **Config as code.** Every component deployed via Ansible. No manual steps.
- **Minimal footprint.** Agents must not become a resource problem on monitored hosts.
- **Independent stages.** Each stage is deployable and testable on its own.

## Repository Layout (planned)

```
blackwall-eye/
├── ansible/
│   ├── roles/
│   │   ├── wazuh-manager/
│   │   ├── wazuh-agent/
│   │   ├── suricata/
│   │   └── zeek/
│   ├── playbooks/
│   │   ├── site.yml
│   │   ├── security-node.yml
│   │   └── monitored-host.yml
│   └── inventory/
│       └── example.yml
├── rules/
│   ├── wazuh/
│   └── suricata/
├── config/
│   ├── auditd/
│   ├── wazuh/
│   └── zeek/
├── docs/
│   ├── ARCHITECTURE.md
│   └── stages/
└── tests/
    └── molecule/
```

## Agent Rules

- Read `STATUS.md` before starting any work to understand current progress.
- Update `STATUS.md` after completing each task.
- Follow Ansible role structure — one role per component.
- Test Ansible roles with Molecule before marking a task complete.
- Commit after each working increment on the `claude-dev` branch.
