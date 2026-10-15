# Fang: bans driven by Wazuh events (active response) — design (future task)

Date: 2026-10-16
Module: blackwall-fang
Status: **planned — not started** (Fang backlog item 3).

## 1. Goal

Ban sources on evidence Wazuh already has but the kernel doesn't: failed SSH
logins (completed TCP sessions — not spoofable), web attacks seen in nginx
logs, repeat offenders across Fang/Eye alerts. Bans land in their own nftables
set `ban_wazuh` (see 2.1), with a timeout.

## 2. Design

### 2.1 Mechanism

- Wazuh active response (manager `<command>` + `<active-response>`) runs a
  Fang-provided script on the agent where the event happened.
- Script `blackwall-fang-ar` (stdlib python, installed by a Fang role on
  every host that has the Fang table): reads the Wazuh AR JSON from stdin,
  validates `srcip` (IPv4, not allowlisted — re-checks `allow_v4` in the live
  table), adds/removes it in `inet blackwall_fang` set `ban_wazuh` (own set,
  `flags timeout`) with the AR timeout. Never touches other sets.
- Wazuh's built-in `firewall-drop` is iptables-only and is not used.

### 2.2 Triggers (initial)

| Wazuh rule(s) | Meaning | Ban timeout |
|---|---|---|
| 5710/5712/5716/5720/5763 (sshd failures, brute force) | failed logins, completed sessions | 1h |
| web attack rules from nginx logs (when collected) | exploit attempts | 1h |

Triggers, levels and timeouts are role variables; start with SSH only.

### 2.3 Mode and safety

- `fang_ar_mode: observe | enforce` — observe: the script logs "would ban"
  (a Wazuh event) but does not change the set.
- Allowlist re-check inside the script (defence in depth beyond Wazuh's own
  `<white_list>`, which is also populated from `fang_allowlist`).
- Rate limit: at most N bans per minute per host (protects against a flood
  of spoofed events making the set huge).
- Every ban/unban is logged to syslog -> Wazuh event (who, which rule, until
  when).

### 2.4 Coupling

The manager-side `<command>`/`<active-response>` blocks live in the Eye
wazuh-manager role behind a variable (`wazuh_active_response_fang`), and the
script ships with Fang — the same file-level coupling as the exporter drop-in.
Fang must remain deployable without it.

## 3. Lab proof

- rocky makes failed SSH logins to the sentinel -> Wazuh fires 5710/5763 on
  the sentinel manager (agent 000) -> AR adds rocky to `ban_wazuh`; rocky's next
  SSH connection times out; after the lab timeout it can connect again.
- observe mode: "would ban" event, set unchanged.
- master (allowlisted) failing logins -> never banned (both Wazuh white_list
  and the script re-check).
- flood of AR events -> rate limit holds.

## 4. Out of scope

IPv6; CrowdSec (separate module idea); blocking at Cloudflare (API) — a
possible later extension.
