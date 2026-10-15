---
module: blackwall-fang
status: in_progress
stage: 5
last_updated: 2026-10-15
last_task: "Layer 1 (reputation blocklists) done: role fang-reputation, Wazuh event 100206, bw-check fang (observe + enforce + junk list) green; real-source observe run on the lab sentinel (4619 entries, exit 0)"
next_task: "Layer 2: in-kernel ban on connection attempts (blocklist_dynamic with timeout, observe first)"
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

**Layer 1 is done** (role `fang-reputation`, see README; `bw-check fang`).

1. **Reputation** (done) — Spamhaus DROP/EDROP and FireHOL level1 loaded into a
   `blocklist_static` nftables set by a refresh script + timer (same pattern
   as `cf-wall`: sanity floor, `nft -c`, load before persist).
2. **In-kernel ban on attempts** (next) — nftables rate rules move a source into
   `blocklist_dynamic` (with timeout) when it makes too many new connection
   attempts within a window: to closed ports (scans) and to open services such
   as SSH (brute force / floods). Thresholds and timeouts are variables; short
   timeouts for the spoofable closed-port case. Reuses blackwall-eye's drop
   accounting for visibility.

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
