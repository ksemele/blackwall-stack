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

## Wazuh server stack: prod readiness

**Why.** The server stack is proven on the lab node `black-sentinel-03`
(`docs/superpowers/specs/2026-10-15-wazuh-server-rhel-design.md`), but the lab
relies on lab-only defaults and one setting that differs from the prod host.
From the final review of that work.

**Due.** Before `sentinel.yml`-style roles are applied to the prod host.

1. **Credentials.** `wazuh_indexer_admin_password` (lab value in three role
   defaults) and the API users (`wazuh` / `wazuh-wui`, packaged defaults) must
   come from ansible-vault in the prod inventory, and the API passwords must be
   changed on the host. Add a role assert that refuses the lab defaults when
   `bw_lab` is not set. Loopback binding narrows exposure but does not remove
   it (any local process or container egress can reach 127.0.0.1).
2. **Podman firewall driver.** The lab fixture pins netavark
   `firewall_driver = "nftables"` (`table inet netavark`); the prod host runs
   the iptables-nft driver (`ip nat` / `ip filter`). Either add a lab fixture
   variant with the iptables driver and re-run `bw-check sentinel`, or decide to
   switch prod to the nftables driver (recreates containers — maintenance
   window and runbook).
3. **Agent enrollment.** `use_password` is `no`; `wazuh_agent_sources` is the
   only gate on 1514/1515. In prod list explicit agent IPs and/or enable a
   vaulted authd password.
4. **Certificate storage.** A prod inventory must set `wazuh_certs_local_dir`
   outside `tests/` (role defaults fall back to the lab path) and keep the CA
   key off the server.
5. **Sizing.** Lab footprint idle with heap 1g: ~2.2 GiB. Re-measure under load
   and leave headroom for indexer page cache before choosing the heap.

**Done when** each item is implemented or explicitly decided, and a prod
inventory dry run (`--check --diff`) is reviewed by the owner.
