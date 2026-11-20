"""Unit check: the knobs that make the test lab come up on rootless podman.

Three of them only break the lab at runtime, so a live `bw-lab up` is the only
other way to catch their loss:

- `cgroup: ${BW_LAB_CGROUP:-host}` — rootless podman cannot mount /sys/fs/cgroup
  in the host cgroup namespace, and systemd exits 255 (tests/README.md, section 2).
- `pids_limit` — rootless podman caps the container at 2048 pids and systemd sizes
  every TasksMax as 15% of it; wazuh-analysisd then cannot create a pthread.
- `wazuh_fim_realtime` — real-time FIM needs one inotify instance per monitored
  directory, and a rootless container is billed for them out of the host user's
  single fs.inotify.max_user_instances budget, which the runtime itself has
  already spent. syscheckd logs "Unable to initialize inotify" and the agent dies
  at its 45 s start timeout (tests/README.md, section 20).
- `wazuh_indexer_memory_lock` — OpenSearch exits 78 when `bootstrap.memory_lock`
  is true and mlockall() fails, which it always does under a rootless runtime.

Run with the interpreter that has jinja2 and yaml (the lab's, or the repo-check venv):
    python tests/unit/rootless_lab_test.py
"""
import os
import re

import jinja2
import yaml

os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))

EYE = "blackwall-eye/ansible"
AGENT_ROLE = f"{EYE}/roles/wazuh-agent"
MANAGER_ROLE = f"{EYE}/roles/wazuh-manager"
INDEXER_ROLE = f"{EYE}/roles/wazuh-indexer"
LAB_VARS = "tests/inventory/group_vars/all.yml"
COMPOSE = "tests/docker/docker-compose.yml"

# Set by the roles' tasks or by fact gathering, not by defaults/main.yml.
RUNTIME_FACTS = {
    "_wazuh_enroll_groups": "docker",
    "ansible_facts": {"os_family": "Debian"},
}

env = jinja2.Environment(undefined=jinja2.StrictUndefined)
# `bool` is an Ansible filter; the templates use it to normalize a value that may
# arrive as a string from the command output or as a real boolean from an override.
env.filters["bool"] = lambda v: v is True or str(v).strip().lower() in ("1", "true", "yes", "on")
fails = []


def render(role, template="ossec.conf.j2", **over):
    ctx = yaml.safe_load(open(f"{role}/defaults/main.yml"))
    ctx.update(RUNTIME_FACTS)
    ctx.update(over)
    return env.from_string(open(f"{role}/templates/{template}").read()).render(**ctx)


def syscheck(src):
    return re.search(r"<syscheck>(.*?)</syscheck>", src, re.S).group(1)


lab = yaml.safe_load(open(LAB_VARS))

# 1. Real-time FIM: on for a real host (role default), off where the lab says so,
#    and switching it off must not switch off file integrity monitoring itself.
for role, name in ((AGENT_ROLE, "agent"), (MANAGER_ROLE, "manager")):
    dirs = re.findall(r"<directories[^>]*>", syscheck(render(role)))
    if not dirs:
        fails.append(f"{name}: template renders no <directories>")
        continue
    lost = [d for d in dirs if 'realtime="yes"' not in d]
    if lost:
        fails.append(f"{name}: role default lost real-time FIM: {lost}")

    dirs = re.findall(r"<directories[^>]*>", syscheck(render(role, wazuh_fim_realtime=False)))
    kept = [d for d in dirs if 'realtime="no"' not in d]
    if kept:
        fails.append(f"{name}: real-time survived in {kept}")
    block = syscheck(render(role, wazuh_fim_realtime=False))
    if "<disabled>yes</disabled>" in block or not re.search(r"<frequency>\d+</frequency>", block):
        fails.append(f"{name}: turning real-time off also stopped scanning")

# 2. The lab must actually use the switch, and scan often enough that the file
#    triggers in tests/triggers show up while a check is still waiting.
if lab.get("wazuh_fim_realtime") is not False:
    fails.append(f"{LAB_VARS}: wazuh_fim_realtime must be false, got {lab.get('wazuh_fim_realtime')}")
if lab.get("bw_lab") is not True:
    fails.append(f"{LAB_VARS}: bw_lab marker lost")
if int(lab.get("wazuh_syscheck_frequency", 43200)) > 3600:
    fails.append(f"{LAB_VARS}: wazuh_syscheck_frequency is too long to observe a trigger")

# 3. Compose anchor: cgroup namespace stays `host` (Docker Desktop/OrbStack) unless
#    BW_LAB_CGROUP says otherwise, and every lab node inherits both knobs.
compose = yaml.safe_load(open(COMPOSE))
node = compose["x-black-node"]
if node.get("cgroup") != "${BW_LAB_CGROUP:-host}":
    fails.append(f"compose anchor: cgroup must default to host, got {node.get('cgroup')}")
if int(node.get("pids_limit", 0)) < 8192:
    fails.append(f"compose anchor: pids_limit {node.get('pids_limit')} leaves TasksMax under 1229")
for name, svc in compose["services"].items():
    # A privileged service is a systemd node, i.e. one that merged x-black-node;
    # aptcache/metrics/grafana are plain containers and must not inherit them.
    if not svc.get("privileged"):
        continue
    for knob in ("cgroup", "pids_limit"):
        if knob not in svc:
            fails.append(f"{name}: lost the {knob} knob of x-black-node")

# bw-lab/bw-check and compose must agree on the default, or bw-check would look for
# the lab's systemd nodes in a namespace they were not started in.
common = open("tests/bin/_bw-common.sh").read()
exported = re.search(r'BW_LAB_CGROUP="\$\{BW_LAB_CGROUP:-([^}]*)\}"', common)
if not exported or exported.group(1) != node["cgroup"].split(":-")[1].rstrip("}"):
    fails.append("tests/bin/_bw-common.sh and compose disagree on the BW_LAB_CGROUP default")

# 4. Memory locking is decided per host, not hardcoded; the template must follow.
indexer_defaults = yaml.safe_load(open(f"{INDEXER_ROLE}/defaults/main.yml"))
if indexer_defaults.get("wazuh_indexer_memory_lock") is not None:
    fails.append("wazuh_indexer_memory_lock must default to null (decide at runtime)")
for value, expected in ((True, "bootstrap.memory_lock: true"), (False, "bootstrap.memory_lock: false")):
    line = render(INDEXER_ROLE, "opensearch.yml.j2", _indexer_memory_lock=value)
    if expected not in line:
        fails.append(f"opensearch.yml.j2 with _indexer_memory_lock={value}: {expected!r} missing")

if fails:
    raise SystemExit("rootless-lab check failed:\n  " + "\n  ".join(fails))
print(f"ROOTLESS LAB OK: realtime on by default / off in the lab ({LAB_VARS}); "
      f"scan {lab['wazuh_syscheck_frequency']}s; cgroup {node['cgroup']}; "
      f"pids_limit {node['pids_limit']}; memory_lock decided by the role")
