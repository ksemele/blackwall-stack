# hosts/fehu.dev — origin cloaking

Closes direct access to the origin `93.115.20.111` that bypasses Cloudflare:
only CF prefixes may reach TCP/80 and TCP/443, everything else is dropped.

The configs are manual (the host is not under Ansible yet). Moving it to
Ansible is covered at the end.

## How the host is set up

From `nft list ruleset` on the host itself:

| Table | Managed by | Do we touch it? |
|---|---|---|
| `ip nat`, `ip filter` | netavark (podman), marked "do not touch" | no |
| `inet firewalld` | firewalld — **being removed**, see MIGRATION.md | replaced |
| `inet blackwall_base` | this directory, replaces firewalld | yes |
| `inet blackwall_wall` | this directory, Cloudflare filter | yes |

What is open to the outside:

- **22** — SSH from anywhere; **9999** — zuzia, only from the service's addresses
  (9090/cockpit removed: nothing was listening)
- **443** — bypasses firewalld, via podman DNAT to the container `10.88.0.31:443`
- **80** — closed (neither in DNAT nor in the allow chain)

## Why hook prerouting and not input

nginx runs in a container, and netavark installs DNAT in prerouting:

```
chain NETAVARK-HOSTPORT-DNAT { tcp dport 443 ... }
chain NETAVARK-DN-...        { tcp dport 443 ... dnat to 10.88.0.31:443 }
```

After DNAT the destination is the container, so the packet goes to **forward**,
not input. Rules on `hook input` never see it. Verified on a test bench that
reproduces this topology (client → host with DNAT → backend container):

| Construct | Traffic | Drop counter |
|---|---|---|
| `hook input priority -10` | passes | `packets 0` — the rule never sees it |
| `hook prerouting priority -150` | blocked | `packets 3` |
| `prerouting`, source in `cf_v4` | passes | — |

Priority layout in prerouting:

```
-200  conntrack       <- before it, ct state is not available
-150  blackwall_wall  <- us
-100  dstnat          <- netavark DNAT
 -90  firewalld nat_PREROUTING
```

## Why this isn't done with firewalld

firewalld's `filter_FORWARD` has `ct status dnat accept` — all DNAT traffic
slips past the zones. That is why 80/443 are absent from
`filter_IN_public_allow`, yet 443 is open from outside. A rich rule in the
`public` zone won't close that hole.

## Contents

| File | Purpose |
|---|---|
| `nftables/blackwall-base.nft` | the host's main firewall, replaces firewalld |
| `nftables/blackwall-wall.nft` | Cloudflare filter for 80/443 (prerouting) |
| `nftables/sysconfig-nftables.conf` | what `nftables.service` loads (`/etc/sysconfig/nftables.conf`) |
| `systemd/nftables.service.d/blackwall.conf` | drop-in: removes `flush ruleset` from reload/stop |
| `bin/blackwall-cf-refresh` | refreshes CF prefixes in nft and nginx `real_ip` |
| `systemd/blackwall-cf-refresh.{service,timer}` | weekly refresh run |
| `nginx/cf-realip.conf` | `set_real_ip_from` snapshot |

Both tables are loaded by `nftables.service` via `include`. There is no
separate unit for `blackwall_wall` — it would be a second owner of the same table.

## Installation

The host currently runs firewalld, so installing means migrating. Step-by-step
procedure with a safety net and rollback: **[MIGRATION.md](MIGRATION.md)**.

Before starting, put your own IP into `ADMIN_ALLOW`
(`/etc/default/blackwall-cf-refresh`) — otherwise your direct origin access for
deploys and debugging disappears along with everyone else's.

## Verification

```bash
# counters must grow on accept, not only on drop
sudo nft list chain inet blackwall_wall prerouting

# the site through Cloudflare — must work
curl -sI https://fehu.dev | head -1

# straight to the origin from a foreign address — must hang until timeout
curl -sI --max-time 5 --resolve fehu.dev:443:93.115.20.111 https://fehu.dev

# who is knocking directly
sudo journalctl -k -g 'blackwall-wall' -n 50
```

`origin-direct` in the logs is a ready-made event source for blackwall-eye.

## Rollback

Only the Cloudflare filter (the base firewall stays):

```bash
sudo nft delete table inet blackwall_wall
```

Full rollback to firewalld — see [MIGRATION.md](MIGRATION.md).

## Don'ts

- **Don't remove `ct status dnat accept` from forward** in `blackwall-base.nft`.
  Verified: without it, 443 via DNAT stops answering immediately.
- **Don't install `nftables.service` without the drop-in**
  `nftables.service.d/blackwall.conf`. The stock `ExecReload`/`ExecStop` run
  `flush ruleset` and wipe netavark's tables along with ours — container
  networking dies until `systemctl restart podman`.
- **Don't define the same `define` in both .nft files.** They are included
  together, and `nft` fails with `redefinition of symbol`, rolling back the
  whole transaction. That is why the variables are named `BASE_WAN_IF` and `WALL_WAN_IF`.

## real_ip

Once the whitelist is on, all traffic arrives from CF addresses. Without
`real_ip`, nginx logs, `limit_req`/`limit_conn` and anything else that sees
`$remote_addr` treat Cloudflare as the client — breaking both incident analysis
and rate limiting.

nginx runs in a container here, so `nginx/cf-realip.conf` must go into the
directory mounted into the container as `/etc/nginx/conf.d`. The path and the
container name are set in `/etc/default/blackwall-cf-refresh` (`REALIP_OUT`,
`NGINX_CONTAINER`). While `NGINX_CONTAINER` is empty, the script skips this step.

Find the mounts:

```bash
podman ps --format '{{.Names}}\t{{.Ports}}'
podman inspect <container> --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{"\n"}}{{end}}'
```

## Still open

- **Port 9999 (zuzia) depends on an external address list.** The service
  publishes its IPs; if they change, monitoring silently stops collecting data
  until the `zuzia_v4` set is updated. Unlike the Cloudflare prefixes, there is
  no auto-update here.
- **SSH/22 is open to everyone** — by the owner's decision, key-only login.
  Make sure the config backs this up: `PasswordAuthentication no` and
  `PermitRootLogin prohibit-password` in `/etc/ssh/sshd_config`.
- **The second host 192.227.212.28** (ravencraft.dev) — left alone by the owner's decision.
- **Origin IP rotation.** The firewall closes access, but the IP itself has
  already leaked into CT logs and historical DNS — it stays known forever. The
  complete fix is to change the origin IP (or move to Cloudflare Tunnel, which
  has no inbound ports at all) and keep it behind the whitelist from day one.

## About Ansible

There is **no need** to backport the existing tables — there is nothing to
version: `ip nat`/`ip filter` are recreated by netavark on every container
start, and `inet firewalld` is firewalld's output, not a config.

After the migration it gets simpler: firewalld is gone, and the whole perimeter
is described by the files in this directory. A role just has to lay them out:

| Ansible resource | Manages |
|---|---|
| `ansible.builtin.template` | `blackwall-base.nft` (WAN_IF, PODMAN_NET, port list as variables) |
| `ansible.builtin.copy` + `systemd` | drop-in, timer, `blackwall-cf-refresh` |
| `containers.podman.podman_container` / quadlet | the nginx container and publishing 443 |

`blackwall-wall.nft` doesn't belong in Ansible — `blackwall-cf-refresh`
generates it on the host; the template and the script are versioned, not the result.

These belong in `blackwall-eye/ansible/roles/`, per the module isolation rule
in the root `CLAUDE.md`. While the host is manual, the directory lives here.
