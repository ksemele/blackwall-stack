---
module: blackwall-eye
status: in_progress
stage: 1
last_updated: 2026-10-15
last_task: "Wazuh agent + auditd on the RHEL family (wazuh-repo role, journald), black-rocky-02 enrolled; stricter agent and event checks"
next_task: "Sub-project B: Wazuh manager/indexer/dashboard on the RHEL family next to podman (dashboard off 443), measure memory — the prod host becomes the manager later"
blockers: []
---

## Done

- 2026-10-15: Wazuh agent on the RHEL family (spec
  `docs/superpowers/specs/2026-10-15-wazuh-agent-rhel-design.md`).
  - New `wazuh-repo` role: Wazuh version + apt (signed-by keyring, no
    `apt_key`) and yum repositories; meta dependency of all four Wazuh roles.
  - `wazuh-agent` on Debian + RedHat (journald source on RedHat), `auditd` on
    both (RedHat: `audit` package, `augenrules --load` / `service auditd restart`).
  - Lab: `black-rocky-02` enrolled; inventory split into OS and functional
    groups; `verify-agents.yml` checks every agent by name;
    `verify-agent-events.yml` proves a failed SSH login on the RHEL agent
    reaches the indexer.
  - Carried into sub-project B (before the prod host is provisioned): smoke
    test auditd handlers on a real RHEL host, incl. immutable rules (`-e 2`).
- 2026-10-15: Wazuh packages pinned against unattended upgrades (apt hold on
  Debian, yum repo `exclude` + `disable_excludes` on RedHat).
- 2026-10-15: prod firewall as code (spec `docs/superpowers/specs/2026-10-14-nftables-prod-port-design.md`).
  - `nftables` role: Debian + RedHat (`nft_conf_path`), `nftables.service`
    drop-in without `flush ruleset`, optional `nft_podman` (anti-spoof,
    container DNS, DNAT forward, egress), named restricted sets,
    forward-drop logging when `nft_forward_policy: drop`.
  - New `cf-wall` role: `blackwall-cf-refresh` + weekly timer; loads the new
    table before persisting it, aborts on a junk fetch or a missing WAN
    interface, never leaves a rejected file on disk.
  - `tools/nft-equiv`: proves a host's rendered table equals a reference file.
  - Lab: `black-rocky-02` (Rocky Linux 9); `bw-lab provision cf-wall`,
    `bw-check cf-wall`.
  - Applied to the prod host by the owner (dry run first): both tables and
    podman's `ip nat` / `ip filter` intact, timer active, SELinux contexts
    clean. The hand-made setup is retired; the roles are the source of truth.
- 2026-10-14: manager host inventory. `wazuh-manager` `ossec.conf` gains
  `<wodle name="syscollector">` and `<sca>` (agent 000 was empty in the TUI
  Hardware/Top/Packages/Network/Security tabs); rootcheck now points at the
  shipped `etc/rootcheck/rootkit_*.txt` instead of missing `etc/shared/` files.
  Intervals reuse the agent role knobs (`wazuh_syscollector_interval`,
  `wazuh_sca_interval`, `wazuh_rootcheck_frequency`).
- 2026-10-14: login sessions snapshot for the blackwall TUI Sessions tab.
  - Manager `ossec.conf` now declares `<ruleset>` explicitly (fixes
    `GET /rules` error 1106); `blackwall_rules.xml` deployed to `etc/rules/`.
  - Rule 100100 (level 3, group `blackwall_sessions`, `check_diff`, child of 530)
    stores an alert whenever the `last` output changes.
  - Agents run `last --time-format iso -w -n 50` every 60 s
    (`wazuh_sessions_enabled`, `wazuh_sessions_frequency`).
  - `bw-check` asserts rule 100100 is loaded; lab: 30 passed, 0 failed.
  - The TUI `:alerts` feed and the agent Sessions tab consume rule 100100 and the
    indexer alerts (`l` jumps from an agent card to its alerts).

- 2026-10-12: full from-scratch lab run and the fixes it led to.
  - `wazuh-agent` was installed unpinned (apt served 4.14.8 against a 4.14.7
    manager) — now `wazuh-agent={{ wazuh_version }}-*` + `allow_downgrade`;
    `verify-versions.yml` checks that the agent version equals the manager's.
  - rootcheck and SCA enabled on agents (CIS Ubuntu 22.04: 103 pass / 84 fail).
  - Containerised agents (`/.dockerenv`) enroll into the Wazuh group `docker`
    (the manager creates it via `wazuh_manager_agent_groups`); the TUI marks
    them in red: the Docker VM kernel has no audit, auditd does not start in the lab.
    A single group, not `default,docker`: agents enrolling simultaneously into a
    not-yet-created multigroup hit a race in wazuh-db, and the loser stays
    `Unknown` with no `NNN.db` (caught on a clean `bw-lab reset`).
  - `wazuh_agents_disconnection_time` is now a variable (lab: 1m); the
    `broken-agents.yml` fixtures were fixed (`manage_agents -a <ip> -n <name>`)
    and run as the last step of `bw-lab up`.
  - The Python `tools/bw-status` was removed — `blackwall --once` replaces it.

- 2026-08-31: `nftables` role + lab checks.
  - `ansible/roles/nftables/` — parametrised role: `nft_enabled`,
    `nft_wan_if`, `nft_public_tcp`, `nft_restricted`, `nft_forward_policy`,
    `nft_log_drops`. Renders `/etc/nftables.d/blackwall-base.nft`, owns
    `/etc/nftables.conf`, runs `nft -c -f` before applying.
  - `ansible/playbooks/nftables.yml`, `verify-nftables.yml` (asserts
    `policy drop` + all expected ports), `verify-versions.yml`, `verify-ports.yml`.
  - Per-group ports live in `tests/inventory/group_vars/`.
  - Verified on the live lab: `policy drop` really filters, the allow-list
    opens a port, SSH and ansible are not lost.

- 2026-08-31: Wazuh pin raised 4.14.5 → 4.14.7 in all four roles
  (manager, agent, indexer, dashboard). Reason: upstream returns HTTP 400 for
  `raw.githubusercontent.com/wazuh/wazuh/v4.14.5/.../wazuh-template.json`,
  which broke the `wazuh-manager` role and a clean `bw-lab up`. On v4.14.7 and
  v4.9.0 the same path returns 200. 4.14.7-1 packages exist in apt for all
  three server components. Side effect: fixed a mismatch where the agent was
  actually installed as 4.14.7 while the roles were pinned to 4.14.5.

- 2026-05-16: test environment reworked — `black-master-00` (manager + indexer + dashboard) + `black-host-01` (agent). Added the `wazuh-indexer` and `wazuh-dashboard` roles. The manager ships alerts to the indexer via filebeat. Certificates are generated once via `wazuh-certificates.yml`. The dashboard is at `https://localhost:5601` (admin / Blackwall1!). The agent is fully parametrised: `wazuh_manager_host`/`port`/`protocol`/`enrollment_*` — any master can be targeted via `-e` or group_vars. Spec: `docs/superpowers/specs/2026-05-15-test-env-master-host-dashboard-design.md`. Plan: `docs/superpowers/plans/2026-05-15-test-env-master-host-dashboard.md`.

- `ansible/roles/wazuh-manager/` — full Ansible role: defaults, tasks, handlers, templates/ossec.conf.j2
  - Installs wazuh-manager from the official apt repository (pinned 4.14.7)
  - ossec.conf: authd (enrollment, port 1515), API (port 55000), remote listener (tcp:1514), FIM on /etc/{passwd,shadow,sudoers,ssh/sshd_config}, log collection from auth.log and syslog
  - systemd: enabled + started, restart handler on config change
  - Active response and vulnerability detection disabled for Stage 1
- `ansible/playbooks/security-node.yml` — updated: wazuh-manager role wired in for the wazuh_managers group
- `ansible/playbooks/verify-agents.yml` — playbook that checks agent registration: `agent_control -l`, asserts Active status

- `ansible/roles/wazuh-agent/` — full Ansible role: defaults, tasks, handlers, templates/ossec.conf.j2
  - Installs wazuh-agent from the official apt repository packages.wazuh.com
  - ossec.conf: connection to the manager (tcp:1514), auto-enrollment, FIM on /etc/{passwd,shadow,sudoers,ssh/sshd_config}, log collection from auth.log and syslog
  - systemd: enabled + started, restart handler on config change
- `ansible/playbooks/monitored-host.yml` — playbook for the wazuh_agents group
- `ansible/ansible.cfg` — roles_path configured, default inventory
- Test: `ansible-playbook` on black-01/02 — 7/7 tasks ok, 0 failed, service active on both hosts

- `docs/ARCHITECTURE.md` — full architecture, components, detection coverage, deployment model
- Repository structure designed (see ARCHITECTURE.md → Repository Layout)
- `docs/stages/stage-1-test-env.md` — design doc for the Docker test env (Options A/B/C compared, Option A chosen)
- Docker test env implemented in `tests/`:
  - `tests/docker/images/ubuntu2204/Dockerfile` — geerlingguy base + openssh-server
  - `tests/docker/docker-compose.yml` — multi-distro via Compose profiles
  - `tests/docker/ssh_config` — handy SSH aliases
  - `tests/inventory/hosts.yml` — full topology (manager + 2 agents)
  - `tests/inventory/hosts.single.yml` — isolated host for role testing
  - `tests/README.md` — up/down docs and the multi-distro strategy

## Open questions

- Wazuh version: pin to 4.x (current stable) or a specific minor?

## Decisions

- **nftables: no global `flush ruleset`.** The role recreates only its own
  table (create/delete/define idiom). A global flush inside Docker wipes the NAT
  rules of the built-in resolver `127.0.0.11` — DNS in the container dies, apt
  fails with `Temporary failure resolving ...`, while the ruleset itself looks
  correct. A container restart recovers it. The same holds for podman hosts
  (netavark's tables) — see `hosts/example.com/`.
- **9200/9300 must not be closed on the manager.** filebeat and the dashboard
  reach the indexer at the interface address (`172.28.0.10`), not via loopback,
  so packets go through the `input` chain. If closed, all services stay
  `active` and alerts silently stop arriving.
- **`group_vars` moved to `tests/inventory/group_vars/`.** The former
  `blackwall-eye/ansible/group_vars/` was never loaded at all: ansible looks for
  group_vars next to the inventory or the playbook, and that path matched
  neither — values were silently taken from role defaults.
- **auditd is not installed on the manager** — the role only runs from
  `monitored-host.yml` for the `wazuh_agents` group; `verify-versions.yml`
  expects it only there.

- packages.wazuh.com is reachable from the containers directly (its CDN is not blocked, unlike Canonical's)
- Task order in the role: gnupg/curl → GPG key → apt repo → install package → template → systemd (template after install, because the package creates the wazuh group)
- Target OS: Ubuntu 22.04 LTS / Debian 12 (primary), Rocky Linux 9 (secondary)
- Deployment: Ansible only, no Terraform at the first stage
- Alerting: Telegram/Slack/webhook (implemented in blackwall-voice, not here)
- Docker test env: geerlingguy/docker-ubuntu2204-ansible as the base image (systemd + Python pre-installed)
- Multi-distro: Compose profiles (Option A) — one compose file, the distro is selected with --profile
- Alpine excluded from roles with systemd dependencies (Wazuh, auditd) — Alpine uses OpenRC
- Runtime: OrbStack (Docker Desktop 29.4.1 on macOS 26 does not pass cgroup2 through correctly)
- cgroup fix: entrypoint.sh mounts cgroup2 before systemd (OrbStack mounts btrfs on /sys/fs/cgroup)
- apt mirror: ftp.uni-erlangen.de (Canonical IPs 91.189.91.x are unreachable from the current network)
