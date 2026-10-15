# Fang layer 2 enforcement: in-kernel bans on attempts — design (future task)

Date: 2026-10-16
Module: blackwall-fang
Status: **planned — not started.** Run only after layer 2 observe
(`2026-10-16-fang-attempts-observe-design.md`) has run on the target host long
enough to tune thresholds from real 100210-100212 alerts.

## 1. Goal

Turn the observe-only detectors into temporary bans: a source flagged by the
SSH, closed-port or rate detector is dropped for a while, automatically,
in the kernel, with every ban visible in Wazuh.

## 2. Design

### 2.1 Mode

- `fang_attempts_mode: observe | enforce` (default `observe`), independent
  of the reputation `fang_mode`. Enforce requires a non-empty
  `fang_allowlist` (lockout guard, same as layer 1).
- Per-detector enable in enforce: `fang_ban_ssh`, `fang_ban_scan`,
  `fang_ban_rate` (defaults: ssh `true`, scan `true`, rate `false` — the rate
  detector is the most false-positive prone).

### 2.2 Ban set and rules

- New set `blocklist_dynamic` (`ipv4_addr`, `flags dynamic,timeout`, size
  65536, counters). Ban timeouts per detector: `fang_ban_timeout_ssh` (1h),
  `fang_ban_timeout_scan` (15m — closed-port SYNs are spoofable, keep it
  short), `fang_ban_timeout_rate` (10m).
- Early in the chain, right after the allowlist and established accepts:
  `ip saddr @blocklist_dynamic counter drop` (its own rule).
- Detector rules, when the detector is banning: the over-rate match adds the
  source to `blocklist_dynamic` with the detector's timeout in a separate rule
  from the observe `update @*_flagged` (so a full flagged set never stops a
  ban, and a full ban set never stops the flag/visibility). A full
  `blocklist_dynamic` is visible (exporter summary / a dedicated counter) and
  fails open (no ban), never closed.
- Reload resets `blocklist_dynamic` (table replace): acceptable — bans are
  short-lived by design; document it.

### 2.3 Events

- Exporter drop-in `kind=dynamic_ban` on `blocklist_dynamic`; Eye rule (next
  free id) level 6 "source banned in kernel" with the detector visible if
  possible (separate kinds per detector, or separate sets per detector:
  `ban_ssh`, `ban_scan`, `ban_rate` — decide in the plan; separate sets make
  "why" visible, as the module rules require).
- Unban is implicit (timeout); the exporter's set shrink is not an event.

### 2.4 Safety

- Allowlist (admin, monitoring, agent sources, Cloudflare, host addresses)
  is accepted before any ban rule — never banned.
- Established connections are accepted before the ban rule: an active SSH
  session of the owner survives a ban of its address for new connections.
- An emergency unban path documented: `nft flush set inet blackwall_fang
  blocklist_dynamic` and switching `fang_attempts_mode` back to `observe`.

## 3. Lab proof (`bw-check fang`, enforce part)

- rocky exceeds the SSH rate -> lands in the SSH ban set; its next new SSH
  connection times out; after the (lab-shortened) timeout it connects again.
- rocky's existing SSH session opened before the ban keeps working.
- master (allowlisted) exceeds the rate -> never banned.
- black-host-01 under the rate -> never banned.
- Ban event visible in the sentinel indexer.
- `fang_ban_rate: false` -> rate-flagged source is not banned.
- Restore observe at the end (trap on INT/TERM).

## 4. Out of scope

IPv6, UDP, Wazuh-driven bans (separate spec
`2026-10-16-fang-active-response-design.md`).
