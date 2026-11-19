"""Unit check: the custom Wazuh rules are self-consistent and match the README.

Reads the rules template as literal XML — standard library only, no lab, no
virtualenv:
    python3 tests/unit/rules_lint_test.py

Fails (exit 1, one readable line per problem) on:
- a duplicated rule ID, or an ID outside Wazuh's custom range 100000-120000;
- a rule without a level, a level outside 0-16, or no <description>;
- an <if_sid> naming a rule that neither exists in the file nor is a stock rule
  from STOCK_SIDS;
- drift with blackwall-eye/README.md: a rule missing from the README, a README
  level that differs from the XML level, a README row for a rule that is gone,
  or two README rows for one rule with different levels.
"""
import os
import re
import sys
import xml.etree.ElementTree as ET

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
RULES = "blackwall-eye/ansible/roles/wazuh-manager/templates/blackwall_rules.xml.j2"
README = "blackwall-eye/README.md"

ID_MIN, ID_MAX = 100000, 120000      # Wazuh's custom rule range
LEVEL_MIN, LEVEL_MAX = 0, 16         # Wazuh rule level: 0 to 16
#   (Wazuh docs, user-manual/ruleset/ruleset-xml-syntax/rules.rst, `level` ->
#    "Allowed values: 0 to 16"; level 0 means "decoded but no alert")

# Stock parents the blackwall rules hang off. Add an ID here when a new stock
# rule is used as <if_sid>; anything else must be defined in the rules file.
STOCK_SIDS = {"530"}                 # 530: ossec: output: '<alias>' (command localfile)


class FileMissing(Exception):
    """A file the lint needs is absent — reported as one FAIL line, not a traceback."""


def read_text(path):
    try:
        return open(os.path.join(ROOT, path), encoding="utf-8").read()
    except OSError as e:
        raise FileMissing(f"{path}: {e.strerror or e}") from e


def parse_rules(path=RULES):
    rules, problems = [], []
    text = read_text(path)
    if "{{" in text or "{%" in text:
        # A conditional rule cannot be linted as literal XML; say so instead of
        # reporting a parse error.
        return rules, [f"{path}: contains Jinja markup — the lint reads the rules as "
                       "literal XML, so a conditional rule needs the lint updated first"]
    try:
        root = ET.fromstring(text)
    except ET.ParseError as e:
        return rules, [f"{path}: not well-formed XML ({e}); Wazuh would refuse to load it"]
    if root.tag != "group":
        problems.append(f"{path}: root element is <{root.tag}>, expected a single <group>")
    # Element line numbers: ElementTree only reports them for some parsers, so count
    # the <rule> tags in the text (iter() walks them in document order).
    lines = [text[:m.start()].count("\n") + 1 for m in re.finditer(r"<rule\b", text)]
    for i, el in enumerate(root.iter("rule")):
        rules.append({
            "line": lines[i] if i < len(lines) else 0,
            "id": (el.get("id") or "").strip(),
            "level": (el.get("level") or "").strip(),
            "desc": (el.findtext("description") or "").strip(),
            # <if_sid> takes "a list of rule IDs separated by commas or spaces".
            "parents": [s for node in el.findall("if_sid")
                        for s in re.split(r"[,\s]+", node.text or "") if s],
        })
    return rules, problems


def parse_readme(path=README):
    """Rules declared in the README: table rows `| <id> | <level> | ... |` plus the
    levels stated in prose paragraphs that name a rule ID."""
    table, prose, problems = {}, {}, []
    para = []
    for n, line in enumerate(read_text(path).splitlines() + [""], 1):
        row = re.match(r"^\|\s*(\d{6})\s*\|\s*(\d{1,2})\s*\|", line)
        if row and ID_MIN <= int(row.group(1)) <= ID_MAX:
            rid, level = row.group(1), int(row.group(2))
            if rid in table and table[rid][0] != level:
                problems.append(f"{path}:{table[rid][1]} and :{n}: two rows for rule "
                                f"{rid} with different levels ({table[rid][0]} vs {level})")
            table[rid] = (level, n)
        if line.strip():
            para.append(line)
            continue
        if para:
            # A level stated in prose counts for the rule ID it sits nearest to, so a
            # paragraph about two rules does not give both the same level.
            text = " ".join(para)
            ids = list(re.finditer(r"\b(\d{6})\b", text))
            for lv in re.finditer(r"\blevel\s+(\d{1,2})\b", text):
                if not ids:
                    break
                nearest = min(ids, key=lambda m: abs(m.start() - lv.start()))
                prose.setdefault(nearest.group(1), set()).add(int(lv.group(1)))
            para = []
    return table, prose, problems


def check_rules(rules):
    problems, lines_of = [], {}
    for r in rules:
        loc = f"{RULES}:{r['line']}"
        if not r["id"].isdigit():
            problems.append(f"{loc}: <rule> without a numeric id")
            continue
        if not ID_MIN <= int(r["id"]) <= ID_MAX:
            problems.append(f"{loc}: rule {r['id']} is outside the custom range "
                            f"{ID_MIN}-{ID_MAX}")
        if r["id"] in lines_of:
            problems.append(f"{loc}: rule {r['id']} duplicates the rule at line "
                            f"{lines_of[r['id']]}")
        lines_of.setdefault(r["id"], r["line"])
        if not r["level"].isdigit():
            problems.append(f"{loc}: rule {r['id']} has no numeric level")
        elif not LEVEL_MIN <= int(r["level"]) <= LEVEL_MAX:
            problems.append(f"{loc}: rule {r['id']} level {r['level']} is outside "
                            f"{LEVEL_MIN}-{LEVEL_MAX}")
        if not r["desc"]:
            problems.append(f"{loc}: rule {r['id']} has no <description>")
    for r in rules:
        for sid in r["parents"]:
            if not sid.isdigit():
                problems.append(f"{RULES}:{r['line']}: rule {r['id']} <if_sid> {sid!r} "
                                "is not a rule ID")
            elif sid not in lines_of and sid not in STOCK_SIDS:
                problems.append(f"{RULES}:{r['line']}: rule {r['id']} <if_sid> {sid} is "
                                f"neither defined in {RULES} nor a stock rule")
    return problems


def check_readme(rules, table, prose):
    problems = []
    for r in rules:
        rid = r["id"]
        if not rid.isdigit() or not r["level"].isdigit():
            continue  # already reported by check_rules
        if rid in table:
            level, line = table[rid]
            if level != int(r["level"]):
                problems.append(f"{README}:{line}: rule {rid} is level {level} in the "
                                f"README but level {r['level']} in {RULES}")
        elif rid in prose:
            if int(r["level"]) not in prose[rid]:
                problems.append(f"{README}: rule {rid} is documented at level(s) "
                                f"{sorted(prose[rid])} but {RULES} has level {r['level']}")
        else:
            problems.append(f"{README}: rule {rid} ({RULES}:{r['line']}) is not in the "
                            "rules table nor described in prose with its level")
    xml_ids = {r["id"] for r in rules}
    for rid, (level, line) in sorted(table.items()):
        if rid not in xml_ids:
            problems.append(f"{README}:{line}: rule {rid} (level {level}) is documented "
                            f"but is not defined in {RULES}")
    return problems


def main():
    try:
        rules, parse_problems = parse_rules()
        table, prose, readme_problems = parse_readme()
    except FileMissing as e:
        print(f"FAIL {e}")
        return 1
    if parse_problems:
        problems = parse_problems + [f"{RULES}: the rule and README checks are skipped "
                                     "until the file parses as XML"]
    elif not rules:
        problems = [f"{RULES}: no <rule> elements found"] + readme_problems
    else:
        problems = (check_rules(rules) + check_readme(rules, table, prose)
                    + readme_problems)
    for p in problems:
        print("FAIL " + p)
    if problems:
        print(f"{len(problems)} problem(s) in {RULES} / {README}")
        return 1
    print(f"RULES OK: {len(rules)} rules in {RULES} match the {len(table)} table rows "
          f"and the prose of {README}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
