FROM ghcr.io/geerlingguy/docker-ubuntu2204-ansible:latest

# Add openssh-server on top of the geerlingguy base image, which already ships:
#   - Ubuntu 22.04 with systemd as PID 1
#   - Python 3
#   - sudo, curl, and common Ansible test dependencies
#
# Security note: root password 'blackwall' and PermitRootLogin yes are intentional
# for this LOCAL TEST ENVIRONMENT ONLY. Never use this image in production or expose
# these containers outside localhost.

RUN apt-get update && apt-get install -y openssh-server \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* \
  && mkdir -p /var/run/sshd \
  && echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config \
  && echo 'PasswordAuthentication yes' >> /etc/ssh/sshd_config \
  && echo 'root:blackwall' | chpasswd \
  && systemctl enable ssh

EXPOSE 22

# SIGRTMIN+3 = graceful systemd shutdown
STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]
