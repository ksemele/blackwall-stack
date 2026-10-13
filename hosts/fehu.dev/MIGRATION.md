# Migrating firewalld → nftables on fehu.dev

Replacing a working perimeter on a live host. Read it all before starting.

Starting state:

```
firewalld.service   enabled, active (running)
nftables.service    disabled, inactive (dead)
```

Target: `nftables.service` is the primary firewall, firewalld is disabled and masked.

## What must be preserved

| What | Where it is now | Where it moves |
|---|---|---|
| 22 SSH from anywhere | `filter_IN_public_allow` | `blackwall_base` input |
| 9090 cockpit | `filter_IN_public_allow` | **removed** — nothing listens |
| 9999 zuzia | `filter_IN_public_allow` | `blackwall_base`, only from `@zuzia_v4` |
| 443 → container | netavark DNAT + `ct status dnat accept` | `blackwall_base` forward |
| containers going out | `filter_FWD_public_allow { oifname eth0 }` | `blackwall_base` forward |
| ICMP/ICMPv6 | `meta l4proto { icmp, ipv6-icmp } accept` | `blackwall_base` input |
| DHCPv6 | `ip6 daddr fe80::/64 udp dport 546` | `blackwall_base` input |
| CF filter on 80/443 | — (did not exist) | `blackwall_wall` prerouting |

Verified on a test bench (two networks, DNAT into a backend container):

```
22/9999 -> OPEN          8080, 53 -> blocked
443 non-CF   -> blocked  443 from cf_v4 -> OPEN
container -> internet: OPEN
```

## Two mines already stepped on

**1. `ct status dnat accept` is load-bearing.** Verified by removing it: without
it, 443 via DNAT is `blocked` immediately. Don't remove it from forward.

**2. The stock `nftables.service` runs `flush ruleset`:**

```
ExecReload=/sbin/nft 'flush ruleset; include "/etc/sysconfig/nftables.conf";'
ExecStop=/sbin/nft flush ruleset
```

`flush ruleset` wipes **all** tables, including netavark's. Reproduced on the
bench: after a stock reload the podman table disappears and 443 becomes
`blocked`, while SSH stays alive (it is in our table). Hence the drop-in
`systemd/nftables.service.d/blackwall.conf`, which limits the service to our
own tables. **Don't migrate without the drop-in.**

## What goes where

| In the repo | On the host |
|---|---|
| `nftables/blackwall-base.nft` | `/etc/nftables.d/blackwall-base.nft` |
| `nftables/blackwall-wall.nft` | `/etc/nftables.d/blackwall-wall.nft` |
| `nftables/sysconfig-nftables.conf` | `/etc/sysconfig/nftables.conf` |
| `systemd/nftables.service.d/blackwall.conf` | `/etc/systemd/system/nftables.service.d/blackwall.conf` |
| `bin/blackwall-cf-refresh` | `/usr/local/sbin/blackwall-cf-refresh` |
| `blackwall-cf-refresh.default` | `/etc/default/blackwall-cf-refresh` |
| `systemd/blackwall-cf-refresh.service` | `/etc/systemd/system/` |
| `systemd/blackwall-cf-refresh.timer` | `/etc/systemd/system/` |
| `nginx/cf-realip.conf` | don't copy by hand — `blackwall-cf-refresh` writes it to `REALIP_OUT` |

`/etc/sysconfig/nftables.conf` already exists (it ships with the `nftables`
package, containing a commented-out example) — we overwrite it, so it is backed
up below.

## Procedure

You need two SSH sessions: a working one and a spare (keep it open until the end).

```bash
# --- 0. Copy the files from your machine to the host ---
# run LOCALLY, from the root of the blackwall repo
scp -r hosts/fehu.dev root@93.115.20.111:/root/blackwall-fw
ssh root@93.115.20.111
cd /root/blackwall-fw          # everything below runs here, on the host

# --- 1. Files ---
# back up what we overwrite
sudo cp -a /etc/sysconfig/nftables.conf /etc/sysconfig/nftables.conf.bak-$(date +%F)

sudo install -d -m 0755 /etc/nftables.d
sudo install -m 0644 nftables/blackwall-base.nft /etc/nftables.d/blackwall-base.nft
sudo install -m 0644 nftables/blackwall-wall.nft /etc/nftables.d/blackwall-wall.nft
sudo install -m 0644 nftables/sysconfig-nftables.conf /etc/sysconfig/nftables.conf
sudo install -D -m 0644 systemd/nftables.service.d/blackwall.conf \
    /etc/systemd/system/nftables.service.d/blackwall.conf
sudo install -D -m 0755 bin/blackwall-cf-refresh /usr/local/sbin/blackwall-cf-refresh
sudo install -D -m 0644 blackwall-cf-refresh.default /etc/default/blackwall-cf-refresh
sudo install -m 0644 systemd/blackwall-cf-refresh.service /etc/systemd/system/
sudo install -m 0644 systemd/blackwall-cf-refresh.timer   /etc/systemd/system/
sudo systemctl daemon-reload

# --- 2. Variables ---
# Checked on the host 2026-08-15 — the values in the files are already correct:
#   BASE_WAN_IF / WALL_WAN_IF = "eth0"       (ip -o -4 route show to default)
#   PODMAN_NET = 10.88.0.0/16                (podman network inspect podman)
# Re-check if the host or podman networking has changed:
ip -o -4 route show to default | awk '{print $5}'
podman network inspect podman -f '{{range .Subnets}}{{.Subnet}}{{end}}'

# Only ADMIN_ALLOW is left — your IPs for direct origin access bypassing CF.
# Empty is safe: it doesn't affect SSH, there's just no direct 80/443 access
# for deploys and debugging.
sudo vi /etc/default/blackwall-cf-refresh

# --- 3. Syntax check, applying nothing ---
sudo nft -c -f /etc/nftables.d/blackwall-base.nft
sudo nft -c -f /etc/nftables.d/blackwall-wall.nft

# --- 4. Safety net: automatic rollback in 10 minutes ---
# systemd-run rather than `sleep &` — survives a dropped SSH session.
sudo systemd-run --on-active=600 --unit=fw-rollback /bin/sh -c '
  nft delete table inet blackwall_base 2>/dev/null
  nft delete table inet blackwall_wall 2>/dev/null
  systemctl unmask firewalld
  systemctl start firewalld'

# --- 5. Apply ON TOP of the running firewalld ---
# Both tables coexist: our rules allow exactly what the public zone allows.
sudo nft -f /etc/sysconfig/nftables.conf
sudo nft list tables      # expect: ip nat, ip filter, inet firewalld, inet blackwall_base, inet blackwall_wall
```

**Stop. Check from the SPARE session — a new SSH connection, not an existing one:**

```bash
ssh root@93.115.20.111 true && echo "SSH alive"
curl -sI https://fehu.dev | head -1          # site through CF
curl -sI --max-time 5 https://93.115.20.111  # straight to origin -> must hang
```

If anything is wrong — `sudo nft delete table inet blackwall_base` and investigate.
If all is well:

```bash
# --- 6. Remove firewalld ---
sudo systemctl disable --now firewalld
sudo systemctl mask firewalld
sudo nft list tables      # inet firewalld must be gone

# --- 7. Check again from the spare session (step above) ---

# --- 8. Enable nftables as the primary firewall ---
sudo systemctl enable --now nftables
sudo systemctl enable --now blackwall-cf-refresh.timer

# --- 9. Remove the safety net ---
sudo systemctl stop fw-rollback.timer 2>/dev/null
sudo systemctl reset-failed fw-rollback.service 2>/dev/null
```

## Post-reboot check

The only thing that truly confirms the migration:

```bash
sudo reboot
# ... wait ...
ssh root@93.115.20.111 'nft list tables; systemctl is-active nftables podman'
curl -sI https://fehu.dev | head -1
```

Expect the tables `ip nat`, `ip filter` (recreated by podman),
`inet blackwall_base`, `inet blackwall_wall`.

## Rollback

```bash
sudo systemctl disable --now nftables
sudo nft delete table inet blackwall_wall
sudo nft delete table inet blackwall_base
sudo systemctl unmask firewalld
sudo systemctl enable --now firewalld
sudo systemctl restart podman   # netavark re-installs its rules

# restore the package's original config (if nftables ever needs reinstalling)
sudo cp -a /etc/sysconfig/nftables.conf.bak-* /etc/sysconfig/nftables.conf
```

## Left to think about

**9090 (cockpit) — removed.** `ss -lntp` showed nothing listening on it
(cockpit.socket not running), the port times out from outside. The rule was
left over from firewalld and kept a port open to nowhere.

**9999 (zuzia) — limited to the service's addresses.** The monitoring service
scrapes the host from its own servers and publishes their addresses; they live
in the `zuzia_v4` set in `blackwall-base.nft`. **Fill it in before applying** —
an empty set closes 9999 for everyone and monitoring stops collecting data.

SSH stays open to everyone — key-only login.
