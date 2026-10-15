# blackwall-fang — Fang

**Role:** Active countermeasures. The first, production part is defensive: automatic
blocking of hostile sources on hosts protected by blackwall-eye's firewall roles.
Counter-attack tooling stays a research-only topic for later.

Read `STATUS.md` before any work; its "Rules for any automatic block" apply to every layer.

## Layer 1: reputation blocklists (role `fang-reputation`)

- Own nftables table `inet blackwall_fang`, chain `prerouting` at priority -160
  (before cf-wall -150 and netavark DNAT -100), WAN interface only.
- Sets: `allow_v4`, `blocklist_static`, `fang_hits` (per-source counters).
- `blackwall-fang-refresh` (python3, stdlib) fetches Spamhaus DROP and FireHOL level1,
  drops private/reserved space, rejects prefixes shorter than /8 and lists below the
  sanity floor, carves the allowlist (config + Cloudflare + host addresses) out of the
  blocklist, runs `nft -c`, loads, then persists `/etc/nftables.d/blackwall-fang.nft`.
- `observe` (default) counts hits and drops nothing; `enforce` drops.

## Layout

```
blackwall-fang/
├── CLAUDE.md  README.md  STATUS.md  ansible.cfg
└── ansible/
    ├── playbooks/{fang.yml,verify-fang.yml}
    └── roles/fang-reputation/{defaults,tasks,handlers,files,templates}
```

Unit test (no network, no nft): `python3 tests/unit/fang_refresh_test.py`.

## Agent Rules

1. Read STATUS.md first; update it after each task.
2. Independent: no imports or path dependencies on Eye roles. Integration is via files
   (`/etc/nftables.d/*.nft`, `/etc/blackwall/*`) and the shared inventory (`tests/inventory`, group `fang`).
3. Default is observe mode. Allowlist and Cloudflare always win; private/reserved is never blocked.
4. A failed refresh must leave the live table untouched.
5. Lab: `tests/bin/bw-lab provision fang`; test lists live in `tests/fang/` and are usable
   only with `fang_test_mode`.
6. Keep docs in sync: roles, variables or commands change -> update README.md in the same commit.
