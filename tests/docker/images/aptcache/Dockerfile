FROM debian:12-slim

# apt-cacher-ng: caching apt proxy for the test environment only.
# Every bw-lab up recreates the nodes and re-downloads ~1.5 GB of Wazuh
# packages; through this proxy only the first run hits the internet.
#
# HTTPS repositories cannot be cached through a CONNECT tunnel: blackwall.conf
# remaps http://packages.wazuh.com to its https:// backend, and the lab
# inventory points apt at the http:// URL (tests/inventory/group_vars/all.yml).

RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
       apt-cacher-ng ca-certificates \
  && rm -rf /var/lib/apt/lists/*

COPY blackwall.conf /etc/apt-cacher-ng/zz_blackwall.conf

EXPOSE 3142

# Runs as root in the foreground: the cache dir is a bind mount from the repo
# (tests/.apt-cache), so it survives `bw-lab down`, which removes volumes.
CMD ["apt-cacher-ng", "-c", "/etc/apt-cacher-ng", "ForeGround=1", \
     "CacheDir=/var/cache/apt-cacher-ng", "LogDir=/var/log/apt-cacher-ng", \
     "PidFile=/run/apt-cacher-ng.pid", "SocketPath=/run/apt-cacher-ng.socket"]
