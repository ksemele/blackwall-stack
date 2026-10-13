# blackwall

k9s-style terminal UI for the Blackwall security project. Today it surfaces
Wazuh agent state; over time it will gain alerts, modules, and other views.

For scripts and quick checks use `blackwall --once`: it prints the agents table
and exits (it replaces the former Python `bw-status`). Timestamps are shown in
local time with UTC in parentheses, e.g. `2026-10-12 09:58 (06:58Z)`.

## Install

Requires Go 1.22+.

```bash
cd tools/blackwall
go install ./cmd/blackwall
```

The binary lands in `$GOBIN` (defaults to `~/go/bin/`).

## Configure

`blackwall` reads `~/.config/blackwall/config.yaml`. On first run it writes
a template there with one example context and exits — edit it before
running again.

A typical config:

```yaml
current: master-00
select_context_on_start: true

contexts:
  master-00:
    description: "Docker test env"
    wazuh:
      server:   172.28.0.10
      port:     55000
      user:     wazuh
      password: ${BW_MASTER_00_WAZUH_PASSWORD}
      tls:
        insecure_skip_verify: true

ui:
  refresh_interval: 3s
  vim_keys:         true
```

### Test environment

`tests/bin/bw-lab up` publishes the Wazuh API on `127.0.0.1:55000`, so a context
for the lab is just:

```yaml
  lab:
    description: "Docker test env (tests/bin/bw-lab)"
    wazuh:
      server:   127.0.0.1
      port:     55000
      user:     wazuh
      password: wazuh
      tls:
        insecure_skip_verify: true
```

The generated template already contains it; `blackwall --context lab --once`.

## Run

```bash
blackwall                              # interactive TUI
blackwall --context prod-eu-1          # skip picker, open this context
blackwall --once                       # print agents and exit (script-friendly)
```

## Keys

| Key | Action |
|---|---|
| `q`, `Ctrl+C` | quit |
| `Tab` / `Shift+Tab` | next / previous view |
| `1`-`9` | jump to view by tab number |
| `:` | open command palette (`:agents`, `:contexts`, `:ctx <name>`, `:q`) |
| `↑↓` / `j k` | move cursor |
| `↵` | open (context picker → open context; agents list → details) |
| `p` | pause / resume auto-refresh |
| `Ctrl+R` | force refresh now |
| `?` | toggle help (planned) |

## Agent detail

Open an agent from the agents list with `↵` to enter detail. The detail screen
has six tabs, switchable by hotkey:

| Tab | Key | Content |
|---|---|---|
| Overview | `o` | Status, IP, version, OS, last-seen, registered, manager |
| Hardware | `h` | CPU, RAM, board, OS details |
| Top | `t` | Running processes (filterable with `/`) |
| Packages | `k` | Installed packages (filterable with `/`) |
| Network | `n` | Interfaces, addresses, open ports |
| Security | `s` | FIM scan, rootcheck findings, SCA compliance |

Agents enrolled into the Wazuh group `docker` (the `wazuh-agent` role does this
for hosts with `/.dockerenv`) get a red `[docker]` tag in the list and a `DOCKER`
badge plus warning in detail: container kernels may lack audit support, so
auditd detections do not fire there. See
[tests/README.md](../../tests/README.md#container-limitations).

Press `esc` to return to the agents list, `r` to force a refresh.

While in Top or Packages, press `/` to start typing a substring filter.
`Enter` applies it, `Esc` clears.

Hardware, packages, and network inventory are cached for 30–60 seconds to
avoid hammering the Wazuh API when switching between tabs. Press `r` to
force a fresh fetch.

## Local development

Rebuild and reinstall after local changes:

```bash
cd tools/blackwall
make install              # go install ./cmd/blackwall — refreshes the binary in $GOBIN
```

Clean rebuild:

```bash
make clean && make install
```

Other useful targets: `make build` (binary into `bin/`), `make test`, `make lint`.

## Distribution to remote hosts

```bash
make build-linux
scp bin/blackwall-linux-amd64 root@security-node:/usr/local/bin/blackwall
```

No runtime required.

## Spec

Full design: [`docs/superpowers/specs/2026-05-17-blackwall-tui-design.md`](../../docs/superpowers/specs/2026-05-17-blackwall-tui-design.md).
