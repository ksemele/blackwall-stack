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

EXPOSE 22

# entrypoint mounts real cgroup2 before systemd starts
# (OrbStack/Docker Desktop mount btrfs/ext4 at /sys/fs/cgroup instead of cgroup2)
COPY entrypoint.sh /entrypoint.sh

STOPSIGNAL SIGRTMIN+3
ENTRYPOINT ["/entrypoint.sh"]
