# Wazuh agent on RHEL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wazuh agent and auditd roles work on Debian and RedHat families; `black-rocky-02` enrolls and its events reach the indexer.

**Architecture:** A new `wazuh-repo` role owns the package source and version for both families and is a meta dependency of all four Wazuh roles. `wazuh-agent` and `auditd` branch on `ansible_facts['os_family']` only where packages, log sources or service control differ. Lab inventory groups become functional (`wazuh_*` at the top level) with OS groups beside them.

**Tech Stack:** Ansible, Wazuh 4.14.7 (apt + yum repos), systemd/journald, Docker lab.

**Spec:** `docs/superpowers/specs/2026-10-15-wazuh-agent-rhel-design.md`

## Global Constraints

- Ubuntu lab nodes must behave exactly as before: `tests/bin/bw-check all` stays green after every task.
- `wazuh_version` stays `4.14.7`; an agent newer than the manager is unsupported.
- Use `ansible_facts['os_family']` (not `ansible_os_family`) — deprecated top-level facts warn on ansible-core 2.20.
- Public repo: no real host identifiers (see implementer rules). English everywhere; brief commits, no AI trailers; branch `claude-dev`; no push.
- Lab only. Never `bw-lab down`/`reset`. Keep `black-dead-98` stopped (`bw-lab provision fixtures` restores it if `up` starts it).

## Review Focus

- A Debian host with the old `apt_key` + `wazuh.list` from before this change must re-provision without apt errors (duplicate source/conflicting Signed-By) — pinned by re-running `bw-lab provision eye` on the existing Ubuntu lab in Task 1.
- The lab http repo override must still apply (no https fetch from inside the lab) — pinned by Task 1 checking the rendered `/etc/apt/sources.list.d/wazuh.list` on an Ubuntu node.
- Re-provisioning an enrolled Rocky agent must not re-enroll or duplicate it — pinned by the second `provision eye` in Task 2 (agent count unchanged).
- `verify-agents.yml` must not go red because of the intentionally stopped `black-dead-98` — pinned by `wazuh_fixture_agents` in Task 2.
- auditd handler on RHEL must not fail the play in the container lab (kernel audit missing) — pinned by Task 3's provision run.

---

### Task 1: role `wazuh-repo`; all Wazuh roles use it

**Files:**
- Create: `blackwall-eye/ansible/roles/wazuh-repo/{defaults,tasks}/main.yml`
- Create: `blackwall-eye/ansible/roles/{wazuh-agent,wazuh-manager,wazuh-indexer,wazuh-dashboard}/meta/main.yml`
- Modify: the four roles' `tasks/main.yml` (drop key/repo tasks; package pins use `wazuh_pkg_suffix`) and `defaults/main.yml` (drop duplicated version/repo vars)
- Modify: `tests/inventory/group_vars/all.yml` (`wazuh_apt_repo` -> `wazuh_apt_repo_url`)

**Interfaces:**
- Produces: vars `wazuh_version`, `wazuh_gpg_key_url`, `wazuh_apt_repo_url`, `wazuh_yum_repo_url`; fact `wazuh_pkg_suffix` (`=4.14.7-*` Debian, `-4.14.7` RedHat).

- [ ] **Step 1: Red** — record the current state: `grep -rn apt_key blackwall-eye/ansible/roles` (expect hits in 4 roles) and `tests/bin/bw-lab ssh black-host-01 'cat /etc/apt/sources.list.d/wazuh.list'`.

- [ ] **Step 2: Implement**

`roles/wazuh-repo/defaults/main.yml`:

```yaml
---
# Single source of the Wazuh package version and repositories for every
# Wazuh role (meta dependency). An agent newer than the manager is unsupported.
wazuh_version: "4.14.7"
wazuh_gpg_key_url: "https://packages.wazuh.com/key/GPG-KEY-WAZUH"
wazuh_apt_repo_url: "https://packages.wazuh.com/4.x/apt/"
wazuh_yum_repo_url: "https://packages.wazuh.com/4.x/yum/"
```

`roles/wazuh-repo/tasks/main.yml`:

```yaml
---
- name: Package version suffix for pins (apt name=ver-*, dnf name-ver)
  ansible.builtin.set_fact:
    wazuh_pkg_suffix: "{{ ('=' ~ wazuh_version ~ '-*') if ansible_facts['os_family'] == 'Debian' else ('-' ~ wazuh_version) }}"

- name: Debian — Wazuh repository
  when: ansible_facts['os_family'] == 'Debian'
  block:
    - name: Wait for dpkg lock to be released
      ansible.builtin.shell: while fuser /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock >/dev/null 2>&1; do sleep 2; done
      changed_when: false

    - name: Keyrings directory
      ansible.builtin.file:
        path: /etc/apt/keyrings
        state: directory
        mode: "0755"

    - name: Wazuh signing key
      ansible.builtin.get_url:
        url: "{{ wazuh_gpg_key_url }}"
        dest: /etc/apt/keyrings/wazuh.asc
        mode: "0644"

    - name: Wazuh apt repository
      ansible.builtin.apt_repository:
        repo: "deb [signed-by=/etc/apt/keyrings/wazuh.asc] {{ wazuh_apt_repo_url }} stable main"
        filename: wazuh
        state: present
        update_cache: true

- name: RedHat — Wazuh repository
  when: ansible_facts['os_family'] == 'RedHat'
  ansible.builtin.yum_repository:
    name: wazuh
    description: Wazuh repository
    baseurl: "{{ wazuh_yum_repo_url }}"
    gpgkey: "{{ wazuh_gpg_key_url }}"
    gpgcheck: true
    enabled: true
```

Each of the four roles gets `meta/main.yml`:

```yaml
---
dependencies:
  - role: wazuh-repo
```

In each role's `tasks/main.yml`: remove "Add Wazuh GPG key" (`apt_key`), "Add Wazuh apt repository", the gnupg prerequisite where only used for the key; change pins like `"wazuh-agent={{ wazuh_version }}-*"` to `"wazuh-agent{{ wazuh_pkg_suffix }}"` (same for `wazuh-manager`, `wazuh-indexer`, `wazuh-dashboard`). Keep everything else (filebeat etc.) as is — server roles remain Debian-only in this plan. Remove `wazuh_version`/`wazuh_apt_key_url`/`wazuh_apt_repo` from the roles' defaults. In `tests/inventory/group_vars/all.yml` rename the override to `wazuh_apt_repo_url: "http://packages.wazuh.com/4.x/apt/"` keeping its comment. Grep the repo for other references to the removed variable names (README, tests/README, scripts) and update them.

Note on the existing Ubuntu lab: the old `apt_repository` wrote `/etc/apt/sources.list.d/wazuh.list` without `signed-by`; the new task with the same `filename` rewrites that file. If apt complains about conflicting `Signed-By` values for the same source, delete the stale line in the same task's file (it is the same file) — verify, don't guess.

- [ ] **Step 3: Green**

Run: `tests/bin/bw-lab provision eye && tests/bin/bw-check all`
Expected: no FAIL (voice/brain may SKIP; cf-wall SKIP/PASS as before). `tests/bin/bw-lab ssh black-host-01 'cat /etc/apt/sources.list.d/wazuh.list'` shows `[signed-by=/etc/apt/keyrings/wazuh.asc] http://packages.wazuh.com/4.x/apt/`. `grep -rn apt_key blackwall-eye/ansible` prints nothing. Second `provision eye`: no `changed` on repo tasks.

- [ ] **Step 4: Commit** — `feat(eye): wazuh-repo role for apt and yum`

---

### Task 2: wazuh-agent on RHEL; Rocky node enrolls; stricter agent check

**Files:**
- Modify: `blackwall-eye/ansible/roles/wazuh-agent/tasks/main.yml`, `templates/ossec.conf.j2`
- Modify: `tests/inventory/hosts.yml`, `tests/inventory/group_vars/all.yml` (`wazuh_fixture_agents`)
- Modify: `blackwall-eye/ansible/playbooks/verify-agents.yml`, `verify-versions.yml`
- Create: `blackwall-eye/ansible/playbooks/verify-agent-events.yml`; wire into `tests/bin/bw-check` `check_wazuh`
- Modify: `tests/README.md` (checks table), `blackwall-eye/README.md` if it lists agent OS support

**Interfaces:**
- Consumes: `wazuh_pkg_suffix` (Task 1).
- Produces: group `wazuh_agents` containing `black-rocky-02`; var `wazuh_fixture_agents`.

- [ ] **Step 1: Inventory** — restructure `tests/inventory/hosts.yml` so functional groups sit under `all.children` and OS groups list hosts:

```yaml
  children:
    ubuntu2204:
      hosts:
        black-master-00: {ansible_host: localhost, ansible_port: 2200}
        black-host-01:   {ansible_host: localhost, ansible_port: 2201}
        black-dead-98:   {ansible_host: localhost, ansible_port: 2202}
    rhel9:
      hosts:
        black-rocky-02:  {ansible_host: localhost, ansible_port: 2203}
    wazuh_managers:
      hosts:
        black-master-00:
    wazuh_agents:
      hosts:
        black-host-01:
        black-dead-98:
        black-rocky-02:
```

(keep `cf_wall`, comments and `vars` as they are). Add to `group_vars/all.yml`:

```yaml
# Agents that are broken on purpose (bw-lab provision fixtures); agent checks
# skip them.
wazuh_fixture_agents: [black-dead-98]
```

- [ ] **Step 2: Red** — tighten `verify-agents.yml`: after listing, assert every name in `groups['wazuh_agents'] | difference(wazuh_fixture_agents | default([]))` appears in `agent_list.stdout_lines` on a line containing `Active` (loop with a clear fail message naming the host). Update `verify-versions.yml` so `rhel9` expects `wazuh-agent` (version equal to the manager's, same rule as Ubuntu agents), `audit` and `nftables`. Run `tests/bin/bw-check wazuh versions` — expect FAIL for `black-rocky-02` (not enrolled / packages missing).

- [ ] **Step 3: Implement the role**
- `tasks/main.yml`: the dpkg-lock wait moved to `wazuh-repo` (Task 1) — remove any leftover here; prerequisites `curl` via `ansible.builtin.package` (skip install when the binary exists — Rocky ships `curl-minimal`); install with

```yaml
- name: Install wazuh-agent package
  ansible.builtin.package:
    name: "wazuh-agent{{ wazuh_pkg_suffix }}"
    state: present
  # an agent newer than the manager is unsupported; allow pinning down
  # (apt: allow_downgrade; dnf: allow_downgrade) — use the family's module if
  # `package` cannot pass it through.
```

  Use `ansible.builtin.apt` (with `allow_downgrade: true`) on Debian and `ansible.builtin.dnf` (with `allow_downgrade: true`) on RedHat if `package` can't express it.
- `templates/ossec.conf.j2`: replace the two system `localfile` blocks with

```jinja
{% if ansible_facts['os_family'] == 'RedHat' %}
  <!-- RHEL family: system logs live in journald (no rsyslog files) -->
  <localfile>
    <location>journald</location>
    <log_format>journald</log_format>
  </localfile>
{% else %}
  <localfile>
    <log_format>syslog</log_format>
    <location>/var/log/auth.log</location>
  </localfile>

  <localfile>
    <log_format>syslog</log_format>
    <location>/var/log/syslog</location>
  </localfile>
{% endif %}
```

- [ ] **Step 4: Event check** — `verify-agent-events.yml` (hosts: `wazuh_managers`, lab-only — gate with `when: bw_lab | default(false) | bool`): for each live agent on the RHEL family in `wazuh_agents` (here: `black-rocky-02`), delegate a failed SSH login to it (`sshpass`-free: from the manager run `ssh -o BatchMode=yes -o StrictHostKeyChecking=no nosuchuser@<agent ip>` and ignore the failure), then poll the indexer (`https://127.0.0.1:9200/wazuh-alerts-*/_search`, creds from the manager's indexer vars) up to 120 s for an alert with `agent.name` = that host and `rule.groups` containing `authentication_failed` or `invalid_login`, newer than the start time. Fail with the last query result if none. Wire it into `check_wazuh` in `tests/bin/bw-check` as `bw_run "agent events reach the indexer" ...`. Use the existing indexer credential variables from the lab inventory/role defaults (look them up; do not hardcode new secrets).

- [ ] **Step 5: Green**

Run: `tests/bin/bw-lab provision eye && tests/bin/bw-check all`
Expected: all PASS incl. `black-rocky-02` Active, versions match, agent events arrive. Run `provision eye` again: agent count from `agent_control -l` unchanged (no duplicate enrollment). `blackwall --context lab --once` lists `black-rocky-02` active.

- [ ] **Step 6: Commit** — `feat(eye): wazuh agent on the rhel family`

---

### Task 3: auditd on RHEL

**Files:**
- Modify: `blackwall-eye/ansible/roles/auditd/tasks/main.yml`, `handlers/main.yml`, `defaults/main.yml`

- [ ] **Step 1: Red** — `tests/bin/bw-lab provision eye` already runs `monitored-host.yml` on `wazuh_agents`, now including `black-rocky-02`; record the failure of the apt task there (or, if Task 2 limited it, run `cd blackwall-eye && ansible-playbook ansible/playbooks/monitored-host.yml --limit black-rocky-02`).

- [ ] **Step 2: Implement**
- defaults: `auditd_packages` per family — Debian `[auditd, audispd-plugins]`, RedHat `[audit]` (a dict keyed by family, selected with `ansible_facts['os_family']`).
- tasks: dpkg-lock wait only on Debian; `ansible.builtin.package` with the selected list.
- handlers: Debian keeps `systemctl restart auditd`; RedHat: rules change -> `augenrules --load`, `auditd.conf` change -> `service auditd restart` (RHEL refuses systemctl restart for auditd). Keep `ignore_errors` on `/.dockerenv` for both. Split notify names: `Reload audit rules` (rules file) and `Restart auditd` (auditd.conf).

- [ ] **Step 3: Green** — `tests/bin/bw-lab provision eye && tests/bin/bw-check all` all PASS; `tests/bin/bw-lab ssh black-rocky-02 'rpm -q audit && ls /etc/audit/rules.d/blackwall.rules'`; second provision changed=0 for auditd tasks.

- [ ] **Step 4: Commit** — `feat(eye): auditd on the rhel family`

---

### Task 4: docs and STATUS (controller)

- `blackwall-eye/README.md`: supported OS families for agent/auditd, `wazuh-repo` role, journald on RHEL.
- `tests/README.md`: inventory layout (functional vs OS groups), `wazuh_fixture_agents`, the new agent-events check.
- `blackwall-eye/STATUS.md`: Done entry; `next_task`: sub-project B (manager/indexer/dashboard on RHEL next to podman, dashboard off 443).
- Commit: `docs(eye): wazuh agent on rhel`
