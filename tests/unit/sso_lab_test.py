"""Unit check: the SSO lab's static wiring is consistent (no Docker, no IdP).

`bw-lab up --sso` stitches four places together: the Keycloak realm import, the
compose service, the ansible variables the eye roles read, and the blackwall
context in tests/blackwall/. A name that drifts between them — realm, client,
group, claim, service user — breaks the OIDC chain only at run time, deep inside
a token verification or a rule match. This check compares them offline.

Run:
    python tests/unit/sso_lab_test.py
"""
import json
import re
import yaml

COMPOSE = "tests/docker/docker-compose.yml"
REALM = "tests/docker/keycloak/blackwall-lab-realm.json"
COMMON = "tests/bin/_bw-common.sh"
BW_CHECK = "tests/bin/bw-check"
ALL_VARS = "tests/inventory/group_vars/all.yml"
MANAGER_VARS = "tests/inventory/group_vars/wazuh_managers.yml"
INDEXER_DEFAULTS = "blackwall-eye/ansible/roles/wazuh-indexer/defaults/main.yml"
LAB_CONFIG = "tests/blackwall/lab-sso.yaml"
# The IdP's address in the lab network: .30 is outside the node ranges the other
# lab profiles use (.9-.17, .20/.21, .90, .98/.99).
IDP_IP = "172.28.0.30"


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def shell_var(text, name):
    """Value of a top-level assignment in a shell file. The NAME="${NAME:-d}"
    form resolves to its default d, which is what a plain run uses."""
    default = re.search(rf'^{name}="\$\{{{name}:-([^"]*)\}}"', text, re.M)
    if default:
        return default.group(1)
    plain = re.search(rf'^{name}="([^"]*)"', text, re.M)
    assert plain, f'{name} is not assigned in {COMMON}'
    return plain.group(1)


compose = yaml.safe_load(read(COMPOSE))
realm = json.loads(read(REALM))
common = read(COMMON)
bw_check = read(BW_CHECK)
compose_text = read(COMPOSE)
all_vars = yaml.safe_load(read(ALL_VARS))
manager_vars = yaml.safe_load(read(MANAGER_VARS))
indexer_defaults = yaml.safe_load(read(INDEXER_DEFAULTS))
lab_config = yaml.safe_load(read(LAB_CONFIG))

analyst_group = shell_var(common, "BW_IDP_ANALYST_GROUP")
outsider_group = shell_var(common, "BW_IDP_OUTSIDER_GROUP")

# --- the realm the IdP imports -------------------------------------------------
clients = {c["clientId"]: c for c in realm["clients"]}
public = clients[shell_var(common, "BW_IDP_CLIENT")]
confidential = clients[shell_var(common, "BW_IDP_TEST_CLIENT")]

assert realm["realm"] == shell_var(common, "BW_IDP_REALM"), \
    f"realm {realm['realm']!r} != BW_IDP_REALM {shell_var(common, 'BW_IDP_REALM')!r}"
# `--import-realm` matches the file name against the realm inside it: Keycloak
# 26.8.2 exits with "File name / realm name mismatch" and the IdP never starts.
# keycloak/docs/guides/server/importExport.adoc:99.
assert REALM.endswith(f"{realm['realm']}-realm.json"), \
    f"{REALM} must be named {realm['realm']}-realm.json for the start-up import"
assert realm["sslRequired"] == "none", "the lab IdP answers over plain HTTP: sslRequired must stay none"

assert public.get("publicClient") is True, \
    f"{public['clientId']}: blackwall is a public client — no secret in its config"
assert public["standardFlowEnabled"] is True, "auth code + PKCE needs the standard flow enabled"
assert not public.get("directAccessGrantsEnabled"), \
    "the client blackwall uses must not accept raw passwords; bw-check has blackwall-test for that"
assert public["attributes"]["oauth2.device.authorization.grant.enabled"] == "true", \
    "device code login is the flow blackwall prefers when the IdP publishes it"
assert "http://127.0.0.1:*" in public["redirectUris"], \
    "blackwall's PKCE callback listens on a random loopback port"
assert public["protocol"] == "openid-connect", "blackwall's go-oidc speaks openid-connect"

groups = {g["name"] for g in realm["groups"]}
assert analyst_group in groups, f"group {analyst_group!r} missing from {REALM}"
assert outsider_group in groups and outsider_group != analyst_group, \
    "the lab needs an unmapped group to show a denial"

users = {u["username"]: u for u in realm["users"]}
policy_len = re.search(r"length\((\d+)\)", realm.get("passwordPolicy", ""))
lab_users = ((shell_var(common, "BW_IDP_ANALYST_USER"),
              shell_var(common, "BW_IDP_ANALYST_PASSWORD"),
              analyst_group),
             (shell_var(common, "BW_IDP_OUTSIDER_USER"),
              shell_var(common, "BW_IDP_OUTSIDER_PASSWORD"),
              outsider_group))
for name, password, group in lab_users:
    assert name in users, f"lab user {name!r} missing from {REALM}"
    assert group in users[name]["groups"], f"{name} must be a member of {group}"
    assert users[name].get("enabled", True), f"{name} must be enabled for a password login"
    # blackwall's default scopes are openid + offline_access; Keycloak issues an
    # offline token only to a user holding the realm-level offline_access role.
    assert "offline_access" in users[name].get("realmRoles", []), \
        f"{name} needs the realm-level offline_access role or blackwall gets no refresh token"
    assert users[name]["credentials"][0]["value"] == password, \
        f"{name}'s password in the realm and the one bw-check logs in with must be the same"
    if policy_len:
        assert len(password) >= int(policy_len.group(1)), \
            f"{name}'s password violates the realm policy {realm['passwordPolicy']!r}"

mappers = [m for m in public.get("protocolMappers", [])
           if m.get("protocolMapper") == "oidc-group-membership-mapper"]
assert mappers, f"{public['clientId']}: no group mapper — roles_key and the RBAC rule have nothing to match"
config = mappers[0]["config"]
claim = config["claim.name"]
assert claim == "groups", "bw-check, the RBAC rule and the lab context all name the claim 'groups'"
assert config["full.path"] == "false", \
    "groups must be plain names (analysts), not paths (/analysts)"
assert config["id.token.claim"] == "true" and config["access.token.claim"] == "true", \
    "the indexer and run_as read the claim in the ID token"

assert not confidential.get("publicClient") and confidential.get("secret"), \
    f"{confidential['clientId']}: bw-check mints tokens with a confidential client"
assert confidential["directAccessGrantsEnabled"] is True, "bw-check has no browser to sign in with"
assert confidential["clientId"] != public["clientId"], \
    "the check-only client must stay separate from the client blackwall itself uses"

# --- the compose service -------------------------------------------------------
idp = compose["services"]["black-idp-00"]
assert idp.get("profiles") == ["sso"], "profile sso keeps the IdP out of a plain `bw-lab up`"
ports = [str(p) for p in idp["ports"]]
assert all(p.startswith("127.0.0.1:") for p in ports), f"the IdP stays on loopback: {ports}"
assert any(p.endswith(":8080") for p in ports), f"the container listens on 8080: {ports}"
assert "${BW_LAB_IDP_PORT:-18080}" in " ".join(ports), \
    f"host port must come from BW_LAB_IDP_PORT with default 18080: {ports}"
assert shell_var(common, "BW_LAB_IDP_PORT") == "18080", \
    "BW_LAB_IDP_PORT has to match the compose default or bw-check probes the wrong port"

ips = re.findall(r"ipv4_address:\s*(\S+)", compose_text)
assert len(ips) == len(set(ips)), f"duplicate static IPs in {COMPOSE}: {sorted(ips)}"
idp_ip = idp["networks"]["blackwall-test"]["ipv4_address"]
assert idp_ip == IDP_IP, f"IdP IP {idp_ip}: {IDP_IP} is the address free of the other lab profiles"
subnets = [n["ipam"]["config"][0]["subnet"] for n in compose["networks"].values() if "ipam" in n]
assert any(idp_ip.startswith(prefix) for prefix in (s.rsplit(".", 1)[0] + "." for s in subnets)), \
    f"{idp_ip} is outside the lab subnets {subnets}"

command = " ".join(idp["command"]) if isinstance(idp["command"], list) else str(idp["command"])
assert "start-dev" in command and "--import-realm" in command, \
    "dev mode plus the realm import (keycloak/docs/guides/server/containers.adoc)"
assert "--hostname=http://127.0.0.1:${BW_LAB_IDP_PORT:-18080}" in command, \
    "the issuer is fixed to the host loopback URL, so one token validates on both sides"
assert "--hostname-backchannel-dynamic=true" in command, \
    "the indexer has to read jwks_uri through the container name"
mounts = " ".join(str(v) for v in idp["volumes"])
assert "/opt/keycloak/data/import/" in mounts, "the realm file goes into Keycloak's import directory"
assert mounts.endswith(":ro") or ":ro " in mounts or mounts.endswith(".json:ro"), \
    "the realm mount is read-only"

# --- the ansible side ----------------------------------------------------------
assert all_vars["bw_sso_enabled"] == "{{ lookup('env', 'BW_SSO') == '1' }}", \
    "the OIDC wiring stays opt-in, driven by the same BW_SSO the scripts export"
assert all_vars["bw_idp_realm"] == realm["realm"], "ansible and the realm import must name one realm"

for role in ("wazuh-indexer", "wazuh-manager"):
    tasks = yaml.safe_load(read(f"blackwall-eye/ansible/roles/{role}/tasks/main.yml"))
    sso_tasks = [t for t in tasks if str(t.get("include_tasks", "")).endswith("sso.yml")]
    assert sso_tasks, f"{role}: tasks/main.yml must include sso.yml"
    assert "bw_sso_enabled" in str(sso_tasks[0].get("when", "")), \
        f"{role}: sso.yml must stay behind bw_sso_enabled — no security changes in a plain lab"

assert manager_vars["wazuh_indexer_oidc_group"] == analyst_group, \
    "the indexer maps this backend role from the group claim; it must be a real IdP group"
discovery = manager_vars["wazuh_indexer_oidc_discovery_url"]
assert discovery.startswith("http://black-idp-00:8080/realms/"), \
    "the indexer reads the discovery document over the lab network, not the host loopback"
assert "{{ bw_idp_realm }}" in discovery and all_vars["bw_idp_realm"] == realm["realm"], \
    "the discovery URL must resolve to the realm the IdP imports (via bw_idp_realm)"
assert indexer_defaults["wazuh_indexer_oidc_roles_key"] == claim, \
    "roles_key must name the claim the realm mapper publishes"
assert indexer_defaults["wazuh_indexer_oidc_subject_key"] == "preferred_username", \
    "subject_key: Keycloak's built-in username claim"
assert manager_vars["wazuh_api_run_as_group"] == analyst_group, \
    "the RBAC rule and the IdP group must carry the same name"

template = read("blackwall-eye/ansible/roles/wazuh-indexer/templates/opensearch.yml.j2")
flag = "plugins.security.unsupported.restapi.allow_securityconfig_modification"
assert flag in template, "the security-config REST API needs this setting (see the role's sso.yml header)"
assert "{% if bw_sso_enabled" in template[:template.index(flag)], \
    f"{flag} must be rendered only in an SSO lab"

# --- blackwall's own context ---------------------------------------------------
context = lab_config["contexts"][lab_config["current"]]
issuer = f"http://127.0.0.1:{shell_var(common, 'BW_LAB_IDP_PORT')}/realms/{realm['realm']}"
for block in ("wazuh", "indexer"):
    auth = context[block]["auth"]
    assert auth["type"] == "oidc", f"{block}: the lab context has to use OIDC"
    assert auth["issuer"] == issuer, \
        f"{block}: issuer {auth['issuer']!r} is not what the IdP publishes ({issuer!r})"
    assert auth["client_id"] == public["clientId"], \
        f"{block}: must name the public client blackwall is registered as"
assert "user" not in context["indexer"] and "password" not in context["indexer"], \
    "the indexer is read with the ID token only — no Basic identity in that block"
assert context["wazuh"]["user"] == shell_var(common, "BW_API_RUN_AS_USER"), \
    "blackwall's Basic identity and the provisioned service user must be the same"
assert context["wazuh"]["password"] == shell_var(common, "BW_API_RUN_AS_PASSWORD"), \
    "the lab context and the ansible task must carry the same service-user password"

run_as = {m["name"]: m["claim"] for m in context["wazuh"]["auth"]["run_as"]}
assert run_as.get("groups") == claim, \
    "the RBAC rule matches the groups claim; run_as has to produce the key 'groups'"
assert run_as.get("user_name") == indexer_defaults["wazuh_indexer_oidc_subject_key"], \
    "run_as user_name and the indexer's subject_key should name the same identity"

# --- bw-check group registration -----------------------------------------------
assert re.search(r"^\s*sso\)\s+check_sso\s*;;", bw_check, re.M), "bw-check must dispatch the sso group"
all_branch = re.search(r"\n\s*all\)\n(.*?)\n\s*;;", bw_check, re.S)
assert all_branch, "bw-check keeps a default `all` group"
assert "check_sso" not in all_branch.group(1), \
    "sso stays out of `bw-check all`: the IdP exists only in an --sso lab"

print("SSO LAB WIRING OK")
