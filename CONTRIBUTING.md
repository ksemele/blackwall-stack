# Contributing to Blackwall Stack

Thanks for your interest. Blackwall Stack is a small, opinionated Wazuh-based
stack for Linux fleets of 1–50 hosts. Its rule of thumb is **signal over
noise**: a few high-confidence detections beat broad, noisy coverage. Keep that
in mind when proposing rules, roles or alerts.

Working on the terminal UI? It lives in its own repo,
[ksemele/blackwall](https://github.com/ksemele/blackwall), with its own
contributing guide. This repo holds the Ansible roles, detection rules and the
Docker lab.

## Questions and ideas

Ask in [GitHub Discussions](https://github.com/ksemele/blackwall-stack/discussions)
rather than opening an issue. Issues are for bugs and concrete feature
requests (use the templates). Security problems go through
[SECURITY.md](./SECURITY.md), never a public issue.

## Development setup

You need:

- Docker (Docker Desktop on macOS, or Docker Engine with Compose v2 on Linux).
  The lab nodes are privileged containers with systemd as PID 1.
- Ansible on the controller (your machine), plus `passlib` and
  `bcrypt<5.0.0` for the indexer role, and the `containers.podman` and
  `ansible.posix` collections for the sentinel node. See
  [controller prerequisites](./tests/README.md#19-controller-prerequisites).
- `python3` (the check scripts use it instead of `jq` and `nc`).
- Optional: `sshpass` so `bw-lab ssh` stops asking for the password, and
  [direnv](https://direnv.net/) for local secrets (see [Secrets](#secrets)).

Two scripts drive the lab. Run them from anywhere in the repo; they resolve
paths from the repo root.

```bash
tests/bin/bw-lab up          # build + start containers, then provision everything
tests/bin/bw-check all       # verify: hosts, packages, firewall, ports, agents, metrics
tests/bin/bw-lab down        # stop and remove the lab, volumes included
tests/bin/bw-lab reset       # down + wipe Wazuh certificates + up (clean slate)
```

The first `bw-lab up` takes 10–20 minutes: it builds the images and downloads
about 1.5 GB of Wazuh packages. Later runs take the packages from a local apt
cache.

While iterating on one part, re-run only what you touched:

```bash
tests/bin/bw-lab provision nftables     # one target: eye, nftables, metrics, voice, cf-wall, brain, fang, sentinel, fixtures, all
tests/bin/bw-check nft                  # one check group
tests/bin/bw-lab ssh black-host-01      # shell into a node
```

`tests/bin/bw-check --help` lists every check group. Some groups (`sentinel`,
`sentinel-agents`, `fw-events`, `fang`, `vm`) are opt-in: they are not part of `all`
and SKIP until their target is provisioned. Every line prints `PASS`, `FAIL` or
`SKIP`, and the exit code is 0 only when nothing failed.

Unit checks for scripts run without the lab. The Voice checks render
the role templates and need `jinja2` and `pyyaml`; the Python that runs Ansible
already has both, a bare system `python3` may not:

```bash
PY="$(brew --prefix ansible)/libexec/bin/python"   # Homebrew Ansible; pipx: ~/.local/pipx/venvs/ansible/bin/python
"$PY" tests/unit/voice_format_test.py
"$PY" tests/unit/alerts_tail_test.py
"$PY" tests/unit/fang_refresh_test.py
```

Brain is a Python package with its own pytest suite (Python 3.12+):

```bash
cd blackwall-brain
python3.12 -m venv .venv && .venv/bin/pip install -e '.[dev]'
.venv/bin/pytest
```

Full lab documentation (containers, ports, credentials, manual debugging, known
limitations): [tests/README.md](./tests/README.md).

## Repository layout

Each module is a directory with one security responsibility:

| Directory | Codename | Purpose |
|---|---|---|
| `blackwall-eye/` | Eye | Host monitoring core: Wazuh, auditd, nftables, detection rules |
| `blackwall-voice/` | Voice | Alerts, metrics and the notification layer |
| `blackwall-brain/` | Brain | AI analysis of alerts |
| `blackwall-trap/` | Trap | Honeypots and canaries (planned, not started) |
| `blackwall-fang/` | Fang | Automatic blocking (reputation blocklists) |
| `blackwall-scan/` | Scan | Outside-in scan of your own perimeter (separate track) |
| `tests/` | — | Docker lab, inventory, `bw-lab`, `bw-check`, triggers, unit checks |

Rules that apply to every module:

- **Independently deployable.** A module must not require another module to be
  installed. Ansible modules have their own `ansible.cfg` and
  `ansible/{playbooks,roles}`; shared lab inventory lives in `tests/inventory/`.
- **State lives in `STATUS.md`.** Each module's `STATUS.md` records its stage,
  what is done, the next task and known issues. Read it before you start; it is
  the best map of what is in flight. Update it when your change completes or
  changes a planned item.
- **Config as code.** No manual steps that can't be reproduced from files in
  the repo. If a change needs a step, put it in a role, a playbook or a script.
- **Keep docs in sync.** When you add or change a playbook, role, inventory
  file, check or CLI command, update the module's `README.md`,
  `tests/README.md` and any script that references it in the same PR.
- **Thematic naming.** New modules keep the Blackwall/Cyberpunk naming.
- **Rule IDs are an interface.** Custom Wazuh rules live in
  `blackwall-eye/ansible/roles/wazuh-manager/templates/blackwall_rules.xml.j2`.
  The TUI reads some of them (for example 100100), so changing an existing ID
  or field needs a matching change in the TUI repo.

## No real hosts, no secrets

This is a public repo.

- **No real hostnames, IPs or domains** in code, docs, examples, tests or
  commit messages. Use `example.com` / `example.org` names and documentation
  ranges (`192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`). Lab addresses
  (`172.28.0.0/24`, `black-*` containers) are fine.
- **Your own hosts stay local.** `hosts/` is gitignored for per-host configs and
  inventories; `blackwall-scan/targets/targets.txt` and scan output are
  gitignored too. Never force-add them.

### Secrets

Roles take secrets as plain variables (for example `wazuh_enrollment_password`,
`telegram_bot_token`, `claude_api_key`); the operator supplies them from their
own secret store. The repo does **not** use ansible-vault, and no real secret
belongs in an inventory or `group_vars` file.

In the lab, `tests/inventory/hosts.yml` reads secrets from environment
variables. Copy `.envrc.example` to `.envrc` (gitignored) and fill it in, or
export the variables yourself. Without them Voice still deploys (only delivery
fails); Brain refuses to deploy while enabled.

The lab's default credentials (`admin` / `Blackwall1!`, `root` / `blackwall`,
and so on) are intentional and lab-only. Don't reuse them in roles' defaults
for real hosts.

## Making a change

1. For anything bigger than a small fix, open an issue or a Discussion first so
   we can agree on the approach.
2. Fork, branch from `main`, keep the change to one topic.
3. Run the lab and the checks for what you touched. For a new behaviour, add a
   `bw-check` assertion (or a unit check for a script) that fails without your
   change.
4. Update docs and the module's `STATUS.md` in the same PR.
5. Open a PR using the template and paste the relevant `bw-check` output.

### Commit style

- English only, in commits, code comments and docs.
- Short [conventional commit](https://www.conventionalcommits.org/) messages,
  scoped by module: `feat(fang): attempt detectors`, `fix(eye): fw-export skips
  unsafe drop-ins`, `docs(tests): bw-check fang`. Types in use: `feat`, `fix`,
  `docs`, `chore`, `refactor`, `test`.
- One logical step per commit. Keep the subject under ~60 characters; add a
  body only when the why isn't obvious.

## How PRs are reviewed

The maintainer reviews every PR. Expect review to check:

- the change works in the lab: `bw-check` for the affected groups passes (paste
  the output), and new behaviour has a check;
- signal over noise: new rules have a clear threat model, sensible levels and
  no flood of alerts on a normal host;
- module isolation, config as code, docs and `STATUS.md` updated;
- no real hosts, IPs, domains or secrets anywhere in the diff.

Small PRs get reviewed faster. A PR may be asked to split, or to move a design
question to Discussions first. Once approved, the maintainer merges it.

## License

By contributing you agree that your contributions are licensed under the
repository's [LICENSE](./LICENSE).
