FROM geerlingguy/docker-debian12-ansible:latest

# Debian 12 with systemd as PID 1 + sshd, mirroring images/ubuntu2204 and
# images/rockylinux9. The base ships systemd, Python 3 and sudo — but no rsyslog,
# exactly like a Debian 12 install. That is on purpose: this node exercises the
# journald branch of the Wazuh log sources (roles/wazuh-{agent,manager}/templates/
# ossec.conf.j2 read journald on Debian 12+).
#
# Security note: root password 'blackwall' and PermitRootLogin yes are intentional
# for this LOCAL TEST ENVIRONMENT ONLY.

RUN apt-get update && apt-get install -y --no-install-recommends openssh-server \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* \
  && mkdir -p /var/run/sshd \
  && echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config \
  && echo 'PasswordAuthentication yes' >> /etc/ssh/sshd_config \
  && echo 'root:blackwall' | chpasswd \
  && systemctl enable ssh

# Lab-only: every apt request goes through black-aptcache-00 (see
# images/ubuntu2204/Dockerfile). Added after the build-time apt-get above, so
# the image itself builds without the proxy.
RUN echo 'Acquire::http::Proxy "http://172.28.0.14:3142";' \
      > /etc/apt/apt.conf.d/01blackwall-aptcache

EXPOSE 22

# entrypoint mounts real cgroup2 before systemd starts
# (OrbStack/Docker Desktop mount btrfs/ext4 at /sys/fs/cgroup instead of cgroup2)
COPY entrypoint.sh /entrypoint.sh

STOPSIGNAL SIGRTMIN+3
ENTRYPOINT ["/entrypoint.sh"]
