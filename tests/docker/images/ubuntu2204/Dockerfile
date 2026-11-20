FROM geerlingguy/docker-ubuntu2204-ansible:latest

# Add openssh-server on top of the geerlingguy base image, which already ships:
#   - Ubuntu 22.04 with systemd as PID 1
#   - Python 3
#   - sudo, curl, and common Ansible test dependencies
#
# Security note: root password 'blackwall' and PermitRootLogin yes are intentional
# for this LOCAL TEST ENVIRONMENT ONLY. Never use this image in production or expose
# these containers outside localhost.

# The apt index files are deliberately NOT deleted here. Containers are recreated
# on every `bw-lab up`, and a node that starts with an empty /var/lib/apt/lists
# re-downloads ~40 MB of Ubuntu and Ubuntu-security index files through the lab
# proxy before it can install anything. Keeping the build-time index turns that
# first refresh into a revalidation (InRelease + a few 304s), which is what a
# provisioning run needs: on 2026-11-20 a cold-cache node (black-host-01) left
# 38649899 bytes of half-written index in /var/lib/apt/lists/partial, three
# /usr/lib/apt/methods/http processes sitting in CLOSE_WAIT on 172.28.0.14:3142,
# and its ansible task hanging for 29 minutes.
RUN sed -i 's|http://ports.ubuntu.com/ubuntu-ports|http://ftp.uni-erlangen.de/ubuntu-ports|g' /etc/apt/sources.list \
  && apt-get update && apt-get install -y openssh-server \
  && apt-get clean \
  && mkdir -p /var/run/sshd \
  && echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config \
  && echo 'PasswordAuthentication yes' >> /etc/ssh/sshd_config \
  && echo 'root:blackwall' | chpasswd \
  && systemctl enable ssh

# Lab-only: every apt request goes through black-aptcache-00 (apt-cacher-ng),
# so Wazuh packages are downloaded once, not on every `bw-lab up`. Added after
# the build-time apt-get above: the image itself builds without the proxy.
#
# Pipeline-Depth 0 and Retries are load-shedding on the proxy, not the cure for
# the stall described above. apt pipelines 5 requests on one connection by default
# (Acquire::http::Pipeline-Depth "5" in /usr/share/doc/apt/examples/configure-index);
# with 0 each request gets its own connection, and a failed fetch is retried rather
# than ending the update. Measured on 2026-11-20 with this config in place:
# black-master-00 and black-dead-98 completed the same index set, black-host-01 did
# not — which is why the lab avoids the index volume instead of tuning the client.
RUN printf '%s\n' 'Acquire::http::Proxy "http://172.28.0.14:3142";' \
                  'Acquire::http::Pipeline-Depth "0";' \
                  'Acquire::Retries "5";' \
        > /etc/apt/apt.conf.d/01blackwall-aptcache

EXPOSE 22

# entrypoint mounts real cgroup2 before systemd starts
# (OrbStack/Docker Desktop mount btrfs/ext4 at /sys/fs/cgroup instead of cgroup2)
COPY entrypoint.sh /entrypoint.sh

STOPSIGNAL SIGRTMIN+3
ENTRYPOINT ["/entrypoint.sh"]
