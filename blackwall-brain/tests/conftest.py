import json

import pytest

VALID_VERDICT = {
    "verdict": "true_positive",
    "severity": "high",
    "category": "privilege_escalation",
    "summary": "New sudoers drop-in on black-host-01",
    "action": "Check who wrote /etc/sudoers.d/90-app and revert it if unknown",
    "confidence": "medium",
    "needs": [],
}


@pytest.fixture
def alert():
    return {
        "id": "1730124123.123456",
        "timestamp": "2026-10-28T14:02:03.123+0000",
        "rule": {"id": "100120", "level": 10, "description": "Sudoers file changed",
                 "groups": ["sudo", "audit"],
                 "mitre": {"tactic": ["Privilege Escalation"], "technique": ["Sudo and Sudo Caching"]}},
        "agent": {"id": "001", "name": "black-host-01", "ip": "172.28.0.11"},
        "manager": {"name": "black-master-00"},
        "full_log": "type=SYSCALL msg=audit(...): comm=\"tee\" name=\"/etc/sudoers.d/90-app\"",
    }


@pytest.fixture
def verdict_json():
    return json.dumps(VALID_VERDICT)
