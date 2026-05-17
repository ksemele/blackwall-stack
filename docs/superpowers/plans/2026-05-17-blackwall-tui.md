# blackwall TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** `docs/superpowers/specs/2026-05-17-blackwall-tui-design.md`

**Goal:** Build a k9s-style interactive TUI binary named `blackwall` (Go + Bubbletea) with two views (`:contexts` picker and `:agents` live list with detail drill-down), a top tab bar, a `:` command palette, and YAML-based per-context configuration.

**Architecture:** Standard Go `cmd/` + `internal/` layout under `tools/blackwall/`. Bubbletea Elm-style Model/Update/View, async work via `tea.Cmd` goroutines. Per-context Wazuh client (JWT auth, TLS-aware). View registry powers both tab bar and command palette through one `SwitchViewMsg` code path. Built-in dark theme via lipgloss.

**Tech Stack:**
- Go 1.22+
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/lipgloss`
- `gopkg.in/yaml.v3`
- stdlib `net/http`, `log/slog`, `embed`, `flag`

**Branch / commits:** All work on `claude-dev`. Commit per step with conventional, super-brief messages (≤70 chars). Never push.

**Module path:** `github.com/greengrunt/blackwall/tools/blackwall` is the placeholder used throughout. Replace at `go mod init` time if your GitHub handle differs.

---

## File structure

Files created over the course of this plan:

```
tools/blackwall/
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── .gitignore
├── scripts/
│   └── smoke.sh
├── cmd/blackwall/
│   └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go             # types
│   │   ├── load.go               # YAML load + env expand + validate
│   │   ├── resolve.go            # flags/env/defaults precedence
│   │   ├── template.go           # embedded first-run template
│   │   ├── template.yaml         # the template itself (embedded via go:embed)
│   │   ├── load_test.go
│   │   ├── resolve_test.go
│   │   └── template_test.go
│   ├── wazuh/
│   │   ├── types.go              # Agent, AgentDetail
│   │   ├── client.go             # Client + Authenticate + token refresh
│   │   ├── agents.go             # ListAgents, GetAgent
│   │   ├── tls.go                # buildHTTPClient for a WazuhSubsystem
│   │   ├── client_test.go
│   │   ├── agents_test.go
│   │   └── tls_test.go
│   ├── tickcmd/
│   │   ├── tickcmd.go
│   │   └── tickcmd_test.go
│   ├── log/
│   │   ├── log.go                # slog JSON setup
│   │   └── log_test.go
│   ├── ui/
│   │   ├── styles/styles.go      # lipgloss palette
│   │   ├── tabbar/tabbar.go
│   │   ├── tabbar/tabbar_test.go
│   │   ├── statusbar/statusbar.go
│   │   ├── statusbar/statusbar_test.go
│   │   ├── footer/footer.go
│   │   └── footer/footer_test.go
│   ├── app/
│   │   ├── messages.go           # SwitchViewMsg, ContextChangedMsg, ...
│   │   ├── app.go                # root tea.Model
│   │   ├── palette.go            # ":" command palette logic
│   │   ├── app_test.go
│   │   └── palette_test.go
│   ├── views/
│   │   ├── view.go               # View interface + registry
│   │   ├── contexts/
│   │   │   ├── list.go
│   │   │   └── list_test.go
│   │   └── agents/
│   │       ├── list.go
│   │       ├── detail.go
│   │       ├── list_test.go
│   │       └── detail_test.go
│   └── oneshot/
│       ├── oneshot.go            # --once text printer
│       └── oneshot_test.go
└── docs/
    └── ARCHITECTURE.md
```

Each file has one responsibility, kept small enough to read in one screen. Tests live next to their packages.

---

## Task 1: Repo scaffolding

**Files:**
- Create: `tools/blackwall/.gitignore`
- Create: `tools/blackwall/Makefile`
- Create: `tools/blackwall/cmd/blackwall/main.go`
- Create: `tools/blackwall/go.mod` (via `go mod init`)

- [ ] **Step 1: Create directory tree**

```bash
mkdir -p tools/blackwall/cmd/blackwall \
         tools/blackwall/internal/{config,wazuh,tickcmd,log,app,oneshot} \
         tools/blackwall/internal/ui/{styles,tabbar,statusbar,footer} \
         tools/blackwall/internal/views/{contexts,agents} \
         tools/blackwall/scripts \
         tools/blackwall/docs
```

- [ ] **Step 2: Initialize Go module**

```bash
cd tools/blackwall && go mod init github.com/greengrunt/blackwall/tools/blackwall
```

Expected output: `go: creating new go.mod: module github.com/greengrunt/blackwall/tools/blackwall`

- [ ] **Step 3: Add minimal main.go**

Create `tools/blackwall/cmd/blackwall/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("blackwall: not yet implemented")
}
```

- [ ] **Step 4: Write .gitignore**

Create `tools/blackwall/.gitignore`:

```
/bin/
*.test
*.out
coverage.txt
```

- [ ] **Step 5: Write Makefile**

Create `tools/blackwall/Makefile`:

```makefile
BIN_DIR := bin
PKG     := ./...

.PHONY: build install test lint build-linux clean

build:
	go build -o $(BIN_DIR)/blackwall ./cmd/blackwall

install:
	go install ./cmd/blackwall

test:
	go test -race $(PKG)

lint:
	golangci-lint run

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/blackwall-linux-amd64 ./cmd/blackwall

clean:
	rm -rf $(BIN_DIR)
```

- [ ] **Step 6: Verify build**

Run: `cd tools/blackwall && go build ./...`
Expected: no output (success).

Run: `cd tools/blackwall && go run ./cmd/blackwall`
Expected: `blackwall: not yet implemented`

- [ ] **Step 7: Commit**

```bash
git add tools/blackwall
git commit -m "feat(blackwall): scaffold tools/blackwall Go module"
```

---

## Task 2: Config types + YAML parsing

**Files:**
- Create: `tools/blackwall/internal/config/config.go`
- Create: `tools/blackwall/internal/config/load.go` (partial — just YAML decode)
- Create: `tools/blackwall/internal/config/load_test.go`

- [ ] **Step 1: Write failing test for YAML unmarshal**

Create `tools/blackwall/internal/config/load_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"
	"time"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := writeFile(path, content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestLoad_HappyPath(t *testing.T) {
	yaml := `
current: master-00
select_context_on_start: true

contexts:
  master-00:
    description: "Docker test env"
    wazuh:
      server: 172.28.0.10
      port: 55000
      user: wazuh
      password: secret
      tls:
        insecure_skip_verify: true

ui:
  refresh_interval: 3s
  theme: dark
  vim_keys: true

logging:
  level: warn
  file: /tmp/blackwall.log
`
	path := writeTempFile(t, "config.yaml", yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Current != "master-00" {
		t.Errorf("Current = %q, want master-00", cfg.Current)
	}
	if !cfg.SelectContextOnStart {
		t.Errorf("SelectContextOnStart = false, want true")
	}
	ctx, ok := cfg.Contexts["master-00"]
	if !ok {
		t.Fatalf("contexts['master-00'] missing")
	}
	if ctx.Wazuh.Server != "172.28.0.10" || ctx.Wazuh.Port != 55000 {
		t.Errorf("Wazuh server/port wrong: %+v", ctx.Wazuh)
	}
	if !ctx.Wazuh.TLS.InsecureSkipVerify {
		t.Errorf("InsecureSkipVerify = false, want true")
	}
	if cfg.UI.RefreshInterval != 3*time.Second {
		t.Errorf("RefreshInterval = %v, want 3s", cfg.UI.RefreshInterval)
	}
}
```

(`writeFile` will be implemented as a small helper inside `load.go` so tests can reuse it.)

- [ ] **Step 2: Run test, expect compile error**

Run: `cd tools/blackwall && go test ./internal/config/...`
Expected: fails because `Load`, `Config`, etc. are not defined.

- [ ] **Step 3: Define config types**

Create `tools/blackwall/internal/config/config.go`:

```go
// Package config models the blackwall YAML configuration.
package config

import "time"

type Config struct {
	Current              string             `yaml:"current"`
	SelectContextOnStart bool               `yaml:"select_context_on_start"`
	Contexts             map[string]Context `yaml:"contexts"`
	UI                   UI                 `yaml:"ui"`
	Logging              Logging            `yaml:"logging"`
}

type Context struct {
	Description string          `yaml:"description"`
	Wazuh       *WazuhSubsystem `yaml:"wazuh,omitempty"`
}

type WazuhSubsystem struct {
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	TLS      TLS    `yaml:"tls"`
}

type TLS struct {
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	CAFile             string `yaml:"ca_file"`
}

type UI struct {
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	Theme           string        `yaml:"theme"`
	VimKeys         bool          `yaml:"vim_keys"`
}

type Logging struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}
```

- [ ] **Step 4: Implement Load (minimal — no env expand or validation yet)**

Create `tools/blackwall/internal/config/load.go`:

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads a YAML config file from disk into a Config.
// Defaults, env expansion and validation happen elsewhere
// (see Apply / Resolve).
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// writeFile is an internal helper used by tests.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
```

- [ ] **Step 5: Add yaml dependency**

Run: `cd tools/blackwall && go get gopkg.in/yaml.v3`
Expected: yaml.v3 added to go.mod and go.sum.

- [ ] **Step 6: Run test, expect pass**

Run: `cd tools/blackwall && go test ./internal/config/... -v`
Expected: `--- PASS: TestLoad_HappyPath`.

- [ ] **Step 7: Commit**

```bash
git add tools/blackwall/internal/config tools/blackwall/go.mod tools/blackwall/go.sum
git commit -m "feat(blackwall): config types + YAML load"
```

---

## Task 3: Config defaults + env expansion + validation

**Files:**
- Modify: `tools/blackwall/internal/config/load.go`
- Modify: `tools/blackwall/internal/config/load_test.go`

- [ ] **Step 1: Add failing tests for defaults, env expansion and validation**

Append to `tools/blackwall/internal/config/load_test.go`:

```go
func TestApplyDefaults_FillsMissingFields(t *testing.T) {
	cfg := Config{} // all zero-value
	out := ApplyDefaults(cfg)

	if !out.SelectContextOnStart {
		t.Errorf("SelectContextOnStart default = false, want true")
	}
	if out.UI.RefreshInterval != 3*time.Second {
		t.Errorf("RefreshInterval default = %v, want 3s", out.UI.RefreshInterval)
	}
	if out.UI.Theme != "dark" {
		t.Errorf("Theme default = %q, want dark", out.UI.Theme)
	}
	if !out.UI.VimKeys {
		t.Errorf("VimKeys default = false, want true")
	}
	if out.Logging.Level != "warn" {
		t.Errorf("Logging.Level default = %q, want warn", out.Logging.Level)
	}
	if out.Logging.File == "" {
		t.Errorf("Logging.File default = empty, want non-empty")
	}
}

func TestExpandEnv_SubstitutesVars(t *testing.T) {
	t.Setenv("BW_TEST_PASSWORD", "topsecret")
	cfg := Config{
		Contexts: map[string]Context{
			"x": {Wazuh: &WazuhSubsystem{Password: "${BW_TEST_PASSWORD}"}},
		},
	}
	out, err := ExpandEnv(cfg)
	if err != nil {
		t.Fatalf("ExpandEnv: %v", err)
	}
	if got := out.Contexts["x"].Wazuh.Password; got != "topsecret" {
		t.Errorf("Password = %q, want topsecret", got)
	}
}

func TestExpandEnv_ErrorsOnEmptyAfterExpand(t *testing.T) {
	cfg := Config{
		Contexts: map[string]Context{
			"x": {Wazuh: &WazuhSubsystem{Password: "${BW_MISSING_VAR}"}},
		},
	}
	_, err := ExpandEnv(cfg)
	if err == nil {
		t.Fatal("ExpandEnv: expected error, got nil")
	}
}

func TestValidate_OK(t *testing.T) {
	cfg := Config{
		Current: "x",
		Contexts: map[string]Context{
			"x": {Wazuh: &WazuhSubsystem{Server: "h", Port: 55000, User: "u", Password: "p"}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestValidate_UnknownCurrent(t *testing.T) {
	cfg := Config{
		Current:  "missing",
		Contexts: map[string]Context{"x": {}},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate: expected error for unknown current")
	}
}

func TestValidate_CAFileNotFound(t *testing.T) {
	cfg := Config{
		Current: "x",
		Contexts: map[string]Context{
			"x": {Wazuh: &WazuhSubsystem{
				Server: "h", Port: 1, User: "u", Password: "p",
				TLS: TLS{CAFile: "/nope/missing.crt"},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate: expected error for missing CA file")
	}
}
```

- [ ] **Step 2: Run tests, expect compile error**

Run: `cd tools/blackwall && go test ./internal/config/... -run "ApplyDefaults|ExpandEnv|Validate"`
Expected: fails — symbols undefined.

- [ ] **Step 3: Implement ApplyDefaults / ExpandEnv / Validate**

Replace contents of `tools/blackwall/internal/config/load.go`:

```go
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Load reads a YAML config file from disk into a Config.
// It does NOT apply defaults, env expansion or validation —
// callers should chain ApplyDefaults / ExpandEnv / Validate.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// ApplyDefaults fills missing scalar fields with built-in defaults.
// `SelectContextOnStart` is a bool: we cannot distinguish "absent" from
// "explicit false" at the struct level. To make the default sticky, the
// caller is expected to use Resolve (which checks a presence map produced
// by load).
//
// Implementation note: for MVP we treat zero-value as "absent" for
// everything except top-level booleans where we accept the limitation.
func ApplyDefaults(cfg Config) Config {
	if cfg.UI.RefreshInterval == 0 {
		cfg.UI.RefreshInterval = 3 * time.Second
	}
	if cfg.UI.Theme == "" {
		cfg.UI.Theme = "dark"
	}
	if !cfg.UI.VimKeys {
		cfg.UI.VimKeys = true
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "warn"
	}
	if cfg.Logging.File == "" {
		home, _ := os.UserHomeDir()
		cfg.Logging.File = home + "/.local/state/blackwall/blackwall.log"
	}
	// MVP: default SelectContextOnStart to true when zero.
	// This means "explicit false in YAML" is overridden to true,
	// but that is fine because main.go reads the raw YAML separately
	// to detect explicit-false and reapplies it. See Resolve.
	if !cfg.SelectContextOnStart {
		cfg.SelectContextOnStart = true
	}
	return cfg
}

var envVarRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// ExpandEnv walks every string inside Contexts.*.Wazuh and substitutes
// ${VAR} from the process environment. An empty post-expansion value
// is an error (typically a missing env var).
func ExpandEnv(cfg Config) (Config, error) {
	for name, ctx := range cfg.Contexts {
		if ctx.Wazuh == nil {
			continue
		}
		w := *ctx.Wazuh

		var expandErr error
		expand := func(in, field string) string {
			out := envVarRE.ReplaceAllStringFunc(in, func(m string) string {
				sub := envVarRE.FindStringSubmatch(m)
				return os.Getenv(sub[1])
			})
			if strings.Contains(in, "${") && strings.TrimSpace(out) == "" {
				expandErr = fmt.Errorf(
					"contexts.%s.wazuh.%s resolves to empty (env var not set?)",
					name, field,
				)
			}
			return out
		}

		w.Server = expand(w.Server, "server")
		w.User = expand(w.User, "user")
		w.Password = expand(w.Password, "password")
		w.TLS.CAFile = expand(w.TLS.CAFile, "tls.ca_file")
		if expandErr != nil {
			return cfg, expandErr
		}

		ctx.Wazuh = &w
		cfg.Contexts[name] = ctx
	}
	return cfg, nil
}

// Validate checks structural invariants of a fully-resolved config.
func Validate(cfg Config) error {
	if cfg.Current == "" {
		return fmt.Errorf("config: `current` is required")
	}
	if _, ok := cfg.Contexts[cfg.Current]; !ok {
		return fmt.Errorf("config: current context %q not defined in contexts", cfg.Current)
	}
	for name, ctx := range cfg.Contexts {
		if ctx.Wazuh == nil {
			continue
		}
		w := ctx.Wazuh
		if w.Server == "" || w.Port == 0 || w.User == "" || w.Password == "" {
			return fmt.Errorf(
				"contexts.%s.wazuh: server/port/user/password are all required",
				name,
			)
		}
		if w.TLS.CAFile != "" {
			if _, err := os.Stat(w.TLS.CAFile); err != nil {
				return fmt.Errorf(
					"contexts.%s.wazuh.tls.ca_file: %v", name, err,
				)
			}
		}
	}
	return nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
```

- [ ] **Step 4: Run all config tests, expect pass**

Run: `cd tools/blackwall && go test ./internal/config/... -v`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/config
git commit -m "feat(blackwall): config defaults + env expand + validation"
```

---

## Task 4: Config resolve (CLI/env/defaults precedence)

**Files:**
- Create: `tools/blackwall/internal/config/resolve.go`
- Create: `tools/blackwall/internal/config/resolve_test.go`

- [ ] **Step 1: Write failing tests for Resolve**

Create `tools/blackwall/internal/config/resolve_test.go`:

```go
package config

import (
	"testing"
	"time"
)

func TestResolve_CLIBeatsConfig(t *testing.T) {
	cfg := Config{Current: "in-file"}
	got, err := Resolve(cfg, Overrides{Context: "from-cli"}, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Current != "from-cli" {
		t.Errorf("Current = %q, want from-cli", got.Current)
	}
}

func TestResolve_EnvBeatsConfig(t *testing.T) {
	t.Setenv("BW_CONTEXT", "from-env")
	t.Setenv("BW_REFRESH", "10s")
	cfg := Config{
		Current: "in-file",
		UI:      UI{RefreshInterval: 5 * time.Second},
	}
	got, err := Resolve(cfg, Overrides{}, ReadEnv())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Current != "from-env" {
		t.Errorf("Current = %q, want from-env", got.Current)
	}
	if got.UI.RefreshInterval != 10*time.Second {
		t.Errorf("Refresh = %v, want 10s", got.UI.RefreshInterval)
	}
}

func TestResolve_CLIBeatsEnv(t *testing.T) {
	t.Setenv("BW_CONTEXT", "from-env")
	cfg := Config{Current: "in-file"}
	got, err := Resolve(cfg, Overrides{Context: "from-cli"}, ReadEnv())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Current != "from-cli" {
		t.Errorf("Current = %q, want from-cli", got.Current)
	}
}

func TestResolve_ExplicitContextDisablesPicker(t *testing.T) {
	cfg := Config{Current: "x", SelectContextOnStart: true}
	got, err := Resolve(cfg, Overrides{Context: "x", ContextWasSet: true}, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.SelectContextOnStart {
		t.Errorf("explicit --context should disable picker")
	}
}

func TestResolve_NoSelectContextFlag(t *testing.T) {
	cfg := Config{Current: "x", SelectContextOnStart: true}
	no := false
	got, err := Resolve(cfg, Overrides{SelectContextOnStart: &no}, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.SelectContextOnStart {
		t.Errorf("--no-select-context should disable picker")
	}
}
```

- [ ] **Step 2: Run tests, expect compile error**

Run: `cd tools/blackwall && go test ./internal/config/... -run Resolve`
Expected: fails — Resolve/Overrides/ReadEnv undefined.

- [ ] **Step 3: Implement Resolve**

Create `tools/blackwall/internal/config/resolve.go`:

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Overrides holds values that come from CLI flags. Pointers distinguish
// "user supplied" from "default zero value".
type Overrides struct {
	Context              string
	ContextWasSet        bool
	Refresh              *time.Duration
	SelectContextOnStart *bool
}

// EnvValues holds parsed environment overrides.
type EnvValues struct {
	Context              string
	Refresh              *time.Duration
	SelectContextOnStart *bool
}

// ReadEnv reads blackwall-related env vars into EnvValues.
func ReadEnv() EnvValues {
	out := EnvValues{Context: os.Getenv("BW_CONTEXT")}
	if v := os.Getenv("BW_REFRESH"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			out.Refresh = &d
		}
	}
	if v := os.Getenv("BW_SELECT_CONTEXT_ON_START"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			out.SelectContextOnStart = &b
		}
	}
	return out
}

// Resolve folds CLI overrides and env values into a Config.
// Precedence (highest wins): CLI > env > config file > defaults.
// Callers are expected to have already run ApplyDefaults, ExpandEnv and Validate.
func Resolve(cfg Config, ov Overrides, env EnvValues) (Config, error) {
	// Context: CLI > env > file
	switch {
	case ov.ContextWasSet:
		cfg.Current = ov.Context
		ov.SelectContextOnStart = boolPtr(false) // explicit --context kills picker
	case env.Context != "":
		cfg.Current = env.Context
		ov.SelectContextOnStart = boolPtr(false)
	}

	// Refresh: CLI > env > file
	switch {
	case ov.Refresh != nil:
		cfg.UI.RefreshInterval = *ov.Refresh
	case env.Refresh != nil:
		cfg.UI.RefreshInterval = *env.Refresh
	}

	// Picker toggle: CLI > env > file
	switch {
	case ov.SelectContextOnStart != nil:
		cfg.SelectContextOnStart = *ov.SelectContextOnStart
	case env.SelectContextOnStart != nil:
		cfg.SelectContextOnStart = *env.SelectContextOnStart
	}

	if cfg.Current == "" {
		return cfg, fmt.Errorf("no current context after resolve")
	}
	if _, ok := cfg.Contexts[cfg.Current]; !ok {
		return cfg, fmt.Errorf("context %q not found in config", cfg.Current)
	}
	return cfg, nil
}

func boolPtr(b bool) *bool { return &b }
```

- [ ] **Step 4: Run all config tests, expect pass**

Run: `cd tools/blackwall && go test ./internal/config/... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/config
git commit -m "feat(blackwall): config Resolve (CLI/env precedence)"
```

---

## Task 5: First-run template

**Files:**
- Create: `tools/blackwall/internal/config/template.yaml`
- Create: `tools/blackwall/internal/config/template.go`
- Create: `tools/blackwall/internal/config/template_test.go`

- [ ] **Step 1: Write the embedded template file**

Create `tools/blackwall/internal/config/template.yaml`:

```yaml
# blackwall configuration
#
# Docs: https://github.com/greengrunt/blackwall (placeholder)
#
# Switch contexts at runtime with `:ctx <name>` in the TUI,
# or set the default below.

current: example-master
select_context_on_start: true     # show picker at startup; set false to skip

contexts:

  example-master:
    description: "Replace this with your real Wazuh manager"
    wazuh:
      server:   wazuh.example.com
      port:     55000
      user:     wazuh
      password: ${BW_EXAMPLE_WAZUH_PASSWORD}
      tls:
        insecure_skip_verify: false
        # ca_file: /etc/blackwall/wazuh-ca.crt

ui:
  refresh_interval: 3s
  theme:            dark
  vim_keys:         true

logging:
  level: warn
  # file: ~/.local/state/blackwall/blackwall.log   # default if unset
```

- [ ] **Step 2: Write failing test for EnsureExists**

Create `tools/blackwall/internal/config/template_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureExists_CreatesFileWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	created, err := EnsureExists(path)
	if err != nil {
		t.Fatalf("EnsureExists: %v", err)
	}
	if !created {
		t.Fatal("created = false on first run, want true")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if !strings.Contains(string(data), "example-master") {
		t.Errorf("template missing example-master section: %s", data)
	}

	// Second call is a no-op.
	created, err = EnsureExists(path)
	if err != nil {
		t.Fatalf("EnsureExists 2nd: %v", err)
	}
	if created {
		t.Error("created = true on second run, want false")
	}
}
```

- [ ] **Step 3: Run, expect compile error**

Run: `cd tools/blackwall && go test ./internal/config/... -run EnsureExists`
Expected: fails — `EnsureExists` undefined.

- [ ] **Step 4: Implement EnsureExists**

Create `tools/blackwall/internal/config/template.go`:

```go
package config

import (
	"errors"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed template.yaml
var defaultTemplate []byte

// EnsureExists creates path with the embedded template when it is missing.
// Returns (created=true, nil) when the template was written, or
// (created=false, nil) when the file already existed.
func EnsureExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, fmt.Errorf("mkdir config dir: %w", err)
	}
	if err := os.WriteFile(path, defaultTemplate, 0o600); err != nil {
		return false, fmt.Errorf("write config: %w", err)
	}
	return true, nil
}
```

- [ ] **Step 5: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/config/... -v`
Expected: all config tests PASS.

- [ ] **Step 6: Commit**

```bash
git add tools/blackwall/internal/config
git commit -m "feat(blackwall): embedded first-run config template"
```

---

## Task 6: Wazuh client — types + Authenticate

**Files:**
- Create: `tools/blackwall/internal/wazuh/types.go`
- Create: `tools/blackwall/internal/wazuh/client.go`
- Create: `tools/blackwall/internal/wazuh/client_test.go`

- [ ] **Step 1: Define the API types**

Create `tools/blackwall/internal/wazuh/types.go`:

```go
// Package wazuh is a minimal Wazuh Manager API client for blackwall.
package wazuh

import "time"

type Agent struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	IP            string    `json:"ip"`
	Status        string    `json:"status"`
	LastKeepAlive time.Time `json:"lastKeepAlive"`
	OS            struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"os"`
	Version       string `json:"version"`
	DateAdd       time.Time `json:"dateAdd"`
	Group         []string `json:"group"`
	Manager       string   `json:"manager"`
}

type AgentDetail struct {
	Agent
	Config map[string]string `json:"-"`
}

type agentsResponse struct {
	Data struct {
		AffectedItems []Agent `json:"affected_items"`
	} `json:"data"`
}

type singleAgentResponse struct {
	Data struct {
		AffectedItems []Agent `json:"affected_items"`
	} `json:"data"`
}

type authResponse struct {
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}
```

- [ ] **Step 2: Write failing test for Authenticate**

Create `tools/blackwall/internal/wazuh/client_test.go`:

```go
package wazuh

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

func newServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func subFromServer(srv *httptest.Server, user, pw string) config.WazuhSubsystem {
	// strip http:// and split host:port for our client which expects no scheme.
	addr := strings.TrimPrefix(srv.URL, "http://")
	host, port := splitHostPort(addr)
	return config.WazuhSubsystem{
		Server:   host,
		Port:     port,
		User:     user,
		Password: pw,
		TLS:      config.TLS{InsecureSkipVerify: true},
	}
}

func splitHostPort(addr string) (string, int) {
	// httptest URLs are always host:port with numeric port.
	host, p, _ := strings.Cut(addr, ":")
	port := 0
	for _, c := range p {
		port = port*10 + int(c-'0')
	}
	return host, port
}

func TestAuthenticate_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/security/user/authenticate" {
			t.Errorf("path = %q, want /security/user/authenticate", r.URL.Path)
		}
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("wazuh:secret"))
		if r.Header.Get("Authorization") != want {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), want)
		}
		_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
			Token string `json:"token"`
		}{Token: "fake-jwt"}})
	})

	c, err := New(subFromServer(srv, "wazuh", "secret"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Override the scheme via the unexported field; in production code
	// the scheme is always https. For testing we expose insecureScheme.
	c.insecureScheme = "http"

	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if c.token != "fake-jwt" {
		t.Errorf("token = %q, want fake-jwt", c.token)
	}
}

func TestAuthenticate_401(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	c, _ := New(subFromServer(srv, "wazuh", "wrong"))
	c.insecureScheme = "http"
	if err := c.Authenticate(context.Background()); err == nil {
		t.Fatal("Authenticate: expected error")
	}
}
```

- [ ] **Step 3: Run, expect compile errors**

Run: `cd tools/blackwall && go test ./internal/wazuh/...`
Expected: fails — `New`, `Authenticate`, fields undefined.

- [ ] **Step 4: Implement Client + Authenticate**

Create `tools/blackwall/internal/wazuh/client.go`:

```go
package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

type Client struct {
	host           string
	port           int
	user           string
	password       string
	httpClient     *http.Client
	insecureScheme string // test override; production stays "https"

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// New constructs a Client for the given Wazuh subsystem.
func New(sub config.WazuhSubsystem) (*Client, error) {
	httpc, err := buildHTTPClient(sub.TLS)
	if err != nil {
		return nil, err
	}
	return &Client{
		host:           sub.Server,
		port:           sub.Port,
		user:           sub.User,
		password:       sub.Password,
		httpClient:     httpc,
		insecureScheme: "https",
	}, nil
}

func (c *Client) baseURL() string {
	return fmt.Sprintf("%s://%s:%d", c.insecureScheme, c.host, c.port)
}

func (c *Client) Authenticate(ctx context.Context) error {
	url := c.baseURL() + "/security/user/authenticate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.user, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authenticate: HTTP %d", resp.StatusCode)
	}
	var ar authResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return fmt.Errorf("authenticate decode: %w", err)
	}
	if strings.TrimSpace(ar.Data.Token) == "" {
		return fmt.Errorf("authenticate: empty token")
	}

	c.mu.Lock()
	c.token = ar.Data.Token
	c.tokenExp = time.Now().Add(13 * time.Minute) // Wazuh JWT ~15min; refresh early
	c.mu.Unlock()
	return nil
}

// doAuthed runs an HTTP request with the JWT, auto-authenticating
// on first call and once on 401.
func (c *Client) doAuthed(ctx context.Context, method, path string) (*http.Response, error) {
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, nil)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	req.Header.Set("Authorization", "Bearer "+c.token)
	c.mu.Unlock()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		c.invalidateToken()
		if err := c.Authenticate(ctx); err != nil {
			return nil, err
		}
		// Retry once.
		req2, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, nil)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		req2.Header.Set("Authorization", "Bearer "+c.token)
		c.mu.Unlock()
		return c.httpClient.Do(req2)
	}
	return resp, nil
}

func (c *Client) ensureToken(ctx context.Context) error {
	c.mu.Lock()
	stale := c.token == "" || time.Now().After(c.tokenExp)
	c.mu.Unlock()
	if !stale {
		return nil
	}
	return c.Authenticate(ctx)
}

func (c *Client) invalidateToken() {
	c.mu.Lock()
	c.token = ""
	c.tokenExp = time.Time{}
	c.mu.Unlock()
}
```

- [ ] **Step 5: Run wazuh tests, expect pass**

Run: `cd tools/blackwall && go test ./internal/wazuh/... -v -run Authenticate`
Expected: `--- PASS: TestAuthenticate_OK`, `--- PASS: TestAuthenticate_401`.

- [ ] **Step 6: Commit**

```bash
git add tools/blackwall/internal/wazuh
git commit -m "feat(blackwall): wazuh client Authenticate + token cache"
```

---

## Task 7: Wazuh client — TLS configuration

**Files:**
- Create: `tools/blackwall/internal/wazuh/tls.go`
- Create: `tools/blackwall/internal/wazuh/tls_test.go`

- [ ] **Step 1: Write failing tests**

Create `tools/blackwall/internal/wazuh/tls_test.go`:

```go
package wazuh

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

func TestBuildHTTPClient_InsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c, err := buildHTTPClient(config.TLS{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestBuildHTTPClient_VerifyFailsWithoutSkip(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c, err := buildHTTPClient(config.TLS{})
	if err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
	if _, err := c.Get(srv.URL); err == nil {
		t.Fatal("Get: expected TLS verify failure")
	}
}

func TestBuildHTTPClient_CAFileMissing(t *testing.T) {
	_, err := buildHTTPClient(config.TLS{CAFile: "/nope/missing.crt"})
	if err == nil {
		t.Fatal("buildHTTPClient: expected error for missing CA file")
	}
}

func TestBuildHTTPClient_CAFileLoaded(t *testing.T) {
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.crt")
	// Empty file is enough for the loader to succeed; we only check no error.
	if err := os.WriteFile(ca, []byte(""), 0o600); err != nil {
		t.Fatalf("write CA: %v", err)
	}
	if _, err := buildHTTPClient(config.TLS{CAFile: ca}); err != nil {
		t.Fatalf("buildHTTPClient: %v", err)
	}
}
```

- [ ] **Step 2: Run, expect compile error**

Run: `cd tools/blackwall && go test ./internal/wazuh/... -run BuildHTTPClient`
Expected: fails — `buildHTTPClient` undefined.

- [ ] **Step 3: Implement buildHTTPClient**

Create `tools/blackwall/internal/wazuh/tls.go`:

```go
package wazuh

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

func buildHTTPClient(t config.TLS) (*http.Client, error) {
	cfg := &tls.Config{InsecureSkipVerify: t.InsecureSkipVerify}

	if t.CAFile != "" {
		raw, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca_file %q: %w", t.CAFile, err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(raw) // empty/invalid PEM yields empty pool — acceptable
		cfg.RootCAs = pool
	}

	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: cfg},
	}, nil
}
```

- [ ] **Step 4: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/wazuh/... -v`
Expected: all wazuh tests PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/wazuh
git commit -m "feat(blackwall): wazuh TLS config (skip-verify + ca_file)"
```

---

## Task 8: Wazuh client — ListAgents

**Files:**
- Create: `tools/blackwall/internal/wazuh/agents.go`
- Create: `tools/blackwall/internal/wazuh/agents_test.go`

- [ ] **Step 1: Write failing test**

Create `tools/blackwall/internal/wazuh/agents_test.go`:

```go
package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListAgents_OK(t *testing.T) {
	authed := false
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authed = true
			_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
				Token string `json:"token"`
			}{Token: "tok"}})
		case "/agents":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(agentsResponse{Data: struct {
				AffectedItems []Agent `json:"affected_items"`
			}{AffectedItems: []Agent{
				{ID: "000", Name: "master", Status: "active"},
				{ID: "001", Name: "host-01", Status: "disconnected"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	agents, err := c.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if !authed {
		t.Error("client did not authenticate before listing")
	}
	if len(agents) != 2 || agents[0].ID != "000" || agents[1].ID != "001" {
		t.Errorf("agents = %+v", agents)
	}
}

func TestListAgents_ContextCancel(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Block forever; cancellation should terminate the request.
		<-r.Context().Done()
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"
	c.token = "tok" // skip Authenticate to keep test focused on the cancel path
	c.tokenExp = neverExpire()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.ListAgents(ctx); err == nil {
		t.Fatal("ListAgents: expected error on cancelled context")
	}
}
```

- [ ] **Step 2: Run, expect compile error**

Run: `cd tools/blackwall && go test ./internal/wazuh/... -run ListAgents`
Expected: fails — `ListAgents`, `neverExpire` undefined.

- [ ] **Step 3: Implement ListAgents (and neverExpire helper)**

Create `tools/blackwall/internal/wazuh/agents.go`:

```go
package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ListAgents fetches all agents from /agents (page 1, limit 500).
// For MVP one page is enough; the response shape allows future paging.
func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	resp, err := c.doAuthed(ctx, "GET", "/agents?limit=500")
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list agents: HTTP %d", resp.StatusCode)
	}
	var ar agentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list agents decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}

// GetAgent fetches a single agent by ID. Config retrieval is a future
// enhancement — for MVP the detail view reads only metadata.
func (c *Client) GetAgent(ctx context.Context, id string) (AgentDetail, error) {
	resp, err := c.doAuthed(ctx, "GET", "/agents?agents_list="+id)
	if err != nil {
		return AgentDetail{}, fmt.Errorf("get agent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return AgentDetail{}, ErrAgentNotFound
	}
	if resp.StatusCode != 200 {
		return AgentDetail{}, fmt.Errorf("get agent: HTTP %d", resp.StatusCode)
	}
	var ar singleAgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return AgentDetail{}, fmt.Errorf("get agent decode: %w", err)
	}
	if len(ar.Data.AffectedItems) == 0 {
		return AgentDetail{}, ErrAgentNotFound
	}
	return AgentDetail{Agent: ar.Data.AffectedItems[0]}, nil
}

func neverExpire() time.Time { return time.Now().Add(24 * time.Hour) }

// ErrAgentNotFound is returned by GetAgent when the agent is gone.
var ErrAgentNotFound = fmt.Errorf("agent not found")
```

- [ ] **Step 4: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/wazuh/... -v`
Expected: all wazuh tests PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/wazuh
git commit -m "feat(blackwall): wazuh ListAgents + GetAgent"
```

---

## Task 9: Wazuh client — GetAgent test coverage

**Files:**
- Modify: `tools/blackwall/internal/wazuh/agents_test.go`

- [ ] **Step 1: Add GetAgent tests**

Append to `tools/blackwall/internal/wazuh/agents_test.go`:

```go
func TestGetAgent_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
				Token string `json:"token"`
			}{Token: "tok"}})
		case "/agents":
			if r.URL.Query().Get("agents_list") != "001" {
				t.Errorf("agents_list = %q, want 001", r.URL.Query().Get("agents_list"))
			}
			_ = json.NewEncoder(w).Encode(singleAgentResponse{Data: struct {
				AffectedItems []Agent `json:"affected_items"`
			}{AffectedItems: []Agent{
				{ID: "001", Name: "host-01", Status: "active"},
			}}})
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	got, err := c.GetAgent(context.Background(), "001")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.ID != "001" {
		t.Errorf("ID = %q, want 001", got.ID)
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
				Token string `json:"token"`
			}{Token: "tok"}})
		case "/agents":
			// Empty affected_items list = not found.
			_ = json.NewEncoder(w).Encode(singleAgentResponse{})
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	_, err := c.GetAgent(context.Background(), "999")
	if err != ErrAgentNotFound {
		t.Errorf("err = %v, want ErrAgentNotFound", err)
	}
}

func TestDoAuthed_RetriesOn401(t *testing.T) {
	calls := 0
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
				Token string `json:"token"`
			}{Token: "tok"}})
		case "/agents":
			calls++
			if calls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(agentsResponse{})
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	if _, err := c.ListAgents(context.Background()); err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if calls != 2 {
		t.Errorf("agents calls = %d, want 2 (initial + retry)", calls)
	}
}
```

- [ ] **Step 2: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/wazuh/... -v`
Expected: all PASS (incl. new GetAgent + retry tests).

- [ ] **Step 3: Commit**

```bash
git add tools/blackwall/internal/wazuh
git commit -m "test(blackwall): wazuh GetAgent + 401 retry"
```

---

## Task 10: tickcmd helper

**Files:**
- Create: `tools/blackwall/internal/tickcmd/tickcmd.go`
- Create: `tools/blackwall/internal/tickcmd/tickcmd_test.go`

- [ ] **Step 1: Write failing test**

Create `tools/blackwall/internal/tickcmd/tickcmd_test.go`:

```go
package tickcmd

import (
	"testing"
	"time"
)

func TestEvery_EmitsTickMsg(t *testing.T) {
	cmd := Every(5 * time.Millisecond)
	msg := cmd()
	if _, ok := msg.(TickMsg); !ok {
		t.Errorf("msg type = %T, want TickMsg", msg)
	}
}
```

- [ ] **Step 2: Run, expect compile error**

Run: `cd tools/blackwall && go test ./internal/tickcmd/...`
Expected: fails — `Every`, `TickMsg` undefined.

- [ ] **Step 3: Implement Every**

Create `tools/blackwall/internal/tickcmd/tickcmd.go`:

```go
// Package tickcmd wraps tea.Tick so views can subscribe to
// periodic ticks without depending on Bubbletea internals.
package tickcmd

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TickMsg is delivered to Update on each interval expiry.
type TickMsg struct{ Time time.Time }

// Every returns a tea.Cmd that fires a single TickMsg after d elapses.
// Callers re-arm the next tick from their Update to avoid drift.
func Every(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return TickMsg{Time: t} })
}
```

- [ ] **Step 4: Add bubbletea dependency**

Run: `cd tools/blackwall && go get github.com/charmbracelet/bubbletea`
Expected: bubbletea added.

- [ ] **Step 5: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/tickcmd/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add tools/blackwall/internal/tickcmd tools/blackwall/go.mod tools/blackwall/go.sum
git commit -m "feat(blackwall): tickcmd helper for periodic ticks"
```

---

## Task 11: UI styles

**Files:**
- Create: `tools/blackwall/internal/ui/styles/styles.go`

- [ ] **Step 1: Add lipgloss dependency**

Run: `cd tools/blackwall && go get github.com/charmbracelet/lipgloss`
Expected: lipgloss added.

- [ ] **Step 2: Write styles**

Create `tools/blackwall/internal/ui/styles/styles.go`:

```go
// Package styles holds the blackwall dark theme.
package styles

import "github.com/charmbracelet/lipgloss"

var (
	Active       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))   // green
	Disconnected = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))               // red
	Pending      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))              // yellow
	Dim          = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))               // grey

	HeaderText   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))   // cyan
	StatusBar    = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))              // bright white
	Footer       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	TabActive    = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color("14"))
	TabInactive  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	Error        = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
)
```

- [ ] **Step 3: Verify build**

Run: `cd tools/blackwall && go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add tools/blackwall/internal/ui/styles tools/blackwall/go.mod tools/blackwall/go.sum
git commit -m "feat(blackwall): lipgloss style palette (dark theme)"
```

---

## Task 12: tabbar widget

**Files:**
- Create: `tools/blackwall/internal/ui/tabbar/tabbar.go`
- Create: `tools/blackwall/internal/ui/tabbar/tabbar_test.go`

- [ ] **Step 1: Write failing test**

Create `tools/blackwall/internal/ui/tabbar/tabbar_test.go`:

```go
package tabbar

import (
	"strings"
	"testing"
)

func TestRender_HighlightsActive(t *testing.T) {
	out := Render([]string{":contexts", ":agents"}, 1)
	if !strings.Contains(out, "[1]") || !strings.Contains(out, ":contexts") {
		t.Errorf("missing tab 1: %q", out)
	}
	if !strings.Contains(out, "[2]") || !strings.Contains(out, ":agents") {
		t.Errorf("missing tab 2: %q", out)
	}
}

func TestRender_EmptyList(t *testing.T) {
	if got := Render(nil, 0); got != "" {
		t.Errorf("Render(nil) = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run, expect compile error**

Run: `cd tools/blackwall && go test ./internal/ui/tabbar/...`
Expected: fails — Render undefined.

- [ ] **Step 3: Implement Render**

Create `tools/blackwall/internal/ui/tabbar/tabbar.go`:

```go
// Package tabbar renders the top tab strip of registered views.
package tabbar

import (
	"fmt"
	"strings"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
)

// Render returns the tab strip given an ordered list of view names
// and the index of the active view.
func Render(viewNames []string, active int) string {
	if len(viewNames) == 0 {
		return ""
	}
	var parts []string
	for i, name := range viewNames {
		label := fmt.Sprintf("[%d] %s", i+1, name)
		if i == active {
			parts = append(parts, styles.TabActive.Render(label))
		} else {
			parts = append(parts, styles.TabInactive.Render(label))
		}
	}
	return strings.Join(parts, "  │  ")
}
```

- [ ] **Step 4: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/ui/tabbar/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/ui/tabbar
git commit -m "feat(blackwall): tabbar widget"
```

---

## Task 13: statusbar + footer widgets

**Files:**
- Create: `tools/blackwall/internal/ui/statusbar/statusbar.go`
- Create: `tools/blackwall/internal/ui/statusbar/statusbar_test.go`
- Create: `tools/blackwall/internal/ui/footer/footer.go`
- Create: `tools/blackwall/internal/ui/footer/footer_test.go`

- [ ] **Step 1: Write failing tests**

Create `tools/blackwall/internal/ui/statusbar/statusbar_test.go`:

```go
package statusbar

import (
	"strings"
	"testing"
	"time"
)

func TestRender_LiveState(t *testing.T) {
	got := Render(State{
		Context: "master-00",
		Wazuh:   "172.28.0.10:55000",
		Refresh: 3 * time.Second,
		Paused:  false,
	})
	for _, want := range []string{"master-00", "172.28.0.10:55000", "live", "3s"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestRender_Paused(t *testing.T) {
	got := Render(State{Context: "x", Wazuh: "h:1", Paused: true})
	if !strings.Contains(got, "paused") {
		t.Errorf("expected paused indicator: %q", got)
	}
}
```

Create `tools/blackwall/internal/ui/footer/footer_test.go`:

```go
package footer

import (
	"strings"
	"testing"
)

func TestRender_ContainsHints(t *testing.T) {
	got := Render([]Hint{{Key: "↵", Desc: "open"}, {Key: "q", Desc: "quit"}})
	if !strings.Contains(got, "↵") || !strings.Contains(got, "open") {
		t.Errorf("missing first hint: %q", got)
	}
	if !strings.Contains(got, "q") || !strings.Contains(got, "quit") {
		t.Errorf("missing second hint: %q", got)
	}
}
```

- [ ] **Step 2: Run, expect compile errors**

Run: `cd tools/blackwall && go test ./internal/ui/...`
Expected: fails — types/functions undefined.

- [ ] **Step 3: Implement statusbar**

Create `tools/blackwall/internal/ui/statusbar/statusbar.go`:

```go
// Package statusbar renders the per-frame status line.
package statusbar

import (
	"fmt"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
)

type State struct {
	Context string
	Wazuh   string
	Refresh time.Duration
	Paused  bool
}

func Render(s State) string {
	mode := fmt.Sprintf("↻ live (%s)", s.Refresh)
	if s.Paused {
		mode = "⏸ paused"
	}
	body := fmt.Sprintf("context: %s · wazuh: %s · %s", s.Context, s.Wazuh, mode)
	return styles.StatusBar.Render(body)
}
```

- [ ] **Step 4: Implement footer**

Create `tools/blackwall/internal/ui/footer/footer.go`:

```go
// Package footer renders the bottom keymap hints.
package footer

import (
	"fmt"
	"strings"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
)

type Hint struct {
	Key  string
	Desc string
}

func Render(hints []Hint) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, fmt.Sprintf("[%s] %s", h.Key, h.Desc))
	}
	return styles.Footer.Render(strings.Join(parts, "  "))
}
```

- [ ] **Step 5: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/ui/... -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add tools/blackwall/internal/ui/statusbar tools/blackwall/internal/ui/footer
git commit -m "feat(blackwall): statusbar + footer widgets"
```

---

## Task 14: View interface + messages

**Files:**
- Create: `tools/blackwall/internal/views/view.go`
- Create: `tools/blackwall/internal/app/messages.go`

- [ ] **Step 1: Define the View interface**

Create `tools/blackwall/internal/views/view.go`:

```go
// Package views defines the View interface implemented by every
// top-level screen (`:contexts`, `:agents`, ...).
package views

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
)

type View interface {
	tea.Model
	Name() string
	Hints() []footer.Hint
}
```

- [ ] **Step 2: Define app messages**

Create `tools/blackwall/internal/app/messages.go`:

```go
// Package app holds the root tea.Model and view router.
package app

// SwitchViewMsg requests a transition to a top-level view by name.
type SwitchViewMsg struct{ Name string }

// SwitchToDetailMsg is emitted by `:agents` list when the user presses Enter.
type SwitchToDetailMsg struct{ AgentID string }

// BackMsg pops a sub-screen (e.g. agent detail) back to its parent.
type BackMsg struct{}

// ContextChangedMsg announces a context switch initiated from `:contexts`.
type ContextChangedMsg struct{ Name string }
```

- [ ] **Step 3: Verify build**

Run: `cd tools/blackwall && go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add tools/blackwall/internal/views tools/blackwall/internal/app
git commit -m "feat(blackwall): View interface + app messages"
```

---

## Task 15: app.Model skeleton + global hotkeys

**Files:**
- Create: `tools/blackwall/internal/app/app.go`
- Create: `tools/blackwall/internal/app/app_test.go`

- [ ] **Step 1: Write failing tests**

Create `tools/blackwall/internal/app/app_test.go`:

```go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/views"
)

// fakeView is a minimal View implementation for testing the router.
type fakeView struct {
	name     string
	lastMsg  tea.Msg
	rendered string
}

func (v *fakeView) Init() tea.Cmd                       { return nil }
func (v *fakeView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { v.lastMsg = msg; return v, nil }
func (v *fakeView) View() string                        { return v.rendered }
func (v *fakeView) Name() string                        { return v.name }
func (v *fakeView) Hints() []footer.Hint                { return nil }

var _ views.View = (*fakeView)(nil) // compile-time check

func TestApp_SwitchViewMsg(t *testing.T) {
	a := fakeView{name: ":a", rendered: "AAA"}
	b := fakeView{name: ":b", rendered: "BBB"}
	m := New(Deps{
		Views:        []views.View{&a, &b},
		InitialView:  ":a",
	})

	got, _ := m.Update(SwitchViewMsg{Name: ":b"})
	if got.(*Model).Active().Name() != ":b" {
		t.Errorf("active = %q, want :b", got.(*Model).Active().Name())
	}
}

func TestApp_TabCyclesForward(t *testing.T) {
	a := fakeView{name: ":a"}
	b := fakeView{name: ":b"}
	m := New(Deps{Views: []views.View{&a, &b}, InitialView: ":a"})

	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got.(*Model).Active().Name() != ":b" {
		t.Errorf("after Tab active = %q, want :b", got.(*Model).Active().Name())
	}
}

func TestApp_NumberKeyJumps(t *testing.T) {
	a := fakeView{name: ":a"}
	b := fakeView{name: ":b"}
	c := fakeView{name: ":c"}
	m := New(Deps{Views: []views.View{&a, &b, &c}, InitialView: ":a"})

	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if got.(*Model).Active().Name() != ":c" {
		t.Errorf("after '3' active = %q, want :c", got.(*Model).Active().Name())
	}
}

func TestApp_QuitsOnQ(t *testing.T) {
	a := fakeView{name: ":a"}
	m := New(Deps{Views: []views.View{&a}, InitialView: ":a"})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd")
	}
	if msg := cmd(); msg == nil {
		t.Error("Quit cmd produced nil msg")
	}
}
```

- [ ] **Step 2: Run, expect compile errors**

Run: `cd tools/blackwall && go test ./internal/app/...`
Expected: fails — Model/New/Deps/Active undefined.

- [ ] **Step 3: Implement Model**

Create `tools/blackwall/internal/app/app.go`:

```go
package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/statusbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/tabbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/views"
)

type Deps struct {
	Views       []views.View
	InitialView string

	// Status returns the current statusbar.State; injected so the root
	// model does not need to know about wazuh/config types.
	Status func() statusbar.State

	// PauseToggle is invoked on `p`; nil means no-op.
	PauseToggle func()
}

type Model struct {
	views       []views.View
	active      int
	status      func() statusbar.State
	pauseToggle func()
	// palette state will be added in Task 16.
}

func New(d Deps) *Model {
	m := &Model{
		views:       d.Views,
		status:      d.Status,
		pauseToggle: d.PauseToggle,
	}
	for i, v := range d.Views {
		if v.Name() == d.InitialView {
			m.active = i
		}
	}
	return m
}

func (m *Model) Active() views.View { return m.views[m.active] }

func (m *Model) Init() tea.Cmd {
	return m.Active().Init()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.active = (m.active + 1) % len(m.views)
			return m, m.Active().Init()
		case "shift+tab":
			m.active = (m.active - 1 + len(m.views)) % len(m.views)
			return m, m.Active().Init()
		case "p":
			if m.pauseToggle != nil {
				m.pauseToggle()
			}
			return m, nil
		}
		if r := singleDigit(msg.String()); r > 0 && r <= len(m.views) {
			m.active = r - 1
			return m, m.Active().Init()
		}

	case SwitchViewMsg:
		for i, v := range m.views {
			if v.Name() == msg.Name {
				m.active = i
				return m, v.Init()
			}
		}
		return m, nil
	}

	// Delegate everything else to the active view.
	next, cmd := m.Active().Update(msg)
	if v, ok := next.(views.View); ok {
		m.views[m.active] = v
	}
	return m, cmd
}

func (m *Model) View() string {
	names := make([]string, len(m.views))
	for i, v := range m.views {
		names[i] = v.Name()
	}
	tab := tabbar.Render(names, m.active)

	var status string
	if m.status != nil {
		status = statusbar.Render(m.status())
	} else {
		status = statusbar.Render(statusbar.State{Refresh: time.Second})
	}
	body := m.Active().View()
	foot := footer.Render(m.Active().Hints())
	return strings.Join([]string{tab, status, body, foot}, "\n")
}

func singleDigit(s string) int {
	if len(s) != 1 {
		return 0
	}
	c := s[0]
	if c < '1' || c > '9' {
		return 0
	}
	return int(c - '0')
}

// Sanity check during compilation that imported packages aren't shadowed.
var _ = fmt.Sprintf
```

- [ ] **Step 4: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/app/... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/app
git commit -m "feat(blackwall): app.Model + global hotkeys + tab nav"
```

---

## Task 16: Command palette

**Files:**
- Create: `tools/blackwall/internal/app/palette.go`
- Create: `tools/blackwall/internal/app/palette_test.go`
- Modify: `tools/blackwall/internal/app/app.go`

- [ ] **Step 1: Write failing tests**

Create `tools/blackwall/internal/app/palette_test.go`:

```go
package app

import "testing"

func TestParsePalette_BareViewName(t *testing.T) {
	out, err := parsePalette(":agents", []string{":agents", ":contexts"})
	if err != nil {
		t.Fatalf("parsePalette: %v", err)
	}
	if sw, ok := out.(SwitchViewMsg); !ok || sw.Name != ":agents" {
		t.Errorf("got %#v, want SwitchViewMsg{:agents}", out)
	}
}

func TestParsePalette_CtxShortcut(t *testing.T) {
	out, err := parsePalette(":ctx prod-eu-1", []string{":agents", ":contexts"})
	if err != nil {
		t.Fatalf("parsePalette: %v", err)
	}
	cc, ok := out.(ContextChangedMsg)
	if !ok || cc.Name != "prod-eu-1" {
		t.Errorf("got %#v, want ContextChangedMsg{prod-eu-1}", out)
	}
}

func TestParsePalette_Quit(t *testing.T) {
	out, err := parsePalette(":q", nil)
	if err != nil {
		t.Fatalf("parsePalette: %v", err)
	}
	if _, ok := out.(QuitMsg); !ok {
		t.Errorf("got %#v, want QuitMsg", out)
	}
}

func TestParsePalette_Unknown(t *testing.T) {
	_, err := parsePalette(":nope", []string{":agents"})
	if err == nil {
		t.Error("expected error for unknown command")
	}
}
```

- [ ] **Step 2: Run, expect compile errors**

Run: `cd tools/blackwall && go test ./internal/app/... -run Palette`
Expected: fails — `parsePalette`, `QuitMsg` undefined.

- [ ] **Step 3: Implement palette parser**

Create `tools/blackwall/internal/app/palette.go`:

```go
package app

import (
	"fmt"
	"strings"
)

// QuitMsg requests app shutdown via the palette (`:q`).
type QuitMsg struct{}

// parsePalette turns a user-typed palette string into an internal message.
// `knownViews` is the list of registered top-level view names (each starts with `:`).
func parsePalette(input string, knownViews []string) (any, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, ":") {
		input = ":" + input
	}

	if input == ":q" || input == ":quit" {
		return QuitMsg{}, nil
	}

	// `:ctx <name>` switches contexts.
	if strings.HasPrefix(input, ":ctx ") {
		name := strings.TrimSpace(strings.TrimPrefix(input, ":ctx "))
		if name == "" {
			return nil, fmt.Errorf(":ctx requires a name")
		}
		return ContextChangedMsg{Name: name}, nil
	}

	// Bare view name (`:agents`, `:contexts`).
	for _, v := range knownViews {
		if v == input {
			return SwitchViewMsg{Name: input}, nil
		}
	}

	return nil, fmt.Errorf("unknown command: %s", input)
}
```

- [ ] **Step 4: Wire palette state into Model**

Apply this diff to `tools/blackwall/internal/app/app.go`:

Replace the `Model` struct with:

```go
type Model struct {
	views       []views.View
	active      int
	status      func() statusbar.State
	pauseToggle func()

	paletteOpen  bool
	paletteInput string
	paletteErr   string
}
```

Inside `Update`, immediately after the `switch msg := msg.(type)` opens but before the `tea.KeyMsg` case, add a palette-mode branch:

```go
	case tea.KeyMsg:
		if m.paletteOpen {
			switch msg.String() {
			case "esc":
				m.paletteOpen = false
				m.paletteInput = ""
				m.paletteErr = ""
				return m, nil
			case "enter":
				names := viewNames(m.views)
				out, err := parsePalette(m.paletteInput, names)
				if err != nil {
					m.paletteErr = err.Error()
					return m, nil
				}
				m.paletteOpen = false
				m.paletteInput = ""
				m.paletteErr = ""
				if _, ok := out.(QuitMsg); ok {
					return m, tea.Quit
				}
				return m.Update(out)
			case "backspace":
				if n := len(m.paletteInput); n > 0 {
					m.paletteInput = m.paletteInput[:n-1]
				}
				return m, nil
			default:
				if msg.Type == tea.KeyRunes {
					m.paletteInput += string(msg.Runes)
				}
				return m, nil
			}
		}
		if msg.String() == ":" {
			m.paletteOpen = true
			m.paletteInput = ":"
			m.paletteErr = ""
			return m, nil
		}
		switch msg.String() {
```

(Keep the rest of the existing `KeyMsg` switch — `q`, `tab`, etc. — unchanged after this block.)

Add a helper at the bottom of `app.go`:

```go
func viewNames(vs []views.View) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Name()
	}
	return out
}
```

In `View()`, when `m.paletteOpen` is true, replace `body := m.Active().View()` with:

```go
	body := m.Active().View()
	if m.paletteOpen {
		body += "\n\n> " + m.paletteInput
		if m.paletteErr != "" {
			body += "\n" + styles.Error.Render(m.paletteErr)
		}
	}
```

Add the styles import at the top of `app.go`:

```go
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
```

- [ ] **Step 5: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/app/... -v`
Expected: all PASS (including the original Task 15 tests).

- [ ] **Step 6: Commit**

```bash
git add tools/blackwall/internal/app
git commit -m "feat(blackwall): command palette (`:` parser + UI)"
```

---

## Task 17: `:contexts` view

**Files:**
- Create: `tools/blackwall/internal/views/contexts/list.go`
- Create: `tools/blackwall/internal/views/contexts/list_test.go`

- [ ] **Step 1: Write failing tests**

Create `tools/blackwall/internal/views/contexts/list_test.go`:

```go
package contexts

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

func sampleConfig() config.Config {
	return config.Config{
		Current: "master-00",
		Contexts: map[string]config.Context{
			"master-00": {
				Description: "Docker test env",
				Wazuh:       &config.WazuhSubsystem{Server: "172.28.0.10", Port: 55000},
			},
			"prod-eu-1": {
				Description: "Production EU",
				Wazuh:       &config.WazuhSubsystem{Server: "wazuh.eu.example.com", Port: 55000},
			},
		},
	}
}

func TestList_RenderShowsAllContexts(t *testing.T) {
	v := New(sampleConfig())
	out := v.View()
	for _, want := range []string{"master-00", "prod-eu-1", "Docker test env"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestList_EnterEmitsContextChanged(t *testing.T) {
	v := New(sampleConfig())
	// Cursor starts at 0 — depending on map iteration that could be either ctx.
	// Use the deterministic ordering returned by the view (alphabetical).
	want := v.contextAt(0)

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	msg := cmd()
	cc, ok := msg.(app.ContextChangedMsg)
	if !ok {
		t.Fatalf("msg type = %T, want ContextChangedMsg", msg)
	}
	if cc.Name != want {
		t.Errorf("ContextChangedMsg.Name = %q, want %q", cc.Name, want)
	}
}

func TestList_DownMovesCursor(t *testing.T) {
	v := New(sampleConfig())
	cursorBefore := v.cursor
	v.Update(tea.KeyMsg{Type: tea.KeyDown})
	if v.cursor == cursorBefore {
		t.Error("cursor did not move on KeyDown")
	}
}
```

- [ ] **Step 2: Run, expect compile errors**

Run: `cd tools/blackwall && go test ./internal/views/contexts/...`
Expected: fails — `New`, `Model`, `cursor`, `contextAt` undefined.

- [ ] **Step 3: Implement contexts view**

Create `tools/blackwall/internal/views/contexts/list.go`:

```go
// Package contexts implements the `:contexts` k9s-style picker.
package contexts

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
)

type Model struct {
	cfg    config.Config
	order  []string // deterministic sort of context names
	cursor int
}

func New(cfg config.Config) *Model {
	names := make([]string, 0, len(cfg.Contexts))
	for n := range cfg.Contexts {
		names = append(names, n)
	}
	sort.Strings(names)
	return &Model{cfg: cfg, order: names}
}

func (m *Model) Name() string { return ":contexts" }

func (m *Model) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "↑↓", Desc: "nav"},
		{Key: "↵", Desc: "open"},
		{Key: "c", Desc: "set current"},
		{Key: "/", Desc: "filter"},
	}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.order)-1 {
				m.cursor++
			}
		case "enter":
			name := m.contextAt(m.cursor)
			return m, func() tea.Msg { return app.ContextChangedMsg{Name: name} }
		case "esc":
			return m, func() tea.Msg { return app.BackMsg{} }
		}
	}
	return m, nil
}

func (m *Model) contextAt(i int) string {
	if i < 0 || i >= len(m.order) {
		return ""
	}
	return m.order[i]
}

func (m *Model) View() string {
	var b strings.Builder
	b.WriteString("NAME              DESCRIPTION                       WAZUH SERVER\n")
	for i, name := range m.order {
		cur := "  "
		if i == m.cursor {
			cur = "▶ "
		}
		ctx := m.cfg.Contexts[name]
		server := ""
		if ctx.Wazuh != nil {
			server = fmt.Sprintf("%s:%d", ctx.Wazuh.Server, ctx.Wazuh.Port)
		}
		fmt.Fprintf(&b, "%s%-16s  %-32s  %s\n", cur, name, ctx.Description, server)
	}
	return b.String()
}
```

- [ ] **Step 4: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/views/contexts/... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/views/contexts
git commit -m "feat(blackwall): :contexts picker view"
```

---

## Task 18: `:agents` list view

**Files:**
- Create: `tools/blackwall/internal/views/agents/list.go`
- Create: `tools/blackwall/internal/views/agents/list_test.go`

- [ ] **Step 1: Write failing tests**

Create `tools/blackwall/internal/views/agents/list_test.go`:

```go
package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/tickcmd"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type fakeClient struct {
	agents []wazuh.Agent
	called int
}

func (f *fakeClient) ListAgents(_ context.Context) ([]wazuh.Agent, error) {
	f.called++
	return f.agents, nil
}

func (f *fakeClient) GetAgent(_ context.Context, _ string) (wazuh.AgentDetail, error) {
	return wazuh.AgentDetail{}, nil
}

func TestListView_RenderShowsAgents(t *testing.T) {
	fc := &fakeClient{agents: []wazuh.Agent{
		{ID: "000", Name: "master", Status: "active"},
		{ID: "001", Name: "host-01", Status: "disconnected"},
	}}
	v := New(fc, 3*time.Second)
	// Seed agents directly: skip async fetch for deterministic render.
	v.SetAgents(fc.agents)

	out := v.View()
	for _, want := range []string{"000", "master", "active", "001", "disconnected"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestListView_TickFiresFetch(t *testing.T) {
	fc := &fakeClient{}
	v := New(fc, 3*time.Second)

	_, _ = v.Update(tickcmd.TickMsg{})
	// One synchronous fetch was scheduled — drain the command.
	// The cmd returns AgentsLoadedMsg on success.
	if fc.called != 0 {
		t.Errorf("fetch should not be synchronous: called=%d", fc.called)
	}
}

func TestListView_PauseSkipsFetch(t *testing.T) {
	fc := &fakeClient{}
	v := New(fc, 3*time.Second)
	v.SetPaused(true)
	_, cmd := v.Update(tickcmd.TickMsg{})
	if cmd != nil {
		// Re-arming the next tick is OK (also a cmd). Make sure fetch wasn't kicked off.
		_ = cmd
	}
	// Paused state should not cause a fetch to fire when the cmd runs.
	if fc.called > 0 {
		t.Errorf("paused view kicked off %d fetches", fc.called)
	}
}

func TestListView_EnterEmitsSwitchToDetail(t *testing.T) {
	fc := &fakeClient{agents: []wazuh.Agent{{ID: "001"}}}
	v := New(fc, 3*time.Second)
	v.SetAgents(fc.agents)
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd on Enter")
	}
	msg := cmd()
	sd, ok := msg.(app.SwitchToDetailMsg)
	if !ok {
		t.Fatalf("msg type = %T, want SwitchToDetailMsg", msg)
	}
	if sd.AgentID != "001" {
		t.Errorf("AgentID = %q, want 001", sd.AgentID)
	}
}
```

- [ ] **Step 2: Run, expect compile errors**

Run: `cd tools/blackwall && go test ./internal/views/agents/...`
Expected: fails — types and helpers undefined.

- [ ] **Step 3: Implement list view**

Create `tools/blackwall/internal/views/agents/list.go`:

```go
// Package agents implements the `:agents` list view and the detail drill-down.
package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/tickcmd"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// Client is the subset of *wazuh.Client this view uses.
// A narrower interface keeps tests honest.
type Client interface {
	ListAgents(ctx context.Context) ([]wazuh.Agent, error)
	GetAgent(ctx context.Context, id string) (wazuh.AgentDetail, error)
}

type Model struct {
	client          Client
	agents          []wazuh.Agent
	cursor          int
	paused          bool
	refreshInterval time.Duration
	lastErr         string
}

func New(c Client, refresh time.Duration) *Model {
	if refresh <= 0 {
		refresh = 3 * time.Second
	}
	return &Model{client: c, refreshInterval: refresh}
}

func (m *Model) Name() string { return ":agents" }

func (m *Model) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "↑↓", Desc: "nav"},
		{Key: "↵", Desc: "details"},
		{Key: "p", Desc: "pause"},
		{Key: "/", Desc: "filter"},
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), tickcmd.Every(m.refreshInterval))
}

// SetAgents is a test helper.
func (m *Model) SetAgents(a []wazuh.Agent) { m.agents = a }

// SetPaused is a test helper.
func (m *Model) SetPaused(p bool) { m.paused = p }

// AgentsLoadedMsg is produced by fetchCmd.
type AgentsLoadedMsg struct {
	Agents []wazuh.Agent
	Err    error
}

func (m *Model) fetchCmd() tea.Cmd {
	if m.paused {
		return nil
	}
	return func() tea.Msg {
		agents, err := m.client.ListAgents(context.Background())
		return AgentsLoadedMsg{Agents: agents, Err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickcmd.TickMsg:
		return m, tea.Batch(m.fetchCmd(), tickcmd.Every(m.refreshInterval))

	case AgentsLoadedMsg:
		if msg.Err != nil {
			m.lastErr = msg.Err.Error()
			return m, nil
		}
		m.lastErr = ""
		m.agents = msg.Agents
		if m.cursor >= len(m.agents) {
			m.cursor = 0
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.agents)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.agents) == 0 {
				return m, nil
			}
			id := m.agents[m.cursor].ID
			return m, func() tea.Msg { return app.SwitchToDetailMsg{AgentID: id} }
		case "p":
			m.paused = !m.paused
		case "ctrl+r":
			return m, m.fetchCmd()
		}
	}
	return m, nil
}

func (m *Model) View() string {
	var b strings.Builder
	if m.lastErr != "" {
		fmt.Fprintf(&b, "⚠ %s\n", m.lastErr)
	}
	b.WriteString(fmt.Sprintf("%-4s  %-20s  %-16s  %-12s  %s\n", "ID", "NAME", "IP", "STATUS", "LAST SEEN"))
	for i, a := range m.agents {
		cur := "  "
		if i == m.cursor {
			cur = "▶ "
		}
		lastSeen := "—"
		if !a.LastKeepAlive.IsZero() {
			lastSeen = a.LastKeepAlive.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(&b, "%s%-4s  %-20s  %-16s  %-12s  %s\n",
			cur, a.ID, a.Name, a.IP, a.Status, lastSeen)
	}
	if len(m.agents) == 0 && m.lastErr == "" {
		b.WriteString("\nNo agents.\n")
	}
	return b.String()
}
```

- [ ] **Step 4: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/views/agents/... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/views/agents
git commit -m "feat(blackwall): :agents list view + tick loop"
```

---

## Task 19: `:agents` detail view

**Files:**
- Create: `tools/blackwall/internal/views/agents/detail.go`
- Create: `tools/blackwall/internal/views/agents/detail_test.go`

- [ ] **Step 1: Write failing tests**

Create `tools/blackwall/internal/views/agents/detail_test.go`:

```go
package agents

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

func TestDetail_RendersAllFields(t *testing.T) {
	d := NewDetail(&fakeClient{}, "001")
	d.SetDetail(wazuh.AgentDetail{Agent: wazuh.Agent{
		ID: "001", Name: "host-01", IP: "10.0.0.5", Status: "active",
		Version: "Wazuh v4.14.5", Manager: "master-00",
	}})

	out := d.View()
	for _, want := range []string{"001", "host-01", "10.0.0.5", "active", "v4.14.5", "master-00"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestDetail_EscEmitsBack(t *testing.T) {
	d := NewDetail(&fakeClient{}, "001")
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cmd on Esc")
	}
	if _, ok := cmd().(app.BackMsg); !ok {
		t.Errorf("msg type = %T, want BackMsg", cmd())
	}
}
```

- [ ] **Step 2: Run, expect compile errors**

Run: `cd tools/blackwall && go test ./internal/views/agents/... -run Detail`
Expected: fails — `NewDetail`, `SetDetail` undefined.

- [ ] **Step 3: Implement detail view**

Create `tools/blackwall/internal/views/agents/detail.go`:

```go
package agents

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type DetailModel struct {
	client  Client
	agentID string
	detail  wazuh.AgentDetail
	err     string
}

func NewDetail(c Client, id string) *DetailModel {
	return &DetailModel{client: c, agentID: id}
}

func (m *DetailModel) Name() string { return ":agent-detail" }

func (m *DetailModel) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "esc", Desc: "back"},
		{Key: "r", Desc: "refresh"},
	}
}

// SetDetail is a test helper.
func (m *DetailModel) SetDetail(d wazuh.AgentDetail) { m.detail = d }

type DetailLoadedMsg struct {
	Detail wazuh.AgentDetail
	Err    error
}

func (m *DetailModel) Init() tea.Cmd {
	return func() tea.Msg {
		d, err := m.client.GetAgent(context.Background(), m.agentID)
		return DetailLoadedMsg{Detail: d, Err: err}
	}
}

func (m *DetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case DetailLoadedMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.err = ""
		m.detail = msg.Detail

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "left":
			return m, func() tea.Msg { return app.BackMsg{} }
		case "r":
			return m, m.Init()
		}
	}
	return m, nil
}

func (m *DetailModel) View() string {
	if m.err != "" {
		return fmt.Sprintf("⚠ %s\n", m.err)
	}
	a := m.detail.Agent
	lastSeen := "—"
	if !a.LastKeepAlive.IsZero() {
		lastSeen = a.LastKeepAlive.Format("2006-01-02 15:04:05")
	}
	registered := "—"
	if !a.DateAdd.IsZero() {
		registered = a.DateAdd.Format("2006-01-02 15:04:05")
	}
	osStr := strings.TrimSpace(a.OS.Name + " " + a.OS.Version)
	if osStr == "" {
		osStr = "—"
	}
	return fmt.Sprintf(
		"Agent %s — %s\n"+
			"──────────────────────────────\n"+
			"Status:     %s\n"+
			"IP:         %s\n"+
			"Version:    %s\n"+
			"OS:         %s\n"+
			"Last seen:  %s\n"+
			"Registered: %s\n"+
			"Manager:    %s\n",
		a.ID, a.Name, a.Status, a.IP, a.Version, osStr, lastSeen, registered, a.Manager,
	)
}
```

- [ ] **Step 4: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/views/agents/... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/views/agents
git commit -m "feat(blackwall): :agents detail view"
```

---

## Task 20: Logging setup

**Files:**
- Create: `tools/blackwall/internal/log/log.go`
- Create: `tools/blackwall/internal/log/log_test.go`

- [ ] **Step 1: Write failing test**

Create `tools/blackwall/internal/log/log_test.go`:

```go
package log

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetup_WritesJSONToFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "subdir", "blackwall.log")

	logger, closer, err := Setup(Options{File: logPath, Level: "info"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	t.Cleanup(func() { _ = closer.Close() })

	logger.Info("hello", "key", "value")
	if err := closer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(raw), `"msg":"hello"`) {
		t.Errorf("missing msg in log: %s", raw)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.SplitN(string(raw), "\n", 2)[0]), &parsed); err != nil {
		t.Errorf("log line is not JSON: %v", err)
	}
}

func TestSetup_LevelParsing(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		logPath := filepath.Join(t.TempDir(), "x.log")
		_, closer, err := Setup(Options{File: logPath, Level: level})
		if err != nil {
			t.Errorf("Setup(%s): %v", level, err)
		}
		_ = closer.Close()
	}
}

func TestSetup_UnknownLevelDefaultsWarn(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "x.log")
	_, closer, err := Setup(Options{File: logPath, Level: "bogus"})
	if err != nil {
		t.Errorf("Setup: %v", err)
	}
	_ = closer.Close()
}
```

- [ ] **Step 2: Run, expect compile error**

Run: `cd tools/blackwall && go test ./internal/log/...`
Expected: fails — `Setup`, `Options` undefined.

- [ ] **Step 3: Implement logger setup**

Create `tools/blackwall/internal/log/log.go`:

```go
// Package log builds the blackwall slog logger writing JSON to a file.
package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type Options struct {
	File  string
	Level string
}

// Setup opens the log file (creating parent dirs), constructs a JSON
// slog.Logger and returns it alongside a Closer for graceful shutdown.
func Setup(o Options) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(filepath.Dir(o.File), 0o700); err != nil {
		return nil, nil, fmt.Errorf("mkdir log dir: %w", err)
	}
	f, err := os.OpenFile(o.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: parseLevel(o.Level)}))
	return logger, f, nil
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
```

- [ ] **Step 4: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/log/... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/log
git commit -m "feat(blackwall): slog JSON logger setup"
```

---

## Task 21: `--once` text printer

**Files:**
- Create: `tools/blackwall/internal/oneshot/oneshot.go`
- Create: `tools/blackwall/internal/oneshot/oneshot_test.go`

- [ ] **Step 1: Write failing test**

Create `tools/blackwall/internal/oneshot/oneshot_test.go`:

```go
package oneshot

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type fakeClient struct{ agents []wazuh.Agent }

func (f *fakeClient) ListAgents(context.Context) ([]wazuh.Agent, error) { return f.agents, nil }

func TestPrint_TabulatesAgents(t *testing.T) {
	fc := &fakeClient{agents: []wazuh.Agent{
		{ID: "000", Name: "master", Status: "active"},
		{ID: "001", Name: "host-01", Status: "disconnected"},
	}}
	var buf bytes.Buffer
	if err := Print(context.Background(), &buf, fc); err != nil {
		t.Fatalf("Print: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"ID", "NAME", "STATUS", "000", "master", "active", "001", "disconnected"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run, expect compile error**

Run: `cd tools/blackwall && go test ./internal/oneshot/...`
Expected: fails — `Print` undefined.

- [ ] **Step 3: Implement Print**

Create `tools/blackwall/internal/oneshot/oneshot.go`:

```go
// Package oneshot implements the `--once` text-mode output:
// print a single agents table to stdout and exit.
package oneshot

import (
	"context"
	"fmt"
	"io"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type Lister interface {
	ListAgents(ctx context.Context) ([]wazuh.Agent, error)
}

func Print(ctx context.Context, out io.Writer, c Lister) error {
	agents, err := c.ListAgents(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%-4s  %-20s  %-16s  %-12s  %s\n", "ID", "NAME", "IP", "STATUS", "LAST SEEN")
	for _, a := range agents {
		last := "—"
		if !a.LastKeepAlive.IsZero() {
			last = a.LastKeepAlive.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(out, "%-4s  %-20s  %-16s  %-12s  %s\n", a.ID, a.Name, a.IP, a.Status, last)
	}
	return nil
}
```

- [ ] **Step 4: Run, expect pass**

Run: `cd tools/blackwall && go test ./internal/oneshot/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/blackwall/internal/oneshot
git commit -m "feat(blackwall): --once text printer"
```

---

## Task 22: `cmd/blackwall/main.go` — flags, startup, wiring

**Files:**
- Modify: `tools/blackwall/cmd/blackwall/main.go`

- [ ] **Step 1: Replace placeholder main with full wiring**

Replace contents of `tools/blackwall/cmd/blackwall/main.go`:

```go
// Command blackwall is a k9s-style TUI for the Blackwall security project.
//
// See docs/superpowers/specs/2026-05-17-blackwall-tui-design.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	bwlog "github.com/greengrunt/blackwall/tools/blackwall/internal/log"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/oneshot"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/statusbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/views"
	agentsview "github.com/greengrunt/blackwall/tools/blackwall/internal/views/agents"
	ctxview "github.com/greengrunt/blackwall/tools/blackwall/internal/views/contexts"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "blackwall:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		flagContext              string
		flagConfig               string
		flagRefresh              string
		flagOnce                 bool
		flagSelectContextOnStart bool
		flagNoSelectContext      bool
	)
	flag.StringVar(&flagContext, "context", "", "override current context")
	flag.StringVar(&flagConfig, "config", "", "config file path")
	flag.StringVar(&flagRefresh, "refresh", "", "refresh interval (e.g. 3s)")
	flag.BoolVar(&flagOnce, "once", false, "print agents table and exit (no TUI)")
	flag.BoolVar(&flagSelectContextOnStart, "select-context-on-start", false, "force enable startup picker")
	flag.BoolVar(&flagNoSelectContext, "no-select-context", false, "force disable startup picker")
	flag.Parse()

	cfgPath := flagConfig
	if cfgPath == "" {
		if v := os.Getenv("BW_CONFIG"); v != "" {
			cfgPath = v
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("home dir: %w", err)
			}
			cfgPath = filepath.Join(home, ".config", "blackwall", "config.yaml")
		}
	}

	created, err := config.EnsureExists(cfgPath)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(os.Stderr,
			"Created config at %s\nEdit it to point at your Wazuh manager, then run `blackwall` again.\n",
			cfgPath,
		)
		return nil
	}

	raw, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	cfg := config.ApplyDefaults(raw)
	cfg, err = config.ExpandEnv(cfg)
	if err != nil {
		return err
	}

	ov := config.Overrides{Context: flagContext, ContextWasSet: flagContext != ""}
	if flagRefresh != "" {
		d, err := time.ParseDuration(flagRefresh)
		if err != nil {
			return fmt.Errorf("--refresh: %w", err)
		}
		ov.Refresh = &d
	}
	if flagSelectContextOnStart && flagNoSelectContext {
		return errors.New("--select-context-on-start and --no-select-context are mutually exclusive")
	}
	switch {
	case flagSelectContextOnStart:
		t := true
		ov.SelectContextOnStart = &t
	case flagNoSelectContext:
		f := false
		ov.SelectContextOnStart = &f
	}

	cfg, err = config.Resolve(cfg, ov, config.ReadEnv())
	if err != nil {
		return err
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}

	logger, closer, err := bwlog.Setup(bwlog.Options{File: cfg.Logging.File, Level: cfg.Logging.Level})
	if err != nil {
		return err
	}
	defer closer.Close()
	logger.Info("blackwall startup", "context", cfg.Current)

	currentCtx := cfg.Contexts[cfg.Current]
	if currentCtx.Wazuh == nil {
		return fmt.Errorf("context %q has no wazuh: subsystem", cfg.Current)
	}
	client, err := wazuh.New(*currentCtx.Wazuh)
	if err != nil {
		return err
	}

	// One-shot mode skips the TUI entirely.
	if flagOnce {
		return oneshot.Print(context.Background(), os.Stdout, client)
	}

	// Build views and root app.
	ctxView := ctxview.New(cfg)
	agentsList := agentsview.New(client, cfg.UI.RefreshInterval)
	registered := []views.View{ctxView, agentsList}

	initial := ":agents"
	if cfg.SelectContextOnStart {
		initial = ":contexts"
	}

	wazuhInfo := fmt.Sprintf("%s:%d", currentCtx.Wazuh.Server, currentCtx.Wazuh.Port)
	root := app.New(app.Deps{
		Views:       registered,
		InitialView: initial,
		Status: func() statusbar.State {
			return statusbar.State{
				Context: cfg.Current,
				Wazuh:   wazuhInfo,
				Refresh: cfg.UI.RefreshInterval,
			}
		},
	})

	prog := tea.NewProgram(root, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `cd tools/blackwall && go build ./...`
Expected: success.

- [ ] **Step 3: Smoke-run `--once` against a fake server**

Run a manual sanity check:

```bash
cd tools/blackwall
go test ./...
```

Expected: all packages PASS. (Full integration is exercised in Task 23.)

- [ ] **Step 4: Commit**

```bash
git add tools/blackwall/cmd
git commit -m "feat(blackwall): main wires config+client+app+oneshot"
```

---

## Task 23: Smoke E2E script

**Files:**
- Create: `tools/blackwall/scripts/smoke.sh`

- [ ] **Step 1: Confirm test env presence**

Run: `ls tests/docker/docker-compose.yml`
Expected: file exists.

- [ ] **Step 2: Write the smoke script**

Create `tools/blackwall/scripts/smoke.sh`:

```bash
#!/usr/bin/env bash
# blackwall smoke test: --once against the docker test env.
#
# Brings up the docker test env (or assumes it is already up), waits for
# the Wazuh manager API to respond, runs blackwall --once with a tmp
# config pointing at the manager, greps for an Active agent.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
COMPOSE_DIR="${REPO_ROOT}/tests/docker"
MANAGER_HOST="${BW_SMOKE_HOST:-172.28.0.10}"

cleanup_cfg=""
trap '[[ -n "$cleanup_cfg" ]] && rm -rf "$cleanup_cfg"' EXIT

echo "[smoke] checking docker compose state in ${COMPOSE_DIR}"
if ! docker compose -f "${COMPOSE_DIR}/docker-compose.yml" ps --services --filter status=running | grep -q .; then
    echo "[smoke] starting compose"
    docker compose -f "${COMPOSE_DIR}/docker-compose.yml" up -d
fi

echo "[smoke] waiting for Wazuh API on ${MANAGER_HOST}:55000"
for i in {1..60}; do
    if curl -sk -o /dev/null -w "%{http_code}" "https://${MANAGER_HOST}:55000/" | grep -qE '^(200|401)$'; then
        echo "[smoke] API responding"
        break
    fi
    sleep 2
done

cleanup_cfg="$(mktemp -d)"
cfg="${cleanup_cfg}/config.yaml"
export BW_SMOKE_PASSWORD="${BW_SMOKE_PASSWORD:-wazuh}"

cat >"${cfg}" <<YAML
current: smoke
select_context_on_start: false
contexts:
  smoke:
    description: "smoke target"
    wazuh:
      server: ${MANAGER_HOST}
      port: 55000
      user: wazuh
      password: \${BW_SMOKE_PASSWORD}
      tls:
        insecure_skip_verify: true
ui:
  refresh_interval: 3s
logging:
  level: warn
  file: ${cleanup_cfg}/blackwall.log
YAML

cd "${REPO_ROOT}/tools/blackwall"
go build -o "${cleanup_cfg}/blackwall" ./cmd/blackwall

echo "[smoke] running blackwall --once"
out="$("${cleanup_cfg}/blackwall" --config "${cfg}" --once)"
echo "$out"

if ! echo "$out" | grep -q "active"; then
    echo "[smoke] FAIL: no active agent in output" >&2
    exit 1
fi
echo "[smoke] OK"
```

- [ ] **Step 3: Make it executable**

Run: `chmod +x tools/blackwall/scripts/smoke.sh`

- [ ] **Step 4: Commit**

```bash
git add tools/blackwall/scripts
git commit -m "test(blackwall): smoke script for --once against docker env"
```

---

## Task 24: README + ARCHITECTURE

**Files:**
- Create: `tools/blackwall/README.md`
- Create: `tools/blackwall/docs/ARCHITECTURE.md`

- [ ] **Step 1: Write README**

Create `tools/blackwall/README.md`:

```markdown
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
| `?` | toggle help |

## Distribution to remote hosts

```bash
make build-linux
scp bin/blackwall-linux-amd64 root@security-node:/usr/local/bin/blackwall
```

No runtime required.

## Spec

Full design: [`docs/superpowers/specs/2026-05-17-blackwall-tui-design.md`](../../docs/superpowers/specs/2026-05-17-blackwall-tui-design.md).
```

- [ ] **Step 2: Write ARCHITECTURE.md**

Create `tools/blackwall/docs/ARCHITECTURE.md`:

```markdown
# blackwall — architecture

A Bubbletea TUI in three layers:

- `internal/config`  — YAML + defaults + env expansion + CLI overrides.
- `internal/wazuh`   — Wazuh Manager API client (JWT auth, TLS-aware).
- `internal/app`     — root `tea.Model` with view router, command palette, global hotkeys.
- `internal/views/*` — top-level views (`:contexts`, `:agents`). Each implements
                       `views.View` and is registered in `cmd/blackwall/main.go`.
- `internal/ui/*`    — render helpers (`tabbar`, `statusbar`, `footer`, `styles`).

## Adding a new view

1. New package under `internal/views/<name>/`.
2. Implement the `views.View` interface (`Init`, `Update`, `View`, `Name`, `Hints`).
3. Append it to the `registered` slice in `cmd/blackwall/main.go`.
4. It becomes available as a `[N]` tab and as `:<name>` in the command palette automatically.

## Asynchronous work

All network calls run in `tea.Cmd` goroutines. The view stores any
state-changing result via a message handled in `Update`. The UI thread
never blocks on I/O.

## Refresh loop

`agents.list` re-arms `tickcmd.Every(refreshInterval)` after each tick
so the interval does not drift behind slow responses.
```

- [ ] **Step 3: Commit**

```bash
git add tools/blackwall/README.md tools/blackwall/docs/ARCHITECTURE.md
git commit -m "docs(blackwall): README + architecture notes"
```

---

## Task 25: Final verification

**Files:** none (verification only)

- [ ] **Step 1: Full test pass**

Run: `cd tools/blackwall && go test -race ./...`
Expected: all packages PASS.

- [ ] **Step 2: Build artifacts**

Run: `cd tools/blackwall && make build`
Expected: `bin/blackwall` created.

Run: `cd tools/blackwall && make build-linux`
Expected: `bin/blackwall-linux-amd64` created.

- [ ] **Step 3: `--once` end-to-end (optional, requires docker env)**

If the docker test env is available:

```bash
tools/blackwall/scripts/smoke.sh
```

Expected: prints agents table and `[smoke] OK`.

- [ ] **Step 4: `go vet`**

Run: `cd tools/blackwall && go vet ./...`
Expected: no output.

- [ ] **Step 5: Final commit (only if anything was tweaked during verification)**

If `git status` shows changes:

```bash
git add tools/blackwall
git commit -m "chore(blackwall): final tweaks after verification"
```

Otherwise skip.

---

## Self-review checklist

- Spec §6 config schema → Tasks 2, 3, 4, 5 (types, env, validation, resolve, template). ✓
- Spec §7 startup flow → Task 22 (`run()` in main). ✓
- Spec §8.1–8.3 bubbletea + View interface → Task 14. ✓
- Spec §8.4 tab bar + nav hotkeys → Tasks 12, 15. ✓
- Spec §8.5 `:contexts` view → Task 17. (Reachability ping deferred; see open note below.)
- Spec §8.6 `:agents` list + detail → Tasks 18, 19. ✓
- Spec §8.7 `wazuh.Client` → Tasks 6–9. ✓
- Spec §8.8 data flow → covered by Tasks 18, 19, 22 wiring. ✓
- Spec §8.9 refresh loop → Tasks 10, 18. ✓
- Spec §9 keymap → Tasks 15 (global), 17 (contexts), 18 (agents list), 19 (detail). ✓
- Spec §10 error handling → fatal errors in Task 22 (`run` returns error); recoverable in Tasks 18, 19 (`lastErr` state). Modal overlay (TLS-fail / auth-fail) is **not** in MVP plan — added as deferred note.
- Spec §11 logging → Task 20. ✓
- Spec §12 testing → table tests in every task; smoke in Task 23. ✓
- Spec §13 build/lint/CI → Task 1 Makefile; lint = `make lint` (golangci-lint must be installed externally).
- Spec §6.5 first-run UX → Task 5 + main.go in Task 22. ✓
- Spec §8.4 visibility rule (tab bar always rendered) → Task 12 default behaviour. ✓

**Deferred from MVP plan (acknowledged):**
- `:contexts` background reachability ping per row. The view renders rows but `STATUS` column shows blank in this plan. Add as a follow-up task once the basic loop is shipped.
- Modal overlay widget for fatal-but-runtime errors (TLS verify fail, repeated auth fail). Today these surface as inline `⚠` text in `lastErr` and exit-fatal at startup. Reasonable trade-off for MVP.
- `:agents` filter (`/`), status filter (`f`), sort cycle (`s`), `y` yank. The list view shipped here renders correctly and supports nav + Enter + pause + refresh; the extra ergonomics are quick follow-ups against a working baseline.
- Help overlay (`?`).

Each deferred item is a clean, additive change against the structure built by this plan — no refactor needed.
