FROM geerlingguy/docker-ubuntu2204-ansible:latest

# Add openssh-server on top of the geerlingguy base image, which already ships:
#   - Ubuntu 22.04 with systemd as PID 1
#   - Python 3
#   - sudo, curl, and common Ansible test dependencies
#
# Security note: root password 'blackwall' and PermitRootLogin yes are intentional
# for this LOCAL TEST ENVIRONMENT ONLY. Never use this image in production or expose
# these containers outside localhost.

RUN sed -i 's|http://ports.ubuntu.com/ubuntu-ports|http://ftp.uni-erlangen.de/ubuntu-ports|g' /etc/apt/sources.list \
  && apt-get update && apt-get install -y openssh-server \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* \
  && mkdir -p /var/run/sshd \
  && echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config \
  && echo 'PasswordAuthentication yes' >> /etc/ssh/sshd_config \
  && echo 'root:blackwall' | chpasswd \
  && systemctl enable ssh

# Lab-only: every apt request goes through black-aptcache-00 (apt-cacher-ng),
# so Wazuh packages are downloaded once, not on every `bw-lab up`. Added after
# the build-time apt-get above: the image itself builds without the proxy.
#
# Pipeline-Depth 0 and Retries are not decoration. apt pipelines 5 requests on
# one connection by default (Acquire::http::Pipeline-Depth "5" in
# /usr/share/doc/apt/examples/configure-index), and through this proxy a pipeline
# can be abandoned mid-flight. On a cold cache every node re-downloads ~15 MB of
# index files in its first apt-get update, and that is enough to stop
# provisioning: three /usr/lib/apt/methods/http processes sat in CLOSE_WAIT
# against 172.28.0.14:3142 for 24 minutes with nothing arriving, the index stayed
# in /var/lib/apt/lists/partial, and the ansible apt_repository task never
# returned. One request per connection, retried, finishes.
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
