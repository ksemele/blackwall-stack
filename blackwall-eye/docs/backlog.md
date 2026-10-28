# blackwall-eye backlog

Work that is known and agreed but not scheduled. Each item says why it exists,
when it becomes due, and how to tell it is done.

## Wazuh server stack: prod readiness

**Why.** The server stack is proven on the lab node `black-sentinel-03`
(`docs/superpowers/specs/2026-10-15-wazuh-server-rhel-design.md`), but the lab
relies on lab-only defaults and one setting that differs from the prod host.
From the final review of that work.

**Due.** Before `sentinel.yml`-style roles are applied to the prod host.

1. **Credentials.** `wazuh_indexer_admin_password` (lab value in three role
   defaults) and the API users (`wazuh` / `wazuh-wui`, packaged defaults) must
   come from the owner's secret management (not the repo; no ansible-vault), and the API passwords must be
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
   authd password supplied like the other secrets.
4. **Certificate storage.** A prod inventory must set `wazuh_certs_local_dir`
   outside `tests/` (role defaults fall back to the lab path) and keep the CA
   key off the server.
5. **Sizing.** Lab footprint idle with heap 1g: ~2.2 GiB. Re-measure under load
   and leave headroom for indexer page cache before choosing the heap.

**Done when** each item is implemented or explicitly decided, and a prod
inventory dry run (`--check --diff`) is reviewed by the owner.
