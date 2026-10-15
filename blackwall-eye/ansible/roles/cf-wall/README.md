# cf-wall

Cloudflare origin filter. On the WAN interface, only Cloudflare prefixes and
`cfwall_admin_allow` may reach `cfwall_ports`; everything else on those ports is
logged (rate-limited) and dropped. Optionally keeps nginx `set_real_ip_from` in
sync with the same prefix list.

The chain hooks **prerouting at priority -150**, not input: when the web server
runs in a container, the runtime DNATs the port in prerouting (priority -100)
and the packet then goes to forward, skipping input entirely. Traffic from other
interfaces (containers' outgoing HTTPS) and established connections are accepted.

Requires the `nftables` role on the same host (it provides `/etc/nftables.d/`,
which its config includes, and a service drop-in that also removes
`inet blackwall_wall` on stop).

## Variables

| Variable | Default | Meaning |
|---|---|---|
| `cfwall_ports` | `[80, 443]` | TCP ports reachable only from Cloudflare and `cfwall_admin_allow` |
| `cfwall_admin_allow` | `[]` | IPv4 only: sources allowed straight to the origin, e.g. `["203.0.113.7", "198.51.100.0/24"]` |
| `cfwall_wan_if` | `""` | external interface; empty = from the default route. Set it explicitly on production hosts; the script aborts if it cannot be detected or does not exist |
| `cfwall_nginx_container` | `""` | podman container running nginx; empty = skip real_ip |
| `cfwall_realip_out` | `""` | host path of `cf-realip.conf` (directory mounted as `/etc/nginx/conf.d`) |
| `cfwall_min_v4` / `cfwall_min_v6` | `10` / `5` | abort when fewer prefixes are fetched |

## How it runs

`/usr/local/sbin/blackwall-cf-refresh` fetches the Cloudflare IPv4/IPv6 lists,
generates `/etc/nftables.d/blackwall-wall.nft`, checks it with `nft -c`, and
only then applies it. `blackwall-cf-refresh.timer` repeats this weekly. The role
runs the script on install and whenever the script or its config change, so a
repeat run without changes does nothing.

Packets that reach the final `counter drop` on the filtered ports first update
the dynamic set `origin_direct` (IPv4, per-source counters, 1h idle timeout,
65536 entries), so only direct-to-origin attempts are accounted. Inspect:
`nft list set inet blackwall_wall origin_direct`. Each refresh replaces the
table, so the counters start from zero again.

## Abort rule

If fewer than `MIN_V4`/`MIN_V6` valid prefixes are fetched (junk page, truncated
response, hijacked DNS) the generated file fails `nft -c`, the live load fails, or the WAN interface is missing, the script exits
non-zero and nothing is applied: the live rules and `/etc/nftables.d/blackwall-wall.nft` stay as they were (the file is written only after a successful load).

## Rollback

```bash
nft delete table inet blackwall_wall
systemctl disable --now blackwall-cf-refresh.timer
rm /etc/nftables.d/blackwall-wall.nft
```

## Run

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/cf-wall.yml
ansible-playbook ansible/playbooks/verify-cf-wall.yml
```
