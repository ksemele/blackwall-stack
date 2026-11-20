"""Unit check: the ossec.conf templates read the log source the host OS really has.

Debian 12 ships without rsyslog. A syslog localfile on such a host reads nothing,
so every syslog-based detection (failed ssh logins, Fang's 100207/100209) stops
firing silently. Both the agent and the manager template must therefore read
journald on the RHEL family and on Debian 12+, and /var/log/auth.log +
/var/log/syslog on Ubuntu and on Debian 11 (which still ships rsyslog).

Renders the role templates with the role defaults — no host, no Wazuh, no network:
    python tests/unit/ossec_log_sources_test.py
Needs jinja2 and pyyaml (see CONTRIBUTING.md).
"""
import os
import re

import jinja2
import yaml

os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))

ROLES = {
    "agent": "blackwall-eye/ansible/roles/wazuh-agent",
    "manager": "blackwall-eye/ansible/roles/wazuh-manager",
}

# ansible_facts keys the templates read (values match /etc/os-release of the lab images).
CASES = {
    "Debian 12": dict(os_family="Debian", distribution="Debian", distribution_major_version="12"),
    "Debian 11": dict(os_family="Debian", distribution="Debian", distribution_major_version="11"),
    "Ubuntu 22.04": dict(os_family="Debian", distribution="Ubuntu", distribution_major_version="22"),
    "Rocky 9": dict(os_family="RedHat", distribution="Rocky", distribution_major_version="9"),
}
JOURNAL_ONLY = {"journald"}
RSYSLOG_FILES = {"/var/log/auth.log", "/var/log/syslog"}
EXPECTED = {
    "Debian 12": JOURNAL_ONLY,
    "Debian 11": RSYSLOG_FILES,
    "Ubuntu 22.04": RSYSLOG_FILES,
    "Rocky 9": JOURNAL_ONLY,
}


def _nums(value):
    return [int(x) for x in re.findall(r"\d+", str(value))]


def version_test(value, target=None, method="=="):
    """Shim for Ansible's `version` test, which plain jinja2 does not provide."""
    a, b = _nums(value), _nums(target)
    n = max(len(a), len(b))
    a, b = a + [0] * (n - len(a)), b + [0] * (n - len(b))
    ops = {"==": a == b, "!=": a != b, ">": a > b, "<": a < b, ">=": a >= b, "<=": a <= b}
    return ops[method]


def render(role_dir, facts, extra):
    """Role defaults + ansible_facts -> rendered ossec.conf."""
    ctx = dict(inventory_hostname="black-test-01", ansible_facts=facts)
    # _wazuh_enroll_groups is set by a role task; the template needs it either way.
    ctx.setdefault("_wazuh_enroll_groups", "default")
    for key, value in yaml.safe_load(open(f"{role_dir}/defaults/main.yml")).items():
        ctx[key] = jinja2.Template(value).render(**ctx) if isinstance(value, str) else value
    ctx.update(extra)
    src = open(f"{role_dir}/templates/ossec.conf.j2").read()
    env = jinja2.Environment(undefined=jinja2.StrictUndefined)
    env.tests["version"] = version_test
    return env.from_string(src).render(**ctx)


def system_log_sources(rendered):
    """<location> values of the system-log localfiles (manager's own ossec logs excluded)."""
    # ossec.conf is not strictly parseable XML (the fw-export command carries a raw
    # '&'), so the localfile blocks are matched by hand.
    opens, closes = rendered.count("<localfile>"), rendered.count("</localfile>")
    assert opens == closes, f"{opens} <localfile> vs {closes} </localfile>"
    return {loc for loc in re.findall(r"<location>([^<]+)</location>", rendered)
            if not loc.startswith("/var/ossec")}


for case, facts in CASES.items():
    found = {}
    for name, role_dir in ROLES.items():
        found[name] = system_log_sources(render(role_dir, facts, {}))
        assert found[name] == EXPECTED[case], f"{name} template on {case}: {sorted(found[name])}"
    assert found["agent"] == found["manager"], f"{case}: agent {found['agent']} vs manager {found['manager']}"
    print(f"{case:14s} -> {sorted(found['agent'])}")

# The journald branch must not leave an rsyslog file behind, and the rsyslog
# branch must not claim journald (reading a journal that is not there is an
# agent error). The manager's own /var/ossec/logs/active-responses.log stays a
# syslog file on every OS — Wazuh writes it itself.
for name, role_dir in ROLES.items():
    journal = render(role_dir, CASES["Debian 12"], {})
    for leftover in RSYSLOG_FILES:
        assert f"<location>{leftover}</location>" not in journal, \
            f"{name}: {leftover} localfile left on Debian 12"
    files = render(role_dir, CASES["Ubuntu 22.04"], {})
    assert "<location>journald</location>" not in files, f"{name}: journald localfile on Ubuntu 22.04"

print("LOG SOURCES OK")
