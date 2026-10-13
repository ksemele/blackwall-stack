# Task 15: Rewrite tests/README.md for 2-container topology

**Status:** Complete  
**Date:** 2026-05-16  
**Artifact:** `tests/README.md`

## Summary

Rewrote `tests/README.md` from the ground up to reflect the new 2-container test topology:
- `black-master-00` — Wazuh manager + indexer + dashboard (all-in-one)
- `black-host-01` — Wazuh agent + auditd target

## Changes

### Removed
- References to 3-container topology (`black-00`, `black-01`, `black-02`)
- `hosts.single.yml` references (single-container test mode)
- Section describing "Full topology (manager + 2 agents)" vs "Single container" distinction
- Outdated port mapping tables (2200–2202)

### Added
- Clear "Quick start" (section 1) with full end-to-end deployment and dashboard access
- Dashboard connection details (URL, credentials, cert handling) as section 2
- Agent endpoint override mechanism (section 3) with documented variables and examples
- Comprehensive per-component verification cookbook (section 6a) covering:
  - Containers
  - SSH/Ansible reachability
  - Indexer health checks (curl + password verification)
  - Manager + filebeat
  - Dashboard
  - Agent registration and logs
  - Agent endpoint override testing (point to different master, verify config, revert)
  - Version/build verification
  - Full clean-room acceptance test
- Service status commands consolidated (section 7)
- Host topology table clarifying 2-container layout with actual port mappings (section 9)
- Certificates section (10) explaining tar generation and rotation
- Multi-distro support clarification (section 11) with Ubuntu 2204 active, Debian/CentOS/Alpine planned
- Controller prerequisites for bcrypt/passlib (section 12)
- Known limitations expanded (section 13) covering auditd Docker constraint, self-signed certs, bcrypt idempotency
- Updated file layout (section 14) reflecting actual structure

## Verification

Stale reference check: **clean**
```bash
grep -E "black-0[012]\b|hosts\.single\.yml" tests/README.md && echo "STALE REFS FOUND" || echo "clean"
# Output: clean
```

No references to old container names (`black-00`, `black-01`, `black-02`) remain.  
No references to `hosts.single.yml` remain.

## Next Steps

None — task complete. The new README is ready for deployment and use.
