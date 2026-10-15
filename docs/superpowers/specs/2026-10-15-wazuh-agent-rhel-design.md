# Wazuh agent + auditd on the RHEL family — design

Date: 2026-10-15
Module: blackwall-eye
Status: approved in chat

## 1. Goal

The production host (RHEL family) will later run the Wazuh manager; until then
everything is developed in the lab. This is sub-project A of "Eye on RHEL": the
Wazuh agent and auditd roles work on Debian and RedHat families, proven on the
lab node `black-rocky-02` (Rocky Linux 9). Sub-project B (manager, indexer,
dashboard on RHEL next to podman, port 443 taken by nginx) gets its own spec.

Success:

- `bw-lab provision eye` enrolls `black-rocky-02` at `black-master-00`; it is
  `Active` and reports inventory (visible in the blackwall TUI).
- An event on the Rocky node (failed SSH login) arrives in the indexer as an
  alert with `agent.name: black-rocky-02`.
- Ubuntu nodes unchanged: `bw-check all` passes as before.

## 2. Context

- All Wazuh roles are Debian-only: `dpkg` lock wait, `apt_key` (deprecated, no
  RHEL equivalent), `apt_repository`, `pkg=<ver>-*` pins. `wazuh_version`,
  `wazuh_apt_key_url`, `wazuh_apt_repo` are duplicated in three roles' defaults;
  the lab overrides `wazuh_apt_repo` with plain http in `group_vars/all.yml`.
- Lab inventory nests `wazuh_managers` / `wazuh_agents` under `ubuntu2204`; the
  Rocky node is in `rhel9` only.
- Agent `ossec.conf` reads `/var/log/auth.log` and `/var/log/syslog`; RHEL 9
  containers have no rsyslog — logs live in journald.

## 3. Design

### 3.1 Role `wazuh-repo` (new)

Owns the Wazuh package source and the version for both families.

| Variable | Default |
|---|---|
| `wazuh_version` | `4.14.7` |
| `wazuh_gpg_key_url` | `https://packages.wazuh.com/key/GPG-KEY-WAZUH` |
| `wazuh_apt_repo_url` | `https://packages.wazuh.com/4.x/apt/` |
| `wazuh_yum_repo_url` | `https://packages.wazuh.com/4.x/yum/` |

- Debian: key into `/etc/apt/keyrings/wazuh.asc` (`get_url`), `apt_repository`
  `deb [signed-by=...] <url> stable main`, filename `wazuh`. The key that
  `apt_key` added to the legacy trusted keyring on existing hosts is left in
  place (a harmless duplicate).
- RedHat: `yum_repository` name `wazuh`, `baseurl`, `gpgkey`, `gpgcheck: true`.
- Exposes `wazuh_pkg_suffix` for pins: `={{ wazuh_version }}-*` on Debian,
  `-{{ wazuh_version }}` on RedHat (set as a fact).
- The lab's http override moves to `wazuh_apt_repo_url`.
- `wazuh-agent`, `wazuh-manager`, `wazuh-indexer`, `wazuh-dashboard` list it in
  `meta/dependencies` and drop their own key/repo tasks and duplicated
  defaults. Server roles stay Debian-tested in this sub-project.

### 3.2 Role `wazuh-agent`

- `dpkg` lock wait only on Debian; prerequisites via `package` (`curl`; `gnupg`
  only on Debian).
- Install `wazuh-agent{{ wazuh_pkg_suffix }}` via `package`
  (`allow_downgrade` on both: apt option / dnf `allow_downgrade`).
- `ossec.conf`: system log sources per family — Debian: `auth.log` + `syslog`
  (unchanged); RedHat: `<localfile><location>journald</location>
  <log_format>journald</log_format></localfile>` (native since Wazuh 4.8).
  Everything else shared (paths are `/var/ossec` on both).

### 3.3 Role `auditd`

- Package `audit` on RedHat, `auditd` + `audispd-plugins` on Debian
  (RHEL 9 ships the plugins in `audit`).
- RedHat refuses `systemctl restart auditd`: the handler runs
  `augenrules --load` there (and `service auditd restart` only for
  `auditd.conf` changes); Debian keeps `systemctl restart`.
- Container lab: kernel audit is unavailable on both families; tasks keep
  `ignore_errors` on `/.dockerenv` as today.

### 3.4 Lab

- Inventory: `wazuh_managers` and `wazuh_agents` move to `all.children`
  (functional groups); `ubuntu2204` / `rhel9` list hosts by OS. `black-rocky-02`
  joins `wazuh_agents`.
- `verify-agents.yml`: every host in `wazuh_agents` except
  `wazuh_fixture_agents` (lab: `[black-dead-98]`) must be `Active` by name —
  not just "at least one".
- `verify-versions.yml`: RHEL expects `wazuh-agent` (= manager version),
  `audit`, `nftables`.
- New check: failed SSH login on `black-rocky-02` produces an alert for that
  agent in the indexer within 2 minutes (`bw-check wazuh`).

## 4. Testing

`bw-lab provision eye` then `bw-check all` — Ubuntu unchanged, Rocky agent
Active, versions match, the SSH event alert arrives. `apt_key` no longer used.

## 5. Out of scope

Manager/indexer/dashboard on RHEL (sub-project B), SELinux policy for the
agent beyond package defaults, auditd rules firing (no kernel audit in
containers).
