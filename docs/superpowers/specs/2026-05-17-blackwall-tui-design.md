# `blackwall` TUI — Design

**Date:** 2026-05-17
**Status:** Approved (pending user spec review)
**Scope:** Design of a new k9s-style interactive TUI binary named `blackwall`, intended as the long-term operator UI for the Blackwall project. MVP delivers two views (`:contexts` picker and `:agents` list with detail drill-down), a tab strip, and a `:` command palette. The existing `bw-status` CLI is not touched.

---

## 1. Motivation

`bw-status` is a one-shot CLI that prints a Wazuh agent table and exits. It is useful for scripts and quick checks but does not satisfy the operator workflow of "open a live console, watch the system, drill into a host." That workflow today requires the Wazuh dashboard or SSH+`agent_control`.

The user wants a long-lived, k9s-inspired terminal UI that:

- shows live agent state, auto-refreshing on a configurable interval;
- lets the operator select a host and drill into details with Enter;
- is the foundation for additional views (alerts, modules, configs) without rewrite;
- can be `scp`'d onto a security node and run there without a Python runtime.

The new tool is a separate binary, distinct from `bw-status`. The existing tool keeps working for scripted use.

## 2. Goals and non-goals

**Goals (MVP):**

- Single statically-linked binary called `blackwall`.
- Two functional views: `:contexts` (k9s-style context picker) and `:agents` (live agent table + detail screen on Enter).
- YAML configuration at `~/.config/blackwall/config.yaml`, k9s-style "contexts" model with per-subsystem sections (`wazuh:` today; `voice:`/`brain:`/... forward-compatible).
- Live refresh of the agent list at a configurable interval, with pause/resume.
- Vim-style and arrow-key navigation, `:` command palette, `/` filter, `?` help overlay.
- Resilient runtime: network errors do not crash the app; auth failures are surfaced clearly.
- One-shot `--once` flag preserved as a convenience for scripts.

**Non-goals (MVP):**

- Mouse support beyond what comes free with bubbletea defaults.
- Multiple themes (only the built-in dark theme).
- Configurable keymaps via config file.
- Mutating actions against the manager (no agent restart/remove/upgrade from TUI).
- Secret-store integration (keychain/libsecret).
- Live log tailing or alert browsing — separate views, future iterations.
- CI pipeline (no CI exists in the repo today; add when one does).
- Visual regression tests / screenshot tests.

## 3. Stack and dependencies

- **Language:** Go 1.22+.
- **TUI runtime:** [`github.com/charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) (Elm-style Model/Update/View).
- **Widgets:** [`github.com/charmbracelet/bubbles`](https://github.com/charmbracelet/bubbles) (`table`, `viewport`, `textinput`, `help`, `key`).
- **Styling:** [`github.com/charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss).
- **YAML:** [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3).
- **HTTP:** stdlib `net/http`. No `resty`/`httpx` — three endpoints are not worth a wrapper.
- **Logging:** stdlib `log/slog` with JSON handler.

Rationale for Go over Python+Textual: this is a TUI dashboard that may be deployed onto remote security nodes; single-binary distribution and ~30ms startup matter. Go+Bubbletea is the direct analogue of k9s' stack. The existing repo being Python+Ansible is not a strong constraint — this is a standalone operator tool, not application code that integrates with Ansible logic.

## 4. Repository layout

```
tools/blackwall/
├── go.mod                              # module path TBD with user (see §13)
├── go.sum
├── Makefile                            # build / install / test / lint / build-linux
├── README.md
├── cmd/
│   └── blackwall/
│       └── main.go                     # parse flags+env, load config, start tea.Program
├── internal/
│   ├── config/
│   │   ├── config.go                   # Load(), Resolve(), defaults, $XDG_CONFIG_HOME
│   │   ├── template.go                 # first-run template embedded with go:embed
│   │   └── config_test.go
│   ├── wazuh/
│   │   ├── client.go                   # Client: Authenticate, ListAgents, GetAgent
│   │   ├── types.go                    # Agent, AgentDetail
│   │   └── client_test.go              # against httptest server
│   ├── app/
│   │   └── app.go                      # root tea.Model: view router, command palette
│   ├── views/
│   │   ├── view.go                     # View interface + registry
│   │   ├── contexts/
│   │   │   └── list.go                 # picker view
│   │   └── agents/
│   │       ├── list.go                 # agents table view + auto-refresh
│   │       └── detail.go               # agent detail view
│   ├── ui/
│   │   ├── styles/styles.go            # lipgloss styles (dark theme)
│   │   ├── tabbar/tabbar.go            # top: tab strip of registered views (`:contexts`, `:agents`, ...)
│   │   ├── statusbar/statusbar.go      # under tabs: context, subsystem, refresh state
│   │   └── footer/footer.go            # bottom: keymap hints
│   └── tickcmd/tickcmd.go              # tea.Cmd helper for periodic ticks
└── docs/
    └── ARCHITECTURE.md                 # short architecture note for new contributors
```

Standard Go layout: `cmd/` for entry points, `internal/` for non-exported packages. `tools/blackwall/` is at the **repo root**, deliberately not under `blackwall-eye/tools/`, because the tool is cross-module by design — Wazuh agents today, alerts/modules/configs tomorrow.

`bw-status` stays where it is, untouched.

## 5. Distribution

- **Local:** `cd tools/blackwall && go install ./cmd/blackwall` → binary in `$GOBIN` (`~/go/bin/blackwall`).
- **Remote:** `make build-linux` → `bin/blackwall-linux-amd64`; `scp` to target host. No runtime dependency.
- **Future (out of MVP):** `goreleaser` for tagged releases with deb/rpm/Homebrew taps.

## 6. Configuration

### 6.1 File location

- Primary: `$XDG_CONFIG_HOME/blackwall/config.yaml`.
- Fallback: `~/.config/blackwall/config.yaml`.
- Override: `--config PATH` flag or `BW_CONFIG` env.

### 6.2 Schema

```yaml
# ~/.config/blackwall/config.yaml

current: master-00                  # active context at startup
select_context_on_start: true       # show :contexts picker on startup (skipped if --context/-c CLI is set)

contexts:

  master-00:
    description: "Test env (docker) — black-master-00"
    wazuh:
      server:   172.28.0.10
      port:     55000
      user:     wazuh
      password: ${BW_MASTER_00_WAZUH_PASSWORD}
      tls:
        insecure_skip_verify: true
    # voice:  ...                    # appears in stage 2
    # brain:  ...                    # appears in stage 3

  prod-eu-1:
    description: "Production EU manager"
    wazuh:
      server:   wazuh.eu.example.com
      port:     55000
      user:     bw-reader
      password: ${BW_PROD_EU_WAZUH_PASSWORD}
      tls:
        insecure_skip_verify: false
        ca_file: /etc/blackwall/wazuh-ca.crt

ui:
  refresh_interval:  3s              # time.Duration: "500ms", "2s", "1m"
  theme:             dark            # only dark is implemented in MVP
  vim_keys:          true

logging:
  level: warn                        # debug | info | warn | error
  file:  ~/.local/state/blackwall/blackwall.log
```

### 6.3 Semantics

- A **context** is a named Blackwall deployment. It contains one section per subsystem (`wazuh:`, future `voice:`, `brain:`, `trap:`, `fang:`).
- `current` is the default context at startup (overridable by CLI/env).
- Views declare which subsystem they read from. `:agents` reads `wazuh:`. If the current context lacks the required subsystem, the view renders a placeholder explaining how to add it — does not crash.
- All string values inside `contexts.*` are processed by `os.ExpandEnv`, so `${BW_FOO}` is substituted from environment. Useful for keeping passwords out of the file.
- Empty post-expansion values fail validation at startup with a precise message (`contexts.master-00.wazuh.password resolves to empty: BW_MASTER_00_WAZUH_PASSWORD not set`).
- If a password is stored verbatim and file permissions are not `0600`, a warning is printed to stderr at startup.

### 6.4 Source priority

Resolved by `internal/config.Resolve(args, env)` at startup. Highest wins:

1. Built-in defaults (`select_context_on_start: true`, `refresh_interval: 3s`, `theme: dark`, `vim_keys: true`, `logging.level: warn`).
2. Config file top-level / `ui:` / `logging:` blocks.
3. Environment variables: `BW_CONTEXT`, `BW_REFRESH`, `BW_CONFIG`, `BW_SELECT_CONTEXT_ON_START` (`true`/`false`).
4. CLI flags (see §6.6).

The `contexts` block itself is read only from the config file. Environment affects only which context is active and `${VAR}` expansion inside values.

### 6.5 First-run UX

If the config file does not exist:

1. Create `~/.config/blackwall/` if missing.
2. Write a template file (embedded via `go:embed`) containing one `example-master` context, all `ui:` / `logging:` defaults made explicit, and inline comments.
3. Print to stderr:

   ```
   Created config at ~/.config/blackwall/config.yaml
   Edit it to point at your Wazuh manager, then run `blackwall` again.
   ```

4. Exit with code 0.

The app does **not** attempt to start with the example template — the example server is `wazuh.example.com` which will not resolve.

### 6.6 CLI flags

```
blackwall [flags]

      --context NAME             override `current`; also implies select_context_on_start=false
      --config PATH              override config file path
      --refresh DURATION         override ui.refresh_interval
      --select-context-on-start  force-enable the startup picker (overrides config)
      --no-select-context        force-disable the startup picker (overrides config)
      --once                     print agents table once to stdout and exit (no TUI)
      --version
      --help
```

`--context` is long-only (no short flag, matches `kubectl`/k9s convention; `-c` is reserved for `--config` in the future if shortened).

Per-subsystem overrides (`--host`, `--user`, `--password`) are intentionally absent — in the context model there is no "default subsystem" to override, and adding a new context in the config (5 lines) is the canonical workflow.

## 7. Startup flow

```
parse CLI / env / config
       │
       ▼
validate (subsystem references, env expansion, TLS files, file perms)
       │
       ▼
construct app.Model with all contexts, mark which one is "current"
       │
       ▼
decide startup view:
  ├── if --context or BW_CONTEXT provided
  │      → go directly to :agents of that context
  ├── else if select_context_on_start == true
  │      → start in :contexts picker
  └── else
         → go to :agents of `current`
       │
       ▼
tea.NewProgram(app).Run()
```

Principle: explicit user intent (flag/env) always beats the picker.

## 8. Architecture

### 8.1 Bubbletea recap

Bubbletea is Elm in Go:

- `Model` is immutable state.
- `Update(msg) → (Model, Cmd)` is a pure function: returns new state and optionally a command for async work.
- `View() → string` renders from state.
- `Cmd` is `func() tea.Msg`, executed in a goroutine; its result is delivered back to `Update` as a message. All network calls and timers use this pattern — the UI is never blocked.

### 8.2 Root `app.Model`

`internal/app/app.go` holds:

- the active view (one of the registered views);
- a reference to the resolved config and a `wazuh.Client` for the current context;
- the command palette state (`:` open/closed, current input);
- the status-bar widget and footer widget.

Root `Update` does only:

- global hotkeys: `q` / `Ctrl+C` quit, `:` open palette, `?` toggle help, `Ctrl+R` force refresh, `p` pause/resume;
- delivers all other messages to the active view;
- handles routing messages (`SwitchViewMsg`, `ContextChangedMsg`, `BackToPreviousViewMsg`).

### 8.3 View interface

```go
// internal/views/view.go

type View interface {
    tea.Model                            // Init / Update / View
    Name() string                        // ":agents", ":contexts", ...
    KeyBindings() []key.Binding          // for the footer / help overlay
}
```

Registration is a simple map in `app.Model.views`. Adding a new view is:

1. New package under `internal/views/<name>/`.
2. Implement `View`.
3. One line in the registry.

### 8.4 Tab bar and view navigation

In addition to the `:` command palette, top-level views are reachable through a **tab strip** at the top of the screen — the same idiom as `lazygit`, `btop`, browser tabs. The two mechanisms coexist intentionally so we can test both UX paths in practice and decide which one the operator actually prefers.

**Render:**

```
┌ [1] :contexts │ [2] :agents ───────────────────────────────────────────┐
│ context: master-00 · wazuh: 172.28.0.10:55000 · ↻ live (3s)            │
├────────────────────────────────────────────────────────────────────────┤
│ <active view content>                                                   │
│                                                                         │
├────────────────────────────────────────────────────────────────────────┤
│ [↑↓] nav  [↵] open  [/] filter  [Tab] next view  [:] cmd  [?] help     │
└────────────────────────────────────────────────────────────────────────┘
```

- Tab strip lives in `internal/ui/tabbar/`. Active tab is bold + underlined; inactive tabs are dim.
- Tabs are built from the **registered view order** in `app.Model.views`. MVP order: `:contexts` → `:agents`. Adding a view in the future means one entry in the registry — the tab appears automatically.
- Only **top-level views** are tabs. Sub-screens (e.g. agent detail) are not separate tabs; they live inside their parent view and are navigated via Enter/Esc.

**Hotkeys (global, handled by `app.Model.Update`):**

| Key | Action |
|---|---|
| `1`–`9` | jump directly to tab N (no-op if out of range) |
| `Tab` | next tab (wraps) |
| `Shift+Tab` | previous tab (wraps) |

Number keys do not conflict with view-local input because any `/` filter or `:` palette is modal — while a text input is focused, all keys go to that input.

**Visibility:** the tab bar is always rendered, even when only one view exists, so the user discovers tabs early. Configurable hiding can be added later if it becomes noise; **not** in MVP.

**Coexistence with `:` palette:**
- `Tab` / numbers — fast switch between known views.
- `:contexts` / `:agents` — discoverability, scriptability, switching when you don't remember the index, and the only way to do non-tab actions (`:q`, `:ctx prod-eu-1`).

Both produce the same internal `SwitchViewMsg{name}` — there is one code path for view switching, two input paths.

### 8.5 `:contexts` view

`internal/views/contexts/list.go`. Reads from `app.Model.config.Contexts`. Background goroutine pings each context's Wazuh server (`GET /`, no auth) on entry to populate the `STATUS` column. Pings are throttled (one per 5s) and cancelled on view exit.

Columns: `NAME | DESCRIPTION | WAZUH SERVER | STATUS`.

Keys:

- `↑↓` / `jk` — move cursor.
- `/` — filter by name/description substring.
- `Enter` — switch to `:agents` of the selected context; `current` in the config file is **not** modified.
- `c` — "set current and select": persist the choice to the config file (with file lock and atomic rename) **and** switch.
- `Esc` — return to previous view (no-op if this was the startup view).

### 8.6 `:agents` view

Two sub-screens — list and detail — implemented as separate models inside `internal/views/agents/`. The list owns the auto-refresh loop; the detail is a transient screen entered via Enter.

**`agents/list.go` (`ListModel`):**

- `bubbles/table.Model` for the table.
- On `Init()`, returns the first `tickMsg` via `tickcmd.Every(refreshInterval)`.
- On each `tickMsg`: if not paused, fires `fetchAgentsCmd(reqID++)`; always re-arms the next tick immediately (interval does not drift while a fetch is in flight).
- On `agentsLoadedMsg{agents, reqID, err}`: if `reqID` matches the latest, update table; otherwise drop (stale response).
- Hotkeys: `↑↓`/`jk`, `g`/`G` top/bottom, `/` filter (substring on id/name/ip), `f` filter by status, `s` cycle sort, `Enter` open detail, `p` pause toggle, `Ctrl+R` force refresh.

**`agents/detail.go` (`DetailModel`):**

- On entry: one-shot `fetchAgentDetailCmd(id)`.
- On `agentDetailLoadedMsg{detail, err}`: render grid (ID, name, status, IP, version, OS, last seen, registered, group(s), manager, key config values).
- Hotkeys: `Esc` / `←` back to list, `r` re-fetch, `y` yank ID to clipboard (if `OSC 52` write succeeds; otherwise no-op + footer message).

### 8.7 `wazuh.Client`

```go
// internal/wazuh/client.go

type Client struct {
    baseURL    string
    httpClient *http.Client            // configured for TLS per context
    user       string
    password   string
    token      string
    tokenExp   time.Time
    mu         sync.Mutex              // guards token under concurrent fetches
}

func New(sub config.WazuhSubsystem) (*Client, error)
func (c *Client) ListAgents(ctx context.Context) ([]Agent, error)
func (c *Client) GetAgent(ctx context.Context, id string) (AgentDetail, error)
```

- Authenticates on first call; auto-refreshes JWT on `401` (Wazuh tokens live ~15min).
- All methods take `context.Context` — in-flight requests are cancelled when the user leaves a screen or switches contexts.
- TLS: per-context `http.Transport` honouring `insecure_skip_verify` and `ca_file`.
- No global state; `Client` is owned by `app.Model` and passed to views as a dependency.

### 8.8 Data flow example (Enter on an agent row)

```
keypress "Enter"
   │
   ▼
tea runtime → app.Update(KeyMsg)
   │
   ▼
app delegates KeyMsg to active view (agents/list)
   │
   ▼
list.Update sees Enter, reads cursor row,
returns (model, SwitchToDetailCmd(id))
   │
   ▼
SwitchToDetailMsg{id} arrives in app.Update
   │
   ▼
app replaces activeView with agents/detail;
detail.Init() returns fetchAgentDetailCmd(id)
   │
   ▼
goroutine: client.GetAgent(ctx, id)
   → returns agentDetailLoadedMsg{detail, err}
   │
   ▼
detail.Update applies data; View() renders the grid
```

The UI remains responsive throughout — network calls run in goroutines, rendering happens after the message returns.

### 8.9 Refresh loop

`internal/tickcmd/tickcmd.go`:

```go
func Every(d time.Duration) tea.Cmd {
    return tea.Tick(d, func(t time.Time) tea.Msg { return TickMsg{} })
}
```

`agents/list.go.Init()` returns `tickcmd.Every(cfg.RefreshInterval)`. On each `TickMsg`:

1. fire `fetchAgentsCmd(reqID)` if `!paused`;
2. **immediately** schedule the next tick — interval does not drift behind slow API responses.

Concurrent fetches are reconciled via monotonic `reqID`; stale responses are dropped.

`p` toggles `paused`. Ticks continue arriving but fetches are skipped. Status bar shows `⏸ paused` vs `↻ live (3s)`.

## 9. Keymap

| Context | Key | Action |
|---|---|---|
| Global | `q`, `Ctrl+C` | quit |
| Global | `?` | toggle help overlay (lists all bindings for active view) |
| Global | `:` | open command palette (`:agents`, `:contexts`, `:ctx prod-eu-1`, `:q`) |
| Global | `Ctrl+R` | force refresh now |
| Global | `p` | pause / resume auto-refresh |
| Global | `Tab` | next view (tab bar) |
| Global | `Shift+Tab` | previous view (tab bar) |
| Global | `1`–`9` | jump to view by tab index |
| `:contexts` | `↑↓` / `jk` | move cursor |
| `:contexts` | `g` / `G` | top / bottom |
| `:contexts` | `/` | filter by name/description |
| `:contexts` | `Enter` | open `:agents` of selected context |
| `:contexts` | `c` | persist as `current` and open |
| `:contexts` | `Esc` | back to previous view |
| `:agents` list | `↑↓` / `jk` | move cursor |
| `:agents` list | `g` / `G` | top / bottom |
| `:agents` list | `/` | filter by id/name/ip substring |
| `:agents` list | `f` | cycle status filter (all → active → disconnected → never_connected) |
| `:agents` list | `s` | cycle sort (id → name → status → last_seen) |
| `:agents` list | `Enter` | open agent detail |
| `:agents` detail | `Esc` / `←` | back to list |
| `:agents` detail | `r` | re-fetch detail |
| `:agents` detail | `y` | yank agent ID to clipboard (best-effort via OSC 52) |

Bindings are declared with `bubbles/key.Binding`; the `?` overlay is built automatically from the active view's `KeyBindings()`.

Custom keymaps via config are **not** in MVP.

## 10. Error handling

TUI rules differ from CLI — no "crash and print." Categories:

| Class | Behaviour |
|---|---|
| Fatal at startup (unreadable config, undefined context, missing required subsystem for default view, empty post-expansion env, missing `ca_file`) | Print precise message to stderr, exit non-zero, before `tea.NewProgram` runs |
| Connection refused / timeout | Status-bar: `⚠ unreachable (retry in 3s)`; table keeps previous data, dimmed; consecutive failures counted (`⚠ unreachable ×3`) |
| `401 Unauthorized` | Immediate re-authenticate (token might have expired early). If second attempt also `401` → modal: `Authentication failed for context "X". Check wazuh.user / wazuh.password.` with options `:contexts` or `q` |
| `5xx` | Same UX as unreachable, with `⚠ manager error (HTTP 503)`. Body logged at debug level |
| TLS verification failure | Modal with explicit guidance: `TLS verification failed. Set tls.insecure_skip_verify: true or provide tls.ca_file.` No silent fallback to insecure |
| Detail view: agent removed between fetches (`404`) | Inline notice `Agent no longer exists`; `Esc` returns to list, which refreshes on next tick |
| Partial response (200 with malformed JSON) | Status-bar warning, retain last valid data, raw body logged at debug |

Modals use a bubbletea overlay widget: dim background, centered message, `Esc` to dismiss.

## 11. Logging

- Destination: file from `logging.file`, default `~/.local/state/blackwall/blackwall.log`. Directory created if missing.
- Format: line-based JSON via `slog.NewJSONHandler` for easy `jq` consumption.
- Levels:
  - `debug` — HTTP request/response bodies, tea message trace.
  - `info` — startup/shutdown, context switch, successful auth.
  - `warn` — recoverable errors.
  - `error` — fatal errors that exit the app.
- `stderr` is used **only** for startup-fatal errors. During TUI runtime, all logs go to the file; writing to stderr would corrupt the render.

In-app log tail (`L` hotkey opening a viewport over the log file) — not MVP.

## 12. Testing

Layered, each layer independent.

**`internal/config` — pure logic:**

- Parse valid YAML → expected struct.
- `current` selection; override via `--context` / `BW_CONTEXT`.
- `${VAR}` expansion; empty post-expansion → error.
- Defaults applied for missing `ui:` / `logging:` fields.
- Invalid YAML / missing context / missing required subsystem → precise error.
- `select_context_on_start` default = `true` when absent; `--no-select-context` flag forces off.

**`internal/wazuh` — real HTTP against `httptest.Server`:**

- `Authenticate` — correct Basic auth header, token parsing, `401` handling.
- `ListAgents` / `GetAgent` — happy path + pagination shape (MVP needs only page 1, but code must not assume single page).
- Token auto-refresh on `401`.
- `context.Cancel` aborts in-flight request.
- TLS: self-signed `httptest.NewTLSServer` + `insecure_skip_verify: true`; separate test for verify-failure without skip.

**`internal/views/agents` — bubbletea models, deterministic:**

- Direct `Update(msg)` / `View()` calls (no running runtime) for speed and clarity.
- `wazuh.Client` substituted by an in-memory fake (struct with predefined `[]Agent`).
- Cases: empty list → "No agents" notice; `tickMsg` → fetch fires; `Enter` on row → `SwitchToDetailMsg{id}`; `p` → `paused=true`, next tick skips fetch; `/foo` → table filters by substring; stale `reqID` response is ignored.

**`internal/views/contexts` — same approach:**

- Picker renders all contexts from config.
- `Enter` produces `ContextChangedMsg`, does not touch config file.
- `c` writes config file atomically (temp file + `rename`).

**Smoke E2E:**

- Script at `tools/blackwall/scripts/smoke.sh` brings up the docker test env, waits for the manager to respond, runs `blackwall --once --context master-00`, greps for `Active`. Exits non-zero on failure. Not a TUI test, but exercises the full `config + wazuh.Client` chain against a real Wazuh API.

**Out of scope for MVP:**

- Visual regression / screenshot tests.
- Concurrency stress (one user, one TUI).
- Mouse event tests.

## 13. Build, lint, CI

- `make build` — local binary in `bin/blackwall`.
- `make build-linux` — `GOOS=linux GOARCH=amd64 go build -o bin/blackwall-linux-amd64 ./cmd/blackwall`.
- `make install` — `go install ./cmd/blackwall`.
- `make test` — `go test ./...`.
- `make lint` — `golangci-lint run` (govet, staticcheck, gofumpt, errcheck).
- CI — not configured in MVP (repo has no CI today). Same `make lint && make test` is the contract when CI arrives.

**Open decision (defer to implementation start):** the Go module path. Options: `github.com/<user>/blackwall/tools/blackwall`, or a short internal path like `blackwall.local/tools/blackwall`. Resolved at the `go mod init` step.

## 14. Future iterations (out of MVP, listed for context)

- `:alerts` view backed by the indexer (OpenSearch query against `alerts-*`).
- `:modules` view showing Blackwall module status (`STATUS.md` per module).
- Live log tail (`L`) for the active agent or manager.
- Themes (light, custom).
- Configurable keymaps.
- Secret-store integration for passwords.
- Mutating actions (restart agent, remove agent) with confirmation.
- Mouse support.
- `goreleaser` packaging.

## 15. Open questions

- Go module path (see §13).
- Whether `:contexts` should also show contexts that fail YAML schema validation (with a `⚠ invalid` status) or hide them entirely. **Tentative:** hide and warn at startup; revisit if confusing in practice.
- Whether to persist the table sort/filter state per context across restarts. **Tentative:** no, restart returns to defaults. Cheap to add later if requested.
- Tab bar vs. command palette: both ship in MVP so we can compare them in practice. After a few weeks of use, decide whether to keep both, hide the tab bar (config opt-in), or simplify the palette. **Tentative:** keep both indefinitely — they serve different ergonomics (muscle-memory tabs vs. discoverable palette).
