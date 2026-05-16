# bw-trigger — Synthetic Wazuh event generator

Fires controlled security events on `black-host-01` so you can verify which detections actually fire on your Wazuh stack. Useful while tuning rules, validating a fresh deploy, or demoing the dashboard.

## Prereqs

- Test environment running: `docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 up -d --build`
- Wazuh stack deployed: `cd blackwall-eye && ansible-playbook ansible/playbooks/all.yml`
- `ansible` on PATH (uses `blackwall-eye/ansible.cfg` + `tests/inventory/hosts.yml`)

## Usage

```bash
# From repo root
tests/triggers/bw-trigger list                # show all events
tests/triggers/bw-trigger user                # fire one event
tests/triggers/bw-trigger user passwd sudo    # fire several in sequence
tests/triggers/bw-trigger all                 # fire every event
tests/triggers/bw-trigger clean               # undo persistent side effects
```

## Where to look after triggering

| | |
|---|---|
| Dashboard (Wazuh app) | `https://localhost:5601/app/wz-home` → Security Events |
| Dashboard (Discover)  | `https://localhost:5601/app/discover` → index `wazuh-alerts-*` → filter `agent.name : "black-host-01"` |
| Live tail on manager  | `ansible black-master-00 -m shell -a 'tail -f /var/ossec/logs/alerts/alerts.json' --become` |
| Quick rule extraction | `ansible black-master-00 -m shell -a "tail -20 /var/ossec/logs/alerts/alerts.json | jq -c '{rid: .rule.id, desc: .rule.description}'" --become` |

## Events

| Event | What it does | Expected fire |
|---|---|---|
| `user` | useradd + userdel | 5901 new group, 5902 new user, 5904 user deleted |
| `passwd-change` | chpasswd × 2 on a temp user | 5901 password changed |
| `ssh-key` | Append fake key to `/root/.ssh/authorized_keys`, revert | requires extending FIM `<directories>` to cover it |
| `sudoers` | Drop file in `/etc/sudoers.d/`, remove | FIM if `/etc/sudoers.d/` added to syscheck |
| `passwd` | `touch /etc/passwd` | FIM realtime on `/etc/passwd` |
| `sudo` | non-root user runs `sudo cat /etc/shadow` | 5402 sudo executed |
| `ssh-fail` | 4 bad SSH attempts from inside container | 5503 failed login → 5712 brute force (needs `sshpass` in container) |
| `daemon` | Background `sleep` + nc-listener on :4444 | rootcheck (next scan) |
| `cron` | Drop file in `/etc/cron.d/`, remove | 2902 cron job added |
| `clear-logs` | `truncate -s 0 /var/log/auth.log` | 591 log file emptied |
| `setuid` | `chmod 4755` on /tmp file | requires extending FIM to `/tmp` |
| `package` | `apt install + remove htop` | 2902 package installed |
| `web-shell` | Drop reverse-shell-shaped script in /tmp | needs vulnerability-detection / rootcheck |
| `process-injection` | log `curl|bash` pattern via `logger` | depends on custom rules |
| `audit-fim` | wget into /tmp + chmod 777 | auditd disabled in Docker — limited fire |
| `firewall` | iptables add/remove | depends on container privileges |
| `kernel-module` | `modprobe dummy` | blocked in unprivileged container |

Events marked "needs extending FIM" require `<directories>` entries beyond the default set. To extend, edit `blackwall-eye/ansible/roles/wazuh-agent/templates/ossec.conf.j2`, add the desired paths, and re-run:

```bash
ansible-playbook ansible/playbooks/monitored-host.yml --tags wazuh-agent
```

## Notes

- `bw-trigger all` takes ~60s end-to-end and leaves the host clean.
- Some events depend on rules that aren't enabled by default in the test config (rootcheck deep scans, vulnerability-detection, command monitoring). Those events fire but the alert won't show in the dashboard until you turn the corresponding feature on in the wazuh-manager role.
- Re-run `bw-trigger clean` if you Ctrl-C mid-run or want to make sure the host has no leftover state.
