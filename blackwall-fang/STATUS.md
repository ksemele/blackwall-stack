---
module: blackwall-fang
status: in_progress
stage: 5
last_updated: 2026-10-16
last_task: "Layer 2 (attempt detectors, observe) done: ssh/scan/rate meters + flagged sets, Eye rules 100210-100212, bw-check fang covers them (rocky flagged, host-01 and master not, handshake still completes)"
next_task: "Tune layer 2 thresholds from real alerts on the target host, then implement the enforce spec docs/superpowers/specs/2026-10-16-fang-attempts-enforce-design.md"
blockers: []
---

## Scope

Fang is the active-countermeasures module. Its first, defensive part is
production code: automatic blocking of hostile sources on hosts protected by
blackwall-eye's `nftables` / `cf-wall` roles. Counter-attack tooling remains a
research-only topic for later.

## Plan (agreed 2026-10-15)

Each layer is opt-in and starts in observe-only mode (sets fill and alert,
nothing is dropped) before enforcement is switched on.

**Layers 1 and 2 (observe) are done** (role `fang-reputation`, see README; `bw-check fang`).

1. **Reputation** (done) — Spamhaus DROP/EDROP and FireHOL level1 loaded into a
   `blocklist_static` nftables set by a refresh script + timer (same pattern
   as `cf-wall`: sanity floor, `nft -c`, load before persist).
2. **In-kernel attempt detectors** (observe done, role `fang-reputation`: ssh/scan/rate
   meters + `*_flagged` sets, Eye rules 100210-100212; enforcement still to come) — the original plan:
   **In-kernel ban on attempts** — nftables rate rules move a source into
   `blocklist_dynamic` (with timeout) when it makes too many new connection
   attempts within a window: to closed ports (scans) and to open services such
   as SSH (brute force / floods). Thresholds and timeouts are variables; short
   timeouts for the spoofable closed-port case. Reuses blackwall-eye's drop
   accounting for visibility.

**Future tasks with specs (not started):**

- Layer 2 enforcement — in-kernel bans: `docs/superpowers/specs/2026-10-16-fang-attempts-enforce-design.md`
  (run after layer 2 observe has been tuned on the target host).
- Wazuh active response bans: `docs/superpowers/specs/2026-10-16-fang-active-response-design.md`.

**Backlog:**

3. **Wazuh active response** — a nftables active-response script that bans on
   Wazuh rules (failed SSH logins 5710/5763, scan 100202, ...) into
   `blocklist_dynamic`. Bans on completed TCP sessions, i.e. not spoofable.

**Idea — separate module:**

4. **CrowdSec** — community blocklist + local detection + nftables bouncer as
   its own module (it is a separate daemon and shares signals with the
   CrowdSec cloud, so it stays optional and isolated from Fang's own sets).

## Rules for any automatic block

- Allowlist always wins: Cloudflare ranges, `cfwall_admin_allow`, monitoring
  and agent sources. Blocking Cloudflare takes the site down.
- Every ban has a timeout; no permanent bans.
- Never ban on spoofable evidence alone (SYNs to closed ports) without a short
  timeout; prefer completed handshakes and reputation.
- Every ban/unban is a Wazuh event (who, why, until when).
- Lab proof: `black-rocky-02` attacks `black-sentinel-03`; `bw-check` verifies
  the ban, its expiry, observe-only mode and that allowlisted sources are never
  touched.

## Known minor issues (from the layer 1 reviews)

- Mode switch: the exporter drop-in kind changes before the table is rebuilt;
  an exporter run in that window can report existing `fang_hits` once under the
  new kind, and the old kind's exporter state is never pruned. Fix: render the
  drop-in after the refresh, or prune state for kinds no drop-in declares.
- `JOURNAL_STREAM` presence alone decides that stderr goes to the journal;
  compare its dev:inode with `fstat(2)` to be exact (only affects hand runs from
  a shell that inherited the variable).
- Debian 12+ ships without rsyslog: the Debian branch of the Wazuh ossec.conf
  templates reads `/var/log/syslog`, so Fang's syslog lines (rules 100207/100209)
  would not be collected there. Switch Debian to journald too (Eye).
- A source URL change drops its cache entry; a failed fetch right after the
  change exits 1 (visible) instead of using older cached data.
- The per-source floor is `fang_min_entries / sources`; raising
  `fang_min_entries` high can reject a normal Spamhaus list.
