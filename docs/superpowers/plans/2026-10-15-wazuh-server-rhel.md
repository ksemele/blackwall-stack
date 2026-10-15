# Wazuh server on RHEL (lab sentinel) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Indexer, manager and dashboard roles run on Rocky 9 next to podman/nginx and our firewall, bound to localhost, on a new lab node `black-sentinel-03`.

**Architecture:** One new lab node + a podman fixture; server roles gain a RedHat path and binding/heap variables; certificates become per server host; a `sentinel.yml` playbook composes firewall + stack; `bw-check sentinel` proves isolation and coexistence.

**Tech Stack:** Ansible, Wazuh 4.14.7 (pinned), podman, nftables, Docker lab.

**Spec:** `docs/superpowers/specs/2026-10-15-wazuh-server-rhel-design.md`

## Global Constraints

- `black-master-00` and its agents unchanged: `tests/bin/bw-check all` green after every task; its certificate bundle (`tests/.wazuh-certs/wazuh-certificates.tar`) must NOT be regenerated.
- Wazuh packages stay pinned (hold / yum exclude); every dnf install of a pinned package uses `disable_excludes: wazuh`.
- `ansible_facts['os_family']`, never `ansible_os_family`.
- Lab only. Never `bw-lab down`/`reset`; keep `black-dead-98` stopped. Nothing touches the prod host.
- Public repo: no real host identifiers (implementer rules grep). English; brief commits, no AI trailers; no push.

## Review Focus

- Re-provisioning the main lab stack after the cert playbook change must not regenerate or redeploy `black-master-00` certs — pinned in Task 2 (bundle checksum before/after).
- Sentinel endpoints must not be reachable on its lab IP — pinned in Task 5's verify (connect to 172.28.0.16:9200/55000/5601 must fail).
- `systemctl reload nftables` on the sentinel must keep podman's tables and the nginx container reachable — pinned in Task 5.
- Indexer on 127.0.0.1 with certs for 127.0.0.1: dashboard and filebeat must connect without TLS errors — pinned by Task 4/5 checks (dashboard 302, alerts flowing).
- The sentinel must not appear in checks that mean "the main stack" (`wazuh_managers`) — pinned by keeping it in its own group (Task 1).

---

### Task 1: lab node `black-sentinel-03` + podman nginx fixture (spike first)

**Files:** `tests/docker/docker-compose.yml` (service), `tests/docker/ssh_config`, `tests/inventory/hosts.yml` (`rhel9` + new group `sentinel`), `tests/inventory/group_vars/sentinel.yml`, `tests/triggers/sentinel-podman.yml`, `tests/bin/bw-lab` (target `sentinel`, fixture part only for now), `tests/README.md` (host tables).

- [ ] **Step 1: Node.** Compose service `black-sentinel-03` (`<<: *black-node`, profile `ubuntu2204`, build `./images/rockylinux9`, `172.28.0.16`, ports `"2204:22"` only). Inventory: host in `rhel9` and in new group `sentinel` (`ansible_host: localhost`, `ansible_port: 2204`). `group_vars/sentinel.yml`: `nft_public_tcp: [22]` for now. `bw-lab up --no-provision` brings it up; `bw-lab ssh black-sentinel-03 'cat /etc/redhat-release'`.
- [ ] **Step 2: Spike — nested podman.** On the node: `dnf -y install podman` and `podman run -d --name web -p 443:80 docker.io/library/nginx:1.27-alpine`; `curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:443` → 200; `nft list tables` shows netavark tables. If it fails, try `--network host`-free fixes in this order: storage driver `vfs` (`/etc/containers/storage.conf`), `cgroup_manager = "cgroupfs"` + `events_logger = "file"` in `/etc/containers/containers.conf`. Record exactly what was needed.
- [ ] **Step 3: Fixture playbook** `tests/triggers/sentinel-podman.yml` (hosts `sentinel`) reproducing Step 2 idempotently (package, config files only if Step 2 needed them, container via `containers.podman.podman_container` if the collection is installed, else `command` with `creates`-style guard and `--replace`), plus a systemd unit or `--restart=always` so it survives restarts. `bw-lab provision sentinel` runs it.
- [ ] **Fallback (only if nested podman cannot work):** the fixture runs nginx as a host service on 443 instead; record the reason in the report; Task 5 then sets no `nft_podman` and skips the podman-table check. This is a controller-visible ruling, not a silent change.
- [ ] **Step 4:** `bw-check all` still green (sentinel is in no checked group yet). Commit `feat(tests): sentinel lab node with podman nginx`.

### Task 2: per-host certificates + indexer on RHEL

**Files:** `blackwall-eye/ansible/playbooks/wazuh-certificates.yml`, `roles/wazuh-indexer/{tasks,defaults,templates}`, `tests/inventory/group_vars/sentinel.yml`.

- [ ] **Step 1: Certs per host.** Make the playbook run per host of a target group (`hosts: "{{ wazuh_cert_hosts | default('wazuh_managers') }}"`, no `run_once`), node name `inventory_hostname`, IP `wazuh_cert_ip` (default `172.28.0.10`-compatible behaviour for the main stack: keep today's values when unset), bundle path `wazuh_certs_local_dir` (main stack unchanged: `tests/.wazuh-certs`; sentinel: `tests/.wazuh-certs/black-sentinel-03` via group_vars). Red/green: record `shasum tests/.wazuh-certs/wazuh-certificates.tar` before; run `bw-lab provision eye`; checksum unchanged.
- [ ] **Step 2: Indexer RedHat path.** dnf install `wazuh-indexer{{ wazuh_pkg_suffix }}` with `disable_excludes: wazuh`; prerequisites per family; drop the role's own dpkg-lock copy (wazuh-repo does it); `vm.max_map_count` if the role sets it. `wazuh_indexer_heap` (JVM options file `/etc/wazuh-indexer/jvm.options` `-Xms/-Xmx`; default = untouched behaviour; sentinel `1g`). Sentinel vars: `wazuh_indexer_network_host: 127.0.0.1`, `wazuh_cert_ip: 127.0.0.1`.
- [ ] **Step 3:** temporary play (or `sentinel.yml` skeleton with certs + indexer only) on the sentinel: indexer `active`, `curl -sk -u admin:<lab pw> https://127.0.0.1:9200` → 200 from inside; `bw-check all` green on the main stack. Commit `feat(eye): wazuh indexer on rhel, per-host certificates`.

### Task 3: manager + filebeat on RHEL, API on localhost

**Files:** `roles/wazuh-manager/{tasks,defaults,templates}`.

- [ ] dnf installs (`wazuh-manager`, filebeat — find the rpm version string matching deb `7.10.2-2`, make it a family-aware var) with `disable_excludes: wazuh`; drop dpkg-lock copy. New `wazuh_api_host` (default `0.0.0.0`, sentinel `127.0.0.1`) rendered into the API config (`/var/ossec/api/configuration/api.yaml` `host:`); filebeat output to `wazuh_indexer_network_host`. Verify on sentinel: manager + filebeat active, `curl -sk https://127.0.0.1:55000/` answers (401/200), filebeat `test output` OK, an alert from the sentinel's own agent 000 appears in its indexer. Main stack green. Commit `feat(eye): wazuh manager on rhel, api bind`.

### Task 4: dashboard on RHEL, localhost:5601

**Files:** `roles/wazuh-dashboard/{tasks,defaults,templates}`.

- [ ] dnf install with `disable_excludes: wazuh`; drop dpkg-lock copy; sentinel vars `wazuh_dashboard_listen_host: 127.0.0.1`, `wazuh_dashboard_listen_port: 5601`, `wazuh_indexer_url: https://127.0.0.1:9200`, `wazuh_manager_api_url: https://127.0.0.1:55000`. The role's "wait for 302" must use the configured host/port. Verify: from inside the sentinel `curl -sk -o /dev/null -w '%{http_code}' https://127.0.0.1:5601` → 302. Main stack green. Commit `feat(eye): wazuh dashboard on rhel`.

### Task 5: `sentinel.yml`, firewall, checks, footprint

**Files:** `ansible/playbooks/sentinel.yml`, `ansible/playbooks/verify-sentinel.yml`, `tests/bin/bw-lab` (`sentinel` = fixture + playbook), `tests/bin/bw-check` (`sentinel`, opt-in SKIP when not deployed), `tests/inventory/group_vars/sentinel.yml` (`nft_podman` per Task 1 outcome, `nft_restricted` for 1514/1515 from `wazuh_agent_sources: [172.28.0.0/24]`, `cfwall_wan_if: eth0`), `tests/README.md`.

- [ ] `sentinel.yml`: certs (`wazuh_cert_hosts: sentinel`) → `nftables` → indexer → manager → dashboard → `cf-wall` (+ the wazuh-agent is not needed: the manager is agent 000).
- [ ] `verify-sentinel.yml` asserts: services active (indexer, manager, filebeat, dashboard, nftables, blackwall-cf-refresh.timer); from inside: 9200/55000/5601 answer on 127.0.0.1; from `black-master-00`: TCP connect to 172.28.0.16 on 9200, 55000, 5601 fails; agent 000 active (`agent_control -l`); nginx answers on 443 locally; after `systemctl reload nftables` the netavark tables still exist and nginx still answers (skip podman parts per Task 1 fallback); Wazuh packages held/excluded.
- [ ] Footprint: `docker stats --no-stream black-sentinel-03` and `ps -o rss,comm` for indexer/manager/dashboard/filebeat after 10 min idle; put the numbers in the report (controller copies them into STATUS).
- [ ] `bw-check sentinel` PASS; `bw-check all` still green; second `bw-lab provision sentinel` changed=0 except known always-changed tasks (list them). Commit `feat(eye): sentinel playbook and checks`.

### Task 6: docs + STATUS (controller)

READMEs (RHEL server support, binding variables, per-host certs, sentinel, SSH-tunnel access `ssh -L 5601:127.0.0.1:5601`), `blackwall-eye/STATUS.md` (Done, footprint numbers, next_task).
