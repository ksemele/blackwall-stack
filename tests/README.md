# Blackwall — Test Environment

Docker environment shared by all Blackwall modules.

| Container | Role |
|---|---|
| `black-master-00` | Wazuh manager + indexer + dashboard (all-in-one) |
| `black-host-01` | Wazuh agent + auditd target |
| `black-dead-98` | agent fixture, stopped on purpose (`disconnected` status) |
| `black-rocky-02` | Rocky Linux 9, firewall roles on the RHEL family |
| `black-metrics-00` | Prometheus |
| `black-grafana-00` | Grafana |
| `black-aptcache-00` | apt-cacher-ng — apt cache for the nodes, lab only |

The `black-*-NN` nodes are privileged with systemd as PID 1, so service
management works identically to production hosts. Prometheus and Grafana are
plain containers configured entirely from files in this repository.

Two scripts drive everything:

| Script | Purpose |
|---|---|
| [`tests/bin/bw-lab`](bin/bw-lab) | lifecycle — up, down, reset, provision, ssh, logs |
| [`tests/bin/bw-check`](bin/bw-check) | verification — hosts, packages, nftables, ports, Wazuh, metrics |

> Run both from anywhere: they resolve every path from the repo root.
> Raw `docker` commands below assume the **repo root**; raw `ansible` commands
> assume the **module directory** (`blackwall-eye/`, `blackwall-voice/`), each of
> which has an `ansible.cfg` pointing at `../tests/inventory/`.

---

## 1. Quick start

```bash
# Containers + Wazuh + node_exporter + nftables
tests/bin/bw-lab up

# Verify everything: hosts, packages, firewall, ports, agents, metrics
tests/bin/bw-check all

# Open the UIs
tests/bin/bw-lab open dashboard     # https://localhost:5601  admin / Blackwall1!
tests/bin/bw-lab open grafana       # http://localhost:3000   admin / blackwall
tests/bin/bw-lab open prometheus    # http://localhost:9090

# Tear down (volumes included)
tests/bin/bw-lab down
```

`bw-lab up` takes roughly 10–20 minutes on the very first start: it builds the
images, then downloads ~1.5 GB of Wazuh packages over apt (indexer 832 MB,
manager 501 MB, dashboard 184 MB). Every later `up`/`reset` takes those packages
from the local apt cache — see [apt cache](#apt-cache).

To start from a clean slate — volumes gone, certificates regenerated:

```bash
tests/bin/bw-lab reset
```

---

## 2. `bw-lab` — environment lifecycle

| Command | What it does |
|---|---|
| `bw-lab up [--no-provision] [--profile P]` | `compose up -d --build`, wait for SSH, then provision |
| `bw-lab down [--keep-data]` | stop the environment; volumes are removed unless `--keep-data` |
| `bw-lab reset` | `down -v` + wipe Wazuh certificates + `up` |
| `bw-lab status` | containers, published ports, health of Prometheus/Grafana/dashboard |
| `bw-lab provision <target>` | ansible only: `eye`, `nftables`, `metrics`, `voice`, `cf-wall`, `brain`, `fixtures`, `all` |
| `bw-lab ssh <host> [cmd...]` | shell into a container (root password `blackwall`) |
| `bw-lab logs <service> [-f]` | compose logs for one service |
| `bw-lab open <target>` | open `dashboard`, `grafana` or `prometheus` in a browser |

`bw-lab provision all` runs, in this order:

1. `blackwall-eye/ansible/playbooks/all.yml` — certificates → indexer + manager + dashboard → agent + auditd
2. `blackwall-voice/ansible/playbooks/metrics.yml` — node_exporter on every host
3. `blackwall-eye/ansible/playbooks/nftables.yml` — firewall
4. `tests/triggers/broken-agents.yml` — fixtures: registers `black-ghost-99`, stops `black-dead-98`

nftables goes after the services on purpose: the firewall should cover services
that are already up rather than block their installation (apt, agent enrollment).
Fixtures go last because they stop `black-dead-98`; while it is stopped,
`provision eye|metrics|nftables` skip it automatically.

Environment variables: `BW_PROFILE` (compose profile, default `ubuntu2204`),
`BW_SSH_PASSWORD` (root password in the containers, default `blackwall`).

Examples:

```bash
tests/bin/bw-lab up --no-provision           # containers only, run ansible by hand
tests/bin/bw-lab provision nftables          # re-apply just the firewall
tests/bin/bw-lab ssh black-host-01 'nft list ruleset'
tests/bin/bw-lab logs black-metrics-00 -f
```

Install `sshpass` (`brew install sshpass`) if you want `bw-lab ssh` to stop
asking for the password.

---

## 3. `bw-check` — verification

```bash
tests/bin/bw-check all        # everything; exit code 0 only when nothing failed
tests/bin/bw-check metrics    # just one group
```

| Check | What it asserts |
|---|---|
| `hosts` | the five running containers exist (incl. the apt cache); ansible reaches every systemd node |
| `versions` | `wazuh-*`, `filebeat`, `auditd`, `nftables` are installed (prints versions); `wazuh-agent` version equals `wazuh-manager` |
| `nft` | `chain input` is `policy drop` and every expected port is in `set public_tcp` |
| `cf-wall` | table `inet blackwall_wall` loaded with Cloudflare prefixes, timer enabled, a refresh that fetches junk leaves the live rules untouched (opt-in: SKIP until `bw-lab provision cf-wall`) |
| `ports` | expected listeners inside the containers, plus published ports probed from the Mac |
| `wazuh` | agents registered on the manager, REST API answers, indexer answers, `black-ghost-99` is `never_connected` and `black-dead-98` is `disconnected`, dashboard responds |
| `metrics` | node_exporter serves metrics, Prometheus targets are `up`, `node_load1` returns data, Grafana is healthy with its datasource and dashboard |
| `voice` | `telegram-alerter` is running and Telegram accepted a synthetic alert |
| `brain` | `brain-analyst` is running; a synthetic level-10 alert was analysed by Claude and delivered to Telegram |

Every line is `PASS`, `FAIL` or `SKIP`; the exit code is 0 only when there is no
`FAIL`, so this is usable straight from CI. Output of a failing step is printed
underneath it, last 25 lines.

`black-dead-98` is excluded from host-level checks: the `broken-agents.yml`
fixture stops it deliberately.

---

## 4. Access

| Service | URL | Credentials |
|---|---|---|
| Wazuh dashboard | `https://localhost:5601` | `admin` / `Blackwall1!` |
| Grafana | `http://localhost:3000` | `admin` / `blackwall` |
| Prometheus | `http://localhost:9090` | none |
| Wazuh API (for `blackwall` TUI) | `https://127.0.0.1:55000` | `wazuh` / `wazuh` |
| Wazuh indexer (for blackwall :alerts) | `https://127.0.0.1:9200` | `admin` / `Blackwall1!` |
| SSH to any node | `tests/bin/bw-lab ssh <host>` | `root` / `blackwall` |

Wazuh dashboard and indexer use a self-signed certificate; browsers show
"Not Secure". Acceptable on `localhost` only.

To change the Wazuh credentials, edit
`tests/inventory/group_vars/wazuh_managers.yml` and re-run:

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-indexer,wazuh-dashboard
```

### Secrets (Telegram, Claude API)

Real secrets never live in the inventory. `tests/inventory/hosts.yml` reads them
from the environment via `lookup('env', ...)`; locally they come from a gitignored
`.envrc` loaded by [direnv](https://direnv.net/):

```bash
cp .envrc.example .envrc   # from repo root, then fill in the values
direnv allow
```

| Env var | Ansible var | Used by |
|---|---|---|
| `BW_TELEGRAM_BOT_TOKEN` | `telegram_bot_token` | voice, brain |
| `BW_TELEGRAM_CHAT_ID` | `telegram_chat_id` | voice, brain |
| `BW_CLAUDE_API_KEY` | `claude_api_key` | brain |
| `BW_CLAUDE_WORKSPACE_ID` | `claude_workspace_id` | brain (only for keys not scoped to a workspace) |

Unset variables render as empty strings — playbooks still run, delivery just fails.

---

## 5. Host topology

| Container | Role | SSH port | Published | Static IP |
|---|---|---|---|---|
| `black-master-00` | manager + indexer + dashboard | `2200→22` | `5601→443`, `127.0.0.1:55000→55000`, `127.0.0.1:9200→9200` | `172.28.0.10` |
| `black-host-01` | agent + auditd target | `2201→22` | — | `172.28.0.11` |
| `black-dead-98` | agent fixture — disconnected | `2202→22` | — | `172.28.0.98` |
| `black-rocky-02` | Rocky Linux 9 — firewall roles | `2203→22` | — | `172.28.0.15` |
| `black-metrics-00` | Prometheus | — | `9090→9090` | `172.28.0.12` |
| `black-grafana-00` | Grafana | — | `3000→3000` | `172.28.0.13` |
| `black-aptcache-00` | apt-cacher-ng | — | — | `172.28.0.14` |
| `black-ghost-99` | ghost fixture — never_connected | no container | — | `172.28.0.99` |

`black-ghost-99` is a manager-side registration only (no container). It is
registered via the `broken-agents.yml` playbook and always appears as
`never_connected` in Wazuh.

`black-dead-98` is a real container that registers as a Wazuh agent; the
`broken-agents.yml` playbook then stops it so it transitions to `disconnected`.

### apt cache

`black-aptcache-00` runs apt-cacher-ng; the node image sends every apt request
through it (`/etc/apt/apt.conf.d/01blackwall-aptcache`). The cache is a bind
mount at `tests/.apt-cache/` (gitignored), so it survives `bw-lab down` and
`bw-lab reset`, which only drop Docker volumes.

`packages.wazuh.com` is HTTPS-only, and a proxy cannot cache through a CONNECT
tunnel. So the lab inventory (`tests/inventory/group_vars/all.yml`) points
`wazuh_apt_repo` at `http://packages.wazuh.com/...`, and the cache remaps that
to the `https://` upstream (`tests/docker/images/aptcache/blackwall.conf`).
Packages are still verified by the Wazuh GPG signature on the repo metadata.
Role defaults keep `https://` — none of this exists outside the lab.

```bash
du -sh tests/.apt-cache        # what is cached
rm -rf tests/.apt-cache        # start cold (e.g. to test a fresh download)
```

### Container limitations

The nodes are containers, so they share the kernel of the Docker VM (OrbStack,
Docker Desktop). What that kernel lacks, the lab cannot test:

| Component | In the lab | Why |
|---|---|---|
| `auditd` | installed, rules deployed, **service fails to start** | no kernel audit support (`audit support not in kernel`); the role ignores the failure only when `/.dockerenv` exists |
| `nftables` | works on OrbStack; `bw-check nft` SKIPs on kernels without `nf_tables` | depends on the VM kernel |
| FIM, rootcheck, SCA, syscollector | work | user-space only |

So auditd-based detections (`exec`, `priv_esc`, `sudo`, `shadow` keys) never fire
here — verify them on a real host. To keep this visible, the `wazuh-agent` role
enrolls containerised agents into the Wazuh group **`docker`**, and the
`blackwall` TUI marks them with a red `[docker]` tag and a warning in agent detail.

---

## 6. Monitoring stack

Prometheus and Grafana are compose services; everything about them lives in
this repository, so there is nothing to click through on first start.

```
tests/docker/
├── prometheus/prometheus.yml                        # scrape config
└── grafana/
    ├── provisioning/datasources/prometheus.yml      # datasource
    ├── provisioning/dashboards/dashboards.yml       # dashboard provider
    └── ../dashboards/blackwall-hosts.json           # the dashboard itself
```

Prometheus scrapes `node_exporter` on `black-master-00`, `black-host-01` and
`black-dead-98` (that last target stays `down` while the fixture is stopped —
expected), plus itself. Grafana ships one dashboard, **Blackwall hosts**: hosts
up, 1-minute load average, memory used.

node_exporter is deployed by the Ansible role
`blackwall-voice/ansible/roles/node-exporter`:

```bash
cd blackwall-voice
ansible-playbook ansible/playbooks/metrics.yml
ansible-playbook ansible/playbooks/verify-metrics.yml
```

Wazuh itself exposes no Prometheus metrics, so it is absent from Grafana on
purpose. Agent health is verified through the Wazuh API instead
(`bw-check wazuh`).

Manual probes:

```bash
curl -s 'http://localhost:9090/api/v1/targets?state=active' | jq -r '.data.activeTargets[] | "\(.labels.instance) \(.health)"'
curl -s --get --data-urlencode 'query=node_load1' http://localhost:9090/api/v1/query | jq '.data.result | length'
curl -s -u admin:blackwall http://localhost:3000/api/health | jq -r .database
curl -s -u admin:blackwall http://localhost:3000/api/datasources | jq -r '.[].name'
```

---

## 7. Firewall (nftables)

The Ansible role `blackwall-eye/ansible/roles/nftables` renders
`/etc/nftables.d/blackwall-base.nft` and owns `/etc/nftables.conf`.

```bash
cd blackwall-eye
ansible-playbook ansible/playbooks/nftables.yml
ansible-playbook ansible/playbooks/verify-nftables.yml
```

| Variable | Default | Purpose |
|---|---|---|
| `nft_enabled` | `true` | set to `false` to skip the host entirely |
| `nft_wan_if` | `eth0` | external interface |
| `nft_public_tcp` | `[22]` | TCP ports open to everyone |
| `nft_restricted` | `[]` | list of `{port, sources[], name?}` — port reachable only from those sources; the set is named `name` (default `src_<port>`) |
| `nft_forward_policy` | `accept` | policy of the `forward` chain; `drop` adds a rate-limited log line and a counter |
| `nft_log_drops` | `true` | rate-limited log line on drops |
| `nft_podman` | `{}` | `{net, if}` of a podman host: anti-spoofing on the WAN, container DNS to the host, `ct status dnat accept` and container egress in `forward` |
| `nft_conf_path` | per OS family | `/etc/nftables.conf` (Debian) or `/etc/sysconfig/nftables.conf` (RedHat) |

Ports per group live in `tests/inventory/group_vars/`:

| Group | Open TCP ports |
|---|---|
| `wazuh_managers` | 22, 443, 1514, 1515, 9100, 9200, 9300, 55000 |
| `wazuh_agents` | 22, 9100 |

**Do not trim 9200/9300 from the manager.** filebeat and the dashboard reach the
indexer over the interface address (`172.28.0.10`), not over loopback, so those
packets traverse the `input` chain. Closing them leaves every service `active`
while alerts silently stop reaching the indexer.

**The role never runs a global `flush ruleset`.** Each file under
`/etc/nftables.d/` replaces only its own table. A global flush destroys tables
the role does not own — inside Docker it wipes the NAT rules of the embedded
resolver `127.0.0.11`, which kills DNS in the container and makes apt fail with
`Temporary failure resolving ...` while the ruleset itself looks perfectly fine.

Inspect or roll back by hand:

```bash
tests/bin/bw-lab ssh black-master-00 'nft list table inet blackwall_base'
tests/bin/bw-lab ssh black-master-00 'nft delete table inet blackwall_base'
```

---

## 8. Manual mode — component-level debugging

`bw-lab` and `bw-check` are wrappers. When something breaks, the raw commands
below narrow it down. All `ansible` commands run from `blackwall-eye/`;
`docker` and `curl` from the repo root.

### Containers
```bash
docker ps --format '{{.Names}}' | sort
# Expect:
#   black-aptcache-00
#   black-dead-98
#   black-grafana-00
#   black-host-01
#   black-master-00
#   black-metrics-00

docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 config --quiet
# Expect: exit 0, no output
```

### SSH / Ansible reachability
```bash
ssh -F tests/docker/ssh_config black-master-00 hostname    # → black-master-00
ssh -F tests/docker/ssh_config black-host-01 hostname      # → black-host-01

ansible all -m wait_for_connection -T 30
# Expect: SUCCESS for all three systemd nodes
```

### Wazuh indexer
```bash
ansible black-master-00 -m command -a "systemctl is-active wazuh-indexer" --become
# Expect: active

ansible black-master-00 -m shell -a \
  "curl -sk -u admin:Blackwall1! https://172.28.0.10:9200/_cluster/health" --become
# Expect: JSON with "status":"green" or "yellow"

ansible black-master-00 -m shell -a \
  "curl -sk -u admin:admin https://172.28.0.10:9200/_cluster/health" --become
# Expect: Unauthorized / 401
```

### Wazuh manager + filebeat
```bash
ansible black-master-00 -m command -a "systemctl is-active wazuh-manager" --become
# Expect: active

ansible black-master-00 -m command -a "systemctl is-active filebeat" --become
# Expect: active

ansible black-master-00 -m command -a "filebeat test output" --become
# Expect: lines ending in "OK" for connection / TLS / talk to server
```

### Wazuh dashboard
```bash
ansible black-master-00 -m command -a "systemctl is-active wazuh-dashboard" --become
# Expect: active

ansible black-master-00 -m shell -a \
  "curl -sk https://127.0.0.1/ -o /dev/null -w '%{http_code}'" --become
# Expect: 302

curl -sk https://localhost:5601/ -o /dev/null -w '%{http_code}\n'
# Expect: 302
```

### Wazuh agent
```bash
ansible black-host-01 -m command -a "systemctl status wazuh-agent" --become

ansible black-host-01 -m command -a "tail -20 /var/ossec/logs/ossec.log" --become

ansible wazuh_managers -m command -a "/var/ossec/bin/agent_control -l" --become
# Expect: at least one Active entry for black-host-01
```

### Agent endpoint override (point at a different master)
```bash
ansible-playbook ansible/playbooks/monitored-host.yml \
  -e wazuh_manager_host=192.0.2.1 \
  -e wazuh_enrollment_password=hunter2 \
  --tags wazuh-agent

ansible black-host-01 -m shell -a \
  "grep -E '<address>|<authorization_pass_path>' /var/ossec/etc/ossec.conf" --become
# Expect: <address>192.0.2.1</address> and <authorization_pass_path>... lines

ansible black-host-01 -m shell -a "cat /var/ossec/etc/authd.pass" --become
# Expect: hunter2

# Revert
ansible-playbook ansible/playbooks/monitored-host.yml --tags wazuh-agent
ansible black-host-01 -m stat -a "path=/var/ossec/etc/authd.pass" --become | grep '"exists":'
# Expect: "exists": false
```

### Version + build
```bash
ansible all -m command -a "/var/ossec/bin/wazuh-control info" --become 2>/dev/null || true

ansible black-master-00 -m command -a "dpkg -l wazuh-manager wazuh-indexer wazuh-dashboard filebeat" --become
# Expect: 'ii' status for all four
```

### Full clean-room acceptance

`bw-lab reset` is exactly this sequence:

```bash
tests/bin/bw-lab reset
tests/bin/bw-check all      # exit 0, no FAIL
tests/bin/bw-lab open dashboard
```

Spelled out, in case you want to run it step by step:

```bash
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 down -v
rm -rf tests/.wazuh-certs/wazuh-certificates*

docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build
cd blackwall-eye
ansible all -m wait_for_connection -T 60
ansible-playbook ansible/playbooks/all.yml
cd ../blackwall-voice && ansible-playbook ansible/playbooks/metrics.yml
cd ../blackwall-eye && ansible-playbook ansible/playbooks/nftables.yml

ansible-playbook ansible/playbooks/verify-agents.yml

open https://localhost:5601    # admin / Blackwall1!
```

---

## 9. Trigger synthetic alerts (`bw-trigger`)

Drive Wazuh detections on `black-host-01` to verify the stack is wired end-to-end and to see real alerts in the dashboard.

```bash
tests/triggers/bw-trigger list           # show available events
tests/triggers/bw-trigger user           # fire one
tests/triggers/bw-trigger all            # fire everything (~60s)
tests/triggers/bw-trigger clean          # undo persistent side effects
```

Events range from harmless (`useradd`/`userdel`, FIM touches) to attacker-flavoured (web-shell drop, sudoers tampering, log clearing). Full event list and Wazuh-rule expectations: [tests/triggers/README.md](triggers/README.md).

Live tail the manager while triggering:

```bash
ansible black-master-00 -m shell -a 'tail -f /var/ossec/logs/alerts/alerts.json' --become
```

---

---

## 10. Broken agent fixtures (`broken-agents.yml`)

Two static fixtures test the TUI's per-status row colouring:

| Agent | Status | Mechanism |
|---|---|---|
| `black-ghost-99` | `never_connected` | Registered on manager; no container ever created |
| `black-dead-98`  | `disconnected`    | Container starts, agent registers, then container is stopped |

Both are created by `bw-lab up` as its last step; to re-apply just the fixtures:

```bash
tests/bin/bw-lab provision fixtures
```

The lab manager uses `wazuh_agents_disconnection_time: 1m`
(`tests/inventory/group_vars/wazuh_managers.yml`, default `10m` in the role), so
`black-dead-98` reads as `disconnected` about a minute after the container stops.
`bw-check wazuh` asserts both statuses.

**Verify** on the manager:

```bash
ansible wazuh_managers -i tests/inventory/hosts.yml \
  -m command -a "/var/ossec/bin/agent_control -l" --become
# Expect:
#   black-host-01   Active
#   black-ghost-99  Never connected
#   black-dead-98   Disconnected
```

The TUI renders each status with a distinct colour:

| Status | Colour |
|---|---|
| `active` | default |
| `pending` | yellow |
| `disconnected` | red |
| `never_connected` | dim grey |

---

---

## 11. Point agents at a different master

The `wazuh-agent` role reads endpoint and enrollment params from variables;
the test-env defaults (in `wazuh-agent/defaults/main.yml`) point at the local
master `172.28.0.10`. Override per run:

```bash
ansible-playbook ansible/playbooks/monitored-host.yml \
  -e wazuh_manager_host=wazuh.prod.example.com \
  -e wazuh_enrollment_password=<secret>
```

For durable multi-host overrides, write your own inventory file:

```yaml
# inventory/prod.yml
all:
  vars:
    ansible_user: deploy
    ansible_ssh_private_key_file: ~/.ssh/id_ed25519
  children:
    wazuh_agents:
      vars:
        wazuh_manager_host: wazuh.prod.example.com
        wazuh_enrollment_password: "{{ lookup('community.general.passwordstore', 'wazuh/enrollment') }}"
      hosts:
        web-01.prod.example.com:
        web-02.prod.example.com:
```

```bash
ansible-playbook -i inventory/prod.yml ansible/playbooks/monitored-host.yml
```

Available variables (full list in `roles/wazuh-agent/defaults/main.yml`):

| Variable | Default | Purpose |
|---|---|---|
| `wazuh_manager_host` | `172.28.0.10` | Manager IP or hostname |
| `wazuh_manager_port` | `1514` | Manager listener port (TCP) |
| `wazuh_manager_protocol` | `tcp` | `tcp` or `udp` |
| `wazuh_enrollment_host` | `wazuh_manager_host` | Enrollment server (usually same) |
| `wazuh_enrollment_port` | `1515` | authd port |
| `wazuh_enrollment_password` | `""` | Shared secret; empty = no password |
| `wazuh_agent_name` | `inventory_hostname` | Agent name registered with manager |
| `wazuh_agent_groups` | `default` | Comma-separated Wazuh agent groups |

---

---

## 12. Run a playbook on a specific host

```bash
# Manager only
ansible-playbook ansible/playbooks/security-node.yml --limit black-master-00

# Agent only
ansible-playbook ansible/playbooks/monitored-host.yml --limit black-host-01
```

---

---

## 13. Run a single role

```bash
# wazuh-indexer
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-indexer

# wazuh-manager
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-manager

# wazuh-dashboard
ansible-playbook ansible/playbooks/security-node.yml --tags wazuh-dashboard

# wazuh-agent on the lone host
ansible-playbook ansible/playbooks/monitored-host.yml --tags wazuh-agent

# auditd on the lone host
ansible-playbook ansible/playbooks/monitored-host.yml --tags auditd
```

---

---

## 14. Verify (high-level)

```bash
# Agent registration
ansible-playbook ansible/playbooks/verify-agents.yml

# Live agent status via REST API
ansible-playbook ansible/playbooks/verify-api.yml

# auditd rules and config (service check skipped in Docker)
ansible-playbook ansible/playbooks/verify-auditd.yml
```

---

---

## 15. Service status (ad-hoc)

```bash
ansible wazuh_managers -m command -a "systemctl status wazuh-manager" --become
ansible wazuh_managers -m command -a "systemctl status wazuh-indexer" --become
ansible wazuh_managers -m command -a "systemctl status wazuh-dashboard" --become
ansible wazuh_managers -m command -a "systemctl status filebeat" --become
ansible wazuh_agents -m command -a "systemctl status wazuh-agent" --become
```

---

---

## 16. SSH access

```bash
ssh -F tests/docker/ssh_config black-master-00
ssh -F tests/docker/ssh_config black-host-01
```

Or append to your personal SSH config:

```bash
cat tests/docker/ssh_config >> ~/.ssh/config
```

---

---

## 17. Certificates

`tests/.wazuh-certs/wazuh-certificates.tar` is generated by
`wazuh-certificates.yml` and consumed by indexer, manager (filebeat),
and dashboard roles. It's per-environment and gitignored.

To rotate certs:

```bash
rm -rf tests/.wazuh-certs/wazuh-certificates*
ansible-playbook ansible/playbooks/wazuh-certificates.yml
# then re-run security-node.yml
```

---

---

## 18. Multi-distro support

The compose file uses [Compose profiles](https://docs.docker.com/compose/profiles/) to select a distro.

| Profile | Base image | systemd | SSH ports | Dashboard | Status |
|---|---|---|---|---|---|
| `ubuntu2204` | geerlingguy/docker-ubuntu2204-ansible | yes | 2200, 2201 | 5601 | Active |
| `debian12` | (planned) | yes | 2210, 2211 | 5611 | Not implemented |
| `centos9` | (planned) | yes | 2220, 2221 | 5621 | Not implemented |
| `alpine` | n/a — OpenRC | **no** | n/a | n/a | Not supported |

To add a new distro:

1. Create `tests/docker/images/<distro>/Dockerfile`.
2. Uncomment the corresponding service block in `tests/docker/docker-compose.yml`.
3. Add host entries to `tests/inventory/hosts.yml`.

---

---

## 19. Controller prerequisites

`password_hash('bcrypt')` filter (used by `wazuh-indexer` role) needs `passlib` on the Ansible controller. For pipx-installed Ansible:

```bash
pipx inject ansible 'passlib' 'bcrypt<5.0.0'
```

(bcrypt 5.0 has a known incompatibility with passlib 1.7.4; pin until upstream fixes.)

---

---

## 20. Known limitations

**auditd:** kernel audit netlink is not accessible inside Docker. The role installs rules correctly, but the service won't start in containers (detected via `/.dockerenv`). On real hosts auditd starts normally.

**Dashboard cert:** self-signed; browsers show "Not Secure". Acceptable for `localhost` only. Never expose port 5601 beyond `localhost`.

**nftables:** works normally in these containers — `nft` applies, `policy drop`
really filters, and `nftables.service` starts under systemd. The one hard rule
is that nothing may run a global `flush ruleset`: it wipes the NAT rules of
Docker's embedded resolver `127.0.0.11`, after which DNS inside the container
dies and apt fails with `Temporary failure resolving ...`. Restarting the
container restores those rules. See section 7.

**Indexer/dashboard re-template:** Because bcrypt salts are random, the `internal_users.yml` template renders a new hash every run, so the indexer security config is re-pushed on every `security-node.yml` execution. Functionally idempotent (same password verifies) but not byte-idempotent. Acceptable.

---

---

## 21. File layout

```
blackwall/                                  # repo root
├── tests/
│   ├── bin/
│   │   ├── bw-lab                          # environment lifecycle
│   │   ├── bw-check                        # verification
│   │   └── _bw-common.sh                   # shared helpers (sourced)
│   ├── docker/
│   │   ├── docker-compose.yml
│   │   ├── images/ubuntu2204/Dockerfile    # nodes; apt goes through the cache
│   │   ├── images/aptcache/                # apt-cacher-ng: Dockerfile + blackwall.conf
│   │   ├── ssh_config
│   │   ├── prometheus/prometheus.yml
│   │   └── grafana/
│   │       ├── provisioning/datasources/prometheus.yml
│   │       ├── provisioning/dashboards/dashboards.yml
│   │       └── dashboards/blackwall-hosts.json
│   ├── inventory/
│   │   ├── hosts.yml
│   │   └── group_vars/                     # loaded because it sits next to the inventory
│   │       ├── all.yml                     # lab: wazuh_apt_repo via the apt cache
│   │       ├── wazuh_managers.yml
│   │       └── wazuh_agents.yml
│   ├── triggers/
│   │   ├── bw-trigger                      # synthetic alert events
│   │   └── broken-agents.yml               # registers ghost + stops dead
│   ├── .apt-cache/                         # gitignored apt-cacher-ng cache
│   └── .wazuh-certs/                       # gitignored TLS bundle
├── blackwall-eye/
│   ├── ansible.cfg
│   └── ansible/
│       ├── playbooks/
│       │   ├── all.yml
│       │   ├── wazuh-certificates.yml
│       │   ├── security-node.yml           # indexer + manager + dashboard
│       │   ├── monitored-host.yml          # agent + auditd
│       │   ├── nftables.yml
│       │   ├── verify-agents.yml
│       │   ├── verify-api.yml
│       │   ├── verify-auditd.yml
│       │   ├── verify-nftables.yml
│       │   ├── verify-versions.yml
│       │   └── verify-ports.yml
│       └── roles/
│           ├── wazuh-indexer/
│           ├── wazuh-manager/              # includes filebeat output
│           ├── wazuh-dashboard/
│           ├── wazuh-agent/
│           ├── auditd/
│           └── nftables/
└── blackwall-voice/
    ├── ansible.cfg
    └── ansible/
        ├── playbooks/
        │   ├── voice.yml
        │   ├── verify-voice.yml
        │   ├── metrics.yml
        │   └── verify-metrics.yml
        └── roles/
            ├── telegram-alerter/
            └── node-exporter/
```

> `group_vars` lives next to the **inventory**, not next to the playbooks.
> Ansible resolves group variables relative to the inventory file or the
> playbook directory; a `group_vars/` under `blackwall-eye/ansible/` matches
> neither and is silently ignored.

---

## Security note

Containers run `root:blackwall` over SSH with `PermitRootLogin yes`.
Indexer/dashboard credentials default to `admin / Blackwall1!`, Grafana to
`admin / blackwall`, and Prometheus has no authentication at all.
**This is intentional for the local test environment. Do not expose any of
these containers or ports beyond `localhost`.**
