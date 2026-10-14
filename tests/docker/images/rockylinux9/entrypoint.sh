#!/bin/bash
# See images/ubuntu2204/entrypoint.sh: shadow a non-cgroup2 /sys/fs/cgroup.
mount -t cgroup2 none /sys/fs/cgroup 2>/dev/null || true
exec /usr/lib/systemd/systemd "$@"
