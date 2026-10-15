# Fang layer 2 observe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** In-kernel detection (no drops) of SSH, closed-port and high-rate connection attempts per source, as Wazuh events.

**Architecture:** `blackwall-fang-refresh` renders meter sets + flagged sets into the existing `inet blackwall_fang` prerouting chain (after allowlist and established accepts); Fang renders three exporter drop-ins; Eye gets rules 100210-100212.

**Tech Stack:** nftables (1.0.2 Ubuntu / 1.0.9 Rocky), python3 stdlib, Wazuh 4.14.7.

**Spec:** `docs/superpowers/specs/2026-10-16-fang-attempts-observe-design.md`

## Global Constraints

- Observe only: no rule added by this plan drops or rejects anything.
- Allowlist (admin, monitoring, host, Cloudflare) and established traffic are never counted.
- Layer 2 renders even when the reputation blocklist is empty (first run / fail-open / stale cache).
- Module isolation; lab only; `bw-check all`, `bw-check sentinel`, `bw-check fang` green after every task.
- No real host identifiers. English; brief commits, no AI trailers; no push.

## Review Focus

- A meter/flagged-set full condition must not break other rules (each update is its own rule) — Task 1.
- Cloudflare-range sources making many connections are never flagged (allowlist accept comes first) — Task 1 render test.
- Existing layer 1 behaviour unchanged (reputation observe/enforce, fail-open paths) — Task 1 unit tests + `bw-check fang`.
- SSH from a flagged source still completes (nothing dropped) — Task 2.

---

### Task 1: detectors in the Fang table + drop-ins + Eye rules

**Files:** `blackwall-fang/ansible/roles/fang-reputation/{files/blackwall-fang-refresh,defaults/main.yml,templates/fang.conf.j2,templates/fw-export-*.j2,tasks/main.yml}`, `tests/unit/fang_refresh_test.py`, `tests/inventory/{group_vars/fang.yml or host_vars/black-sentinel-03.yml}` (`fang_open_tcp` for the sentinel), Eye `roles/wazuh-manager/templates/blackwall_rules.xml.j2` (100210-100212), READMEs.

- [ ] Find nft syntax for set-backed per-source `limit rate over` that works on nft 1.0.2 and 1.0.9 (prove with `nft -c` on black-host-01 and black-rocky-02) and render: meter sets (dynamic, timeout 5m, size 65536), flagged sets `ssh_flagged`, `scan_flagged` (only when `fang_open_tcp` non-empty; `open_tcp` set), `rate_flagged` (dynamic, timeout 1h, size 65536, counters); rules after the reputation rules, `ct state new` TCP only, each `update @*_flagged` in its own rule. Variables per spec §2.2 + `fang_attempts_enabled`.
- [ ] Unit tests: render with/without `fang_open_tcp`, disabled, no `drop` from layer 2 in either reputation mode, allowlist accept before detectors, layer 2 present with empty blocklist.
- [ ] Drop-ins `kind=ssh_attempts|scan_attempts|rate_attempts` (only for enabled detectors); Eye rules 100210 (L6), 100211 (L6), 100212 (L5); wazuh-logtest proof.
- [ ] `bw-lab provision fang` on the sentinel; manual proof: rocky 20 quick SSH SYNs -> in `ssh_flagged`. All checks green. Commit `feat(fang): attempt detectors (observe)`.

### Task 2: `bw-check fang` extension + docs

**Files:** `tests/bin/bw-check` (check_fang), `tests/README.md`, `blackwall-fang/README.md`, `blackwall-fang/STATUS.md`, Eye README (rule table).

- [ ] Extend check_fang per spec §2.4 (rocky SSH x20 and closed ports x30 -> flagged + alerts 100210/100211 newer than start; host-01 2 SSH attempts -> not flagged; master 20 SSH attempts -> not flagged; rocky's SSH handshake still completes). Keep layer 1 steps; keep runtime reasonable.
- [ ] Docs; STATUS: layer 2 observe done, next_task: tune thresholds on the target host, then the enforce spec. Commit `feat(tests): bw-check fang attempts`.
