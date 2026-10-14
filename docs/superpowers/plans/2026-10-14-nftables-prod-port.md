# nftables prod port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The prod firewall (base table + Cloudflare origin filter) is reproducible from two Ansible roles, with host values only in a git-ignored local inventory.

**Architecture:** Extend role `nftables` (RHEL support, service drop-in, optional podman rules, named restricted sets, forward-drop logging) and add role `cf-wall` (deploys `blackwall-cf-refresh` + timer + config; the script generates `blackwall-wall.nft` on the host). A local tool `nft-equiv` proves a rendered ruleset equals a reference file by comparing nft's own listings in a throwaway container.

**Tech Stack:** Ansible, nftables, systemd, bash, Docker (lab + throwaway alpine container).

**Spec:** `docs/superpowers/specs/2026-10-14-nftables-prod-port-design.md`

## Global Constraints

- The public repo must not contain real hostnames, domains, IPs, PTRs or address lists of the owner's hosts. Use RFC 5737 (`192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`) / RFC 2606 (`example.com`) placeholders. Before every commit: `git diff --cached | grep -iE '<identifiers the controller names>'` must print nothing.
- The prod host directory is referred to as `hosts/<prod-host>/` in anything committed. `hosts/` is git-ignored.
- Never run anything against the prod host. Lab only (`tests/bin/bw-lab`, `tests/bin/bw-check`).
- English in all files and commit messages. Commits: super-brief, conventional prefix (`feat(eye):`, `fix(eye):`, `docs(eye):`), no AI/Claude trailers, branch `claude-dev`, no push.
- Role variable defaults must keep the lab ruleset unchanged (`nft_podman: {}`, `nft_forward_policy: accept`).
- Commands that need lab secrets run through `direnv exec .` from the repo root; cf-wall needs none.

## Review Focus

- Lab forward chain with policy `accept` must not gain a log rule or trailing counter (it would log every forwarded packet) — pinned by the `default` fixture in Task 2.
- A restricted entry without `name` keeps the set name `src_<port>` — pinned by the `default` fixture in Task 2.
- A Cloudflare fetch that returns junk must abort without touching the live `blackwall_wall` file — pinned in `verify-cf-wall.yml` (Task 4).
- Re-running the `cf-wall` role on an installed host changes nothing and does not refetch — pinned by the idempotence step in Task 4.
- `systemctl reload/stop nftables` must not flush tables the roles do not own (Docker NAT in the lab: container DNS keeps resolving) — pinned in `verify-nftables.yml` (Task 1).

---

### Task 0: RHEL-family host in the lab

The prod host is RHEL family; the lab is Ubuntu only. Add one Rocky Linux 9
node so the RedHat paths of both roles (`/etc/sysconfig/nftables.conf`, `dnf`)
run for real.

**Files:**
- Create: `tests/docker/images/rockylinux9/Dockerfile`
- Create: `tests/docker/images/rockylinux9/entrypoint.sh` (same as the ubuntu one, systemd path `/usr/lib/systemd/systemd`)
- Modify: `tests/docker/docker-compose.yml` (service `black-rocky-02`)
- Modify: `tests/inventory/hosts.yml` (group `rhel9`)
- Create: `tests/inventory/group_vars/rhel9.yml`
- Modify: any playbook with `hosts: all` that fails on the new node for apt-only reasons (see Step 4)
- Modify: `tests/README.md`, root `README.md` host table

**Interfaces:**
- Produces: host `black-rocky-02` (172.28.0.15, SSH on host port 2203), group `rhel9`. Not a Wazuh agent.

- [ ] **Step 1: Image** — `tests/docker/images/rockylinux9/Dockerfile`:

```dockerfile
FROM geerlingguy/docker-rockylinux9-ansible:latest

# Rocky Linux 9 with systemd as PID 1 + sshd, mirroring images/ubuntu2204.
# Root password 'blackwall' and PermitRootLogin yes: LOCAL TEST ENVIRONMENT ONLY.
RUN dnf -y install openssh-server iproute \
  && dnf clean all \
  && ssh-keygen -A \
  && echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config \
  && echo 'PasswordAuthentication yes' >> /etc/ssh/sshd_config \
  && echo 'root:blackwall' | chpasswd \
  && systemctl enable sshd

EXPOSE 22

COPY entrypoint.sh /entrypoint.sh
STOPSIGNAL SIGRTMIN+3
ENTRYPOINT ["/entrypoint.sh"]
```

`entrypoint.sh` (chmod +x):

```bash
#!/bin/bash
# See images/ubuntu2204/entrypoint.sh: shadow a non-cgroup2 /sys/fs/cgroup.
mount -t cgroup2 none /sys/fs/cgroup 2>/dev/null || true
exec /usr/lib/systemd/systemd "$@"
```

- [ ] **Step 2: Compose + inventory**

`docker-compose.yml`, after `black-dead-98`:

```yaml
  # RHEL-family node: exercises the RedHat paths of the firewall roles.
  black-rocky-02:
    <<: *black-node
    profiles: [ubuntu2204]
    build:
      context: ./images/rockylinux9
    container_name: black-rocky-02
    hostname: black-rocky-02
    ports:
      - "2203:22"
    networks:
      blackwall-test:
        ipv4_address: 172.28.0.15
```

`tests/inventory/hosts.yml`, under `all.children` next to `ubuntu2204`:

```yaml
    rhel9:
      hosts:
        black-rocky-02:
          ansible_host: localhost
          ansible_port: 2203
```

`tests/inventory/group_vars/rhel9.yml`:

```yaml
---
# RHEL-family lab node (Rocky Linux 9). Firewall only — not a Wazuh agent.
nft_public_tcp: [22, 9100]
```

(Add `node_exporter_install_method: tarball` here only if `metrics.yml` fails on this host with the apt method.)

- [ ] **Step 3: Bring it up**

Run: `tests/bin/bw-lab up --no-provision && ssh -F tests/docker/ssh_config black-rocky-02 true` (or `tests/bin/bw-lab ssh black-rocky-02 'cat /etc/redhat-release'`).
Expected: `Rocky Linux release 9.x`.

- [ ] **Step 4: Full provision + checks**

Run: `tests/bin/bw-lab provision all && tests/bin/bw-check all`
Expected: the new node gets `nftables` and `node_exporter`; no FAIL. If a `hosts: all` playbook (`verify-versions.yml`, `verify-ports.yml`, `metrics.yml`, `verify-metrics.yml`) fails on `black-rocky-02` for an apt/Ubuntu-only reason, make it OS-aware the smallest way (skip the apt-only assertion when `ansible_os_family != 'Debian'`, or scope the play to `ubuntu2204` when it is Wazuh-specific). Do not touch the `nftables` role in this task: if it fails on Rocky (`apt` task), that is the red test for Task 1 — record it and limit `provision all`/`bw-check` to work around nothing.

- [ ] **Step 5: Docs** — `tests/README.md` host table and root `README.md` host table gain `black-rocky-02 | Rocky Linux 9, firewall roles on the RHEL family`.

- [ ] **Step 6: Commit**

```bash
git add tests README.md blackwall-*/ansible/playbooks
git commit -m "feat(tests): rocky linux 9 node in the lab"
```

---

### Task 1: nftables role — OS family config path + service drop-in

**Files:**
- Modify: `blackwall-eye/ansible/roles/nftables/defaults/main.yml`
- Modify: `blackwall-eye/ansible/roles/nftables/tasks/main.yml`
- Modify: `blackwall-eye/ansible/roles/nftables/handlers/main.yml`
- Create: `blackwall-eye/ansible/roles/nftables/templates/nftables-dropin.conf.j2`
- Modify: `blackwall-eye/ansible/playbooks/verify-nftables.yml`

**Interfaces:**
- Produces: variable `nft_conf_path`; drop-in `/etc/systemd/system/nftables.service.d/blackwall.conf`; handler names `Reload systemd`, `Reload nftables` (cf-wall does not use them).

- [ ] **Step 1: Write the failing check** — append to `verify-nftables.yml` after "Report open ports":

```yaml
    - name: Read nftables.service ExecStop / ExecReload
      ansible.builtin.command: systemctl show nftables -p ExecStop -p ExecReload
      register: nft_unit
      changed_when: false

    - name: Assert the unit never flushes the whole ruleset
      ansible.builtin.assert:
        that: "'flush' not in nft_unit.stdout"
        fail_msg: >-
          nftables.service still runs `flush ruleset` on reload/stop on
          {{ inventory_hostname }} — it would wipe tables the role does not own
          (Docker NAT in the lab, netavark on podman hosts)

    # Docker's embedded DNS lives in the container's `ip nat` table. A global
    # flush kills it; resolving another container's name proves it survived.
    - name: Reload nftables
      ansible.builtin.systemd:
        name: nftables
        state: reloaded

    - name: Stop and start nftables
      ansible.builtin.shell: systemctl stop nftables && systemctl start nftables
      changed_when: false

    - name: Container DNS still resolves after reload/stop
      ansible.builtin.command: >-
        getent hosts {{ 'black-host-01' if inventory_hostname == 'black-master-00' else 'black-master-00' }}
      changed_when: false
```

- [ ] **Step 2: Run it, expect FAIL at the new assert** (it stops before reload/stop, so the lab is not damaged)

Run: `tests/bin/bw-check nft`
Expected: `FAIL ruleset does not match expectations`, output contains `still runs \`flush ruleset\``.

- [ ] **Step 3: Implement**

`defaults/main.yml` — append:

```yaml

# Main nftables config loaded by nftables.service (differs per OS family)
nft_conf_path: "{{ '/etc/sysconfig/nftables.conf' if ansible_os_family == 'RedHat' else '/etc/nftables.conf' }}"
```

`templates/nftables-dropin.conf.j2`:

```ini
# Managed by the Ansible role blackwall-eye/nftables. Do not edit by hand.
#
# The stock nftables.service runs `flush ruleset` on reload and stop (Debian
# and RHEL alike). That wipes tables this role does not own: Docker's NAT
# (embedded DNS 127.0.0.11) in containers, netavark's `ip nat` / `ip filter`
# on podman hosts. Reload re-reads our config instead; stop removes only our
# tables. Our .nft files use the create / delete / define idiom, so a reload
# is atomic per table.

[Service]
ExecReload=
ExecReload=/usr/sbin/nft -f {{ nft_conf_path }}

ExecStop=
ExecStop=-/usr/sbin/nft delete table inet blackwall_wall
ExecStop=-/usr/sbin/nft delete table inet blackwall_base
```

`tasks/main.yml`:
- replace the `ansible.builtin.apt` task with:

```yaml
- name: Install nftables
  ansible.builtin.package:
    name: nftables
    state: present
```

- change `Install nftables.conf` `dest:` to `"{{ nft_conf_path }}"`;
- add before `Enable and start nftables`:

```yaml
- name: Create nftables.service drop-in directory
  ansible.builtin.file:
    path: /etc/systemd/system/nftables.service.d
    state: directory
    owner: root
    group: root
    mode: "0755"

- name: Install nftables.service drop-in (no global flush)
  ansible.builtin.template:
    src: nftables-dropin.conf.j2
    dest: /etc/systemd/system/nftables.service.d/blackwall.conf
    owner: root
    group: root
    mode: "0644"
  notify:
    - Reload systemd
    - Reload nftables

- name: Apply the drop-in before the service is touched
  ansible.builtin.meta: flush_handlers
```

`handlers/main.yml` (order matters — systemd first):

```yaml
---
- name: Reload systemd
  ansible.builtin.systemd:
    daemon_reload: true

- name: Reload nftables
  ansible.builtin.systemd:
    name: nftables
    state: reloaded
```

- [ ] **Step 4: Apply and verify**

Run: `tests/bin/bw-lab provision nftables && tests/bin/bw-check nft`
Expected: `PASS ruleset applied, expected ports are open` — including `black-rocky-02` (RedHat path `/etc/sysconfig/nftables.conf`; check it with `tests/bin/bw-lab ssh black-rocky-02 'cat /etc/sysconfig/nftables.conf'`).
Then: `tests/bin/bw-check all` — no new FAIL (voice/brain may SKIP).

- [ ] **Step 5: Commit**

```bash
git add blackwall-eye/ansible/roles/nftables blackwall-eye/ansible/playbooks/verify-nftables.yml
git commit -m "fix(eye): nftables drop-in without global flush; RHEL config path"
```

---

### Task 2: nft-equiv tool + render entry point

**Files:**
- Create: `blackwall-eye/ansible/roles/nftables/tasks/render.yml`
- Modify: `blackwall-eye/ansible/roles/nftables/tasks/main.yml` (use render.yml)
- Create: `blackwall-eye/tools/nft-render.yml`
- Create: `blackwall-eye/tools/nft-equiv`
- Create: `blackwall-eye/tools/testdata/inventory.yml`
- Create: `blackwall-eye/tools/testdata/default.nft`

**Interfaces:**
- Consumes: role `nftables` template `blackwall-base.nft.j2`.
- Produces: `tools/nft-equiv <inventory> <host> <reference.nft>` (exit 0 = equal, 1 = differ, 2 = usage/setup error); `tasks/render.yml` taking `nft_render_dest`; fixture inventory hosts `default` (Task 2) and `podman` (Task 3).

- [ ] **Step 1: render entry point**

`roles/nftables/tasks/render.yml`:

```yaml
---
# Renders blackwall_base to nft_render_dest. Used by main.yml (staging file on
# the host) and by tools/nft-equiv (a local file).
- name: Render blackwall_base ruleset
  ansible.builtin.template:
    src: blackwall-base.nft.j2
    dest: "{{ nft_render_dest }}"
    mode: "0600"
  changed_when: false
```

In `tasks/main.yml` replace the task `Render ruleset to a staging path` with:

```yaml
- name: Render ruleset to a staging path
  ansible.builtin.include_tasks: render.yml
  vars:
    nft_render_dest: /etc/nftables.d/.blackwall-base.nft.new
```

`tools/nft-render.yml`:

```yaml
---
# Local render of the nftables role template for one inventory host.
# Role defaults keep their usual (lowest) precedence, so inventory values win.
- name: Render blackwall_base for one host
  hosts: "{{ target }}"
  gather_facts: false
  tasks:
    - name: Render
      ansible.builtin.include_role:
        name: nftables
        tasks_from: render
```

- [ ] **Step 2: fixtures**

`tools/testdata/inventory.yml`:

```yaml
# Fixture hosts for tools/nft-equiv. Placeholder values only (RFC 5737).
all:
  hosts:
    default:
      nft_public_tcp: [22, 9100]
      nft_restricted:
        - port: 9999
          sources: [192.0.2.10]
```

`tools/testdata/default.nft` — what the current template must produce for `default` (lab-style: forward policy accept, no podman):

```
table inet blackwall_base
delete table inet blackwall_base

table inet blackwall_base {
    set public_tcp {
        type inet_service
        elements = { 22, 9100 }
    }

    set src_9999 {
        type ipv4_addr
        flags interval
        auto-merge
        elements = { 192.0.2.10 }
    }

    chain input {
        type filter hook input priority filter; policy drop;
        iif lo accept
        ct state established,related accept
        ct state invalid drop
        meta l4proto { icmp, ipv6-icmp } accept
        ip6 daddr fe80::/64 udp dport 546 accept
        tcp dport @public_tcp ct state new counter accept
        tcp dport 9999 ip saddr @src_9999 ct state new counter accept
        limit rate 5/minute log prefix "[blackwall-base] input-drop " level info
        counter
    }

    chain forward {
        type filter hook forward priority filter; policy accept;
        ct state established,related accept
        ct state invalid drop
    }

    chain output {
        type filter hook output priority filter; policy accept;
    }
}
```

- [ ] **Step 3: the tool**

`tools/nft-equiv` (chmod +x):

```bash
#!/usr/bin/env bash
#
# nft-equiv — does the nftables role render the same blackwall_base table as a
# reference .nft file?
#
#   tools/nft-equiv <inventory> <host> <reference.nft>
#
# Renders the role template for <host> locally, loads both files one after the
# other into a throwaway container (its own network namespace) and diffs
# `nft list table inet blackwall_base`. nft's listing is the normal form:
# comments, defines, whitespace and set element order do not matter.
# Exit: 0 equal, 1 different, 2 usage or setup error.

set -euo pipefail

[ $# -eq 3 ] || { echo "usage: $0 <inventory> <host> <reference.nft>" >&2; exit 2; }
inv="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
host="$2"
ref="$(cd "$(dirname "$3")" && pwd)/$(basename "$3")"
here="$(cd "$(dirname "$0")/.." && pwd)"   # blackwall-eye/

work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
cp "$ref" "$work/reference.nft"

( cd "$here" && ansible-playbook -i "$inv" tools/nft-render.yml \
    -e target="$host" -e nft_render_dest="$work/rendered.nft" \
    -e ansible_connection=local \
    -e 'ansible_python_interpreter={{ ansible_playbook_python }}' >/dev/null ) \
  || { echo "render failed for host $host" >&2; exit 2; }

docker run --rm --cap-add NET_ADMIN -v "$work:/w" alpine:3.20 sh -euc '
  apk add -q nftables >/dev/null
  nft -f /w/reference.nft
  nft list table inet blackwall_base > /w/reference.list
  nft delete table inet blackwall_base
  nft -f /w/rendered.nft
  nft list table inet blackwall_base > /w/rendered.list
' || { echo "nft could not load one of the files" >&2; exit 2; }

if diff -u "$work/reference.list" "$work/rendered.list"; then
  echo "equal: $host renders the same blackwall_base as $(basename "$ref")"
else
  exit 1
fi
```

- [ ] **Step 4: Run — expect PASS against the current template**

Run (from repo root): `blackwall-eye/tools/nft-equiv blackwall-eye/tools/testdata/inventory.yml default blackwall-eye/tools/testdata/default.nft`
Expected: `equal: default renders ...`, exit 0.
Sanity: change `9100` to `9101` in a copy of the fixture, run against the copy, expect a diff and exit 1; delete the copy.

- [ ] **Step 5: lab unchanged**

Run: `tests/bin/bw-lab provision nftables && tests/bin/bw-check nft`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add blackwall-eye/ansible/roles/nftables/tasks blackwall-eye/tools
git commit -m "feat(eye): nft-equiv tool and render entry point"
```

---

### Task 3: nftables template — podman rules, named restricted sets, forward drop logging

**Files:**
- Modify: `blackwall-eye/ansible/roles/nftables/defaults/main.yml`
- Modify: `blackwall-eye/ansible/roles/nftables/templates/blackwall-base.nft.j2`
- Modify: `blackwall-eye/tools/testdata/inventory.yml`
- Create: `blackwall-eye/tools/testdata/podman.nft`

**Interfaces:**
- Consumes: `tools/nft-equiv` (Task 2).
- Produces: variables `nft_podman: {net, if}`, `nft_restricted[].name`.

- [ ] **Step 1: failing fixture** — add host `podman` to `testdata/inventory.yml`:

```yaml
    podman:
      nft_wan_if: eth0
      nft_public_tcp: [22]
      nft_restricted:
        - port: 9999
          name: monitor_v4
          sources: [192.0.2.10, 198.51.100.0/24]
      nft_podman:
        net: 10.88.0.0/16
        if: podman0
      nft_forward_policy: drop
```

`testdata/podman.nft` — the prod structure with placeholder values:

```
table inet blackwall_base
delete table inet blackwall_base

table inet blackwall_base {
    set public_tcp {
        type inet_service
        elements = { 22 }
    }

    set monitor_v4 {
        type ipv4_addr
        flags interval
        auto-merge
        elements = { 192.0.2.10, 198.51.100.0/24 }
    }

    chain input {
        type filter hook input priority filter; policy drop;
        iif lo accept
        ct state established,related accept
        ct state invalid drop
        iifname "eth0" ip saddr 10.88.0.0/16 counter drop
        iifname "podman0" ip saddr 10.88.0.0/16 udp dport 53 counter accept
        iifname "podman0" ip saddr 10.88.0.0/16 tcp dport 53 counter accept
        meta l4proto { icmp, ipv6-icmp } accept
        ip6 daddr fe80::/64 udp dport 546 accept
        tcp dport @public_tcp ct state new counter accept
        tcp dport 9999 ip saddr @monitor_v4 ct state new counter accept
        limit rate 5/minute log prefix "[blackwall-base] input-drop " level info
        counter
    }

    chain forward {
        type filter hook forward priority filter; policy drop;
        ct state established,related accept
        ct state invalid drop
        ct status dnat counter accept
        oifname "eth0" ip saddr 10.88.0.0/16 counter accept
        limit rate 5/minute log prefix "[blackwall-base] fwd-drop " level info
        counter
    }

    chain output {
        type filter hook output priority filter; policy accept;
    }
}
```

- [ ] **Step 2: Run, expect FAIL (exit 1, diff shows missing podman rules and set name)**

Run: `blackwall-eye/tools/nft-equiv blackwall-eye/tools/testdata/inventory.yml podman blackwall-eye/tools/testdata/podman.nft`

- [ ] **Step 3: Implement**

`defaults/main.yml` — document `name` in the `nft_restricted` comment and append:

```yaml

# podman host: container subnet and bridge. When set, the base table adds
# anti-spoofing, container DNS to the host and forwarding for published ports.
#   nft_podman:
#     net: 10.88.0.0/16
#     if: podman0
nft_podman: {}
```

(`nft_restricted` comment example gains `name: monitor_v4   # optional, default src_<port>`.)

`blackwall-base.nft.j2` — replace from `set public_tcp` to the end of `chain forward` with:

```jinja
    set public_tcp {
        type inet_service
        elements = { {{ nft_public_tcp | join(', ') }} }
    }
{% for r in nft_restricted %}

    set {{ r.name | default('src_' ~ r.port) }} {
        type ipv4_addr
        flags interval
        auto-merge
        elements = { {{ r.sources | join(', ') }} }
    }
{% endfor %}

    chain input {
        type filter hook input priority filter; policy drop;

        # Scaffolding: without these rules the host locks itself out.
        iif lo accept
        ct state established,related accept
        ct state invalid drop
{% if nft_podman %}

        # Anti-spoofing: container subnet addresses cannot arrive from outside.
        # Must come before the rules below that trust the subnet.
        iifname "{{ nft_wan_if }}" ip saddr {{ nft_podman.net }} counter drop

        # DNS from containers to aardvark-dns on the host. netavark accepts it in
        # its own table, but our policy drop here kills it regardless — without
        # these lines containers silently cannot resolve names.
        iifname "{{ nft_podman.if }}" ip saddr {{ nft_podman.net }} udp dport 53 counter accept
        iifname "{{ nft_podman.if }}" ip saddr {{ nft_podman.net }} tcp dport 53 counter accept
{% endif %}

        # All of ICMP: dropping icmpv6 breaks IPv6 (neighbor discovery, PMTU),
        # dropping icmp breaks path MTU discovery on IPv4.
        meta l4proto { icmp, ipv6-icmp } accept
        ip6 daddr fe80::/64 udp dport 546 accept

        tcp dport @public_tcp ct state new counter accept
{% for r in nft_restricted %}
        tcp dport {{ r.port }} ip saddr @{{ r.name | default('src_' ~ r.port) }} ct state new counter accept
{% endfor %}
{% if nft_log_drops %}

        limit rate 5/minute log prefix "[blackwall-base] input-drop " level info
{% endif %}
        counter
    }

    chain forward {
        type filter hook forward priority filter; policy {{ nft_forward_policy }};

        ct state established,related accept
        ct state invalid drop
{% if nft_podman %}

        # Published container ports arrive here after netavark's DNAT. Who may
        # reach them is decided earlier (e.g. blackwall_wall in prerouting).
        ct status dnat counter accept

        # Containers going out.
        oifname "{{ nft_wan_if }}" ip saddr {{ nft_podman.net }} counter accept
{% endif %}
{% if nft_forward_policy == 'drop' %}
{% if nft_log_drops %}

        limit rate 5/minute log prefix "[blackwall-base] fwd-drop " level info
{% endif %}
        counter
{% endif %}
    }
```

- [ ] **Step 4: Both fixtures pass**

Run:
```bash
blackwall-eye/tools/nft-equiv blackwall-eye/tools/testdata/inventory.yml podman blackwall-eye/tools/testdata/podman.nft
blackwall-eye/tools/nft-equiv blackwall-eye/tools/testdata/inventory.yml default blackwall-eye/tools/testdata/default.nft
```
Expected: both `equal`, exit 0.

- [ ] **Step 5: Lab unchanged**

Run: `tests/bin/bw-lab provision nftables && tests/bin/bw-check nft`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add blackwall-eye/ansible/roles/nftables blackwall-eye/tools/testdata
git commit -m "feat(eye): nftables podman rules, named restricted sets"
```

---

### Task 4: cf-wall role, playbooks, lab wiring

**Files:**
- Create: `blackwall-eye/ansible/roles/cf-wall/defaults/main.yml`
- Create: `blackwall-eye/ansible/roles/cf-wall/tasks/main.yml`
- Create: `blackwall-eye/ansible/roles/cf-wall/handlers/main.yml`
- Create: `blackwall-eye/ansible/roles/cf-wall/files/blackwall-cf-refresh`
- Create: `blackwall-eye/ansible/roles/cf-wall/files/blackwall-cf-refresh.service`
- Create: `blackwall-eye/ansible/roles/cf-wall/files/blackwall-cf-refresh.timer`
- Create: `blackwall-eye/ansible/roles/cf-wall/templates/blackwall-cf-refresh.default.j2`
- Create: `blackwall-eye/ansible/roles/cf-wall/README.md`
- Create: `blackwall-eye/ansible/playbooks/cf-wall.yml`
- Create: `blackwall-eye/ansible/playbooks/verify-cf-wall.yml`
- Modify: `tests/inventory/hosts.yml` (group `cf_wall`)
- Modify: `tests/bin/bw-lab` (target `cf-wall`)
- Modify: `tests/bin/bw-check` (check `cf-wall`)
- Modify: `tests/README.md`

**Interfaces:**
- Consumes: `/etc/nftables.d/*.nft` include and drop-in from the `nftables` role (Task 1).
- Produces: `/usr/local/sbin/blackwall-cf-refresh`, table `inet blackwall_wall` with sets `cf_v4`, `cf_v6`, `admin_v4`, chain `prerouting` (hook prerouting priority -150); group `cf_wall`.

- [ ] **Step 1: failing verify** — `ansible/playbooks/verify-cf-wall.yml`:

```yaml
---
- name: Verify the Cloudflare origin filter
  hosts: cf_wall
  become: true
  gather_facts: false

  vars:
    wall_file: /etc/nftables.d/blackwall-wall.nft

  tasks:
    - name: Dump table as JSON
      ansible.builtin.command: nft -j list table inet blackwall_wall
      register: wall_json
      changed_when: false

    - name: Collect sets and the prerouting chain
      ansible.builtin.set_fact:
        wall_sets: >-
          {{ (wall_json.stdout | from_json).nftables | selectattr('set', 'defined')
             | map(attribute='set') | list }}
        wall_chain: >-
          {{ (wall_json.stdout | from_json).nftables | selectattr('chain', 'defined')
             | map(attribute='chain') | selectattr('name', 'eq', 'prerouting') | first | default({}) }}

    - name: Assert Cloudflare prefix sets are filled and the hook is prerouting -150
      ansible.builtin.assert:
        that:
          - (wall_sets | selectattr('name', 'eq', 'cf_v4') | map(attribute='elem') | first | default([]) | length) >= 10
          - (wall_sets | selectattr('name', 'eq', 'cf_v6') | map(attribute='elem') | first | default([]) | length) >= 5
          - wall_chain.hook | default('') == 'prerouting'
          - wall_chain.prio | default(0) | int == -150
        fail_msg: "blackwall_wall on {{ inventory_hostname }} is incomplete: {{ wall_chain }}"

    - name: Generated file passes nft -c
      ansible.builtin.command: nft -c -f {{ wall_file }}
      changed_when: false

    - name: Timer is enabled and active
      ansible.builtin.command: systemctl is-enabled blackwall-cf-refresh.timer
      changed_when: false

    # A refresh that fetches junk must abort and leave the live rules alone.
    - name: Checksum before a bad refresh
      ansible.builtin.stat:
        path: "{{ wall_file }}"
        checksum_algorithm: sha256
      register: before

    - name: Refresh against a page that is not a prefix list
      ansible.builtin.command: /usr/local/sbin/blackwall-cf-refresh
      environment:
        V4_URL: https://example.com/
      register: bad_refresh
      changed_when: false
      failed_when: bad_refresh.rc == 0

    - name: Checksum after the bad refresh
      ansible.builtin.stat:
        path: "{{ wall_file }}"
        checksum_algorithm: sha256
      register: after

    - name: Assert the live file is untouched
      ansible.builtin.assert:
        that: before.stat.checksum == after.stat.checksum
        fail_msg: "a failed refresh changed {{ wall_file }}"
```

Lab wiring now (so the check can run):
- `tests/inventory/hosts.yml`, under `all.children` (one Ubuntu and one RHEL-family node):

```yaml
    # Cloudflare origin filter (cf-wall role). The lab serves nothing on
    # 80/443, so the filter is inert here; it proves deploy and refresh.
    cf_wall:
      hosts:
        black-host-01:
        black-rocky-02:
```

- `tests/bin/bw-lab`: usage/die lists gain `cf-wall`; new case after `voice)`:

```bash
    cf-wall)
      bw_info "provision: cf-wall (Cloudflare origin filter)"
      bw_ansible blackwall-eye ansible/playbooks/cf-wall.yml --limit "$(bw_live_limit)"
      ;;
```

- `tests/bin/bw-check`: usage line `  cf-wall    Cloudflare origin filter (cf-wall role)`; function after `check_nft`:

```bash
check_cf_wall() {
  bw_info "cf-wall"
  bw_require_ansible

  # Opt-in: `bw-lab provision all` leaves it out (it fetches from cloudflare.com).
  local installed
  installed="$(bw_ansible_adhoc blackwall-eye cf_wall -b \
                 -m stat -a 'path=/usr/local/sbin/blackwall-cf-refresh' 2>&1)"
  if ! grep -q '"exists": true' <<<"$installed"; then
    bw_skip "cf-wall is not deployed (run: bw-lab provision cf-wall)"
    return
  fi

  bw_run "Cloudflare filter loaded, refresh is safe" \
    bw_ansible blackwall-eye ansible/playbooks/verify-cf-wall.yml --limit "$BW_LIVE_HOSTS"
}
```

and `cf-wall) check_cf_wall ;;` in `main`, plus `check_cf_wall` after `check_nft` in `all`.

- [ ] **Step 2: Run, expect FAIL** — `tests/bin/bw-check cf-wall` → SKIP (not deployed). Then run the verify directly to see the red: `cd blackwall-eye && ansible-playbook ansible/playbooks/verify-cf-wall.yml` → fails at "Dump table as JSON" (no such table).

- [ ] **Step 3: Implement the role**

`files/blackwall-cf-refresh`: copy the script from the local prod directory the controller names, then:
1. header line `# Template source: ...` → `# Source: blackwall-eye/ansible/roles/cf-wall (Ansible-managed)`;
2. after `V6_URL=...` add:

```bash
# TCP ports the filter covers (space-separated).
PORTS="${PORTS:-80 443}"
```

3. before the `{` that starts writing the ruleset add:

```bash
ports_nft="$(printf '%s\n' $PORTS | awk 'NF' | paste -sd, - | sed 's/,/, /g')"
[ -n "$ports_nft" ] || die "PORTS is empty"
```

4. change `cat <<'CHAIN'` … `CHAIN` to `sed "s/@PORTS@/$ports_nft/g" <<'CHAIN'` … `CHAIN` and replace every `{ 80, 443 }` inside it with `{ @PORTS@ }`;
5. grep the result for host identifiers (Global Constraints) — none allowed.

`files/blackwall-cf-refresh.service` and `.timer`: copy the prod units unchanged (they carry no host data); verify with grep.

`templates/blackwall-cf-refresh.default.j2`:

```jinja
# Managed by the Ansible role blackwall-eye/cf-wall. Do not edit by hand.
# Read by /usr/local/sbin/blackwall-cf-refresh.

# Sources allowed straight to the origin on the filtered ports (space-separated).
ADMIN_ALLOW="{{ cfwall_admin_allow | join(' ') }}"

# External interface; empty = from the default route.
WAN_IF="{{ cfwall_wan_if }}"

# Filtered TCP ports.
PORTS="{{ cfwall_ports | join(' ') }}"

# nginx real_ip: container name and host path of cf-realip.conf inside the
# directory mounted as /etc/nginx/conf.d. Empty container = skip real_ip.
NGINX_CONTAINER="{{ cfwall_nginx_container }}"
{% if cfwall_realip_out %}
REALIP_OUT="{{ cfwall_realip_out }}"
{% endif %}

# Sanity floor: fewer fetched prefixes than this aborts the refresh.
MIN_V4={{ cfwall_min_v4 }}
MIN_V6={{ cfwall_min_v6 }}
```

`defaults/main.yml`:

```yaml
---
# TCP ports reachable only from Cloudflare (and cfwall_admin_allow)
cfwall_ports: [80, 443]

# Sources allowed straight to the origin (deploys, health checks)
cfwall_admin_allow: []

# External interface; empty = detect from the default route
cfwall_wan_if: ""

# nginx real_ip (container runtime: podman). Empty container = skip
cfwall_nginx_container: ""
cfwall_realip_out: ""

# Abort the refresh when Cloudflare returns fewer prefixes than this
cfwall_min_v4: 10
cfwall_min_v6: 5
```

`tasks/main.yml`:

```yaml
---
- name: Install curl
  ansible.builtin.package:
    name: curl
    state: present

- name: Require nftables from the nftables role
  ansible.builtin.stat:
    path: /etc/nftables.d
  register: nft_dir
  failed_when: not nft_dir.stat.exists

- name: Install blackwall-cf-refresh
  ansible.builtin.copy:
    src: blackwall-cf-refresh
    dest: /usr/local/sbin/blackwall-cf-refresh
    owner: root
    group: root
    mode: "0755"
  notify: Refresh Cloudflare filter

- name: Install refresh config
  ansible.builtin.template:
    src: blackwall-cf-refresh.default.j2
    dest: /etc/default/blackwall-cf-refresh
    owner: root
    group: root
    mode: "0644"
  notify: Refresh Cloudflare filter

- name: Install refresh service and timer
  ansible.builtin.copy:
    src: "{{ item }}"
    dest: "/etc/systemd/system/{{ item }}"
    owner: root
    group: root
    mode: "0644"
  loop:
    - blackwall-cf-refresh.service
    - blackwall-cf-refresh.timer
  notify: Reload systemd

- name: Run pending refresh and systemd reload now
  ansible.builtin.meta: flush_handlers

# Covers a host where the files were already in place but the rules were
# removed; on a fresh install the handler above has already created them.
- name: Generate the filter if it is missing
  ansible.builtin.command: /usr/local/sbin/blackwall-cf-refresh
  args:
    creates: /etc/nftables.d/blackwall-wall.nft

- name: Enable the weekly refresh timer
  ansible.builtin.systemd:
    name: blackwall-cf-refresh.timer
    enabled: true
    state: started
```

`handlers/main.yml`:

```yaml
---
- name: Reload systemd
  ansible.builtin.systemd:
    daemon_reload: true

- name: Refresh Cloudflare filter
  ansible.builtin.command: /usr/local/sbin/blackwall-cf-refresh
```

`ansible/playbooks/cf-wall.yml`:

```yaml
---
- name: Cloudflare origin filter
  hosts: cf_wall
  become: true
  gather_facts: true

  roles:
    - role: cf-wall
      tags: [cf-wall, firewall]
```

`roles/cf-wall/README.md`: purpose (only Cloudflare + `cfwall_admin_allow` reach `cfwall_ports` on the WAN interface; prerouting -150, why not input — DNAT to containers skips input), variables table (defaults above), how it runs (script generates `/etc/nftables.d/blackwall-wall.nft`, weekly timer, role refreshes only on change), the abort rule (fewer than `MIN_V4`/`MIN_V6` prefixes or `nft -c` failure = nothing applied), rollback (`nft delete table inet blackwall_wall`; `systemctl disable --now blackwall-cf-refresh.timer`), requires the `nftables` role on the same host. Placeholder addresses only.

- [ ] **Step 4: Apply, verify, idempotence**

Run:
```bash
tests/bin/bw-lab provision cf-wall
tests/bin/bw-check cf-wall
tests/bin/bw-lab provision cf-wall 2>&1 | grep -E 'black-host-01 +:'
```
Expected: `PASS Cloudflare filter loaded, refresh is safe` (both hosts); the second provision recap shows `changed=0` for `black-host-01` and `black-rocky-02`.
Then `tests/bin/bw-check nft` still PASS (reload of the base config includes the wall file).

- [ ] **Step 5: tests/README.md** — provision target list gains `cf-wall`; checks table gains `| \`cf-wall\` | table \`inet blackwall_wall\` loaded with Cloudflare prefixes, timer enabled, a refresh that fetches junk leaves the live rules untouched |`.

- [ ] **Step 6: Commit**

```bash
git add blackwall-eye/ansible/roles/cf-wall blackwall-eye/ansible/playbooks/cf-wall.yml blackwall-eye/ansible/playbooks/verify-cf-wall.yml tests
git commit -m "feat(eye): cf-wall role for the Cloudflare origin filter"
```

---

### Task 5: docs, STATUS, local prod inventory and equivalence (controller)

**Files:**
- Modify: `blackwall-eye/ansible/roles/nftables/README.md` if present, else the nftables section of `blackwall-eye/README.md` (new variables `nft_podman`, `nft_restricted[].name`, `nft_conf_path`, drop-in, RHEL)
- Modify: `blackwall-eye/README.md` (mention `cf-wall`, `tools/nft-equiv`)
- Modify: `blackwall-eye/STATUS.md` (`last_task`, `next_task`, Done entry; drop "Port hosts/.../nftables into the role" from next_task)
- Local only (git-ignored): `hosts/prod.yml`, a note in `hosts/<prod-host>/README.md`

- [ ] **Step 1: Docs + STATUS** — as listed above; placeholders only.

- [ ] **Step 2: Commit**

```bash
git add blackwall-eye
git commit -m "docs(eye): nftables podman/RHEL, cf-wall, nft-equiv"
```

- [ ] **Step 3: Local prod inventory** (not committed) — `hosts/prod.yml` with the prod host in groups `all` and `cf_wall`, values taken from the local prod files: `nft_wan_if`, `nft_public_tcp`, `nft_restricted` (named set for the monitoring port), `nft_podman`, `nft_forward_policy: drop`, `nft_log_drops: true`, `cfwall_*` from `/etc/default/blackwall-cf-refresh`.

- [ ] **Step 4: Equivalence against the live prod file**

Run: `blackwall-eye/tools/nft-equiv hosts/prod.yml <prod-host> hosts/<prod-host>/nftables/blackwall-base.nft`
Expected: `equal`. Any diff is fixed in the inventory (or reported to the owner when it is a real template gap) before the owner applies anything.

- [ ] **Step 5: Hand-off note** — tell the owner the exact commands to apply to prod (`ansible-playbook -i hosts/prod.yml ansible/playbooks/nftables.yml` then `cf-wall.yml`, both with `--check --diff` first) and the rollback commands.
