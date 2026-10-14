FROM geerlingguy/docker-rockylinux9-ansible:latest

# Rocky Linux 9 with systemd as PID 1 + sshd, mirroring images/ubuntu2204.
# Root password 'blackwall' and PermitRootLogin yes: LOCAL TEST ENVIRONMENT ONLY.
RUN dnf -y install openssh-server iproute \
  && dnf clean all \
  && ssh-keygen -A \
  && echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config \
  && echo 'PasswordAuthentication yes' >> /etc/ssh/sshd_config \
  && echo 'root:blackwall' | chpasswd \
  && systemctl enable sshd

EXPOSE 22

COPY entrypoint.sh /entrypoint.sh
STOPSIGNAL SIGRTMIN+3
ENTRYPOINT ["/entrypoint.sh"]
