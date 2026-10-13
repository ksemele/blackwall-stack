# Task 23 — Smoke E2E script
**Status:** DONE
**Date:** 2026-05-17
**Commits:** 02614a4

## What was done
- `tools/blackwall/scripts/smoke.sh`: ensures docker compose up, polls Wazuh API, writes tmp config with insecure_skip_verify, builds blackwall, runs `--once`, greps for "active". chmod +x.

## Verification
Not executed against live docker env in this session — requires `tests/docker` up. Script is syntactically valid bash; logic mirrors plan.

## Deviations
none

## Concerns / open questions
- Smoke needs a live docker test env to validate end-to-end; can run via `tools/blackwall/scripts/smoke.sh` when ready.
