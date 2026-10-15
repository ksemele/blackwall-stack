# Fang reputation blocklists Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reputation lists become an nftables blocklist (observe/enforce) on protected hosts, with Wazuh events, proven in the lab.

**Architecture:** New module blackwall-fang with role `fang-reputation` (own nftables table + refresh script + timer); Eye's exporter reads extra sets from a drop-in dir; one Wazuh rule.

**Tech Stack:** nftables, bash/python3 stdlib (no new packages), systemd timer, Wazuh 4.14.7.

**Spec:** `docs/superpowers/specs/2026-10-15-fang-reputation-design.md`

## Global Constraints

- Module isolation: blackwall-fang must not import Eye roles; integration only through files (`/etc/nftables.d/*.nft`, `/etc/blackwall/fw-export.d/*.conf`) and the shared inventory.
- Default `fang_mode: observe`. Allowlist always wins; Cloudflare ranges are always allowlisted; private/reserved ranges never enter the blocklist.
- Any refresh failure leaves the live table untouched; load before persist.
- Lab only; never `bw-lab down`/`reset`; keep black-dead-98 stopped. Nothing on the prod host.
- `bw-check all` and `bw-check sentinel` stay green after every task.
- No real host identifiers in the repo. English; brief commits, no AI trailers; no push.

## Review Focus

- A list containing Cloudflare or the admin address must never block them — pinned in Task 1's script test.
- Private ranges from FireHOL (10/8, 172.16/12, 192.168/16 ...) never reach `blocklist_static` — pinned in Task 1.
- Enforce mode must not break podman egress/DNS or Docker DNS (WAN-only rules) — pinned in Task 3.
- Junk / truncated / too-small list leaves the live table unchanged — pinned in Task 1 and Task 3.
- `systemctl reload nftables` keeps Fang's table; stop behaviour documented — Task 1.

---

### Task 1: module scaffold + role `fang-reputation` (table, script, timer)

**Files:** `blackwall-fang/{CLAUDE.md,README.md,ansible.cfg}`, `blackwall-fang/ansible/{roles/fang-reputation/{defaults,tasks,handlers,files,templates},playbooks/{fang.yml,verify-fang.yml}}`, `blackwall-fang/STATUS.md`, `tests/inventory/hosts.yml` (group `fang`: black-sentinel-03), `tests/inventory/group_vars/fang.yml`, lab test list file (served via `file://` override), `tests/bin/bw-lab` (target `fang`, not in `all`; exclude `fang` hosts only where needed — sentinel is already excluded from main-stack limits).

- [ ] Script `blackwall-fang-refresh` (python3 stdlib recommended for CIDR math via `ipaddress`): fetch sources (https; `file://` only when a test flag/env is set), parse Spamhaus JSON `cidr` lines and FireHOL netset, drop reserved ranges (use `ipaddress` `is_private`/`is_reserved`/`is_multicast`/`is_loopback`/`is_link_local` + 100.64.0.0/10 + 0.0.0.0/8), reject prefixes shorter than /8, enforce `fang_min_entries`, fetch Cloudflare v4 ranges, build allowlist (`fang_allowlist` + CF + host addresses), remove overlaps (subtract allowlisted space from blocklist CIDRs — `address_exclude`), render the table for `fang_mode`, `nft -c`, load, persist `/etc/nftables.d/blackwall-fang.nft`. Never touches the live table on failure.
- [ ] Unit test (fixtures, no network, no nft): private ranges removed, CF/admin overlap carved out, /0-/7 rejected, small list rejected, observe vs enforce rendering.
- [ ] Role installs script, config (`/etc/blackwall/fang.conf`), service + daily timer (RandomizedDelaySec, Persistent), runs it once on install/change; drop-in or doc for nftables stop.
- [ ] Lab: `bw-lab provision fang` on sentinel in observe mode; `nft list table inet blackwall_fang` shows rocky's address in `blocklist_static`, no private ranges, master in `allow_v4`. `bw-check all`, `bw-check sentinel` green. Commit `feat(fang): reputation blocklist role`.

### Task 2: exporter drop-in sets + Wazuh rule

**Files:** Eye `roles/nftables/files/blackwall-fw-export` (+ tests), `roles/wazuh-manager/templates/{blackwall_decoders.xml.j2,blackwall_rules.xml.j2}`, Fang role (drop `/etc/blackwall/fw-export.d/fang.conf`), READMEs.

- [ ] Exporter reads `/etc/blackwall/fw-export.d/*.conf` (`kind=`, `table=`, `set=`; ignore malformed files with a stderr line), treats them like built-in sets (same baseline/cooldown/cap; priority after origin_direct). Unit tests.
- [ ] Decoder accepts the new kind; rule 100206 level 5 for `kind=blocklist_hit`; wazuh-logtest proof.
- [ ] Lab: rocky -> sentinel 443 in observe mode -> 100206 for srcip 172.28.0.15 in the sentinel indexer. Commit `feat(eye,fang): blocklist hit events`.

### Task 3: `bw-check fang` (observe + enforce) + docs

**Files:** `tests/bin/bw-check` (opt-in `fang`), `tests/README.md`, Fang README/STATUS, Eye README (exporter drop-in).

- [ ] Check: private ranges absent from `blocklist_static`; observe -> rocky in `fang_hits` and 100206 alert newer than start; switch to enforce (provision with `-e fang_mode=enforce`) -> rocky dropped in prerouting (fang counter grows, cf-wall `origin_direct` for rocky does not), master still 200 on 443, podman egress (`podman exec web wget --spider example.com`) and container DNS still work; junk list run leaves the table unchanged; restore observe. SKIP/FAIL semantics as in the other opt-in checks (explicit absent = SKIP, ssh error = FAIL).
- [ ] Docs + STATUS (Fang status -> in_progress, layer 1 done, next: layer 2). Commit `feat(tests): bw-check fang`.
