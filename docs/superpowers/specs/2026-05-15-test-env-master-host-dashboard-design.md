# Test Environment: master + host with Wazuh dashboard

**Status:** design
**Date:** 2026-05-15
**Module:** blackwall-eye (test environment + roles)

## Goal

Replace the current 3-container Docker test environment (`black-00/01/02`) with a
focused 2-container topology:

- `black-master-00` — Wazuh manager + indexer + dashboard (all-in-one)
- `black-host-01` — Wazuh agent (and target for auditd / future Suricata)

The user must be able to:

1. Bring the env up/down with a single documented command.
2. Understand from the Ansible code how the Wazuh server + dashboard are deployed
   (no opaque vendor shell installers).
3. Point any agent at *any* master (test, staging, prod) by overriding a small set
   of variables, with the test-env master as the default.
4. Open the Wazuh dashboard from the macOS host browser.

## Non-goals

- Production-grade Wazuh HA (single-node indexer is fine for test).
- TLS cert rotation tooling — certs are generated once per environment lifetime.
- Hardening of default dashboard credentials — these are fixed test values.
- Multi-distro: future `debian12`/`centos9` profiles stay commented stubs.
- Fixing the Telegram bot token leak in `tests/inventory/hosts.yml`
  (a real secret is committed at line 9; flagged here, addressed separately).
- Changes to `bw-status` CLI — keeps working against the manager API on the
  docker network unchanged.

## Container topology

| Container | Role | Host ports | Static IP |
|---|---|---|---|
| `black-master-00` | Wazuh manager + indexer + dashboard | `2200→22`, `5601→443` | `172.28.0.10` |
| `black-host-01`   | Wazuh agent + auditd | `2201→22` | `172.28.0.11` |

Compose profile `ubuntu2204` retained. `black-02` removed. `hosts.single.yml`
removed (covered by `--limit black-master-00`). Future-distro stub blocks remain
in `docker-compose.yml` but are renamed to match the new naming
(`black-master-00-debian`, `black-host-01-debian`, …).

## Ansible roles

Three roles live under `blackwall-eye/ansible/roles/`:

| Role | Purpose | Existing? |
|---|---|---|
| `wazuh-indexer`    | install indexer pkg, deploy `opensearch.yml`, install TLS, start service, push internal users via `securityadmin.sh` | NEW |
| `wazuh-manager`    | install manager pkg, deploy `ossec.conf`, install `filebeat` (Wazuh-shipped variant), template `/etc/filebeat/filebeat.yml` with indexer host + `wazuh_api_user`/`wazuh_api_password` + cert paths, start manager + filebeat services | EXISTS — needs filebeat indexer-output wiring added |
| `wazuh-dashboard`  | install dashboard pkg, deploy `opensearch_dashboards.yml` pointing at local indexer, install TLS, start service on `:443` | NEW |
| `wazuh-agent`      | install agent pkg, deploy `ossec.conf` with parameterized endpoint/enrollment, write `authd.pass` if set, start service | EXISTS — needs endpoint/secret vars |
| `auditd`           | unchanged | EXISTS |

All three Wazuh roles install from the existing `packages.wazuh.com` apt repo
(same key, same `wazuh_version` default of `4.14.5`). Pattern mirrors the
current `wazuh-manager` role: prereqs → key → repo → install → template → systemd.

## Certificates

One-shot playbook: `ansible/playbooks/wazuh-certificates.yml`.

- Runs on `black-master-00` only.
- Downloads the official `wazuh-certs-tool.sh` and the matching `config.yml`
  template (with node list = master).
- Generates a cert bundle in `/etc/wazuh-certs/` on the container, then fetches
  the bundle to the controller (`./tests/.wazuh-certs/`, gitignored).
- Subsequent plays (`security-node.yml`) copy the right cert files into each
  service's expected location.
- Idempotent: if the bundle exists on disk on the controller, regeneration is
  skipped. To rotate, `rm -rf tests/.wazuh-certs/` and re-run.

Separate from `security-node.yml` so that re-deploying the indexer/manager/dashboard
roles never invalidates trust between components.

## Agent endpoint and secrets model

Pure Ansible variables in `wazuh-agent/defaults/main.yml`:

```yaml
# Identity
wazuh_agent_name: "{{ inventory_hostname }}"
wazuh_agent_groups: "default"

# Master endpoint
wazuh_manager_host: "172.28.0.10"
wazuh_manager_port: 1514
wazuh_manager_protocol: "tcp"

# Enrollment (authd)
wazuh_enrollment_host: "{{ wazuh_manager_host }}"
wazuh_enrollment_port: 1515
wazuh_enrollment_password: ""
```

Template behavior in `roles/wazuh-agent/templates/ossec.conf.j2`:

- `<server>` block always uses the four `wazuh_manager_*` vars.
- `<enrollment>` block always present.
- When `wazuh_enrollment_password` is a non-empty string, an
  `<authorization_pass_path>/var/ossec/etc/authd.pass</authorization_pass_path>`
  child is added AND a tasks/main.yml step writes the password to that file
  (mode `0640`, owner `root:wazuh`) before the service is started/restarted.

Override mechanisms (in increasing scope):
- `-e wazuh_manager_host=...` on a single playbook run
- `host_vars/<host>.yml` for a specific agent
- `group_vars/<group>.yml` for a group of agents
- Editing `tests/inventory/hosts.yml` — only for the test env defaults

### Real-host deployment

The same `wazuh-agent` role and `monitored-host.yml` playbook are used for real
hosts; only the inventory differs. The test-env inventory (`tests/inventory/hosts.yml`)
is Docker-specific (localhost SSH ports). For production, the user writes their
own inventory — example shape:

```yaml
# inventory/prod.yml (user-maintained, outside this repo or in a separate file)
all:
  vars:
    ansible_user: deploy
    ansible_ssh_private_key_file: ~/.ssh/id_ed25519
  children:
    wazuh_agents:
      vars:
        wazuh_manager_host: "wazuh.prod.example.com"
        wazuh_enrollment_password: "<from-vault>"
      hosts:
        web-01.prod.example.com: {}
        web-02.prod.example.com: {}
```

```bash
ansible-playbook -i inventory/prod.yml ansible/playbooks/monitored-host.yml
```

The `tests/README.md` "Point agents at a different master" section will show
both the `-e`-override form (quick, one-off) and the inventory form (durable,
multi-host).

## Dashboard access

- Container `black-master-00` publishes `5601→443`.
- Browse `https://localhost:5601` from macOS host.
- Self-signed cert; browser warning expected; documented as such.

Wazuh has **two independent credential systems** and we treat them separately:

1. **Indexer / OpenSearch security plugin** — used by the dashboard to authenticate
   to the indexer, by filebeat (on the manager) to ship alerts to the indexer,
   and by the dashboard login UI. One user covers all three.
2. **Manager REST API** (`:55000`, auth via `/var/ossec/api/configuration/api.yaml`)
   — used by the dashboard to call manager APIs and by `bw-status` / `verify-api.yml`.
   Stays at the existing test default (`wazuh / wazuh`) for now; out of scope to change.

Credentials live in `group_vars/wazuh_managers.yml` (consumed by indexer, manager,
and dashboard roles — role-defaults are per-role):

```yaml
# Indexer / dashboard / filebeat credentials
wazuh_indexer_admin_user: "admin"
wazuh_indexer_admin_password: "Blackwall1!"

# Manager REST API credentials (the wazuh-dashboard role uses these to talk
# to the manager API; verify-api.yml uses these too)
wazuh_manager_api_user: "wazuh"
wazuh_manager_api_password: "wazuh"
```

- `wazuh-indexer` role hashes `wazuh_indexer_admin_password` via
  `password_hash('bcrypt')`, templates it into
  `/etc/wazuh-indexer/opensearch-security/internal_users.yml`, then runs
  `/usr/share/wazuh-indexer/plugins/opensearch-security/tools/securityadmin.sh`
  to push the security config (idempotent: re-runs apply only on diff).
- `wazuh-manager` role uses the indexer credentials in
  `/etc/filebeat/filebeat.yml` so the manager can ship alerts to the indexer.
- `wazuh-dashboard` role uses indexer credentials for `opensearch.username` /
  `opensearch.password` and manager-API credentials for the embedded Wazuh app
  config in `/usr/share/wazuh-dashboard/data/wazuh/config/wazuh.yml`.

OpenSearch password complexity is enforced upstream — `Blackwall1!` satisfies
the default policy (length ≥ 8, upper, lower, digit, special).

## Playbook orchestration

- `playbooks/all.yml` — runs `wazuh-certificates.yml` → `security-node.yml` → `monitored-host.yml`. Single command for full env bring-up.
- `playbooks/security-node.yml` — applies, on `wazuh_managers` group, in order: `wazuh-indexer`, `wazuh-manager`, `wazuh-dashboard`. Tags `wazuh-indexer`, `wazuh-manager`, `wazuh-dashboard` for targeted runs.
- `playbooks/monitored-host.yml` — unchanged shape; `wazuh-agent` + `auditd` roles on `wazuh_agents` group.
- `playbooks/verify-agents.yml`, `verify-api.yml`, `verify-auditd.yml` — unchanged in spirit; host names updated.

## Inventory

`tests/inventory/hosts.yml`:

```yaml
all:
  vars:
    ansible_user: root
    ansible_password: blackwall
    ansible_ssh_common_args: >-
      -o StrictHostKeyChecking=no
      -o UserKnownHostsFile=/dev/null
    ansible_python_interpreter: /usr/bin/python3
    # NOTE: telegram_* vars currently in this file are secrets and should
    # move to an out-of-band secrets file. Tracked separately.

  children:
    ubuntu2204:
      children:
        wazuh_managers:
          hosts:
            black-master-00:
              ansible_host: localhost
              ansible_port: 2200
        wazuh_agents:
          hosts:
            black-host-01:
              ansible_host: localhost
              ansible_port: 2201
```

`tests/inventory/hosts.single.yml` — deleted.

## Documentation

**Root `README.md`** gains a `## Test environment` section: 2-container summary,
the two bring-up commands (compose + ansible), the dashboard URL/creds, the
tear-down command, and a pointer to `tests/README.md`.

**`tests/README.md`** rewritten end-to-end:

- Host topology table reflects `black-master-00` / `black-host-01`.
- Section "Containers: up / down" — single command (no separate full vs single).
- Section "Full deployment" — `ansible-playbook ansible/playbooks/all.yml`.
- Section "Run a playbook on a specific host" — `--limit` examples updated.
- Section "Run a single role" — tags updated, `--limit` examples updated.
- Section "Single-container test" — removed (no longer applicable).
- NEW section "Connect to the dashboard" — URL, credentials, browser warning,
  how to change defaults.
- NEW section "Point agents at a different master" — `-e` override recipe with
  and without `wazuh_enrollment_password`.
- File-layout block updated.

## Acceptance criteria

1. `docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build` brings up exactly two containers: `black-master-00`, `black-host-01`.
2. `cd blackwall-eye && ansible-playbook ansible/playbooks/all.yml` completes with `failed=0` on a clean env.
3. Browser request to `https://localhost:5601` returns the Wazuh dashboard login page; `admin / Blackwall1!` logs in.
4. In the dashboard, the "Agents" page lists `black-host-01` with status `Active`.
5. `ansible-playbook ansible/playbooks/verify-agents.yml` passes.
6. Running the agent playbook with `-e wazuh_manager_host=192.0.2.1 -e wazuh_enrollment_password=hunter2` rewrites `/var/ossec/etc/ossec.conf` to target `192.0.2.1` and writes `/var/ossec/etc/authd.pass` with `hunter2` (no need to verify connection succeeds — only that config is rendered correctly).
7. `docker compose ... down -v` removes all volumes; subsequent `up` rebuilds cleanly.
8. Root `README.md` and `tests/README.md` reflect all of the above; no references to `black-00`, `black-01`, `black-02`, or `hosts.single.yml` remain.

## Out of scope / follow-ups

- Telegram bot token leak at `tests/inventory/hosts.yml:9` — move to secrets file.
- `bw-status` running from macOS host (would need API port mapping + cert trust).
- Suricata / Zeek roles (Stage 3 of blackwall-eye).
- Multi-distro support beyond Ubuntu 22.04.
