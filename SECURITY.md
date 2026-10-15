# Security Policy

## Reporting a vulnerability

Please report vulnerabilities **privately** through GitHub's private
vulnerability reporting: open the repo's **Security** tab and choose
**Report a vulnerability**
([direct link](https://github.com/ksemele/blackwall-stack/security/advisories/new)).

Do not open a public issue, pull request or Discussion for a security problem.

Include what you can:

- the affected module, role, rule or script, and the commit or release;
- the Wazuh version and target distro, if relevant;
- steps to reproduce (the Docker lab in `tests/` is ideal), and the impact you
  see;
- a suggested fix, if you have one.

Never include real hostnames, IPs, credentials or logs from production systems
you don't own. Redact what you can't leave out.

This is a small project maintained in spare time. Expect an acknowledgement
within a few days. Once a fix is ready we publish a GitHub security advisory
and credit you, unless you prefer to stay anonymous.

## Supported versions

Only the latest commit on `main` is supported. Fixes are not backported.

## Scope

In scope, for code in this repo:

- **Ansible roles and playbooks**: for example a role that opens a port it
  shouldn't, writes secrets with loose permissions or into logs, weakens a
  service's default configuration, or leaves a host less protected than its
  docs claim (firewall, cf-wall, Fang blocklists).
- **Detection rules and decoders**: for example a rule an attacker can trivially
  evade or silence, or one that can be abused to hide other alerts.
- **Scripts**: the refresh, export and alerting scripts the roles install
  (Fang, cf-wall, firewall export, Voice, Brain), and `blackwall-scan`, for
  example command injection, unsafe parsing of fetched data, or a fail-open
  path that drops protection silently.

Out of scope:

- **The lab's default credentials** (`admin` / `Blackwall1!`, `root` /
  `blackwall`, `wazuh` / `wazuh`, Grafana `admin` / `blackwall`), the
  self-signed certificates and the privileged containers in `tests/`. They are
  intentional, lab-only and documented. The lab must never be exposed beyond
  `localhost`. A role that ships one of these as a default for real hosts *is*
  in scope.
- Vulnerabilities in upstream software (Wazuh, OpenSearch, auditd, nftables,
  Prometheus, Grafana and others). Report those upstream; a report about how
  this repo configures them is welcome here.
- The terminal UI. Report it in [ksemele/blackwall](https://github.com/ksemele/blackwall)
  following that repo's security policy.
- Findings that need an attacker who already has root on the monitored host or
  the manager.
