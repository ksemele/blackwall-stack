# Enable syscollector wodle in wazuh-agent role
**Status:** DONE
**Module:** blackwall-eye
**Date:** 2026-05-18
**Commits:** ad486ee

## What was done
- Added `<wodle name="syscollector">` block to `blackwall-eye/ansible/roles/wazuh-agent/templates/ossec.conf.j2` — inserted after `<syscheck>`, before `<active-response>`
- Interval uses Jinja default: `{{ wazuh_syscollector_interval | default('5m') }}`
- Added `wazuh_syscollector_interval: "5m"` to `blackwall-eye/ansible/roles/wazuh-agent/defaults/main.yml`
- Re-deployed role to `black-host-01` via `ansible-playbook ansible/playbooks/monitored-host.yml`
- No README update needed: no existing variables table found in any role or module README

## Verification
- ansible-playbook output: `ok=16 changed=2 unreachable=0 failed=0 skipped=1 rescued=0 ignored=1`
  - `changed`: Deploy ossec.conf from template + Restart wazuh-agent handler
- agent: `grep -c wodle /var/ossec/etc/ossec.conf` → `2`
- agent: `systemctl is-active wazuh-agent` → `active`
- agent: `ls /var/ossec/queue/syscollector/db/` → `local.db` (196608 bytes)
- agent log: `INFO: Module started.` / `INFO: Starting evaluation.` / `INFO: Evaluation finished.`
- API: hardware `total_affected_items: 1` — CPU 10-core @ 2GHz, RAM 16GB
- API: processes `total_affected_items: 16`
- Sample process record:
  ```json
  {
    "name": "systemd",
    "pid": "1",
    "cmd": "/lib/systemd/systemd",
    "state": "S",
    "euser": "root",
    "vm_size": 166840,
    "agent_id": "001"
  }
  ```

## Deviations from plan
- `blackwall-eye/tests/inventory/hosts.yml` does not exist; inventory lives at `tests/inventory/hosts.yml` (repo root). Used absolute path; ansible.cfg references `../tests/inventory/hosts.yml` which resolves correctly when CWD is `blackwall-eye/`.
- No README variables table existed in any wazuh-agent README (no `README.md` in the role dir), so no doc update was required.

## Open questions / concerns
- none
