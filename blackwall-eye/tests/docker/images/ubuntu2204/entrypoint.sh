#!/bin/bash
# OrbStack (and some Docker Desktop versions) mount btrfs at /sys/fs/cgroup
# instead of the real cgroup2 filesystem. Shadow it before systemd starts.
mount -t cgroup2 none /sys/fs/cgroup 2>/dev/null || true
exec /lib/systemd/systemd "$@"
