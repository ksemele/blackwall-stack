# blackwall

k9s-style terminal UI for the Blackwall security project. Today it surfaces
Wazuh agent state; over time it will gain alerts, modules, and other views.

The lightweight one-shot CLI [`bw-status`](../../blackwall-eye/tools/bw-status)
remains available for scripts and quick checks.

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

## Distribution to remote hosts

```bash
make build-linux
scp bin/blackwall-linux-amd64 root@security-node:/usr/local/bin/blackwall
```

No runtime required.

## Spec

Full design: [`docs/superpowers/specs/2026-05-17-blackwall-tui-design.md`](../../docs/superpowers/specs/2026-05-17-blackwall-tui-design.md).
