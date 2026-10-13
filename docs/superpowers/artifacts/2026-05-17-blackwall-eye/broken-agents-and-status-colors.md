# Broken agent fixtures + per-status row colors
**Status:** DONE
**Module:** blackwall-eye
**Date:** 2026-05-17
**Commits:** d76b421, eb46293, c1232d3, edb4c27, b32e57f

## What was done

- Added `RowPending` (yellow, color 11) and `RowGhost` (dim grey, color 8) styles to `tools/blackwall/internal/ui/styles/styles.go` alongside existing `RowDown`
- Replaced `status != "active"` blanket-red in `tools/blackwall/internal/views/agents/list.go` with a `switch` on status: `active` → default, `pending` → RowPending, `disconnected` → RowDown, `never_connected` → RowGhost, default fallthrough → RowDown
- Added `black-dead-98` container to `tests/docker/docker-compose.yml` under `ubuntu2204` profile (port 2202, IP 172.28.0.98, same shape as `black-host-01`)
- Added `black-dead-98` to `tests/inventory/hosts.yml` under `wazuh_agents` (localhost:2202)
- Created `tests/triggers/broken-agents.yml` — two-play idempotent playbook:
  - Play 1 (targets `wazuh_managers`): registers `black-ghost-99` (IP 172.28.0.99) via `manage_agents -a -i -q`, skips if already registered
  - Play 2 (targets `localhost`): runs `docker stop black-dead-98` if container is in running state, skips otherwise
- Updated `tests/README.md`: topology table, new section 6c describing the fixtures and TUI color mapping, file layout tree updated to show `triggers/` contents

## Verification

- `ansible-playbook --syntax-check -i tests/inventory/hosts.yml tests/triggers/broken-agents.yml` → exit 0, no warnings
- `cd tools/blackwall && go build ./... && go test ./...` → all 11 packages pass (0 failures)
- Live env test: **not run** — Docker env was not started during this session. The playbook requires the env to be up and `monitored-host.yml` run against `black-dead-98` before `broken-agents.yml` is executed.

## Deviations

- `black-ghost-99` registration uses `manage_agents -a <name> -i <ip> -q` (quiet flag). Some Wazuh versions use different `manage_agents` syntax; if the manager version differs, the `-q` flag may need to be replaced with a here-document approach (`printf "A\n<name>\n<ip>\ny\n" | manage_agents`). Documented as an open question.
- `tests/triggers/broken-agents.yml` placed in `tests/triggers/` (same dir as `bw-trigger`) rather than `blackwall-eye/ansible/playbooks/` — matches the convention that test-env tooling lives in `tests/triggers/`.

## Open questions

- `manage_agents -a <name> -i <ip> -q` quiet-mode syntax: needs verification against the exact Wazuh 4.14.x build installed in `black-master-00`. If `-q` is unsupported, replace with `printf "A\n<name>\n<ip>\ny\n" | manage_agents`.
- Should `black-dead-98` be provisioned as part of `all.yml` automatically, or always kept as an explicit `--limit` step? Currently it must be provisioned separately.
