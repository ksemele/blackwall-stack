# Lab: publish the Wazuh indexer

- Status: done
- Module: tests (lab)
- Date: 2026-10-14
- Commits: 4084292

## What was done
- docker-compose: black-master-00 publishes 127.0.0.1:9200:9200 (loopback only); ports comment extended.
- _bw-common.sh: BW_INDEXER_URL / BW_INDEXER_AUTH.
- bw-check: port 9200 added to the published-port probe; "indexer answers" check added to the wazuh group.
- Docs: tests/README.md (access table, topology, bw-check table) and root README.md service table.

## Verification
`tests/bin/bw-lab reset` then `tests/bin/bw-check all`: 29 passed, 0 failed, 1 skipped (voice not deployed).

    PASS published port 9200 (wazuh-indexer) reachable from the host
    PASS indexer answers at https://127.0.0.1:9200

`curl -sk -u 'admin:Blackwall1!' 'https://127.0.0.1:9200/wazuh-alerts-*/_count'` -> `{"count":450,...}`

## Deviations
None.

## Open questions
None.
