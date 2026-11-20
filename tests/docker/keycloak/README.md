# Keycloak — the lab's OIDC identity provider

`black-idp-00` (compose profile `sso`) is a Keycloak 26.8.2 server for testing
blackwall's OIDC login (`bw-lab up --sso`, `bw-check sso`, blackwall's
`auth: {type: oidc}`). Plain HTTP, no TLS, localhost only — every credential in
this directory is a lab credential, never a real one.

Started only by `bw-lab up --sso`: a plain `bw-lab up` never pulls it in
(`bw-lab down` removes it either way).

## Ports

| Where | URL |
|---|---|
| host (blackwall, bw-check) | `http://127.0.0.1:18080` (`BW_LAB_IDP_PORT`) |
| lab network (indexer, ansible) | `http://black-idp-00:8080` |

18080 instead of 8080: 8080, 9090 and 3000 are already taken on the shared lab
host.

## One issuer, two ways in

Keycloak resolves the URLs it publishes (issuer, authorization, token, JWKS)
either dynamically from the request — `Host` header, scheme, port — or from
server-wide configuration like `hostname` (`keycloak/docs/guides/server/hostname.adoc`,
"Sources for resolving the URL"). The dynamic default is a trap here: a host
client and the indexer would see *different* `issuer` values
(`127.0.0.1:18080` vs `black-idp-00:8080`), and an ID token whose `iss` is one
of them can never validate against the other. OpenSearch's `openid` domain
validates tokens against the JWKS it reads out of the discovery document pointed
to by `openid_connect_url`
(`opensearch-docs/_security/authentication-backends/openid-connect.md`), so the
document and the token have to agree on the issuer while the JWKS still has to
be reachable from inside the lab network.

The two settings that produce that, as documented in `hostname.adoc`:

```
--hostname http://127.0.0.1:18080        # fixed frontchannel: the issuer and the login URLs
--hostname-backchannel-dynamic true      # backend URLs (token, JWKS) resolved per request
```

`--hostname` pins the issuer to the loopback URL blackwall uses;
`--hostname-backchannel-dynamic` makes the *backend* endpoints ("the token and
token introspection endpoint, userinfo endpoint, JWKS URI endpoint", hostname.adoc)
follow the incoming request headers, so the indexer fetches `jwks_uri` over the
container name. Two rules from the doc's Validations section are respected here:
with backchannel dynamic, `--hostname` must be a full URL including scheme and
port (hostname.adoc, Validations), and `start-dev` is the dev profile, where
`--hostname-strict false` is the default, so the same container answers both
`127.0.0.1:18080` and `black-idp-00:8080`.

`bw-check sso` reads the discovery document twice — from the host and from
inside `black-master-00` — and asserts the two halves of this claim: equal
`issuer`, and a `jwks_uri` that follows the caller. That is how the setup was
checked; it is the acceptance criterion for this design.

## Realm `blackwall-lab` (blackwall-lab-realm.json)

Imported at start-up: `start-dev --import-realm` reads every `*.json` file in
`/opt/keycloak/data/import` (`keycloak/docs/guides/server/containers.adoc`,
`keycloak/docs/guides/server/importExport.adoc`). The file name is part of the
contract: "The realm file to be imported must be named
`<realm name>-realm.json`" (importExport.adoc:99) — Keycloak 26.8.2 refuses to
start with `File name / realm name mismatch` otherwise, which is why this file is
`blackwall-lab-realm.json` and not something shorter. An import **skips a realm
that already exists** (importExport.adoc: "If a realm already exists in the
server, the import operation is skipped"), so the realm file only takes effect on
a fresh IdP container — which is what `bw-lab up` / `bw-lab down` produce here:
the dev mode database lives in the container layer, and `bw-lab down -v` removes
the container. To re-import into a running IdP, recreate it
(`docker compose … up -d --force-recreate black-idp-00`) or run the `import`
command explicitly (importExport.adoc).

Realm fields, users and clients are the shapes in
`keycloak/core/src/main/java/org/keycloak/representations/idm/`
(`RealmRepresentation`, `ClientRepresentation`, `UserRepresentation`,
`CredentialRepresentation`):

- Users: `alice` in group `analysts`, `bob` in group `read-only`; password
  `blackwall-lab` (lab-only, satisfying `passwordPolicy: length(12)`). Passwords
  are given as `credentials: [{type: password, value: …}]` — `CredentialRepresentation`
  has `type`/`value`, no `secret`.
- `realmRoles: ["offline_access"]` on both users. Keycloak issues an offline
  (refresh) token only to a user that has the role mapping for the realm-level
  `offline_access` role (`keycloak/docs/documentation/server_admin/topics/sessions/offline.adoc`).
  blackwall requests `openid offline_access` by default (`internal/config`
  `DefaultAuthScopes`), so without that mapping the login that blackwall performs
  cannot get a refresh token — `bw-check sso` asks for the offline token and
  fails loudly if it is missing.
- Groups: `analysts` (mapped to roles), `read-only` (deliberately unmapped, so
  the lab can show a denial).
- Group claim: `oidc-group-membership-mapper` on both clients publishes the
  `groups` claim in the ID token, the access token and userinfo — that claim is
  what the indexer's `roles_key` and the Wazuh RBAC rule match on.
- Token lifetimes: access token 900s, `refreshTokenMaxReuse: 0` (a refresh token
  is used once, offline.adoc's "Revoke Refresh Token" behaviour), offline session
  max 3600s — so blackwall's refresh path is testable rather than theoretical.
- Clients:
  - `blackwall-cli` — public (`publicClient: true`, no secret): authorization-code
    flow enabled (`standardFlowEnabled`), device authorization grant enabled
    (client attribute `oauth2.device.authorization.grant.enabled: "true"`, from
    the task's spec note — the admin-console switch is "Auth 2.0 Device
    Authorization Grant", `server_admin/topics/clients/oidc/con-basic-settings.adoc`),
    redirect URIs `http://127.0.0.1:*` — wildcards at the end of a redirect
    pattern are accepted (`con-basic-settings.adoc`), which is what blackwall's
    PKCE callback on a random loopback port needs. PKCE itself needs no client
    attribute: Keycloak advertises the methods it accepts in the discovery
    document, and `bw-check sso` asserts `code_challenge_methods_supported`
    contains `S256`. This is the client blackwall's config names.
  - `blackwall-test` — confidential (`secret: blackwall-lab-test-secret`,
    lab-only) with `directAccessGrantsEnabled: true`, used only by
    `bw-check sso` to mint tokens without a browser. Not for blackwall itself.

## Administration and health

`KC_BOOTSTRAP_ADMIN_USERNAME` / `KC_BOOTSTRAP_ADMIN_PASSWORD` (`labadmin` /
`labadmin`, lab-only) create the initial admin user
(`keycloak/docs/guides/server/containers.adoc`,
`keycloak/docs/guides/server/configuration.adoc`); the admin console is
`http://127.0.0.1:18080/admin` — nothing in the lab uses it, the realm file is
the source of truth.

There is no compose healthcheck on purpose: Keycloak's health endpoints live on
the management port 9000, not on the application port
(`keycloak/docs/guides/server/haproxy-passthrough.adoc`: "Directs health checks
to the management port (9000)"). `bw-check sso` and the ansible tasks wait on the
discovery document instead, which also proves the realm import finished.

## Reset

The dev-mode database lives in the container layer, so a recreate is a full reset:

```
docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 --profile sso \
  up -d --force-recreate black-idp-00
```

After changing the realm file, re-run `BW_SSO=1 tests/bin/bw-lab provision eye`:
the indexer caches signing keys from `jwks_uri`, and its
`openid_auth_domain`/role mapping must be re-applied against the new realm.
