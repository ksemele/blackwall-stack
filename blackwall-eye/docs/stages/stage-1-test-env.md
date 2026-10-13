# Stage 1 — Docker Test Environment Design

> **Historical design doc.** Names (`black-00..02`) and paths (`blackwall-eye/tests/`)
> below are from the original design and no longer match the repo. The current test
> environment lives in [`tests/`](../../../tests/README.md) (`black-master-00`,
> `black-host-01`, `black-dead-98`, driven by `tests/bin/bw-lab` / `bw-check`).

**Purpose:** Provide a fast, reproducible local environment for iterating on Ansible roles
before deploying to real hosts. Targets macOS (Docker Desktop) and must support systemd
inside containers (required by Wazuh, auditd).

---

## 1. Approach: Options Compared

### Option A — Docker Compose + privileged systemd containers (Recommended)

Docker Compose defines `N` containers, each running a systemd-capable Ubuntu 22.04 image,
with SSH enabled. Ansible reaches them over a shared Docker network using a static inventory.
One `docker compose up -d` starts everything; `docker compose down -v` tears it all down.

**Pros:**
- Single file (`docker-compose.yml`) describes the entire topology.
- One command up, one command down — exactly what is needed.
- No extra tooling beyond Docker Desktop + Ansible.
- Compose profiles allow selecting 1-host or 3-host topologies from the same file.
- Fast iteration: containers start in seconds.

**Cons:**
- Privileged containers with systemd are less isolated than VMs.
- Some systemd units behave differently from real hosts (e.g., `journald` may need tweaks).
- Docker Desktop on macOS runs containers in a Linux VM (transparent, but adds one layer).

---

### Option B — Molecule with Docker driver

Molecule is an Ansible role testing framework. Its Docker driver can spin up containers per
role and run the full test lifecycle (create → converge → verify → destroy).

**Pros:**
- Structured test lifecycle with `verify` phase (idempotency checks built in).
- Integrates with CI (GitHub Actions).
- Standard in the Ansible ecosystem.

**Cons:**
- Adds Molecule as a dependency (Python package + config per role).
- Molecule's Docker driver has known friction with systemd: requires custom `command` and
  `capabilities` in `molecule.yml`; less intuitive than plain Compose.
- Overkill for the current goal of "can I SSH in and run a playbook."
- Per-role config duplication when the stack has 5+ roles.

**Verdict:** Worth adding later (CI gate), but too heavy for day-to-day iteration right now.

---

### Option C — Vagrant + VirtualBox

Vagrant manages full Ubuntu VMs via VirtualBox. Each `black-XX` host is a real VM.

**Pros:**
- True VM isolation: systemd behaves exactly as on a real server.
- No Docker-specific workarounds.

**Cons:**
- VMs are slow to start (60–120 s per VM vs. 2–5 s for a container).
- VirtualBox does not run on Apple Silicon (arm64) Macs without performance compromises.
  The user's machine runs Darwin 25.4.0 (likely arm64); VirtualBox support is experimental.
- High RAM/disk footprint: a 3-node cluster consumes 3–6 GB RAM.

**Verdict:** Excluded. arm64 incompatibility is a hard blocker on modern macOS.

---

### Decision: Option A

Docker Compose + privileged systemd containers is the right fit. It is simple, fast, and
produces exactly the one-command up/down experience required. Molecule can be layered on top
later as a CI gate without changing this environment.

---

## 2. Container Spec

### Base image

Use `ghcr.io/geerlingguy/docker-ubuntu2204-ansible` — a community-maintained image built
specifically for Ansible + systemd testing. It ships:

- Ubuntu 22.04
- systemd as PID 1 (correctly configured)
- Python 3 (required by Ansible)
- `sudo`, `curl`, basic utilities

Alternatively build a custom image from `ubuntu:22.04` (see `tests/docker/Dockerfile` below),
which gives full control over pre-installed packages.

### What must be pre-installed in the image

| Package | Why |
|---|---|
| `systemd` | PID 1 for service management |
| `openssh-server` | Ansible connection |
| `python3` | Ansible requires Python on managed hosts |
| `sudo` | Ansible privilege escalation (`become: true`) |
| `curl`, `gnupg`, `ca-certificates` | Wazuh/apt repo setup |
| `iproute2`, `net-tools` | Basic network diagnostics |

### Making systemd work in Docker

systemd as PID 1 inside a Docker container requires:

1. `privileged: true` in Compose (grants all Linux capabilities).
2. `/sys/fs/cgroup` mounted read-only (Docker Desktop provides this automatically).
3. `tmpfs` mounts on `/run` and `/run/lock` (prevents systemd startup errors).
4. `STOPSIGNAL SIGRTMIN+3` in the Dockerfile (graceful systemd shutdown).
5. The container `command` must be `/lib/systemd/systemd` (not a shell).

Example Dockerfile (`tests/docker/Dockerfile.ubuntu2204`):

```dockerfile
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    systemd \
    systemd-sysv \
    openssh-server \
    python3 \
    sudo \
    curl \
    gnupg \
    ca-certificates \
    iproute2 \
    net-tools \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

# Allow root SSH login for test env only
RUN mkdir /var/run/sshd \
  && echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config \
  && echo 'root:blackwall' | chpasswd

# Remove systemd units that fail inside containers
RUN (cd /lib/systemd/system/sysinit.target.wants/ && \
     for i in *; do [ "$i" = "systemd-tmpfiles-setup.service" ] || rm -f "$i"; done) \
  && rm -f /lib/systemd/system/multi-user.target.wants/* \
  && rm -f /etc/systemd/system/*.wants/* \
  && rm -f /lib/systemd/system/local-fs.target.wants/* \
  && rm -f /lib/systemd/system/sockets.target.wants/*udev* \
  && rm -f /lib/systemd/system/sockets.target.wants/*initctl* \
  && rm -f /lib/systemd/system/basic.target.wants/*

# SSH host key generation on first start
RUN systemctl enable ssh

EXPOSE 22

STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd"]
```

> **Security note:** root password `blackwall` and `PermitRootLogin yes` are intentional for
> this local test environment. Never use this image in production or expose these containers
> to a network outside localhost.

---

## 3. Networking

### Approach: Docker bridge network + container hostnames

All containers join a dedicated Docker bridge network (`blackwall-test`). Ansible connects
to containers using their container names as hostnames within that network, which Docker DNS
resolves automatically.

SSH port 22 is exposed on the Docker network (not port-mapped to the host). Ansible runs
inside the same Docker network context by using `ansible_host` set to the container name.

For cases where Ansible runs on the macOS host (outside Docker), each container also
port-maps SSH to a unique host port (`2200`, `2201`, etc.). The inventory uses these ports.

```
macOS host
│
└── Docker Desktop VM
    └── docker network: blackwall-test (172.28.0.0/24)
        ├── black-00  172.28.0.10  →  host port 2200
        ├── black-01  172.28.0.11  →  host port 2201
        └── black-02  172.28.0.12  →  host port 2202
```

Ansible on macOS connects via `localhost:220X` → container SSH.

---

## 4. Up/Down Commands

### Files location

```
blackwall-eye/tests/
├── docker/
│   ├── Dockerfile.ubuntu2204
│   ├── docker-compose.yml          # full topology (manager + 2 agents)
│   ├── docker-compose.single.yml   # 1-host topology for single-role tests
│   └── ssh_config                  # convenience SSH config for manual access
└── inventory/
    ├── hosts.yml                   # full topology inventory
    └── hosts.single.yml            # single-host inventory
```

### Bring up (full topology: 1 manager + 2 agents)

```bash
docker compose -f blackwall-eye/tests/docker/docker-compose.yml up -d --build
```

### Bring up (single host, for testing one role)

```bash
docker compose -f blackwall-eye/tests/docker/docker-compose.single.yml up -d --build
```

### Tear down and remove all volumes

```bash
docker compose -f blackwall-eye/tests/docker/docker-compose.yml down -v
```

Or for single-host:

```bash
docker compose -f blackwall-eye/tests/docker/docker-compose.single.yml down -v
```

### Wait for SSH to be ready (run after `up`)

```bash
until ssh -p 2200 -o StrictHostKeyChecking=no -o ConnectTimeout=2 root@localhost echo ok 2>/dev/null; do
  echo "Waiting for black-00..."; sleep 2
done
```

Or use Ansible's built-in wait:

```bash
ansible all -i blackwall-eye/tests/inventory/hosts.yml -m wait_for_connection
```

---

## 5. Docker Compose File

`blackwall-eye/tests/docker/docker-compose.yml`:

```yaml
name: blackwall-test

networks:
  blackwall-test:
    driver: bridge
    ipam:
      config:
        - subnet: 172.28.0.0/24

x-black-node: &black-node
  build:
    context: .
    dockerfile: Dockerfile.ubuntu2204
  privileged: true
  tmpfs:
    - /run
    - /run/lock
  volumes:
    - /sys/fs/cgroup:/sys/fs/cgroup:ro
  networks:
    - blackwall-test
  restart: "no"

services:
  black-00:
    <<: *black-node
    container_name: black-00
    hostname: black-00
    ports:
      - "2200:22"

  black-01:
    <<: *black-node
    container_name: black-01
    hostname: black-01
    ports:
      - "2201:22"

  black-02:
    <<: *black-node
    container_name: black-02
    hostname: black-02
    ports:
      - "2202:22"
```

`blackwall-eye/tests/docker/docker-compose.single.yml`:

```yaml
name: blackwall-test-single

networks:
  blackwall-test:
    driver: bridge

services:
  black-00:
    build:
      context: .
      dockerfile: Dockerfile.ubuntu2204
    container_name: black-00
    hostname: black-00
    privileged: true
    tmpfs:
      - /run
      - /run/lock
    volumes:
      - /sys/fs/cgroup:/sys/fs/cgroup:ro
    networks:
      - blackwall-test
    ports:
      - "2200:22"
    restart: "no"
```

---

## 6. Ansible Inventory

`blackwall-eye/tests/inventory/hosts.yml`:

```yaml
all:
  vars:
    ansible_user: root
    ansible_password: blackwall
    ansible_ssh_common_args: >-
      -o StrictHostKeyChecking=no
      -o UserKnownHostsFile=/dev/null
    ansible_python_interpreter: /usr/bin/python3

  children:
    wazuh_managers:
      hosts:
        black-00:
          ansible_host: localhost
          ansible_port: 2200

    wazuh_agents:
      hosts:
        black-01:
          ansible_host: localhost
          ansible_port: 2201
        black-02:
          ansible_host: localhost
          ansible_port: 2202
```

`blackwall-eye/tests/inventory/hosts.single.yml`:

```yaml
all:
  vars:
    ansible_user: root
    ansible_password: blackwall
    ansible_ssh_common_args: >-
      -o StrictHostKeyChecking=no
      -o UserKnownHostsFile=/dev/null
    ansible_python_interpreter: /usr/bin/python3

  hosts:
    black-00:
      ansible_host: localhost
      ansible_port: 2200
```

### Run a playbook against the test environment

```bash
# Full topology
ansible-playbook -i blackwall-eye/tests/inventory/hosts.yml \
  blackwall-eye/ansible/playbooks/all.yml

# Single host (e.g. testing wazuh-agent role only)
ansible-playbook -i blackwall-eye/tests/inventory/hosts.single.yml \
  blackwall-eye/ansible/playbooks/monitored-host.yml
```

---

## 7. Naming Convention and Topology Sizing

### Host naming

Containers follow the `black-NN` scheme:

| Name | Default role | SSH port |
|---|---|---|
| `black-00` | Wazuh Manager / security-node | 2200 |
| `black-01` | Wazuh Agent / monitored-host | 2201 |
| `black-02` | Wazuh Agent / monitored-host | 2202 |

Higher numbers (`black-03`, `black-04`, …) can be added by copying the service block in
`docker-compose.yml` and incrementing the port.

### When to use which topology

| Scenario | Compose file | Inventory |
|---|---|---|
| Testing a single role in isolation (wazuh-agent, auditd) | `docker-compose.single.yml` | `hosts.single.yml` |
| Testing manager + 1 agent wiring | `docker-compose.yml` (black-00 + black-01) | `hosts.yml` |
| Full stage-1 topology (manager + 2 agents) | `docker-compose.yml` | `hosts.yml` |

To use only a subset of the full compose file, pass `--scale` or target specific services:

```bash
# Start only black-00 and black-01
docker compose -f blackwall-eye/tests/docker/docker-compose.yml \
  up -d --build black-00 black-01
```

---

## 8. Convenience SSH Config

`blackwall-eye/tests/docker/ssh_config` (add to `~/.ssh/config` or use with `-F`):

```
Host black-00
  HostName localhost
  Port 2200
  User root
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null

Host black-01
  HostName localhost
  Port 2201
  User root
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null

Host black-02
  HostName localhost
  Port 2202
  User root
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
```

Usage:

```bash
ssh -F blackwall-eye/tests/docker/ssh_config black-00
```

---

## 9. Open Questions / Next Steps

| Question | Status |
|---|---|
| Use pre-built `geerlingguy/docker-ubuntu2204-ansible` vs custom Dockerfile? | Prefer custom for full control; revisit if build times are slow |
| Add Molecule on top as CI gate? | Planned for stage 2; not needed for stage-1 iteration |
| SSH key auth vs password auth? | Password used for simplicity; switch to key-based if secrets management is added |
| cgroup v2 compatibility? | Docker Desktop on macOS uses cgroup v2; image must use systemd >= 245 (Ubuntu 22.04 ships 249, OK) |

---

*Last updated: 2026-05-09*
