# Firewall events Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Drops by our nftables tables become Wazuh alerts (origin-direct, scan bursts) on Ubuntu, RHEL and in the lab.

**Architecture:** nftables dynamic sets with per-element counters record who was dropped; an exporter run by the Wazuh agent as a `command` localfile prints only growth; a manager decoder + rules turn lines into alerts.

**Tech Stack:** nftables (1.0.2 Ubuntu / 1.0.9 Rocky), Python 3 or bash+jq for the exporter (whatever is present on both families without new packages — check), Wazuh 4.14.7.

**Spec:** `docs/superpowers/specs/2026-10-15-firewall-events-design.md`

## Global Constraints

- Lab only; never `bw-lab down`/`reset`; keep `black-dead-98` stopped. Nothing on the prod host.
- `bw-check all` and `bw-check sentinel` stay green after every task; `nft-equiv` fixtures (`blackwall-eye/tools/testdata`) updated in the same task as template changes.
- No new packages on hosts (exporter uses what both families already have: python3 is present for Ansible).
- `ansible_facts['os_family']` only. No real host identifiers in the repo. English; brief commits, no AI trailers; no push.

## Review Focus

- Accounting must count only packets that are actually dropped (rule placement right before the final drop / `counter drop`) — pinned by Task 1's check that accepted traffic (e.g. SSH from the controller) never appears in the set.
- A firewall reload resets counters; the exporter must not crash or emit negative deltas — pinned by the exporter fixture test in Task 2.
- The exporter must never break the agent (missing table/set, nft error) — pinned in Task 2.
- Set size/timeouts bound memory — set definitions carry `size` and `timeout` (Task 1).
- Rule 100202 must not fire on a handful of drops — pinned by Task 3's below-threshold case.

---

### Task 1: dynamic sets in nftables role and cf-wall

**Files:** `roles/nftables/{defaults,templates/blackwall-base.nft.j2}`, `roles/cf-wall/files/blackwall-cf-refresh`, `tools/testdata/{default,podman}.nft` (+ inventory if needed), role READMEs.

- [ ] Find the nft syntax that works on BOTH nft 1.0.2 and 1.0.9 for a dynamic IPv4 set with timeout, size and per-element counters updated from a rule (`update @set { ip saddr counter }` or equivalent); prove with `nft -c` on black-host-01 and black-rocky-02 before templating.
- [ ] Base table: `dropped_input` (and `dropped_forward` when forward policy is drop), vars `nft_track_drops` (true), `nft_track_timeout` (1h), `nft_track_size` (65536). Update right before the final input `counter` (after the log rule), so accepted packets never touch it.
- [ ] cf-wall script: `origin_direct` set updated right before `counter drop`.
- [ ] Update nft-equiv fixtures; `nft-equiv` default/podman equal.
- [ ] Lab: `bw-lab provision nftables`, `bw-lab provision cf-wall`, `bw-lab provision sentinel`; generate a drop (rocky-02 -> host-01 closed port; rocky-02 -> sentinel 443) and show the source appears with a counter in `nft list set`; SSH from the controller does not appear. All checks green. Commit `feat(eye): per-source drop accounting in nftables`.

### Task 2: exporter

**Files:** `roles/nftables/files/blackwall-fw-export`, `roles/nftables/tasks/main.yml` (install, state dir `/var/lib/blackwall-fw`), `blackwall-eye/tools/testdata/fw-export/` (fixture JSONs), a test runner (e.g. `blackwall-eye/tools/test-fw-export`), README.

- [ ] Write the fixture test first (red): fixtures for run 1 (sources A=5, B=1), run 2 (A=12, B=1, C=30), run 3 after reload (A=2, set empty for others) and a missing set; expected outputs: run1 lines for A(new=5) and B(new=1); run2 A(new=7), C(new=30, burst=yes with threshold 20); run3 A(new=2); missing set -> no output, exit 0. The exporter takes the nft JSON from an env/arg override for tests (e.g. `BLACKWALL_FW_NFT_JSON_DIR`) and the threshold from `BLACKWALL_FW_SCAN_THRESHOLD` / a config file written by the role.
- [ ] Implement (python3, stdlib only, works on Ubuntu 22.04 python3.10 and Rocky 9 python3.9); line format exactly: `blackwall-fw: kind=<kind> src=<ip> new=<n> total=<n> burst=<yes|no>`.
- [ ] Deploy with the role; run on lab hosts: after traffic from Task 1 it prints lines, second run prints nothing. Commit `feat(eye): blackwall-fw-export`.

### Task 3: Wazuh decoder, rules, localfile

**Files:** `roles/wazuh-agent/{defaults,templates/ossec.conf.j2}`, `roles/wazuh-manager/{defaults,templates/ossec.conf.j2,templates/blackwall_rules.xml.j2,templates/blackwall_decoders.xml.j2,tasks}` (deploy decoder, ensure `<ruleset>` includes `etc/decoders`), README.

- [ ] localfile block (`log_format command`, alias `blackwall-fw`, frequency `wazuh_fw_events_interval`, default 60, lab 10 in tests group_vars) in agent and manager ossec.conf when `wazuh_fw_events_enabled`.
- [ ] Decoder on the command-output event (`ossec: output: 'blackwall-fw': blackwall-fw: kind=...`) extracting kind, srcip, new, total, burst; rules 100200-100203 per spec (origin_direct 10; input_drop burst 8; forward_drop burst 6; base 0). Verify with `/var/ossec/bin/wazuh-logtest` on black-master-00 for each case incl. below-threshold (no alert).
- [ ] `bw-lab provision eye` + `sentinel`; all checks green. Commit `feat(eye): wazuh rules for firewall drops`.

### Task 4: `bw-check fw-events` + docs (end-to-end)

**Files:** `blackwall-eye/ansible/playbooks/verify-fw-events.yml` or a bw-check function, `tests/bin/bw-check`, `tests/README.md`, `blackwall-eye/README.md`, `blackwall-eye/STATUS.md`.

- [ ] Opt-in check: origin-direct from rocky-02 to sentinel 443 -> rule 100201 alert with srcip 172.28.0.15 in the sentinel indexer (newer than the test start); scan: rocky-02 -> 30 closed ports on host-01 -> rule 100202 for agent black-host-01 in the main indexer; a few drops (3 ports) -> no 100202 for that window. Poll up to 2-3 min. Note: origin-direct is level 10 — in the lab Voice forwards it to Telegram if deployed; say so in the README.
- [ ] Docs + STATUS. Commit `feat(tests): bw-check fw-events`.
