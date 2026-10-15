# blackwall-eye backlog

Work that is known and agreed but not scheduled. Each item says why it exists,
when it becomes due, and how to tell it is done.

## auditd handlers on a real RHEL host

**Why.** The `auditd` role's RedHat handlers were only exercised in the lab,
where containers have no kernel audit: `augenrules --load` and
`service auditd restart` fail there and are ignored on `/.dockerenv`. Their
real behaviour is unproven.

**Due.** Before the `monitored-host.yml` / `auditd` role is applied to any real
RHEL-family host (in particular the prod host when it becomes the Wazuh manager).

**Steps** (on a disposable RHEL 9 / Rocky 9 VM with kernel audit, not a container):

1. Apply `monitored-host.yml` to the VM; `systemctl is-active auditd` is
   `active`, `auditctl -l` lists the rules from `blackwall.rules`.
2. Change a rule in `roles/auditd/templates/blackwall.rules.j2` (e.g. add a
   watch), re-run: the RedHat "Reload audit rules" handler runs
   `augenrules --load`, the play does not fail, `auditctl -l` shows the new rule,
   auditd was not restarted (`systemctl show auditd -p ExecMainStartTimestamp`
   unchanged).
3. Change `auditd_log_format`, re-run: the "Restart auditd" handler runs
   `service auditd restart`, the play does not fail, the new format is in
   `/etc/audit/auditd.conf` and auditd restarted (timestamp changed).
4. Immutable rules: add `-e 2` (as CIS/STIG profiles do), reboot, change a rule,
   re-run. Expected today: `augenrules --load` fails ("The audit system is in
   immutable mode"). Decide and implement the behaviour — e.g. detect
   `auditctl -s | grep 'enabled 2'` and report "reboot required" instead of
   failing the play.
5. A failure on the VM must fail the play (no `/.dockerenv` there, so
   `ignore_errors` is false) — confirm by breaking a rule on purpose.

**Done when** steps 1–5 pass and the immutable-mode behaviour from step 4 is
implemented and documented in the role.
