"""Unit check: the logtest fixtures in tests/rules/ agree with the deployed rules
and decoders, without a running lab.

For every fixture: the declared rule id/level exists in blackwall_rules.xml.j2, the
log is decoded by the decoder it claims (blackwall_decoders.xml.j2), the rule's
match / program_name / field conditions hold on those decoded fields, and no rule
that must NOT fire would fire. Rules 530 and 100200's level-0 parent are stock Wazuh
rules (framework/wazuh/core/tests/data/rules/0015-ossec_rules.xml) and are checked
against the values documented in tests/README.md.

check_diff is stateful and is not simulated here: the fixtures for rule 100100 are
covered by their ordering and their `seed: yes` marker (see tests/README.md).

Run: python tests/unit/rules_fixtures_test.py
"""
import ipaddress
import os
import re
import sys

os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))
RULES_XML = "blackwall-eye/ansible/roles/wazuh-manager/templates/blackwall_rules.xml.j2"
DECODERS_XML = "blackwall-eye/ansible/roles/wazuh-manager/templates/blackwall_decoders.xml.j2"
FIXTURE_DIR = "tests/rules"
STOCK_OSSEC_PREFIX = "ossec: "
# Stock rules the fixtures may name instead of a blackwall rule, with their level
# and match (framework/wazuh/core/tests/data/rules/0015-ossec_rules.xml).
STOCK_RULES = {530: 0}
STOCK_MATCH = {530: r"^ossec: output: "}
# Parent prematch of the stock "ossec" decoder (0005-wazuh_decoders.xml).
OSSEC_PREMATCH = re.compile(r"^ossec: ")
LAB_NETS = [ipaddress.ip_network(n) for n in
            ("172.28.0.0/16", "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "127.0.0.0/8")]


def load_rules():
    rules = {}
    for rid, level, attrs, body in re.findall(r'<rule id="(\d+)" level="(\d+)"([^>]*)>(.*?)</rule>',
                                              open(RULES_XML, encoding="utf-8").read(), re.S):
        def tag(name):
            m = re.search(rf"<{name}>(.*?)</{name}>", body, re.S)
            return m.group(1) if m else None
        rules[int(rid)] = {
            "level": int(level),
            "if_sid": [int(x) for x in re.findall(r"<if_sid>(\d+)</if_sid>", body)],
            "match": tag("match"),
            "program_name": tag("program_name"),
            "fields": re.findall(r'<field name="([^"]+)">(.*?)</field>', body, re.S),
            "desc_fields": re.findall(r"\$\((\w+)\)", tag("description") or ""),
            "noalert": "noalert" in attrs,
        }
    return rules


def load_decoders():
    decoders = []
    for name, body in re.findall(r'<decoder name="([^"]+)">(.*?)</decoder>',
                                 open(DECODERS_XML, encoding="utf-8").read(), re.S):
        def tag(n):
            m = re.search(rf"<{n}[^>]*>(.*?)</{n}>", body, re.S)
            return m.group(1) if m else None
        decoders.append({
            "name": name,
            "parent": tag("parent"),
            "prematch": tag("prematch"),
            "regex": tag("regex"),
            "order": (tag("order") or "").split(","),
        })
    return decoders


def parse_fixture(path):
    head, sep, body = open(path, encoding="utf-8").read().partition("\n---\n")
    if not sep:
        fail(f"{path}: no '---' between the header and the log")
    fields = {}
    for line in head.splitlines():
        line = line.strip()
        if line and not line.startswith("#"):
            key, _, value = line.partition(":")
            fields[key.strip()] = value.strip()
    for key in ("rule", "level"):
        if not fields.get(key, "").isdigit():
            fail(f"{path}: '{key}' must be a number")
    for key in ("log_format", "location"):
        if not fields.get(key):
            fail(f"{path}: missing '{key}'")
    if fields.get("seed", "no") not in ("yes", "no"):
        fail(f"{path}: seed must be yes or no")
    return {"name": os.path.basename(path), "rule": int(fields["rule"]), "level": int(fields["level"]),
            "seed": fields.get("seed", "no") == "yes", "log": body.rstrip("\n")}


def fail(msg):
    print("FAIL " + msg)
    sys.exit(1)


def decode(log, decoders):
    """Emulate analysisd for the manager-side internal logs: the stock 'ossec'
    decoder eats 'ossec: ', its children are tried in file order (prematch first,
    then regex), and <order> names the regex groups."""
    if not log.startswith(STOCK_OSSEC_PREFIX):
        return None, {}
    remainder = OSSEC_PREMATCH.sub("", log, count=1)
    for dec in decoders:
        if dec["parent"] != "ossec":
            continue
        if dec["prematch"] and not re.search(dec["prematch"], remainder):
            continue
        if not dec["regex"]:
            continue
        m = re.search(dec["regex"], remainder)
        if not m:
            continue
        names = [n.strip() for n in dec["order"]]
        if len(names) != len(m.groups()):
            fail(f"decoder {dec['name']}: <order> has {len(names)} names for {len(m.groups())} groups")
        return dec, dict(zip(names, m.groups()))
    return None, {}


def rule_matches(rule, log, fields):
    if rule["program_name"] and not re.search(rule["program_name"], log):
        return False
    if rule["match"] and not re.search(rule["match"], log):
        return False
    for name, pattern in rule["fields"]:
        value = fields.get(name)
        if value is None or not re.search(pattern, value):
            return False
    return True


def check_lab_addresses(name, log):
    for ip in re.findall(r"\b\d{1,3}(?:\.\d{1,3}){3}\b", log):
        addr = ipaddress.ip_address(ip)
        if not any(addr in net for net in LAB_NETS):
            fail(f"{name}: address {ip} is not a lab (172.28.0.0/16) or documentation range")


def check_internal(fx, rules, decoders):
    dec, fields = decode(fx["log"], decoders)
    if fx["rule"] in STOCK_RULES:
        if fx["level"] != STOCK_RULES[fx["rule"]]:
            fail(f"{fx['name']}: stock rule {fx['rule']} is level {STOCK_RULES[fx['rule']]}")
        if not re.search(STOCK_MATCH[fx["rule"]], fx["log"]):
            fail(f"{fx['name']}: rule {fx['rule']} needs a log starting with 'ossec: output: '")
        # 100100's static conditions do match this log: the only reason matching
        # stays on 530 is check_diff, and only a seeded fixture makes that known.
        if not fx["seed"]:
            fail(f"{fx['name']}: expecting {fx['rule']} while 100100 matches needs 'seed: yes'")
        if not rule_matches(rules[100100], fx["log"], fields):
            fail(f"{fx['name']}: 100100 does not match its own baseline log")
        return
    rule = rules.get(fx["rule"])
    if rule is None:
        fail(f"{fx['name']}: rule {fx['rule']} is not in {RULES_XML}")
    if rule["level"] != fx["level"]:
        fail(f"{fx['name']}: rule {fx['rule']} is level {rule['level']}, fixture says {fx['level']}")
    if not rule_matches(rule, fx["log"], fields):
        fail(f"{fx['name']}: rule {fx['rule']}'s match/fields do not hold on this log")
    if (rule["fields"] or rule["desc_fields"]) and dec is None:
        fail(f"{fx['name']}: rule {fx['rule']} needs decoded fields, no decoder matches this log")
    for parent_id in rule["if_sid"]:
        parent = rules.get(parent_id)
        if parent is None:
            if not re.search(STOCK_MATCH[parent_id], fx["log"]):
                fail(f"{fx['name']}: stock if_sid {parent_id} does not match this log")
        elif not rule_matches(parent, fx["log"], fields):
            fail(f"{fx['name']}: if_sid {parent_id} does not match this log either")
    for desc_field in rule["desc_fields"]:
        if desc_field not in fields:
            fail(f"{fx['name']}: description uses $({desc_field}), decoder "
                 f"{dec['name'] if dec else 'none'} does not set it")
    # Rules that must stay silent: every other child of the rule we reached.
    for other_id, other in rules.items():
        if other_id == fx["rule"] or fx["rule"] not in other["if_sid"]:
            continue
        if rule_matches(other, fx["log"], fields):
            fail(f"{fx['name']}: sibling rule {other_id} matches this log too")


def check_syslog(fx, rules):
    m = re.match(r"^[A-Z][a-z]{2} \d{1,2} \d{2}:\d{2}:\d{2} (\S+) ([^:\[]+)(?:\[\d+\])?: (.*)$",
                 fx["log"], re.S)
    if not m:
        fail(f"{fx['name']}: not a syslog line (timestamp, host, program[: message])")
    host, program, message = m.groups()
    rule = rules.get(fx["rule"])
    if rule is None:
        fail(f"{fx['name']}: rule {fx['rule']} is not in {RULES_XML}")
    if rule["level"] != fx["level"]:
        fail(f"{fx['name']}: rule {fx['rule']} is level {rule['level']}, fixture says {fx['level']}")
    if not rule["program_name"] or not re.search(rule["program_name"], program):
        fail(f"{fx['name']}: program_name '{program}' does not match rule {fx['rule']}")
    if not rule["match"] or not re.search(rule["match"], message):
        fail(f"{fx['name']}: rule {fx['rule']} does not match the message")
    # 100207 is evaluated before 100209 in the group: a WARNING line must not
    # contain ERROR, otherwise the higher-level rule wins.
    if fx["rule"] == 100209 and re.search(rules[100207]["match"], message):
        fail(f"{fx['name']}: 100209 fixture also matches 100207")
    if not host.startswith("black-"):
        fail(f"{fx['name']}: host {host} is not a lab host (black-*)")


def main():
    rules, decoders = load_rules(), load_decoders()
    if not rules:
        fail(f"no <rule> found in {RULES_XML}")
    paths = sorted(os.path.join(FIXTURE_DIR, p) for p in os.listdir(FIXTURE_DIR) if p.endswith(".txt"))
    if not paths:
        fail(f"no fixtures in {FIXTURE_DIR}")
    fixtures = [parse_fixture(p) for p in paths]
    for fx in fixtures:
        check_lab_addresses(fx["name"], fx["log"])
        if fx["log"].startswith(STOCK_OSSEC_PREFIX):
            check_internal(fx, rules, decoders)
        else:
            check_syslog(fx, rules)
    # 100100: the seeded baseline must be evaluated before the changed snapshot.
    names = [fx["name"] for fx in fixtures]
    for required in ("100100-baseline.txt", "100100-changed.txt"):
        if required not in names:
            fail(f"{FIXTURE_DIR} must keep {required}")
    if names.index("100100-baseline.txt") > names.index("100100-changed.txt"):
        fail("100100-baseline.txt must sort before 100100-changed.txt")
    for fx in fixtures:
        if fx["rule"] == 530 and not fx["seed"]:
            fail(f"{fx['name']}: rule 530 fixture needs 'seed: yes'")
    print(f"{len(fixtures)} fixtures checked against {RULES_XML} and {DECODERS_XML}")
    print("FIXTURES OK")


main()
