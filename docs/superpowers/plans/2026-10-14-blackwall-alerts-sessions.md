# Blackwall alerts feed and host sessions — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a live `:alerts` feed (Wazuh indexer) and a Sessions tab in the agent card (`last` output shipped by the agent) to the `blackwall` TUI, with the lab and Ansible roles to back them.

**Architecture:** A new `internal/indexer` client queries `wazuh-alerts-*` in OpenSearch. A new `internal/views/alerts` view polls it incrementally and opens an alert card. The agent role runs `last` as a Wazuh `full_command`; a manager rule (100100, `check_diff`) stores each change as an alert, which `internal/sessions` parses for the new Sessions tab. App-level plumbing (key capture, pause, view jump, shared scrolling) is fixed first because the new view depends on it.

**Tech Stack:** Go 1.22+, Bubbletea v1.3.10, Lipgloss v1.1.0, yaml.v3; Ansible; Wazuh 4.14.7; OpenSearch (wazuh-indexer); Docker Compose lab driven by `tests/bin/bw-lab` / `bw-check`.

**Spec:** `docs/superpowers/specs/2026-10-14-blackwall-alerts-sessions-design.md`

## Global Constraints

- Branch: all work on `claude-dev`. **Run `git branch --show-current` as a separate command before every commit**; if it is not `claude-dev`, stop and report — the owner sometimes leaves the checkout on `main` after merging PRs. Never push. Never create branches.
- Commit messages: one line, conventional-commit style, English, under ~70 chars. No body, no `Co-Authored-By`, no `Claude-Session`, no "Generated with" lines.
- Everything written into the repo (code comments, docs, STATUS.md, artifacts) is in English.
- Go code lives in `tools/blackwall/`; run Go commands from that directory. `make test` runs `go test -race ./...`.
- Wazuh version pin: `4.14.7` (unchanged).
- Lab indexer: `127.0.0.1:9200`, user `admin`, password `Blackwall1!` (public lab value). Lab Wazuh API: `127.0.0.1:55000`, `wazuh` / `wazuh`.
- Alert defaults: `min_level: 5`, `exclude_groups: [sca, blackwall_sessions]`.
- Feed: first load = last 24 h, newest 200; incremental polls every `ui.refresh_interval`; at most 500 rows in memory.
- Session rule: id `100100`, level `3`, group `blackwall_sessions`, alias `blackwall-sessions`, command `last --time-format iso -w -n 50`, frequency `60` s.
- Times shown to the user: local with UTC in parentheses, via `wazuh.FormatLocalUTC`.
- Tests use fixtures captured from the real lab (never structs marshalled by the test itself — that is how the syscollector schema bug of 2026-10-14 slipped through).
- Keep docs in sync in the same commit as the behaviour (root `CLAUDE.md` rule).
- Every task ends by writing a completion artifact at `docs/superpowers/artifacts/2026-10-14-blackwall-alerts-sessions/<task-slug>.md` (format: title, Status, Module, Date, Commits, What was done, Verification, Deviations, Open questions) and committing it with the task.
- Lab commands: `tests/bin/bw-lab reset` rebuilds everything (~4 min with the apt cache warm), `tests/bin/bw-check all` must end with `0 failed`.

## Review Focus

1. **Alerts sharing one millisecond.** Several alerts carry the identical `timestamp`; an incremental query of "strictly newer than the newest shown" would lose ones indexed later. Use `gte` + de-duplication by alert `id` — pinned by a test in Task 8.
2. **Wazuh timestamp format.** `timestamp` is `2026-10-13T23:58:03.908+0000` (no colon in the offset), which `time.RFC3339` rejects; a silent zero time would break ordering and incremental polling — pinned by a test in Task 4.
3. **Nil interface trap.** Passing a nil `*indexer.Client` as a `Searcher` interface yields a non-nil interface and a nil-pointer panic on first poll; `main.go` must pass an untyped `nil` when the context has no `indexer:` — pinned by a test in Task 8 (view with nil searcher renders the "not configured" hint) and the `SearcherFor` helper wired in Task 8 Step 5.
4. **Long descriptions / narrow terminals.** A 200-char `rule.description` must not wrap the row and shift every line below; rows are cut to the terminal width — pinned by a test in Task 8.
5. **Typing a filter.** `q`, digits, `Tab`, `p` typed into the `/` prompt must go into the prompt, not quit or switch views — pinned by tests in Task 1 (app) and Task 8 (view reports `CapturesKeys`).

---

## File Structure

| Path | Responsibility | Task |
|---|---|---|
| `tools/blackwall/internal/views/view.go` | add `KeyCapturer` interface | 1 |
| `tools/blackwall/internal/app/app.go` | key capture, `p` fall-through, `ShowAgentAlertsMsg` routing | 1 |
| `tools/blackwall/internal/app/messages.go` | `ShowAgentAlertsMsg` | 1 |
| `tools/blackwall/internal/ui/statusbar/statusbar.go` | `Indexer` field | 1 |
| `tools/blackwall/internal/views/agents/view.go` | `CapturesKeys` | 1 |
| `tools/blackwall/internal/ui/scroll/scroll.go` | shared scroll list (moved from agents) | 2 |
| `tools/blackwall/internal/config/{config,load,template.yaml}.go` | `indexer:` section | 3 |
| `tools/blackwall/internal/httpclient/httpclient.go` | TLS-aware `*http.Client` shared by API clients | 4 |
| `tools/blackwall/internal/indexer/*` | indexer client, `Alert` types, query builder | 4 |
| `tests/docker/docker-compose.yml`, `tests/bin/*`, `tests/README.md` | publish 9200, checks, docs | 5 |
| `blackwall-eye/ansible/roles/wazuh-{manager,agent}/*` | `<ruleset>`, rule 100100, `last` command | 6 |
| `tools/blackwall/internal/sessions/*` | `last` output parser | 7 |
| `tools/blackwall/internal/views/alerts/{view,feed,render}.go` | `:alerts` list | 8 |
| `tools/blackwall/internal/views/alerts/card.go` | alert card | 9 |
| `tools/blackwall/internal/views/agents/{sessions,detail}.go` | Sessions tab, `l` jump | 10 |
| `tools/blackwall/cmd/blackwall/main.go` | wiring | 8, 10 |
| READMEs, ARCHITECTURE, STATUS | docs | 5, 6, 11 |

---

### Task 1: App plumbing — key capture, pause, alerts jump message, status line

**Files:**
- Modify: `tools/blackwall/internal/views/view.go`
- Modify: `tools/blackwall/internal/app/app.go` (KeyMsg handling ~lines 120–200, message switch)
- Modify: `tools/blackwall/internal/app/messages.go`
- Modify: `tools/blackwall/internal/ui/statusbar/statusbar.go`
- Modify: `tools/blackwall/internal/views/agents/view.go`
- Modify: `tools/blackwall/cmd/blackwall/main.go` (status func)
- Test: `tools/blackwall/internal/app/app_test.go`, `tools/blackwall/internal/ui/statusbar/statusbar_test.go`, `tools/blackwall/internal/views/agents/view_test.go`

**Interfaces:**
- Produces: `views.KeyCapturer { CapturesKeys() bool }`; `app.ShowAgentAlertsMsg{AgentID, AgentName string}` (app activates the view named `":alerts"` and passes it the message, returning the view's command); `statusbar.State.Indexer string`; `(*agents.View).CapturesKeys() bool`.

- [ ] **Step 1: Write the failing app tests**

Append to `tools/blackwall/internal/app/app_test.go`:

```go
// capturingView claims every key while capturing is true, like a view whose
// filter prompt is open.
type capturingView struct {
	fakeView
	capturing bool
}

func (v *capturingView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { v.lastMsg = msg; return v, nil }
func (v *capturingView) CapturesKeys() bool                       { return v.capturing }

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestApp_CapturingViewGetsGlobalKeys(t *testing.T) {
	a := &capturingView{fakeView: fakeView{name: ":a"}, capturing: true}
	b := &fakeView{name: ":b"}
	m := New(Deps{Views: []views.View{a, b}, InitialView: ":a"})

	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyRunes, Runes: []rune{'2'}},
		{Type: tea.KeyTab},
		{Type: tea.KeyRunes, Runes: []rune{'p'}},
		{Type: tea.KeyRunes, Runes: []rune{':'}},
	} {
		_, cmd := m.Update(k)
		if isQuit(cmd) {
			t.Fatalf("key %q quit while the view was capturing", k.String())
		}
		if m.Active().Name() != ":a" {
			t.Fatalf("key %q switched view while the view was capturing", k.String())
		}
		if got, ok := a.lastMsg.(tea.KeyMsg); !ok || got.String() != k.String() {
			t.Errorf("view got %v, want key %q", a.lastMsg, k.String())
		}
	}
}

func TestApp_CtrlCQuitsEvenWhileCapturing(t *testing.T) {
	a := &capturingView{fakeView: fakeView{name: ":a"}, capturing: true}
	m := New(Deps{Views: []views.View{a}, InitialView: ":a"})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !isQuit(cmd) {
		t.Fatal("ctrl+c must always quit")
	}
}

func TestApp_PReachesViewWithoutPauseHook(t *testing.T) {
	a := &fakeView{name: ":a"}
	m := New(Deps{Views: []views.View{a}, InitialView: ":a"})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if got, ok := a.lastMsg.(tea.KeyMsg); !ok || got.String() != "p" {
		t.Errorf("view got %v, want key p", a.lastMsg)
	}
}

func TestApp_ShowAgentAlertsActivatesAlertsView(t *testing.T) {
	agents := &fakeView{name: ":agents"}
	alerts := &fakeView{name: ":alerts"}
	m := New(Deps{Views: []views.View{agents, alerts}, InitialView: ":agents"})

	msg := ShowAgentAlertsMsg{AgentID: "002", AgentName: "black-host-01"}
	_, _ = m.Update(msg)
	if m.Active().Name() != ":alerts" {
		t.Fatalf("active = %q, want :alerts", m.Active().Name())
	}
	if got, ok := alerts.lastMsg.(ShowAgentAlertsMsg); !ok || got != msg {
		t.Errorf("alerts view got %v, want %v", alerts.lastMsg, msg)
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `cd tools/blackwall && go test ./internal/app/ -run 'Capturing|CtrlC|PReaches|ShowAgentAlerts'`
Expected: compile error `undefined: ShowAgentAlertsMsg`.

- [ ] **Step 3: Add the interface and the message**

`tools/blackwall/internal/views/view.go` — append:

```go
// KeyCapturer is implemented by views that sometimes need every key — for
// example while the user types into a filter prompt. While CapturesKeys()
// returns true the app hands keys straight to the view and skips its global
// shortcuts (q, Tab, digits, :, ?, p). ctrl+c always quits.
type KeyCapturer interface {
	CapturesKeys() bool
}
```

`tools/blackwall/internal/app/messages.go` — append:

```go
// ShowAgentAlertsMsg asks the app to open `:alerts` filtered to one agent
// (sent by the agent card's `l` key). The app activates the view named
// ":alerts" and passes it this message; the view applies the filter.
type ShowAgentAlertsMsg struct {
	AgentID   string
	AgentName string
}
```

- [ ] **Step 4: Route keys and the new message in the app**

In `tools/blackwall/internal/app/app.go`, inside `case tea.KeyMsg:`, directly **after** the block that delegates to the open help view and **before** `if msg.String() == ":" {`, insert:

```go
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if kc, ok := m.Active().(views.KeyCapturer); ok && kc.CapturesKeys() {
			next, cmd := m.Active().Update(msg)
			if v, ok := next.(views.View); ok {
				m.views[m.active] = v
			}
			return m, cmd
		}
```

Replace the existing `case "p":` body:

```go
		case "p":
			// Only consume `p` when a pause hook is wired; otherwise the
			// active view owns pause (the agents list and :alerts do).
			if m.pauseToggle != nil {
				m.pauseToggle()
				return m, nil
			}
```

Add a case to the message `switch` (next to `case SwitchViewMsg:`):

```go
	case ShowAgentAlertsMsg:
		for i, v := range m.views {
			if v.Name() == ":alerts" {
				m.active = i
				next, cmd := v.Update(msg)
				if vv, ok := next.(views.View); ok {
					m.views[i] = vv
				}
				return m, cmd
			}
		}
		return m, nil
```

- [ ] **Step 5: Run the app tests**

Run: `go test ./internal/app/`
Expected: PASS (all, including the existing ones).

- [ ] **Step 6: Status line shows the indexer — failing test**

Append to `tools/blackwall/internal/ui/statusbar/statusbar_test.go`:

```go
func TestRender_IndexerShownOnlyWhenSet(t *testing.T) {
	with := Render(State{Context: "lab", Wazuh: "127.0.0.1:55000", Indexer: "127.0.0.1:9200", Refresh: time.Second})
	if !strings.Contains(with, "indexer: 127.0.0.1:9200") {
		t.Errorf("missing indexer in %q", with)
	}
	without := Render(State{Context: "lab", Wazuh: "127.0.0.1:55000", Refresh: time.Second})
	if strings.Contains(without, "indexer") {
		t.Errorf("unexpected indexer in %q", without)
	}
}
```

(Add `"strings"` / `"time"` imports if the file lacks them.) Run `go test ./internal/ui/statusbar/` — expected: compile error `unknown field Indexer`.

- [ ] **Step 7: Implement the status field**

In `statusbar.go`, add `Indexer string` to `State` and replace the `body :=` line:

```go
	body := fmt.Sprintf("context: %s · wazuh: %s", ctxBadge, s.Wazuh)
	if s.Indexer != "" {
		body += " · indexer: " + s.Indexer
	}
	body += " · " + mode
```

Run `go test ./internal/ui/statusbar/` — expected: PASS.

- [ ] **Step 8: Agents view reports key capture — failing test**

Append to `tools/blackwall/internal/views/agents/view_test.go`:

```go
// While a detail tab's filter prompt is open, the app must not treat typed
// letters as global shortcuts.
func TestView_CapturesKeysWhileDetailFilterIsOpen(t *testing.T) {
	v := NewView(&fakeClient{}, 0)
	if v.CapturesKeys() {
		t.Fatal("list view must not capture keys")
	}
	_, _ = v.Update(app.SwitchToDetailMsg{AgentID: "002"})
	v.detail.activeTab = 3
	pk := v.detail.subs[3].(*PackagesModel)
	pk.filtering = true
	if !v.CapturesKeys() {
		t.Error("view must capture keys while the packages filter is open")
	}
}
```

Run `go test ./internal/views/agents/ -run CapturesKeys` — expected: compile error `v.CapturesKeys undefined`.

- [ ] **Step 9: Implement `CapturesKeys`**

Append to `tools/blackwall/internal/views/agents/view.go`:

```go
// CapturesKeys implements views.KeyCapturer: true while the active detail tab
// is in a modal state (e.g. typing into its filter prompt).
func (v *View) CapturesKeys() bool {
	if v.detail == nil {
		return false
	}
	sub, ok := v.detail.subs[v.detail.activeTab]
	if !ok {
		return false
	}
	kc, ok := sub.(keyCapturer)
	return ok && kc.CapturesKeys()
}
```

- [ ] **Step 10: Full test run, build, commit**

Run: `go vet ./... && go test ./...` — expected: PASS.
Run: `go build ./cmd/blackwall` — expected: success.
Run: `git branch --show-current` — expected: `claude-dev`.

```bash
git add internal/views/view.go internal/app internal/ui/statusbar internal/views/agents
git commit -m "fix(blackwall): let views capture keys and own p; add alerts jump msg"
```

Write the artifact `docs/superpowers/artifacts/2026-10-14-blackwall-alerts-sessions/app-plumbing.md` and commit it (`docs: artifact for app plumbing task`).

---

### Task 2: Move the scroll list to `internal/ui/scroll`

**Files:**
- Create: `tools/blackwall/internal/ui/scroll/scroll.go` (from `internal/views/agents/scrollist.go`)
- Create: `tools/blackwall/internal/ui/scroll/scroll_test.go`
- Delete: `tools/blackwall/internal/views/agents/scrollist.go`
- Modify: `tools/blackwall/internal/views/agents/{packages,processes}.go`, `view_test.go`

**Interfaces:**
- Produces: `scroll.List{HeaderRows, FooterRows int}` with methods `SetSize(height int)`, `Height() int`, `Reset()`, `HandleKey(msg tea.KeyMsg, total int) bool`, `Indicator(total int) string`, `Render(headerBlock string, rows []string, footerBlock string) string`. Behaviour identical to today's `scrollist`.

- [ ] **Step 1: Write the package tests (they fail: package missing)**

`tools/blackwall/internal/ui/scroll/scroll_test.go`:

```go
package scroll

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func rows(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "r"
	}
	return out
}

func TestList_RenderFitsHeight(t *testing.T) {
	l := List{HeaderRows: 2, FooterRows: 1}
	l.SetSize(10)
	out := l.Render("h1\nh2\n", rows(100), "f\n")
	if got := strings.Count(out, "\n"); got != 10 {
		t.Errorf("rendered %d lines, want 10", got)
	}
}

func TestList_UnsizedRendersEverything(t *testing.T) {
	var l List
	out := l.Render("", rows(5), "")
	if got := strings.Count(out, "\n"); got != 5 {
		t.Errorf("rendered %d lines, want 5", got)
	}
}

func TestList_KeysAndIndicator(t *testing.T) {
	l := List{HeaderRows: 0, FooterRows: 0}
	l.SetSize(10)
	l.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}, 100)
	if got := l.Indicator(100); got != "91–100 of 100" {
		t.Errorf("after G indicator = %q", got)
	}
	l.Reset()
	if got := l.Indicator(100); got != "1–10 of 100" {
		t.Errorf("after Reset indicator = %q", got)
	}
	if l.Height() != 10 {
		t.Errorf("Height() = %d", l.Height())
	}
}
```

Run: `go test ./internal/ui/scroll/` — expected: FAIL (`no non-test Go files` / undefined `List`).

- [ ] **Step 2: Create the package**

`git mv internal/views/agents/scrollist.go internal/ui/scroll/scroll.go`, then edit it:
- `package agents` → `package scroll`, add a package comment `// Package scroll keeps scroll state for fixed-height lists.`
- `type scrollist struct` → `type List struct`; fields `headerRows`/`footerRows` → exported `HeaderRows`/`FooterRows` (update every use inside the file); keep `scroll` and `height` unexported.
- Receivers `(s *scrollist)` → `(s *List)`.
- Add:

```go
// Height returns the height last set with SetSize (0 = unconstrained).
func (s *List) Height() int { return s.height }

// Reset scrolls back to the first row (used when the data or filter changes).
func (s *List) Reset() { s.scroll = 0 }
```

- [ ] **Step 3: Switch the agents views over**

In `internal/views/agents/packages.go` and `processes.go`:
- import `"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/scroll"`;
- field `sl scrollist` → `sl scroll.List`;
- `scrollist{headerRows: 2, footerRows: 2}` → `scroll.List{HeaderRows: 2, FooterRows: 2}`;
- every `m.sl.scroll = 0` → `m.sl.Reset()`.

In `internal/views/agents/view_test.go`: `pk.sl.height` → `pk.sl.Height()` (both occurrences).

- [ ] **Step 4: Run everything**

Run: `go vet ./... && go test ./...` — expected: PASS (agents tests unchanged in behaviour).

- [ ] **Step 5: Commit**

`git branch --show-current` → `claude-dev`, then:

```bash
git add -A internal/ui/scroll internal/views/agents
git commit -m "refactor(blackwall): move scroll list to internal/ui/scroll"
```

Artifact: `.../scroll-extract.md`, committed.

---

### Task 3: `indexer:` section in the config

**Files:**
- Modify: `tools/blackwall/internal/config/config.go`
- Modify: `tools/blackwall/internal/config/load.go` (`ApplyDefaults`, `ExpandEnv`, `Validate`)
- Modify: `tools/blackwall/internal/config/template.yaml`
- Test: `tools/blackwall/internal/config/load_test.go`, `template_test.go`

**Interfaces:**
- Produces:

```go
type Context struct {
	Description string            `yaml:"description"`
	Wazuh       *WazuhSubsystem   `yaml:"wazuh,omitempty"`
	Indexer     *IndexerSubsystem `yaml:"indexer,omitempty"`
}
type IndexerSubsystem struct {
	Server   string        `yaml:"server"`
	Port     int           `yaml:"port"`
	User     string        `yaml:"user"`
	Password string        `yaml:"password"`
	TLS      TLS           `yaml:"tls"`
	Alerts   AlertDefaults `yaml:"alerts"`
}
type AlertDefaults struct {
	MinLevel      *int     `yaml:"min_level"`      // nil → DefaultMinLevel
	ExcludeGroups []string `yaml:"exclude_groups"` // nil → DefaultExcludeGroups()
}
const DefaultMinLevel = 5
func DefaultExcludeGroups() []string // returns a fresh []string{"sca", "blackwall_sessions"}
```

After `ApplyDefaults`, every non-nil `Indexer` has a non-nil `Alerts.MinLevel` and non-nil `Alerts.ExcludeGroups`.

- [ ] **Step 1: Write the failing tests**

Append to `tools/blackwall/internal/config/load_test.go` (reuse the file's existing helper for writing a temp config if present; otherwise this self-contained helper):

```go
func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const indexerYAML = `
current: lab
contexts:
  lab:
    wazuh:
      server: 127.0.0.1
      port: 55000
      user: wazuh
      password: wazuh
    indexer:
      server: 127.0.0.1
      port: 9200
      user: admin
      password: ${BW_TEST_IDX_PW}
      tls:
        insecure_skip_verify: true
`

func TestIndexer_ParsedDefaultedExpanded(t *testing.T) {
	t.Setenv("BW_TEST_IDX_PW", "s3cret")
	raw, err := Load(writeTemp(t, indexerYAML))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ExpandEnv(ApplyDefaults(raw))
	if err != nil {
		t.Fatal(err)
	}
	idx := cfg.Contexts["lab"].Indexer
	if idx == nil || idx.Port != 9200 || idx.User != "admin" || !idx.TLS.InsecureSkipVerify {
		t.Fatalf("indexer = %+v", idx)
	}
	if idx.Password != "s3cret" {
		t.Errorf("password = %q, want expanded value", idx.Password)
	}
	if idx.Alerts.MinLevel == nil || *idx.Alerts.MinLevel != DefaultMinLevel {
		t.Errorf("min_level = %v, want %d", idx.Alerts.MinLevel, DefaultMinLevel)
	}
	if got := idx.Alerts.ExcludeGroups; len(got) != 2 || got[0] != "sca" || got[1] != "blackwall_sessions" {
		t.Errorf("exclude_groups = %v", got)
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// An explicit 0 and an explicit empty list are real choices, not "absent".
func TestIndexer_ExplicitZeroAndEmptyKept(t *testing.T) {
	body := indexerYAML + "      alerts:\n        min_level: 0\n        exclude_groups: []\n"
	t.Setenv("BW_TEST_IDX_PW", "x")
	raw, err := Load(writeTemp(t, body))
	if err != nil {
		t.Fatal(err)
	}
	a := ApplyDefaults(raw).Contexts["lab"].Indexer.Alerts
	if a.MinLevel == nil || *a.MinLevel != 0 {
		t.Errorf("min_level = %v, want explicit 0", a.MinLevel)
	}
	if a.ExcludeGroups == nil || len(a.ExcludeGroups) != 0 {
		t.Errorf("exclude_groups = %#v, want explicit empty", a.ExcludeGroups)
	}
}

func TestIndexer_ValidateRequiresFields(t *testing.T) {
	t.Setenv("BW_TEST_IDX_PW", "x")
	raw, _ := Load(writeTemp(t, strings.Replace(indexerYAML, "      user: admin\n", "", 1)))
	cfg, _ := ExpandEnv(ApplyDefaults(raw))
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "contexts.lab.indexer") {
		t.Errorf("Validate err = %v, want contexts.lab.indexer error", err)
	}
}

func TestIndexer_ValidateLevelRange(t *testing.T) {
	t.Setenv("BW_TEST_IDX_PW", "x")
	raw, _ := Load(writeTemp(t, indexerYAML+"      alerts:\n        min_level: 16\n"))
	cfg, _ := ExpandEnv(ApplyDefaults(raw))
	if err := Validate(cfg); err == nil {
		t.Error("min_level 16 must be rejected (Wazuh levels are 0–15)")
	}
}
```

(Ensure imports `os`, `path/filepath`, `strings`.)

Append to `template_test.go`:

```go
func TestTemplate_HasLabIndexer(t *testing.T) {
	if !strings.Contains(templateYAML, "indexer:") || !strings.Contains(templateYAML, "port:     9200") {
		t.Error("template lab context must carry an indexer section on port 9200")
	}
}
```

(`templateYAML` — use the name of the embedded template variable in `template.go`; check the file and adapt this one identifier.)

Run: `go test ./internal/config/` — expected: compile errors (`unknown field Indexer`, `DefaultMinLevel`).

- [ ] **Step 2: Types**

Add to `config.go` the `Indexer` field on `Context`, the `IndexerSubsystem` and `AlertDefaults` types from **Interfaces**, plus:

```go
// DefaultMinLevel hides routine events (level < 5) from the alerts feed.
const DefaultMinLevel = 5

// DefaultExcludeGroups hides CIS audit results (shown in the agent card
// already) and session snapshots (shown in the Sessions tab) from the feed.
func DefaultExcludeGroups() []string { return []string{"sca", "blackwall_sessions"} }
```

- [ ] **Step 3: Defaults, env expansion, validation**

In `ApplyDefaults`, before `return cfg`:

```go
	for name, ctx := range cfg.Contexts {
		if ctx.Indexer == nil {
			continue
		}
		idx := *ctx.Indexer
		if idx.Alerts.MinLevel == nil {
			lvl := DefaultMinLevel
			idx.Alerts.MinLevel = &lvl
		}
		if idx.Alerts.ExcludeGroups == nil {
			idx.Alerts.ExcludeGroups = DefaultExcludeGroups()
		}
		ctx.Indexer = &idx
		cfg.Contexts[name] = ctx
	}
```

Refactor `ExpandEnv` so the same expansion runs for both subsystems. Replace the body with:

```go
func ExpandEnv(cfg Config) (Config, error) {
	for name, ctx := range cfg.Contexts {
		var expandErr error
		expand := func(in, field string) string {
			out := envVarRE.ReplaceAllStringFunc(in, func(m string) string {
				return os.Getenv(envVarRE.FindStringSubmatch(m)[1])
			})
			if strings.Contains(in, "${") && strings.TrimSpace(out) == "" && expandErr == nil {
				expandErr = fmt.Errorf("contexts.%s.%s resolves to empty (env var not set?)", name, field)
			}
			return out
		}
		if ctx.Wazuh != nil {
			w := *ctx.Wazuh
			w.Server = expand(w.Server, "wazuh.server")
			w.User = expand(w.User, "wazuh.user")
			w.Password = expand(w.Password, "wazuh.password")
			w.TLS.CAFile = expand(w.TLS.CAFile, "wazuh.tls.ca_file")
			ctx.Wazuh = &w
		}
		if ctx.Indexer != nil {
			x := *ctx.Indexer
			x.Server = expand(x.Server, "indexer.server")
			x.User = expand(x.User, "indexer.user")
			x.Password = expand(x.Password, "indexer.password")
			x.TLS.CAFile = expand(x.TLS.CAFile, "indexer.tls.ca_file")
			ctx.Indexer = &x
		}
		if expandErr != nil {
			return cfg, expandErr
		}
		cfg.Contexts[name] = ctx
	}
	return cfg, nil
}
```

(Existing tests assert messages like `contexts.<n>.wazuh.<field>`; the new field paths keep that exact form.)

In `Validate`, inside the contexts loop after the Wazuh checks (the loop currently `continue`s when `ctx.Wazuh == nil` — restructure so the indexer check runs regardless):

```go
		if x := ctx.Indexer; x != nil {
			if x.Server == "" || x.Port == 0 || x.User == "" || x.Password == "" {
				return fmt.Errorf("contexts.%s.indexer: server/port/user/password are all required", name)
			}
			if x.TLS.CAFile != "" {
				if _, err := os.Stat(x.TLS.CAFile); err != nil {
					return fmt.Errorf("contexts.%s.indexer.tls.ca_file: %v", name, err)
				}
			}
			if l := x.Alerts.MinLevel; l != nil && (*l < 0 || *l > 15) {
				return fmt.Errorf("contexts.%s.indexer.alerts.min_level: %d is outside 0–15", name, *l)
			}
		}
```

- [ ] **Step 4: Template**

In `template.yaml`, inside the existing `lab:` context, after its `wazuh:` block, add:

```yaml
    indexer:
      server:   127.0.0.1
      port:     9200
      user:     admin
      password: Blackwall1!         # lab-only, public (tests/README.md)
      tls:
        insecure_skip_verify: true  # self-signed lab certificate
      # alerts:
      #   min_level: 5                            # default
      #   exclude_groups: [sca, blackwall_sessions]  # default
```

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/config/ && go vet ./...` — expected: PASS.
`git branch --show-current` → `claude-dev`:

```bash
git add internal/config
git commit -m "feat(blackwall): indexer section in context config"
```

Artifact `.../config-indexer.md`, committed.

---

### Task 4: Indexer client

**Files:**
- Create: `tools/blackwall/internal/httpclient/httpclient.go`
- Modify: `tools/blackwall/internal/wazuh/tls.go` (delegate to `httpclient.New`)
- Create: `tools/blackwall/internal/indexer/{client.go,types.go,query.go}`
- Create: `tools/blackwall/internal/indexer/testdata/{search_alerts.json,search_empty.json}`
- Test: `tools/blackwall/internal/indexer/{client_test.go,query_test.go,types_test.go}`

**Interfaces:**
- Consumes: `config.IndexerSubsystem` (Task 3).
- Produces:

```go
package httpclient
func New(t config.TLS) (*http.Client, error)

package indexer
type Client struct{ /* unexported */ }
func New(sub config.IndexerSubsystem) (*Client, error)
func (c *Client) Addr() string // "host:port", for the status line
func (c *Client) SearchAlerts(ctx context.Context, q AlertQuery) ([]Alert, error)

type AlertQuery struct {
	MinLevel      int
	ExcludeGroups []string
	AgentID       string    // "" = all
	RuleID        string    // "" = any
	Since         time.Time // zero = now-24h; otherwise timestamp >= Since
	Size          int       // 0 = 200
}
type Alert struct {
	ID       string         `json:"id"`
	Time     time.Time      `json:"-"`         // parsed from "timestamp"
	RawTime  string         `json:"timestamp"`
	Rule     Rule           `json:"rule"`
	Agent    AgentRef       `json:"agent"`
	Data     map[string]any `json:"data"`
	FullLog  string         `json:"full_log"`
	Location string         `json:"location"`
}
type Rule struct {
	ID          string   `json:"id"`
	Level       int      `json:"level"`
	Description string   `json:"description"`
	Groups      []string `json:"groups"`
	FiredTimes  int      `json:"firedtimes"`
	MITRE       MITRE    `json:"mitre"`
	PCIDSS      []string `json:"pci_dss"`
	GDPR        []string `json:"gdpr"`
	NIST        []string `json:"nist_800_53"`
	HIPAA       []string `json:"hipaa"`
	TSC         []string `json:"tsc"`
}
type MITRE struct {
	ID        []string `json:"id"`
	Tactic    []string `json:"tactic"`
	Technique []string `json:"technique"`
}
type AgentRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}
func ParseTime(s string) (time.Time, error) // Wazuh "…+0000" or RFC3339
func BuildSearch(q AlertQuery) map[string]any
```

- [ ] **Step 1: Shared HTTP client**

Create `internal/httpclient/httpclient.go` with the body of today's `buildHTTPClient` from `internal/wazuh/tls.go`:

```go
// Package httpclient builds TLS-aware HTTP clients for the Wazuh API and indexer.
package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

// New returns an *http.Client honouring insecure_skip_verify and ca_file.
func New(t config.TLS) (*http.Client, error) {
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

Replace `internal/wazuh/tls.go` with:

```go
package wazuh

import (
	"net/http"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/httpclient"
)

func buildHTTPClient(t config.TLS) (*http.Client, error) { return httpclient.New(t) }
```

Run `go test ./internal/wazuh/` — expected: PASS (TLS tests unchanged).

- [ ] **Step 2: Fixtures (captured from the lab on 2026-10-14)**

`internal/indexer/testdata/search_alerts.json`:

```json
{
 "took": 23, "timed_out": false,
 "_shards": {"total": 3, "successful": 3, "skipped": 0, "failed": 0},
 "hits": {
  "total": {"value": 37, "relation": "eq"},
  "max_score": null,
  "hits": [
   {
    "_index": "wazuh-alerts-4.x-2026.10.13", "_id": "9iizN6EBlKj58HtVtcL2", "_score": null,
    "_source": {
     "agent": {"ip": "172.28.0.11", "name": "black-host-01", "id": "002"},
     "data": {"uid": "0", "dstuser": "root"},
     "rule": {
      "mail": false, "level": 9,
      "pci_dss": ["10.2.5", "10.6.1"],
      "description": "Successful login during non-business hours.",
      "groups": ["policy_violation", "login_time"],
      "gdpr": ["IV_35.7.d", "IV_32.2"],
      "firedtimes": 8,
      "mitre": {"technique": ["Valid Accounts"], "id": ["T1078"],
                "tactic": ["Defense Evasion", "Persistence", "Privilege Escalation", "Initial Access"]},
      "id": "17101"
     },
     "full_log": "Oct 13 23:58:03 black-host-01 sshd[6900]: pam_unix(sshd:session): session opened for user root(uid=0) by (uid=0)",
     "@timestamp": "2026-10-13T23:58:03.908Z",
     "location": "/var/log/auth.log",
     "id": "1791935883.1688070",
     "timestamp": "2026-10-13T23:58:03.908+0000"
    },
    "sort": [1791935883908]
   },
   {
    "_index": "wazuh-alerts-4.x-2026.10.13", "_id": "x2", "_score": null,
    "_source": {
     "agent": {"name": "black-master-00", "id": "000"},
     "data": {"uid": "108", "gid": "110", "dstuser": "prometheus", "home": "/var/lib/prometheus"},
     "rule": {
      "level": 8, "description": "New user added to the system.",
      "groups": ["syslog", "adduser"], "firedtimes": 1,
      "mitre": {"technique": ["Create Account"], "id": ["T1136"], "tactic": ["Persistence"]},
      "id": "5902"
     },
     "full_log": "Oct 13 23:12:37 black-master-00 useradd[56047]: new user: name=prometheus, UID=108, GID=110, home=/var/lib/prometheus, shell=/usr/sbin/nologin, from=/dev/pts/1",
     "location": "/var/log/auth.log",
     "id": "1791933159.1681460",
     "timestamp": "2026-10-13T23:12:39.516+0000"
    },
    "sort": [1791933159516]
   }
  ]
 }
}
```

`internal/indexer/testdata/search_empty.json` (real response for a pattern matching no index):

```json
{"took":0,"timed_out":false,"_shards":{"total":0,"successful":0,"skipped":0,"failed":0},"hits":{"total":{"value":0,"relation":"eq"},"max_score":0.0,"hits":[]}}
```

- [ ] **Step 3: Failing tests**

`internal/indexer/types_test.go`:

```go
package indexer

import (
	"testing"
	"time"
)

// Wazuh writes "+0000" (no colon); time.RFC3339 rejects it.
func TestParseTime_WazuhFormat(t *testing.T) {
	got, err := ParseTime("2026-10-13T23:58:03.908+0000")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 13, 23, 58, 3, 908_000_000, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseTime_RFC3339AndGarbage(t *testing.T) {
	if _, err := ParseTime("2026-10-13T23:58:03.908Z"); err != nil {
		t.Errorf("RFC3339: %v", err)
	}
	if _, err := ParseTime("yesterday"); err == nil {
		t.Error("garbage must be an error")
	}
}
```

`internal/indexer/query_test.go`:

```go
package indexer

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBuildSearch_Defaults(t *testing.T) {
	got := asJSON(t, BuildSearch(AlertQuery{MinLevel: 5, ExcludeGroups: []string{"sca"}}))
	for _, want := range []string{
		`"size":200`,
		`{"timestamp":{"order":"desc"}}`,
		`{"range":{"rule.level":{"gte":5}}}`,
		`{"range":{"timestamp":{"gte":"now-24h"}}}`,
		`"must_not":[{"terms":{"rule.groups":["sca"]}}]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, "agent.id") || strings.Contains(got, "rule.id") {
		t.Errorf("unexpected agent/rule term in %s", got)
	}
}

func TestBuildSearch_AllFilters(t *testing.T) {
	since := time.Date(2026, 10, 13, 23, 58, 3, 908_000_000, time.UTC)
	got := asJSON(t, BuildSearch(AlertQuery{AgentID: "002", RuleID: "100100", Since: since, Size: 1}))
	for _, want := range []string{
		`"size":1`,
		`{"term":{"agent.id":"002"}}`,
		`{"term":{"rule.id":"100100"}}`,
		`{"range":{"timestamp":{"gte":"2026-10-13T23:58:03.908Z"}}}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, "must_not") {
		t.Errorf("no excluded groups → no must_not, got %s", got)
	}
}
```

`internal/indexer/client_test.go`:

```go
package indexer

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

// serve starts an HTTP test server and returns a client pointed at it.
func serve(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	p, _ := strconv.Atoi(port)
	c, err := New(config.IndexerSubsystem{Server: host, Port: p, User: "admin", Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	c.scheme = "http"
	return c
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSearchAlerts_RequestAndDecode(t *testing.T) {
	var gotBody map[string]any
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/wazuh-alerts-*/_search" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if u, p, ok := r.BasicAuth(); !ok || u != "admin" || p != "pw" {
			t.Errorf("basic auth = %q/%q/%v", u, p, ok)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write(fixture(t, "search_alerts.json"))
	})

	alerts, err := c.SearchAlerts(context.Background(), AlertQuery{MinLevel: 5})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["size"] != float64(200) {
		t.Errorf("request size = %v", gotBody["size"])
	}
	if len(alerts) != 2 {
		t.Fatalf("len = %d, want 2", len(alerts))
	}
	a := alerts[0]
	if a.ID != "1791935883.1688070" || a.Rule.ID != "17101" || a.Rule.Level != 9 {
		t.Errorf("alert = %+v", a)
	}
	if a.Agent.Name != "black-host-01" || a.Agent.ID != "002" || a.Data["dstuser"] != "root" {
		t.Errorf("agent/data = %+v / %v", a.Agent, a.Data)
	}
	if len(a.Rule.MITRE.Tactic) != 4 || a.Rule.MITRE.ID[0] != "T1078" || a.Rule.FiredTimes != 8 {
		t.Errorf("rule = %+v", a.Rule)
	}
	if !a.Time.Equal(time.Date(2026, 10, 13, 23, 58, 3, 908_000_000, time.UTC)) {
		t.Errorf("time = %v (zero means the +0000 format was not parsed)", a.Time)
	}
	if !alerts[1].Time.Before(a.Time) {
		t.Error("results must keep the server's newest-first order")
	}
}

func TestSearchAlerts_EmptyIsNotAnError(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture(t, "search_empty.json"))
	})
	alerts, err := c.SearchAlerts(context.Background(), AlertQuery{})
	if err != nil || len(alerts) != 0 {
		t.Errorf("alerts=%v err=%v, want empty and nil", alerts, err)
	}
}

func TestSearchAlerts_Unauthorized(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	})
	_, err := c.SearchAlerts(context.Background(), AlertQuery{})
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("err = %v, want HTTP 401 with body", err)
	}
}

func TestClient_Addr(t *testing.T) {
	c, _ := New(config.IndexerSubsystem{Server: "127.0.0.1", Port: 9200})
	if c.Addr() != "127.0.0.1:9200" {
		t.Errorf("Addr = %q", c.Addr())
	}
}
```

Run: `go test ./internal/indexer/` — expected: compile errors (package has no non-test files).

- [ ] **Step 4: Implement**

`internal/indexer/types.go`:

```go
// Package indexer queries Wazuh alerts stored in the Wazuh indexer (OpenSearch).
package indexer

import (
	"encoding/json"
	"fmt"
	"time"
)

// (Alert, Rule, MITRE, AgentRef exactly as in Interfaces above.)

// wazuhTimeLayout matches Wazuh's "2026-10-13T23:58:03.908+0000".
const wazuhTimeLayout = "2006-01-02T15:04:05.999-0700"

// ParseTime accepts Wazuh's alert timestamp and plain RFC3339.
func ParseTime(s string) (time.Time, error) {
	if t, err := time.Parse(wazuhTimeLayout, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", s)
}

// UnmarshalJSON fills Time from the raw "timestamp" field.
func (a *Alert) UnmarshalJSON(b []byte) error {
	type plain Alert
	if err := json.Unmarshal(b, (*plain)(a)); err != nil {
		return err
	}
	if a.RawTime != "" {
		t, err := ParseTime(a.RawTime)
		if err != nil {
			return err
		}
		a.Time = t
	}
	return nil
}
```

`internal/indexer/query.go`:

```go
package indexer

import "time"

const defaultSize = 200

// BuildSearch turns q into an OpenSearch _search body, newest first.
// Since uses gte, not gt: alerts can share one millisecond, and the caller
// de-duplicates by Alert.ID.
func BuildSearch(q AlertQuery) map[string]any {
	size := q.Size
	if size <= 0 {
		size = defaultSize
	}
	since := any("now-24h")
	if !q.Since.IsZero() {
		since = q.Since.UTC().Format(time.RFC3339Nano)
	}
	filter := []any{
		map[string]any{"range": map[string]any{"rule.level": map[string]any{"gte": q.MinLevel}}},
		map[string]any{"range": map[string]any{"timestamp": map[string]any{"gte": since}}},
	}
	if q.AgentID != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"agent.id": q.AgentID}})
	}
	if q.RuleID != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"rule.id": q.RuleID}})
	}
	boolQ := map[string]any{"filter": filter}
	if len(q.ExcludeGroups) > 0 {
		boolQ["must_not"] = []any{map[string]any{"terms": map[string]any{"rule.groups": q.ExcludeGroups}}}
	}
	return map[string]any{
		"size":  size,
		"sort":  []any{map[string]any{"timestamp": map[string]any{"order": "desc"}}},
		"query": map[string]any{"bool": boolQ},
	}
}
```

Note: `encoding/json` sorts map keys, so `{"range":{"rule.level":{"gte":5}}}` serialises exactly as the test expects.

`internal/indexer/client.go`:

```go
package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/httpclient"
)

// Client talks to the Wazuh indexer's search API.
type Client struct {
	host, user, password string
	port                 int
	scheme               string // "https"; tests switch to "http"
	http                 *http.Client
}

func New(sub config.IndexerSubsystem) (*Client, error) {
	h, err := httpclient.New(sub.TLS)
	if err != nil {
		return nil, err
	}
	return &Client{host: sub.Server, port: sub.Port, user: sub.User, password: sub.Password, scheme: "https", http: h}, nil
}

// Addr is "host:port" for the status line.
func (c *Client) Addr() string { return c.host + ":" + strconv.Itoa(c.port) }

type searchResponse struct {
	Hits struct {
		Hits []struct {
			Source Alert `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// SearchAlerts runs q against wazuh-alerts-* and returns hits newest first.
// A pattern that matches no index yields an empty slice, not an error.
func (c *Client) SearchAlerts(ctx context.Context, q AlertQuery) ([]Alert, error) {
	body, err := json.Marshal(BuildSearch(q))
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s://%s/wazuh-alerts-*/_search", c.scheme, c.Addr())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("indexer search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return nil, fmt.Errorf("indexer search: HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(msg))
	}
	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("indexer search decode: %w", err)
	}
	out := make([]Alert, 0, len(sr.Hits.Hits))
	for _, h := range sr.Hits.Hits {
		out = append(out, h.Source)
	}
	return out, nil
}
```

- [ ] **Step 5: Run, vet, commit**

Run: `go vet ./... && go test ./internal/indexer/ ./internal/wazuh/` — expected: PASS.
`git branch --show-current` → `claude-dev`:

```bash
git add internal/httpclient internal/indexer internal/wazuh/tls.go
git commit -m "feat(blackwall): wazuh indexer client for alert search"
```

Artifact `.../indexer-client.md`, committed.

---

### Task 5: Publish the indexer in the lab

**Files:**
- Modify: `tests/docker/docker-compose.yml` (black-master-00 `ports:` and the ports comment)
- Modify: `tests/bin/_bw-common.sh`, `tests/bin/bw-check`
- Modify: `tests/README.md` (access table, topology row, `bw-check` table), root `README.md` (service table)

**Interfaces:**
- Produces: indexer reachable at `https://127.0.0.1:9200` (admin / `Blackwall1!`) on the Mac after `bw-lab up`.

- [ ] **Step 1: Compose**

In `tests/docker/docker-compose.yml`, `black-master-00.ports`, add after `"127.0.0.1:55000:55000"`:

```yaml
      - "127.0.0.1:9200:9200"
```

and extend the ports comment above the service:

```yaml
  #                     9200 (Wazuh indexer, loopback only — for blackwall :alerts)
```

Run: `docker compose -f tests/docker/docker-compose.yml --profile ubuntu2204 config -q` — expected: exit 0.

- [ ] **Step 2: Checks**

In `tests/bin/_bw-common.sh`, next to `BW_DASHBOARD_URL`:

```bash
BW_INDEXER_URL="https://127.0.0.1:9200"
BW_INDEXER_AUTH="admin:Blackwall1!"
```

In `tests/bin/bw-check`:
- `check_ports`: add `"9200:wazuh-indexer"` to the published-port list (after `"55000:wazuh-api"`).
- `check_wazuh`: after the "manager REST API answers" check, add:

```bash
  local icode
  icode="$(curl -sk -m 5 -o /dev/null -w '%{http_code}' -u "$BW_INDEXER_AUTH" "$BW_INDEXER_URL/" 2>/dev/null)"
  if [[ "$icode" == "200" ]]; then
    bw_pass "indexer answers at $BW_INDEXER_URL"
  else
    bw_fail "indexer does not answer at $BW_INDEXER_URL (code '$icode')"
  fi
```

Run: `bash -n tests/bin/bw-check && bash -n tests/bin/_bw-common.sh` — expected: no output.

- [ ] **Step 3: Docs**

`tests/README.md`:
- Access table: add `| Wazuh indexer (for blackwall :alerts) | https://127.0.0.1:9200 | admin / Blackwall1! |`.
- Host topology table, `black-master-00` "Published" cell: append `, 127.0.0.1:9200→9200`.
- `bw-check` table, `wazuh` row: add "indexer answers".
- Containers `check ports` description unchanged.

Root `README.md` service table: add `| Wazuh indexer (blackwall :alerts) | https://127.0.0.1:9200 | admin / Blackwall1! |`.

- [ ] **Step 4: Verify on the lab**

Run: `tests/bin/bw-lab reset` (the master container must be recreated for the new port; ~4 min with the apt cache).
Run: `tests/bin/bw-check all` — expected: `0 failed`, lines `PASS published port 9200 (wazuh-indexer) reachable from the host` and `PASS indexer answers at https://127.0.0.1:9200`.
Run: `curl -sk -u 'admin:Blackwall1!' 'https://127.0.0.1:9200/wazuh-alerts-*/_count'` — expected: JSON with a `count`.

- [ ] **Step 5: Commit**

`git branch --show-current` → `claude-dev`:

```bash
git add tests README.md
git commit -m "feat(tests): publish wazuh indexer on 127.0.0.1:9200"
```

Artifact `.../lab-indexer.md`, committed.

---

### Task 6: Session snapshots in the Wazuh roles

**Files:**
- Modify: `blackwall-eye/ansible/roles/wazuh-manager/templates/ossec.conf.j2` (add `<ruleset>`)
- Create: `blackwall-eye/ansible/roles/wazuh-manager/templates/blackwall_rules.xml.j2`
- Modify: `blackwall-eye/ansible/roles/wazuh-manager/tasks/main.yml`
- Modify: `blackwall-eye/ansible/roles/wazuh-agent/templates/ossec.conf.j2`, `.../defaults/main.yml`
- Modify: `tests/bin/bw-check` (rule check), `blackwall-eye/README.md`, `blackwall-eye/STATUS.md`

**Interfaces:**
- Produces: alerts with `rule.id = "100100"`, `rule.groups` containing `blackwall_sessions`, `full_log` = Wazuh's command-output wrapper + `last --time-format iso -w -n 50` output, stored whenever the output changes. Record the exact `full_log` layout in the artifact — Task 7 parses it.

- [ ] **Step 1: `<ruleset>` in the manager config**

In `wazuh-manager/templates/ossec.conf.j2`, inside `<ossec_config>` (after the `<global>` block), add:

```xml
  <!-- Explicit ruleset: without it the built-in rules still load, but
       GET /rules fails (error 1106) and etc/rules is not guaranteed. -->
  <ruleset>
    <decoder_dir>ruleset/decoders</decoder_dir>
    <rule_dir>ruleset/rules</rule_dir>
    <rule_exclude>0215-policy_rules.xml</rule_exclude>
    <list>etc/lists/audit-keys</list>
    <list>etc/lists/amazon/aws-eventnames</list>
    <list>etc/lists/security-eventchannel</list>
    <list>etc/lists/malicious-ioc/malicious-ip</list>
    <list>etc/lists/malicious-ioc/malicious-domains</list>
    <list>etc/lists/malicious-ioc/malware-hashes</list>

    <decoder_dir>etc/decoders</decoder_dir>
    <rule_dir>etc/rules</rule_dir>
  </ruleset>
```

- [ ] **Step 2: Rule 100100**

`wazuh-manager/templates/blackwall_rules.xml.j2`:

```xml
<!-- Managed by blackwall-eye (ansible role wazuh-manager). Do not edit on the host. -->
<group name="blackwall,">

  <!-- Login sessions snapshot: the agent runs `last` (alias blackwall-sessions,
       see the wazuh-agent role). check_diff stores an alert only when the
       output changes — one snapshot per login/logout, read by the blackwall
       TUI Sessions tab. -->
  <rule id="100100" level="3">
    <if_sid>530</if_sid>
    <match>ossec: output: 'blackwall-sessions'</match>
    <check_diff />
    <description>Blackwall: login sessions changed (last).</description>
    <group>blackwall_sessions,</group>
  </rule>

</group>
```

In `wazuh-manager/tasks/main.yml`, directly after the "Deploy ossec.conf from template" task:

```yaml
- name: Deploy blackwall rules
  template:
    src: blackwall_rules.xml.j2
    dest: /var/ossec/etc/rules/blackwall_rules.xml
    owner: wazuh
    group: wazuh
    mode: "0660"
  notify: Restart wazuh-manager
```

- [ ] **Step 3: The `last` command on agents**

`wazuh-agent/defaults/main.yml` — append:

```yaml

# Login sessions snapshot for the blackwall TUI Sessions tab: the agent runs
# `last` every wazuh_sessions_frequency seconds; the manager rule 100100
# stores the output whenever it changes.
wazuh_sessions_enabled: true
wazuh_sessions_frequency: 60
```

`wazuh-agent/templates/ossec.conf.j2` — after the last `<localfile>` block:

```xml
{% if wazuh_sessions_enabled %}
  <!-- Login sessions for the blackwall Sessions tab (manager rule 100100) -->
  <localfile>
    <log_format>full_command</log_format>
    <command>last --time-format iso -w -n 50</command>
    <alias>blackwall-sessions</alias>
    <frequency>{{ wazuh_sessions_frequency }}</frequency>
  </localfile>
{% endif %}
```

- [ ] **Step 4: Check the rule is loaded**

In `tests/bin/bw-check`, `check_wazuh`, after the indexer check from Task 5:

```bash
  local token rules
  token="$(curl -sk -m 5 -u wazuh:wazuh -X POST \
             'https://127.0.0.1:55000/security/user/authenticate?raw=true' 2>/dev/null)"
  rules="$(curl -sk -m 5 -H "Authorization: Bearer $token" \
             'https://127.0.0.1:55000/rules?rule_ids=100100' 2>/dev/null)"
  if bw_json_true "d['data']['total_affected_items'] == 1" <<<"$rules"; then
    bw_pass "rule 100100 (blackwall sessions) is loaded"
  else
    bw_fail "rule 100100 is not loaded on the manager"
  fi
```

Run: `bash -n tests/bin/bw-check`.

- [ ] **Step 5: Verify on the lab**

Run: `tests/bin/bw-lab reset` then `tests/bin/bw-check all` — expected: `0 failed`, including `PASS rule 100100 (blackwall sessions) is loaded`.

Make a login and capture the snapshot:

```bash
tests/bin/bw-lab ssh black-host-01 'sleep 70'
curl -sk -u 'admin:Blackwall1!' 'https://127.0.0.1:9200/wazuh-alerts-*/_search' \
  -H 'Content-Type: application/json' \
  -d '{"size":1,"sort":[{"timestamp":{"order":"desc"}}],"query":{"bool":{"filter":[{"term":{"rule.id":"100100"}},{"term":{"agent.name":"black-host-01"}}]}}}' \
  | python3 -c 'import json,sys; h=json.load(sys.stdin)["hits"]["hits"]; print(repr(h[0]["_source"]["full_log"][:600]) if h else "NO SNAPSHOT")'
```

Expected: a `full_log` that starts with `ossec: output: 'blackwall-sessions':` followed by `last` lines including one `still logged in` row from `172.28.0.1`. Paste the exact first ~600 characters (repr) into the artifact — Task 7 depends on them.

Also record in the artifact whether a snapshot existed **before** the first login (i.e. whether `check_diff` alerts on the very first output). If it did not, the lab is still fine — provisioning and `bw-check` log in over SSH constantly — note it, do not add workarounds.

Run: `tests/bin/bw-check all` once more — expected: `0 failed` (the session alerts are level 3 and excluded from nothing in bw-check).

- [ ] **Step 6: Docs**

`blackwall-eye/README.md` — new section after "Package source (production)":

```markdown
### Login sessions snapshot

The `wazuh-agent` role makes every agent run `last --time-format iso -w -n 50`
every `wazuh_sessions_frequency` seconds (default 60; disable with
`wazuh_sessions_enabled: false`). The `wazuh-manager` role installs
`/var/ossec/etc/rules/blackwall_rules.xml` with rule **100100** (level 3, group
`blackwall_sessions`, `check_diff`): the output is stored as an alert only when
it changes, i.e. on each login or logout. The `blackwall` TUI reads the newest
snapshot for its Sessions tab.

The manager config now also declares the standard `<ruleset>` explicitly;
before, `GET /rules` failed with error 1106.
```

`blackwall-eye/STATUS.md` — add a `## Done` entry dated 2026-10-14 summarising Steps 1–5 (ruleset fix, rule 100100, sessions command, bw-check assertion) and set `last_updated: 2026-10-14`, `last_task` accordingly.

- [ ] **Step 7: Commit**

`git branch --show-current` → `claude-dev`:

```bash
git add blackwall-eye tests/bin/bw-check
git commit -m "feat(eye): session snapshots via last + rule 100100; explicit ruleset"
```

Artifact `.../eye-sessions.md` (with the captured `full_log`), committed.

---

### Task 7: `last` output parser

**Files:**
- Create: `tools/blackwall/internal/sessions/sessions.go`
- Create: `tools/blackwall/internal/sessions/testdata/full_log.txt`
- Test: `tools/blackwall/internal/sessions/sessions_test.go`

**Interfaces:**
- Consumes: the `full_log` layout recorded in Task 6's artifact (`docs/superpowers/artifacts/2026-10-14-blackwall-alerts-sessions/eye-sessions.md`). **Read it first**; if the first line differs from `ossec: output: 'blackwall-sessions':`, adapt the fixture's first line and `stripHeader` to the real one.
- Produces:

```go
package sessions
type Session struct {
	User, TTY, From string
	Login           time.Time
	Logout          time.Time // zero while active or when End is set
	Active          bool      // "still logged in" / "still running"
	End             string    // "down", "crash" or "gone" when there is no logout time
	Raw             string    // set (and nothing else) for lines that could not be parsed
}
func Parse(fullLog string) []Session          // in input order (last prints newest first)
func (s Session) Duration(now time.Time) time.Duration
func (s Session) IsReboot() bool               // User == "reboot"
```

- [ ] **Step 1: Fixture**

`testdata/full_log.txt` (real lines from the lab plus every `last` shape; keep the trailing blank line and footer):

```text
ossec: output: 'blackwall-sessions':
root     pts/1        10.0.0.5         2026-10-14T02:40:00+00:00   still logged in
root     pts/0        172.28.0.1       2026-10-13T23:13:01+00:00 - 2026-10-13T23:13:05+00:00  (00:00)
akrug    tty1                          2026-10-13T20:00:00+00:00 - down                       (01:10)
bob      pts/2        192.0.2.7        2026-10-12T10:00:00+00:00 - crash                      (00:30)
eve      pts/3        198.51.100.4     2026-10-11T09:00:00+00:00   gone - no logout
reboot   system boot  7.0.14-orbstack- 2026-10-13T23:09:49+00:00   still running
this line is not from last

wtmp begins 2026-10-13T23:09:49+00:00
```

- [ ] **Step 2: Failing tests**

`sessions_test.go`:

```go
package sessions

import (
	"os"
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func load(t *testing.T) []Session {
	t.Helper()
	b, err := os.ReadFile("testdata/full_log.txt")
	if err != nil {
		t.Fatal(err)
	}
	return Parse(string(b))
}

func TestParse_AllShapes(t *testing.T) {
	got := load(t)
	if len(got) != 7 {
		t.Fatalf("len = %d, want 7 (6 sessions + 1 raw): %+v", len(got), got)
	}

	active := got[0]
	if active.User != "root" || active.TTY != "pts/1" || active.From != "10.0.0.5" ||
		!active.Active || !active.Login.Equal(ts("2026-10-14T02:40:00Z")) || !active.Logout.IsZero() {
		t.Errorf("active = %+v", active)
	}

	closed := got[1]
	if closed.Active || !closed.Logout.Equal(ts("2026-10-13T23:13:05Z")) || closed.From != "172.28.0.1" {
		t.Errorf("closed = %+v", closed)
	}

	console := got[2]
	if console.User != "akrug" || console.TTY != "tty1" || console.From != "" || console.End != "down" {
		t.Errorf("console/down = %+v", console)
	}
	if got[3].End != "crash" || got[4].End != "gone" {
		t.Errorf("crash/gone = %+v / %+v", got[3], got[4])
	}

	reboot := got[5]
	if !reboot.IsReboot() || !reboot.Active || reboot.TTY != "system boot" {
		t.Errorf("reboot = %+v", reboot)
	}

	if got[6].Raw != "this line is not from last" || got[6].User != "" {
		t.Errorf("raw = %+v", got[6])
	}
}

func TestParse_EmptyAndHeaderOnly(t *testing.T) {
	if got := Parse(""); len(got) != 0 {
		t.Errorf("empty → %+v", got)
	}
	if got := Parse("ossec: output: 'blackwall-sessions':\n\nwtmp begins 2026-10-13T23:09:49+00:00\n"); len(got) != 0 {
		t.Errorf("header only → %+v", got)
	}
}

func TestDuration(t *testing.T) {
	got := load(t)
	now := ts("2026-10-14T02:49:00Z")
	if d := got[0].Duration(now); d != 9*time.Minute {
		t.Errorf("active duration = %v, want 9m", d)
	}
	if d := got[1].Duration(now); d != 4*time.Second {
		t.Errorf("closed duration = %v, want 4s", d)
	}
	if d := got[2].Duration(now); d != 0 {
		t.Errorf("down (no logout time) duration = %v, want 0", d)
	}
}
```

Run: `go test ./internal/sessions/` — expected: compile error (package missing).

- [ ] **Step 3: Implement**

`sessions.go`:

```go
// Package sessions parses `last --time-format iso -w` output shipped by the
// Wazuh agent (rule 100100) into login sessions.
package sessions

import (
	"strings"
	"time"
)

type Session struct {
	User, TTY, From string
	Login           time.Time
	Logout          time.Time
	Active          bool
	End             string
	Raw             string
}

// IsReboot reports the pseudo-session `last` prints for each boot.
func (s Session) IsReboot() bool { return s.User == "reboot" }

// Duration is now−login for active sessions, logout−login for closed ones,
// and 0 when the end time is unknown (down/crash/gone) or the line is raw.
func (s Session) Duration(now time.Time) time.Duration {
	switch {
	case s.Raw != "" || s.Login.IsZero():
		return 0
	case s.Active:
		return now.Sub(s.Login)
	case !s.Logout.IsZero():
		return s.Logout.Sub(s.Login)
	}
	return 0
}

// Parse returns one Session per `last` line, in input order. The Wazuh
// command-output header, blank lines and the "wtmp begins" footer are skipped;
// a line that does not look like a `last` record is kept verbatim in Raw.
func Parse(fullLog string) []Session {
	var out []Session
	for _, line := range strings.Split(fullLog, "\n") {
		line = strings.TrimRight(line, " \r")
		switch {
		case strings.TrimSpace(line) == "",
			strings.HasPrefix(line, "ossec: output:"),
			strings.HasPrefix(line, "wtmp begins"):
			continue
		}
		if s, ok := parseLine(line); ok {
			out = append(out, s)
		} else {
			out = append(out, Session{Raw: strings.TrimSpace(line)})
		}
	}
	return out
}

func parseLine(line string) (Session, bool) {
	fields := strings.Fields(line)
	at := -1
	var login time.Time
	for i, f := range fields {
		if t, err := time.Parse(time.RFC3339, f); err == nil {
			at, login = i, t
			break
		}
	}
	if at < 1 {
		return Session{}, false
	}
	s := Session{User: fields[0], Login: login}
	switch who := fields[1:at]; len(who) {
	case 0:
	case 1:
		s.TTY = who[0]
	case 2:
		s.TTY, s.From = who[0], who[1]
	default: // "reboot system boot <kernel>"
		s.TTY, s.From = strings.Join(who[:len(who)-1], " "), who[len(who)-1]
	}

	rest := strings.Join(fields[at+1:], " ")
	switch {
	case strings.HasPrefix(rest, "still logged in"), strings.HasPrefix(rest, "still running"):
		s.Active = true
	case strings.HasPrefix(rest, "gone"):
		s.End = "gone"
	case strings.HasPrefix(rest, "- "):
		end := strings.Fields(strings.TrimPrefix(rest, "- "))
		if len(end) == 0 {
			return Session{}, false
		}
		if t, err := time.Parse(time.RFC3339, end[0]); err == nil {
			s.Logout = t
		} else {
			s.End = end[0] // down, crash
		}
	default:
		return Session{}, false
	}
	return s, true
}
```

- [ ] **Step 4: Run, commit**

Run: `go test ./internal/sessions/ && go vet ./...` — expected: PASS.
`git branch --show-current` → `claude-dev`:

```bash
git add internal/sessions
git commit -m "feat(blackwall): parser for last output (sessions)"
```

Artifact `.../sessions-parser.md`, committed.

---

### Task 8: The `:alerts` view (feed)

**Files:**
- Create: `tools/blackwall/internal/views/alerts/{view.go,feed.go,render.go}`
- Test: `tools/blackwall/internal/views/alerts/{view_test.go,feed_test.go}`
- Modify: `tools/blackwall/internal/ui/styles/styles.go` (`LevelCritical`)
- Modify: `tools/blackwall/cmd/blackwall/main.go` (build indexer client, register view, context switch, status)

**Interfaces:**
- Consumes: `indexer.AlertQuery`, `indexer.Alert` (Task 4); `views.KeyCapturer`, `app.ShowAgentAlertsMsg` (Task 1); `config.IndexerSubsystem`, `config.AlertDefaults` (Task 3); `wazuh.FormatLocalUTC`; `table.Widths/Row/Header`; `tickcmd` is **not** used (own tick type).
- Produces:

```go
package alerts
type Searcher interface {
	SearchAlerts(ctx context.Context, q indexer.AlertQuery) ([]indexer.Alert, error)
}
type Settings struct {
	Context       string        // context name, for the "not configured" hint
	MinLevel      int
	ExcludeGroups []string
	Refresh       time.Duration
}
func New(s Searcher, st Settings) *View          // s may be nil → "not configured"
func (v *View) Replace(s Searcher, st Settings)   // on context switch
// View implements views.View and views.KeyCapturer; Name() == ":alerts".
func SearcherFor(sub *config.IndexerSubsystem) (Searcher, string, error)
// returns (nil, "", nil) when sub == nil — an untyped nil, never a typed-nil client;
// the string is the "host:port" for the status line.
```

Constants: `initialSize = 200`, `maxRows = 500`.

- [ ] **Step 1: Feed logic tests (pure functions)**

`feed_test.go`:

```go
package alerts

import (
	"testing"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
)

func al(id string, sec int) indexer.Alert {
	return indexer.Alert{ID: id, Time: time.Date(2026, 10, 14, 0, 0, sec, 0, time.UTC)}
}

func ids(as []indexer.Alert) string {
	s := ""
	for _, a := range as {
		s += a.ID
	}
	return s
}

// Several alerts share one millisecond; the incremental query uses gte, so the
// newest shown alert comes back again and must not be duplicated.
func TestMerge_DedupesByID(t *testing.T) {
	rows := []indexer.Alert{al("B", 2), al("A", 1)}
	got, added := merge(rows, []indexer.Alert{al("C", 2), al("B", 2)}, maxRows)
	if ids(got) != "CBA" || added != 1 {
		t.Errorf("got %s added %d, want CBA 1", ids(got), added)
	}
}

func TestMerge_CapsAtMax(t *testing.T) {
	rows := []indexer.Alert{al("B", 2), al("A", 1)}
	got, added := merge(rows, []indexer.Alert{al("C", 3)}, 2)
	if ids(got) != "CB" || added != 1 {
		t.Errorf("got %s added %d, want CB 1", ids(got), added)
	}
}
```

`feed.go`:

```go
package alerts

import "github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"

const (
	initialSize = 200
	maxRows     = 500
)

// merge prepends the alerts in fresh (newest first) that rows does not have
// yet, keeps newest-first order and caps the result at max rows. It returns
// the merged rows and how many were added.
func merge(rows, fresh []indexer.Alert, max int) ([]indexer.Alert, int) {
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		seen[r.ID] = true
	}
	var add []indexer.Alert
	for _, a := range fresh {
		if !seen[a.ID] {
			seen[a.ID] = true
			add = append(add, a)
		}
	}
	out := append(add, rows...)
	if len(out) > max {
		out = out[:max]
	}
	return out, len(add)
}
```

Run: `go test ./internal/views/alerts/ -run Merge` — expected: PASS after creating `feed.go` (write the test first, see it fail with `undefined: merge`, then add the file).

- [ ] **Step 2: View tests (fail: View undefined)**

`view_test.go`:

```go
package alerts

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
)

// fakeSearcher returns scripted results in order (repeating the last one once
// the script runs out, like a feed that has nothing new) and records every query.
type fakeSearcher struct {
	queries []indexer.AlertQuery
	results [][]indexer.Alert
	err     error
}

func (f *fakeSearcher) SearchAlerts(_ context.Context, q indexer.AlertQuery) ([]indexer.Alert, error) {
	f.queries = append(f.queries, q)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.results) == 0 {
		return nil, nil
	}
	r := f.results[0]
	if len(f.results) > 1 {
		f.results = f.results[1:]
	}
	return r, nil
}

func alert(id string, sec, level int, agentID, agentName, desc string) indexer.Alert {
	return indexer.Alert{
		ID: id, Time: time.Date(2026, 10, 14, 0, 0, sec, 0, time.UTC),
		Rule:  indexer.Rule{ID: "17101", Level: level, Description: desc},
		Agent: indexer.AgentRef{ID: agentID, Name: agentName},
	}
}

func settings() Settings {
	return Settings{Context: "lab", MinLevel: 5, ExcludeGroups: []string{"sca", "blackwall_sessions"}, Refresh: time.Second}
}

// drain runs cmd and every command nested in a tea.BatchMsg, feeding each
// resulting message back into v. Ticks are disabled in tests (v.tick = nil).
func drain(t *testing.T, v *View, cmd tea.Cmd) {
	t.Helper()
	for steps := 0; cmd != nil && steps < 50; steps++ {
		msg := cmd()
		if b, ok := msg.(tea.BatchMsg); ok {
			for _, c := range b {
				drain(t, v, c)
			}
			return
		}
		if msg == nil {
			return
		}
		_, cmd = v.Update(msg)
	}
}

func newTestView(f *fakeSearcher) *View {
	v := New(f, settings())
	v.tick = func(int) tea.Cmd { return nil }
	_, _ = v.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return v
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestInit_FirstLoadUsesDefaults(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{alert("A", 1, 9, "002", "black-host-01", "Successful login during non-business hours.")}}}
	v := newTestView(f)
	drain(t, v, v.Init())

	q := f.queries[0]
	if q.MinLevel != 5 || q.Size != initialSize || !q.Since.IsZero() || len(q.ExcludeGroups) != 2 || q.AgentID != "" {
		t.Errorf("first query = %+v", q)
	}
	out := v.View()
	for _, want := range []string{"level ≥ 5", "sca, blackwall_sessions", "black-host-01", "Successful login", "17101"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestTick_PrependsKeepsCursorCountsNew(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{
		{alert("B", 2, 7, "002", "h1", "b"), alert("A", 1, 7, "002", "h1", "a")},
		{alert("C", 2, 7, "002", "h1", "c"), alert("B", 2, 7, "002", "h1", "b")}, // B again: gte on the same ms
	}}
	v := newTestView(f)
	drain(t, v, v.Init())
	_, _ = v.Update(key("down")) // cursor on A

	drain(t, v, func() tea.Msg { return tickMsg{gen: v.gen} })

	if q := f.queries[1]; !q.Since.Equal(alert("B", 2, 0, "", "", "").Time) || q.Size != 0 {
		t.Errorf("incremental query = %+v, want Since=B.Time and default size", q)
	}
	if got := ids(v.rows); got != "CBA" {
		t.Fatalf("rows = %s, want CBA", got)
	}
	if v.rows[v.cursor].ID != "A" {
		t.Errorf("cursor moved to %s, want it to stay on A", v.rows[v.cursor].ID)
	}
	if v.newCount != 1 || !strings.Contains(v.View(), "+1 new") {
		t.Errorf("newCount = %d, view:\n%s", v.newCount, v.View())
	}
}

func TestTick_StaleGenerationIgnored(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{alert("A", 1, 7, "002", "h1", "a")}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	_, cmd := v.Update(tickMsg{gen: v.gen - 1})
	if cmd != nil || len(f.queries) != 1 {
		t.Errorf("stale tick must neither query nor re-arm (queries=%d)", len(f.queries))
	}
	_, _ = v.Update(loadedMsg{gen: v.gen - 1, alerts: []indexer.Alert{alert("Z", 9, 9, "", "", "z")}})
	if ids(v.rows) != "A" {
		t.Errorf("stale result applied: %s", ids(v.rows))
	}
}

func TestKeys_LevelGroupsAgentRequery(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{
		alert("B", 2, 7, "002", "black-host-01", "b"),
		alert("A", 1, 7, "000", "black-master-00", "a"),
	}}}
	v := newTestView(f)
	drain(t, v, v.Init())

	drain(t, v, func() tea.Msg { return key("+") })
	if q := f.queries[len(f.queries)-1]; q.MinLevel != 6 {
		t.Errorf("after + MinLevel = %d", q.MinLevel)
	}
	for i := 0; i < 10; i++ {
		_, cmd := v.Update(key("-"))
		drain(t, v, cmd)
	}
	if q := f.queries[len(f.queries)-1]; q.MinLevel != 0 {
		t.Errorf("min level must clamp at 0, got %d", q.MinLevel)
	}
	_, cmd := v.Update(key("s"))
	drain(t, v, cmd)
	if q := f.queries[len(f.queries)-1]; len(q.ExcludeGroups) != 0 {
		t.Errorf("after s ExcludeGroups = %v, want none", q.ExcludeGroups)
	}
	// Agents cycle in name order over agents seen in the feed, then back to all.
	for _, want := range []string{"002", "000", ""} {
		_, cmd = v.Update(key("a"))
		drain(t, v, cmd)
		if q := f.queries[len(f.queries)-1]; q.AgentID != want {
			t.Errorf("agent filter = %q, want %q", q.AgentID, want)
		}
	}
}

func TestFilter_TypingCapturesKeysAndDoesNotQuery(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{
		alert("B", 2, 7, "002", "h1", "Successful login"),
		alert("A", 1, 7, "002", "h1", "New user added"),
	}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	n := len(f.queries)

	_, _ = v.Update(key("/"))
	if !v.CapturesKeys() {
		t.Fatal("view must capture keys while the filter prompt is open")
	}
	for _, r := range "user" {
		_, _ = v.Update(key(string(r)))
	}
	_, _ = v.Update(key("enter"))
	if v.CapturesKeys() {
		t.Error("enter must close the prompt")
	}
	out := v.View()
	if !strings.Contains(out, "New user added") || strings.Contains(out, "Successful login") {
		t.Errorf("filter not applied:\n%s", out)
	}
	if len(f.queries) != n {
		t.Errorf("filtering must not query (queries %d → %d)", n, len(f.queries))
	}
}

func TestShowAgentAlertsMsg_FiltersAndReloads(t *testing.T) {
	f := &fakeSearcher{}
	v := newTestView(f)
	_, cmd := v.Update(app.ShowAgentAlertsMsg{AgentID: "002", AgentName: "black-host-01"})
	drain(t, v, cmd)
	if len(f.queries) == 0 || f.queries[len(f.queries)-1].AgentID != "002" {
		t.Fatalf("queries = %+v", f.queries)
	}
	if !strings.Contains(v.View(), "agent: black-host-01") {
		t.Errorf("filter line must name the agent:\n%s", v.View())
	}
}

func TestNilSearcher_ShowsHint(t *testing.T) {
	v := New(nil, settings())
	v.tick = func(int) tea.Cmd { return nil }
	drain(t, v, v.Init()) // must not panic
	if !strings.Contains(v.View(), "indexer not configured for context lab") {
		t.Errorf("view:\n%s", v.View())
	}
}

func TestError_KeepsRowsAndShowsWarning(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{alert("A", 1, 7, "002", "h1", "a")}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	f.err = errors.New("HTTP 401: Unauthorized")
	drain(t, v, func() tea.Msg { return tickMsg{gen: v.gen} })
	out := v.View()
	if !strings.Contains(out, "⚠ indexer: HTTP 401") || ids(v.rows) != "A" {
		t.Errorf("rows %s, view:\n%s", ids(v.rows), out)
	}
}

func TestRender_LongDescriptionFitsWidth(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{alert("A", 1, 7, "002", "h1", strings.Repeat("x", 300))}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	for _, line := range strings.Split(v.View(), "\n") {
		if w := lipgloss.Width(line); w > 120 {
			t.Errorf("line width %d > 120: %q", w, line)
		}
	}
}

func TestRender_FitsHeight(t *testing.T) {
	var many []indexer.Alert
	for i := 0; i < 100; i++ {
		many = append(many, alert(string(rune('a'+i%26))+string(rune('A'+i/26)), 100-i, 7, "002", "h1", "d"))
	}
	f := &fakeSearcher{results: [][]indexer.Alert{many}}
	v := newTestView(f)
	drain(t, v, v.Init())
	if h := lipgloss.Height(v.View()); h > 30 {
		t.Errorf("view height %d > 30", h)
	}
}
```

Run: `go test ./internal/views/alerts/` — expected: compile errors (`undefined: New`, `tickMsg`, `loadedMsg`).

- [ ] **Step 3: Styles**

In `internal/ui/styles/styles.go`, next to `RowDown`:

```go
	// LevelCritical marks alerts of level 12 and above.
	LevelCritical = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
```

- [ ] **Step 4: Implement the view**

`view.go`:

```go
// Package alerts implements the `:alerts` view: a live feed of Wazuh alerts
// from the indexer, with an alert card on Enter.
package alerts

import (
	"context"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
)

type Searcher interface {
	SearchAlerts(ctx context.Context, q indexer.AlertQuery) ([]indexer.Alert, error)
}

type Settings struct {
	Context       string
	MinLevel      int
	ExcludeGroups []string
	Refresh       time.Duration
}

// SearcherFor builds the indexer client for a context. It returns an untyped
// nil (never a nil *indexer.Client wrapped in the interface) when the context
// has no indexer section, so `searcher == nil` checks work.
func SearcherFor(sub *config.IndexerSubsystem) (Searcher, string, error) {
	if sub == nil {
		return nil, "", nil
	}
	c, err := indexer.New(*sub)
	if err != nil {
		return nil, "", err
	}
	return c, c.Addr(), nil
}

// tickMsg and loadedMsg carry the generation that issued them; a filter
// change or re-activation bumps gen, so older ticks and results are dropped.
// The view uses its own tick type: tickcmd.TickMsg goes to whichever view is
// active and would let another view's timer drive this one.
type tickMsg struct{ gen int }

type loadedMsg struct {
	gen         int
	incremental bool
	alerts      []indexer.Alert
	err         error
}

type View struct {
	searcher Searcher
	st       Settings

	rows      []indexer.Alert // newest first
	cursor    int
	offset    int // first visible row
	minLevel  int
	excludeOn bool
	agentID   string
	agentName string
	filter    string
	filtering bool
	paused    bool
	newCount  int
	err       string
	gen       int
	width     int
	height    int
	card      *card // non-nil while the alert card is open (Task 9)

	tick func(gen int) tea.Cmd // replaced in tests
}

func New(s Searcher, st Settings) *View {
	v := &View{}
	v.Replace(s, st)
	return v
}

// Replace swaps the searcher and settings (context switch) and drops all state.
func (v *View) Replace(s Searcher, st Settings) {
	if st.Refresh <= 0 {
		st.Refresh = 3 * time.Second
	}
	*v = View{searcher: s, st: st, minLevel: st.MinLevel, excludeOn: true,
		width: v.width, height: v.height, gen: v.gen + 1}
	v.tick = func(gen int) tea.Cmd {
		return tea.Tick(v.st.Refresh, func(time.Time) tea.Msg { return tickMsg{gen: gen} })
	}
}

func (v *View) Name() string { return ":alerts" }

func (v *View) CapturesKeys() bool { return v.filtering }

func (v *View) Hints() []footer.Hint {
	if v.card != nil {
		return v.card.hints()
	}
	return []footer.Hint{
		{Key: "↵", Desc: "details"}, {Key: "+/-", Desc: "level"}, {Key: "s", Desc: "groups"},
		{Key: "a", Desc: "agent"}, {Key: "/", Desc: "filter"}, {Key: "p", Desc: "pause"},
	}
}

// Init (re)loads from scratch whenever the view is activated.
func (v *View) Init() tea.Cmd { return v.reload() }

func (v *View) reload() tea.Cmd {
	v.gen++
	v.err = ""
	if v.searcher == nil {
		return nil
	}
	return tea.Batch(v.load(false), v.tick(v.gen))
}

func (v *View) query(incremental bool) indexer.AlertQuery {
	q := indexer.AlertQuery{MinLevel: v.minLevel, AgentID: v.agentID}
	if v.excludeOn {
		q.ExcludeGroups = v.st.ExcludeGroups
	}
	if incremental && len(v.rows) > 0 {
		q.Since = v.rows[0].Time
	} else {
		q.Size = initialSize
	}
	return q
}

func (v *View) load(incremental bool) tea.Cmd {
	s, q, gen := v.searcher, v.query(incremental), v.gen
	inc := incremental && len(v.rows) > 0
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		alerts, err := s.SearchAlerts(ctx, q)
		return loadedMsg{gen: gen, incremental: inc, alerts: alerts, err: err}
	}
}

func (v *View) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width, v.height = msg.Width, msg.Height
		if v.card != nil {
			v.card.setSize(msg.Width, msg.Height)
		}
		return v, nil

	case app.ShowAgentAlertsMsg:
		v.agentID, v.agentName = msg.AgentID, msg.AgentName
		v.card = nil
		return v, v.reload()

	case tickMsg:
		if msg.gen != v.gen {
			return v, nil
		}
		if v.paused || v.searcher == nil {
			return v, v.tick(v.gen)
		}
		return v, tea.Batch(v.load(true), v.tick(v.gen))

	case loadedMsg:
		if msg.gen != v.gen {
			return v, nil
		}
		if msg.err != nil {
			v.err = msg.err.Error()
			return v, nil
		}
		v.err = ""
		if !msg.incremental {
			v.rows, _ = merge(nil, msg.alerts, maxRows)
			v.cursor, v.offset, v.newCount = 0, 0, 0
			return v, nil
		}
		var added int
		v.rows, added = merge(v.rows, msg.alerts, maxRows)
		if v.cursor > 0 {
			v.cursor = min(v.cursor+added, len(v.rows)-1)
			v.newCount += added
		}
		return v, nil

	case tea.KeyMsg:
		if v.card != nil {
			if msg.String() == "esc" {
				v.card = nil
				return v, nil
			}
			v.card.handleKey(msg)
			return v, nil
		}
		if v.filtering {
			v.filterKey(msg)
			return v, nil
		}
		return v, v.key(msg)
	}
	return v, nil
}

func (v *View) filterKey(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyEnter:
		v.filtering = false
	case tea.KeyEsc:
		v.filtering, v.filter = false, ""
	case tea.KeyBackspace:
		if r := []rune(v.filter); len(r) > 0 {
			v.filter = string(r[:len(r)-1])
		}
	case tea.KeyRunes, tea.KeySpace:
		v.filter += string(msg.Runes)
	}
	v.cursor, v.offset = 0, 0
}

func (v *View) key(msg tea.KeyMsg) tea.Cmd {
	rows := v.visible()
	switch msg.String() {
	case "up", "k":
		if v.cursor > 0 {
			v.cursor--
		}
		if v.cursor == 0 {
			v.newCount = 0
		}
	case "down", "j":
		if v.cursor < len(rows)-1 {
			v.cursor++
		}
	case "enter":
		if len(rows) > 0 {
			v.card = newCard(rows[v.cursor], v.width, v.height)
		}
	case "+", "=":
		if v.minLevel < 15 {
			v.minLevel++
			return v.reload()
		}
	case "-":
		if v.minLevel > 0 {
			v.minLevel--
			return v.reload()
		}
	case "s":
		v.excludeOn = !v.excludeOn
		return v.reload()
	case "a":
		v.nextAgent()
		return v.reload()
	case "/":
		v.filtering, v.filter = true, ""
	case "p":
		v.paused = !v.paused
	case "ctrl+r":
		return v.reload()
	}
	return nil
}

// nextAgent cycles over the agents present in the feed, by name, then "all".
func (v *View) nextAgent() {
	names := map[string]string{}
	for _, r := range v.rows {
		names[r.Agent.ID] = r.Agent.Name
	}
	ids := make([]string, 0, len(names))
	for id := range names {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return names[ids[i]] < names[ids[j]] })
	if v.agentID == "" {
		if len(ids) > 0 {
			v.agentID, v.agentName = ids[0], names[ids[0]]
		}
		return
	}
	for i, id := range ids {
		if id == v.agentID {
			if i+1 < len(ids) {
				v.agentID, v.agentName = ids[i+1], names[ids[i+1]]
			} else {
				v.agentID, v.agentName = "", ""
			}
			return
		}
	}
	v.agentID, v.agentName = "", ""
}
```

Also: when the agent filter is set but the feed is narrowed to that agent, `v.rows` only contains it — cycling continues to "all" and then re-learns all agents on the next load. That is acceptable.

`render.go`:

```go
package alerts

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/table"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

var cols = []table.Col{
	{Title: "TIME", Min: 25},
	{Title: "LVL", Min: 3},
	{Title: "AGENT", Min: 16},
	{Title: "RULE", Min: 7},
	{Title: "DESCRIPTION", Min: 20, Flex: true},
}

// visible returns the rows that pass the local text filter.
func (v *View) visible() []indexer.Alert {
	if v.filter == "" {
		return v.rows
	}
	f := strings.ToLower(v.filter)
	var out []indexer.Alert
	for _, a := range v.rows {
		hay := strings.ToLower(a.Rule.Description + " " + a.Agent.Name + " " + a.Rule.ID + " " + a.FullLog)
		if strings.Contains(hay, f) {
			out = append(out, a)
		}
	}
	return out
}

// fit cuts s to w display cells, ending with "…" when cut.
func fit(s string, w int) string {
	if w <= 0 || lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func levelStyle(l int) lipgloss.Style {
	switch {
	case l >= 12:
		return styles.LevelCritical
	case l >= 7:
		return styles.RowDown
	case l >= 5:
		return styles.RowPending
	}
	return lipgloss.NewStyle()
}

func (v *View) View() string {
	if v.card != nil {
		return v.card.view()
	}
	if v.searcher == nil {
		return fmt.Sprintf("indexer not configured for context %s — add an `indexer:` section (see tools/blackwall/README.md)\n", v.st.Context)
	}

	var b strings.Builder
	groups := "all groups"
	if v.excludeOn && len(v.st.ExcludeGroups) > 0 {
		groups = "hiding " + strings.Join(v.st.ExcludeGroups, ", ")
	}
	agent := "all"
	if v.agentID != "" {
		agent = v.agentName
	}
	rows := v.visible()
	left := fmt.Sprintf("level ≥ %d · %s · agent: %s · last 24h", v.minLevel, groups, agent)
	right := fmt.Sprintf("%d shown", len(rows))
	if v.newCount > 0 {
		right += fmt.Sprintf("  +%d new", v.newCount)
	}
	if v.paused {
		right += "  ⏸ paused"
	}
	pad := max(v.width-lipgloss.Width(left)-lipgloss.Width(right), 2)
	b.WriteString(fit(left+strings.Repeat(" ", pad)+right, v.width) + "\n")
	fixed := 2 // summary + table header

	if v.filtering || v.filter != "" {
		b.WriteString(fit("/"+v.filter, v.width) + "\n")
		fixed++
	}
	if v.err != "" {
		b.WriteString(styles.Error.Render(fit("⚠ indexer: "+v.err, v.width)) + "\n")
		fixed++
	}

	widths := table.Widths(cols, v.width)
	b.WriteString(fit(table.Header(cols, widths), v.width) + "\n")
	if len(rows) == 0 {
		if v.err == "" {
			b.WriteString("no alerts yet\n")
		}
		return b.String()
	}

	space := len(rows)
	if v.height > 0 {
		space = max(v.height-fixed, 1)
	}
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+space {
		v.offset = v.cursor - space + 1
	}
	end := min(v.offset+space, len(rows))
	for i := v.offset; i < end; i++ {
		a := rows[i]
		row := fit(table.Row(widths, []string{
			wazuh.FormatLocalUTC(a.Time, "2006-01-02 15:04", "15:04Z"),
			fmt.Sprintf("%3d", a.Rule.Level),
			a.Agent.Name, a.Rule.ID, a.Rule.Description,
		}), v.width)
		if i == v.cursor {
			row = styles.SelectedRow.Render(row)
		} else {
			row = levelStyle(a.Rule.Level).Render(row)
		}
		b.WriteString(row + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
```

Temporary stub for the card (replaced in Task 9) — `card.go`:

```go
package alerts

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
)

type card struct{ alert indexer.Alert }

func newCard(a indexer.Alert, width, height int) *card { return &card{alert: a} }
func (c *card) setSize(width, height int)               {}
func (c *card) handleKey(tea.KeyMsg)                    {}
func (c *card) hints() []footer.Hint                    { return []footer.Hint{{Key: "esc", Desc: "back"}} }
func (c *card) view() string                            { return c.alert.Rule.Description }
```

Run: `go test ./internal/views/alerts/` — expected: PASS. Fix the implementation (not the tests) until it does.

- [ ] **Step 5: Wire it into `main.go`**

In `cmd/blackwall/main.go`:

```go
	// after the Wazuh client is built:
	idx, idxAddr, err := alertsview.SearcherFor(currentCtx.Indexer)
	if err != nil {
		return err
	}
	alertsSettings := func(name string, sub *config.IndexerSubsystem) alertsview.Settings {
		st := alertsview.Settings{Context: name, MinLevel: config.DefaultMinLevel,
			ExcludeGroups: config.DefaultExcludeGroups(), Refresh: cfg.UI.RefreshInterval}
		if sub != nil {
			st.MinLevel = *sub.Alerts.MinLevel
			st.ExcludeGroups = sub.Alerts.ExcludeGroups
		}
		return st
	}
	alertsView := alertsview.New(idx, alertsSettings(cfg.Current, currentCtx.Indexer))
	registered := []views.View{ctxView, agentsList, alertsView}
	liveIndexer := idxAddr
```

(import `alertsview "github.com/greengrunt/blackwall/tools/blackwall/internal/views/alerts"`; replace the existing `registered :=` line.)

In `switchContext`, after the new Wazuh client is built and before `return`:

```go
			newIdx, newIdxAddr, err := alertsview.SearcherFor(ctx.Indexer)
			if err != nil {
				return app.ContextSwitchedMsg{Name: name, Err: err}
			}
			alertsView.Replace(newIdx, alertsSettings(name, ctx.Indexer))
			liveIndexer = newIdxAddr
```

In the `Status` func add `Indexer: liveIndexer,`.

Run: `go build ./cmd/blackwall && go vet ./...` — expected: success.

- [ ] **Step 6: Try it on the lab**

Requires Task 5 (indexer published). Run:

```bash
make build
tmux new-session -d -s bwt -x 140 -y 40 "bin/blackwall --context lab"; sleep 3
tmux send-keys -t bwt 3; sleep 3; tmux capture-pane -pt bwt
```

Expected: tab `[3] :alerts` active, status line has `indexer: 127.0.0.1:9200`, rows with level ≥ 5 and no SCA (`CIS Ubuntu` must not appear). Then `../../tests/triggers/bw-trigger user` from the repo root and within ~10 s capture again — a new row on top. Note: your `~/.config/blackwall/config.yaml` needs an `indexer:` block in its `lab` context (copy it from `internal/config/template.yaml`) — tell the owner in the artifact if you added it. `tmux kill-session -t bwt` afterwards.

- [ ] **Step 7: Commit**

`git branch --show-current` → `claude-dev`:

```bash
git add internal/views/alerts internal/ui/styles cmd/blackwall
git commit -m "feat(blackwall): :alerts live feed from the wazuh indexer"
```

Artifact `.../alerts-feed.md` (include the tmux captures), committed.

---

### Task 9: Alert card

**Files:**
- Modify (replace stub): `tools/blackwall/internal/views/alerts/card.go`
- Test: `tools/blackwall/internal/views/alerts/card_test.go`

**Interfaces:**
- Consumes: `scroll.List` (Task 2), `indexer.Alert` (Task 4), `wazuh.FormatLocalUTC`.
- Produces: same unexported API as the stub: `newCard(a, width, height) *card`, `setSize`, `handleKey`, `hints`, `view`.

- [ ] **Step 1: Failing tests**

`card_test.go`:

```go
package alerts

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
)

func richAlert() indexer.Alert {
	return indexer.Alert{
		ID:   "1791935883.1688070",
		Time: time.Date(2026, 10, 13, 23, 58, 3, 0, time.UTC),
		Rule: indexer.Rule{
			ID: "17101", Level: 9, Description: "Successful login during non-business hours.",
			Groups: []string{"policy_violation", "login_time"}, FiredTimes: 8,
			MITRE:  indexer.MITRE{ID: []string{"T1078"}, Tactic: []string{"Initial Access"}, Technique: []string{"Valid Accounts"}},
			PCIDSS: []string{"10.2.5"},
		},
		Agent:    indexer.AgentRef{ID: "002", Name: "black-host-01"},
		Data:     map[string]any{"srcip": "172.28.0.1", "dstuser": "root", "win": map[string]any{"eventdata": map[string]any{"user": "x"}}},
		FullLog:  "Oct 13 23:58:03 black-host-01 sshd[6900]: " + strings.Repeat("long ", 60),
		Location: "/var/log/auth.log",
	}
}

func TestCard_ShowsAllSections(t *testing.T) {
	out := newCard(richAlert(), 100, 0).view()
	for _, want := range []string{
		"[9] Successful login during non-business hours.",
		"17101", "policy_violation, login_time", "fired 8×",
		"Initial Access", "Valid Accounts", "T1078",
		"black-host-01 (002)", "/var/log/auth.log",
		"srcip", "172.28.0.1", "win.eventdata.user",
		"sshd[6900]", "PCI DSS", "10.2.5",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestCard_WrapsToWidthAndFitsHeight(t *testing.T) {
	c := newCard(richAlert(), 60, 12)
	out := c.view()
	for _, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("line width %d > 60: %q", w, line)
		}
	}
	if h := lipgloss.Height(out); h > 12 {
		t.Errorf("height %d > 12", h)
	}
	before := out
	c.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if c.view() == before {
		t.Error("G must scroll to the bottom")
	}
}

func TestView_EnterOpensCardEscCloses(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{richAlert()}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	_, _ = v.Update(key("enter"))
	if !strings.Contains(v.View(), "PCI DSS") {
		t.Fatalf("card not shown:\n%s", v.View())
	}
	_, _ = v.Update(key("esc"))
	if strings.Contains(v.View(), "PCI DSS") {
		t.Error("esc must close the card")
	}
}
```

Run: `go test ./internal/views/alerts/ -run Card` — expected: FAIL (stub renders only the description).

- [ ] **Step 2: Implement**

Replace `card.go`:

```go
package alerts

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/scroll"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// card is the scrollable detail screen for one alert.
type card struct {
	alert indexer.Alert
	width int
	sl    scroll.List
}

func newCard(a indexer.Alert, width, height int) *card {
	c := &card{alert: a, sl: scroll.List{FooterRows: 1}}
	c.setSize(width, height)
	return c
}

func (c *card) setSize(width, height int) {
	c.width = width
	c.sl.SetSize(height)
}

func (c *card) handleKey(msg tea.KeyMsg) { c.sl.HandleKey(msg, len(c.lines())) }

func (c *card) hints() []footer.Hint {
	return []footer.Hint{{Key: "esc", Desc: "back"}, {Key: "j/k", Desc: "scroll"}, {Key: "g/G", Desc: "top/bottom"}}
}

func (c *card) view() string {
	lines := c.lines()
	return strings.TrimSuffix(c.sl.Render("", lines, c.sl.Indicator(len(lines))+"\n"), "\n")
}

// wrap splits s into chunks of at most w runes (w <= 0: no wrapping).
func wrap(s string, w int) []string {
	if w <= 0 {
		return []string{s}
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		r := []rune(para)
		for len(r) > w {
			out = append(out, string(r[:w]))
			r = r[w:]
		}
		out = append(out, string(r))
	}
	return out
}

// flatten turns nested data into sorted "a.b.c: value" lines.
func flatten(prefix string, v any, out map[string]string) {
	if m, ok := v.(map[string]any); ok {
		for k, child := range m {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flatten(key, child, out)
		}
		return
	}
	out[prefix] = fmt.Sprint(v)
}

func (c *card) lines() []string {
	a := c.alert
	w := c.width
	var l []string
	add := func(s string) { l = append(l, wrap(s, w)...) }
	section := func(title string) {
		l = append(l, "", styles.HeaderText.Render(title), strings.Repeat("─", min(len(title), max(w, 1))))
	}

	add(levelStyle(a.Rule.Level).Render(fmt.Sprintf("[%d] %s", a.Rule.Level, a.Rule.Description)))
	add(wazuh.FormatLocalUTC(a.Time, "2006-01-02 15:04:05 MST", "2006-01-02 15:04:05 UTC"))

	section("Rule")
	add(fmt.Sprintf("id %s · level %d · fired %d×", a.Rule.ID, a.Rule.Level, a.Rule.FiredTimes))
	add("groups: " + strings.Join(a.Rule.Groups, ", "))

	if m := a.Rule.MITRE; len(m.ID)+len(m.Tactic)+len(m.Technique) > 0 {
		section("MITRE ATT&CK")
		add("tactic:    " + strings.Join(m.Tactic, ", "))
		add("technique: " + strings.Join(m.Technique, ", "))
		add("id:        " + strings.Join(m.ID, ", "))
	}

	section("Source")
	add(fmt.Sprintf("agent:    %s (%s)", a.Agent.Name, a.Agent.ID))
	add("location: " + a.Location)

	if len(a.Data) > 0 {
		section("Data")
		flat := map[string]string{}
		flatten("", a.Data, flat)
		keys := make([]string, 0, len(flat))
		for k := range flat {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			add(k + ": " + flat[k])
		}
	}

	if a.FullLog != "" {
		section("Full log")
		add(a.FullLog)
	}

	comp := []struct {
		name string
		vals []string
	}{{"PCI DSS", a.Rule.PCIDSS}, {"GDPR", a.Rule.GDPR}, {"NIST 800-53", a.Rule.NIST}, {"HIPAA", a.Rule.HIPAA}, {"TSC", a.Rule.TSC}}
	first := true
	for _, cv := range comp {
		if len(cv.vals) == 0 {
			continue
		}
		if first {
			section("Compliance")
			first = false
		}
		add(cv.name + ": " + strings.Join(cv.vals, ", "))
	}
	return l
}
```

Run: `go test ./internal/views/alerts/` — expected: PASS.

- [ ] **Step 3: Try it, commit**

Lab check as in Task 8 Step 6, then `Enter` on a row and capture: the card shows Rule/MITRE/Source/Data/Full log, `esc` returns.
`git branch --show-current` → `claude-dev`:

```bash
git add internal/views/alerts
git commit -m "feat(blackwall): alert card with rule, MITRE, data and full log"
```

Artifact `.../alert-card.md`, committed.

---

### Task 10: Sessions tab and `l` jump in the agent card

**Files:**
- Create: `tools/blackwall/internal/views/agents/sessions.go`
- Test: `tools/blackwall/internal/views/agents/sessions_test.go`
- Modify: `tools/blackwall/internal/views/agents/{detail.go,view.go,subview.go}`, every `NewDetail(` call site (15, incl. tests)
- Modify: `tools/blackwall/cmd/blackwall/main.go`

**Interfaces:**
- Consumes: `indexer.AlertQuery/Alert` (Task 4); `sessions.Parse/Session` (Task 7); `scroll.List` (Task 2); `app.ShowAgentAlertsMsg` (Task 1); `alerts.Searcher`-shaped interface (declared locally to avoid importing the alerts view).
- Produces:

```go
package agents
type AlertSearcher interface {
	SearchAlerts(ctx context.Context, q indexer.AlertQuery) ([]indexer.Alert, error)
}
func NewDetail(c Client, idx AlertSearcher, id string) *DetailModel // idx may be nil
func (v *View) SetIndexer(idx AlertSearcher)                        // context switch
func NewSessions(idx AlertSearcher, agentID string) *SessionsModel
```

Detail tabs become `Overview, Hardware, Top, Packages, Network, Security, Sessions` with keys `O H T K N S E`; `tabIndex["e"] = 6`. Key `l` in the detail returns a command producing `app.ShowAgentAlertsMsg{AgentID, AgentName}`.

- [ ] **Step 1: Failing tests**

`sessions_test.go`:

```go
package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
)

const snapshot = `ossec: output: 'blackwall-sessions':
root     pts/1        10.0.0.5         2026-10-14T02:40:00+00:00   still logged in
alice    pts/0        172.28.0.1       2026-10-13T23:13:01+00:00 - 2026-10-13T23:13:05+00:00  (00:00)
reboot   system boot  7.0.14-orbstack- 2026-10-13T23:09:49+00:00   still running
`

type fakeIdx struct{ queries []indexer.AlertQuery }

func (f *fakeIdx) SearchAlerts(_ context.Context, q indexer.AlertQuery) ([]indexer.Alert, error) {
	f.queries = append(f.queries, q)
	if q.RuleID == "100100" {
		return []indexer.Alert{{RawTime: "x", Time: time.Date(2026, 10, 14, 2, 49, 0, 0, time.UTC), FullLog: snapshot}}, nil
	}
	// Suspicious-login lookup: a level-9 alert for alice 30 s after her login.
	return []indexer.Alert{{
		Time: time.Date(2026, 10, 13, 23, 13, 31, 0, time.UTC),
		Rule: indexer.Rule{Level: 9}, Data: map[string]any{"dstuser": "alice"},
	}}, nil
}

func runCmd(t *testing.T, m *SessionsModel, cmd tea.Cmd) {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			return
		}
		var next subView
		next, cmd = m.Update(msg)
		m = next.(*SessionsModel)
	}
}

func TestSessions_ActiveFirstMarkedHistoryAfter(t *testing.T) {
	f := &fakeIdx{}
	m := NewSessions(f, "002")
	m.now = func() time.Time { return time.Date(2026, 10, 14, 2, 49, 0, 0, time.UTC) }
	runCmd(t, m, m.Init())

	if q := f.queries[0]; q.RuleID != "100100" || q.AgentID != "002" || q.Size != 1 || q.MinLevel != 0 || len(q.ExcludeGroups) != 0 {
		t.Errorf("snapshot query = %+v", q)
	}
	out := m.View()
	root := strings.Index(out, "root")
	alice := strings.Index(out, "alice")
	if root < 0 || alice < 0 || root > alice {
		t.Fatalf("active root must come before history:\n%s", out)
	}
	for _, want := range []string{"still logged in", "9m", "10.0.0.5", "⚠", "snapshot"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "reboot") {
		t.Errorf("reboot pseudo-sessions must be hidden:\n%s", out)
	}
}

func TestSessions_NoSnapshotAndNoIndexer(t *testing.T) {
	m := NewSessions(nil, "002")
	runCmd(t, m, m.Init())
	if !strings.Contains(m.View(), "indexer not configured") {
		t.Errorf("nil indexer view:\n%s", m.View())
	}

	empty := NewSessions(emptyIdx{}, "002")
	runCmd(t, empty, empty.Init())
	if !strings.Contains(empty.View(), "no session snapshot yet") {
		t.Errorf("empty view:\n%s", empty.View())
	}
}

type emptyIdx struct{}

func (emptyIdx) SearchAlerts(context.Context, indexer.AlertQuery) ([]indexer.Alert, error) {
	return nil, nil
}

func TestDetail_SessionsTabAndAlertsJump(t *testing.T) {
	d := NewDetail(&fakeClient{}, &fakeIdx{}, "002")
	d.SetDetail(wazuhDetail("002", "black-host-01"))
	_, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if d.activeTab != 6 || !strings.Contains(d.View(), "[E] Sessions") {
		t.Fatalf("tab = %d, view:\n%s", d.activeTab, d.View())
	}
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if cmd == nil {
		t.Fatal("l must emit a command")
	}
	got, ok := cmd().(app.ShowAgentAlertsMsg)
	if !ok || got.AgentID != "002" || got.AgentName != "black-host-01" {
		t.Errorf("l emitted %#v", got)
	}
}
```

Add to the same file the helper used above (adjust to the `wazuh.AgentDetail` shape):

```go
func wazuhDetail(id, name string) wazuh.AgentDetail {
	return wazuh.AgentDetail{Agent: wazuh.Agent{ID: id, Name: name, Status: "active"}}
}
```

(import `"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"`.)

Run: `go test ./internal/views/agents/ -run 'Sessions|AlertsJump'` — expected: compile errors (`NewSessions`, 3-arg `NewDetail`).

- [ ] **Step 2: Sessions sub-view**

`sessions.go`:

```go
package agents

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/sessions"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/scroll"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/table"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// AlertSearcher is the indexer capability the agent card needs; nil when the
// context has no indexer.
type AlertSearcher interface {
	SearchAlerts(ctx context.Context, q indexer.AlertQuery) ([]indexer.Alert, error)
}

// sessionsRuleID is the manager rule that stores `last` snapshots
// (blackwall-eye role wazuh-manager, blackwall_rules.xml).
const sessionsRuleID = "100100"

type sessionsLoadedMsg struct {
	snapshot   time.Time
	rows       []sessions.Session
	suspicious map[int]bool
	none       bool
	err        error
}

type SessionsModel struct {
	idx        AlertSearcher
	agentID    string
	now        func() time.Time
	loaded     bool
	none       bool
	err        string
	snapshot   time.Time
	rows       []sessions.Session // active first, then newest first; raw rows last
	suspicious map[int]bool       // index into rows
	sl         scroll.List
	width      int
}

func NewSessions(idx AlertSearcher, agentID string) *SessionsModel {
	return &SessionsModel{idx: idx, agentID: agentID, now: time.Now, sl: scroll.List{HeaderRows: 4, FooterRows: 1}}
}

func (m *SessionsModel) SetSize(width, height int) { m.width = width; m.sl.SetSize(height) }

func (m *SessionsModel) Hints() []footer.Hint {
	return []footer.Hint{{Key: "j/k", Desc: "scroll"}, {Key: "l", Desc: "alerts"}}
}

func (m *SessionsModel) Init() tea.Cmd {
	if m.idx == nil {
		return nil
	}
	idx, id := m.idx, m.agentID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		snaps, err := idx.SearchAlerts(ctx, indexer.AlertQuery{RuleID: sessionsRuleID, AgentID: id, Size: 1})
		if err != nil {
			return sessionsLoadedMsg{err: err}
		}
		if len(snaps) == 0 {
			return sessionsLoadedMsg{none: true}
		}
		var rows []sessions.Session
		for _, s := range sessions.Parse(snaps[0].FullLog) {
			if !s.IsReboot() {
				rows = append(rows, s)
			}
		}
		sort.SliceStable(rows, func(i, j int) bool {
			if (rows[i].Raw == "") != (rows[j].Raw == "") {
				return rows[i].Raw == ""
			}
			return rows[i].Active && !rows[j].Active
		})
		return sessionsLoadedMsg{snapshot: snaps[0].Time, rows: rows,
			suspicious: markSuspicious(ctx, idx, id, rows)}
	}
}

// markSuspicious flags logins that coincide (±1 min, same user) with an
// alert of level 7 or higher on the agent, e.g. 17101 "login during
// non-business hours". Lookup failures leave everything unmarked.
func markSuspicious(ctx context.Context, idx AlertSearcher, agentID string, rows []sessions.Session) map[int]bool {
	var oldest time.Time
	for _, s := range rows {
		if s.Raw == "" && (oldest.IsZero() || s.Login.Before(oldest)) {
			oldest = s.Login
		}
	}
	out := map[int]bool{}
	if oldest.IsZero() {
		return out
	}
	alerts, err := idx.SearchAlerts(ctx, indexer.AlertQuery{AgentID: agentID, MinLevel: 7, Since: oldest.Add(-time.Minute)})
	if err != nil {
		return out
	}
	for i, s := range rows {
		for _, a := range alerts {
			user, _ := a.Data["dstuser"].(string)
			d := a.Time.Sub(s.Login)
			if s.Raw == "" && user == s.User && d >= -time.Minute && d <= time.Minute {
				out[i] = true
			}
		}
	}
	return out
}

func (m *SessionsModel) Update(msg tea.Msg) (subView, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsLoadedMsg:
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err, m.none = "", msg.none
		m.snapshot, m.rows, m.suspicious = msg.snapshot, msg.rows, msg.suspicious
		m.sl.Reset()
	case tea.KeyMsg:
		m.sl.HandleKey(msg, len(m.rows))
	}
	return m, nil
}

func humanDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "—"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
}

var sessionCols = []table.Col{
	{Title: "USER", Min: 10}, {Title: "TTY", Min: 7}, {Title: "FROM", Min: 16},
	{Title: "LOGIN", Min: 25}, {Title: "LOGOUT", Min: 25}, {Title: "DURATION", Min: 9, Flex: true},
}

func (m *SessionsModel) View() string {
	switch {
	case m.idx == nil:
		return "indexer not configured for this context — Sessions need the Wazuh indexer (see tools/blackwall/README.md)\n"
	case !m.loaded:
		return "loading sessions…\n"
	case m.err != "":
		return styles.Error.Render("⚠ indexer: "+m.err) + "\n"
	case m.none:
		return "no session snapshot yet — appears after the next login/logout (rule 100100)\n"
	}
	const local, utc = "2006-01-02 15:04", "15:04Z"
	widths := table.Widths(sessionCols, m.width)
	header := fmt.Sprintf("Sessions — snapshot %s\n\n", wazuh.FormatLocalUTC(m.snapshot, local, utc)) +
		table.Header(sessionCols, widths) + "\n" + strings.Repeat("─", max(m.width, 40)) + "\n"

	now := m.now()
	rows := make([]string, 0, len(m.rows))
	for i, s := range m.rows {
		if s.Raw != "" {
			rows = append(rows, styles.Dim.Render(s.Raw))
			continue
		}
		user := s.User
		if m.suspicious[i] {
			user = "⚠ " + user
		}
		logout := wazuh.FormatLocalUTC(s.Logout, local, utc)
		switch {
		case s.Active:
			logout = "still logged in"
		case s.End != "":
			logout = s.End
		}
		dur := humanDuration(s.Duration(now))
		if s.Active {
			dur += " ●"
		}
		row := table.Row(widths, []string{user, s.TTY, s.From, wazuh.FormatLocalUTC(s.Login, local, utc), logout, dur})
		if s.Active {
			row = styles.Active.Render(row)
		} else if m.suspicious[i] {
			row = styles.RowDown.Render(row)
		}
		rows = append(rows, row)
	}
	return m.sl.Render(header, rows, m.sl.Indicator(len(rows))+"\n")
}
```

Add to `subview.go`'s assertions block:

```go
	_ sizedView    = (*SessionsModel)(nil)
	_ hintProvider = (*SessionsModel)(nil)
```

- [ ] **Step 3: Wire the tab and `l` into the detail**

In `detail.go`:
- `detailTabs` → append `"Sessions"`; `detailTabKeys` → append `"E"`; `tabIndex` → add `"e": 6`.
- `NewDetail(c Client, id string)` → `NewDetail(c Client, idx AlertSearcher, id string)`; in its `subs` map add `6: NewSessions(idx, id),`.
- In `Update`, inside `case tea.KeyMsg:` next to `case "r":`:

```go
		case "l":
			id, name := m.agentID, m.detail.Name
			return m, func() tea.Msg { return app.ShowAgentAlertsMsg{AgentID: id, AgentName: name} }
```

- In `Hints()` add `{Key: "l", Desc: "alerts"}` and change the tabs hint to `o/h/t/k/n/s/e`.

In `view.go`:
- add field `idx AlertSearcher` to `View`;
- `func (v *View) SetIndexer(idx AlertSearcher) { v.idx = idx; v.detail = nil }`;
- `NewDetail(v.list.client, sd.AgentID)` → `NewDetail(v.list.client, v.idx, sd.AgentID)`.

Update every other `NewDetail(` call (tests): `NewDetail(&fakeClient{}, "001")` → `NewDetail(&fakeClient{}, nil, "001")` — `grep -rn "NewDetail(" internal/` to find all 15.

- [ ] **Step 4: `main.go`**

After `agentsList := agentsview.NewView(...)`:

```go
	if idx != nil {
		agentsList.SetIndexer(idx)
	}
```

In `switchContext`, after `alertsView.Replace(...)`:

```go
			if newIdx != nil {
				agentsList.SetIndexer(newIdx)
			} else {
				agentsList.SetIndexer(nil)
			}
```

(`idx`/`newIdx` are `alertsview.Searcher`; both interfaces have the same method set, so the assignment compiles. The explicit nil branch keeps an untyped nil.)

- [ ] **Step 5: Run everything**

Run: `go vet ./... && go test ./...` — expected: PASS.

- [ ] **Step 6: Try it on the lab**

```bash
make build
tests/bin/bw-lab ssh black-host-01 'sleep 120' &   # from the repo root: an open session
sleep 70
tmux new-session -d -s bwt -x 140 -y 40 "tools/blackwall/bin/blackwall --context lab"; sleep 3
tmux send-keys -t bwt Down Down Enter; sleep 2; tmux send-keys -t bwt e; sleep 3
tmux capture-pane -pt bwt
```

Expected: `[E] Sessions` active, a `root … 172.28.0.1 … still logged in … ●` row on top, history below. Then `tmux send-keys -t bwt l; sleep 3; tmux capture-pane -pt bwt` — `:alerts` with `agent: black-host-01`. Kill the session and the background ssh afterwards. (Agent IDs/order in the list may differ; select `black-host-01`.)

- [ ] **Step 7: Commit**

`git branch --show-current` → `claude-dev`:

```bash
git add internal/views/agents cmd/blackwall
git commit -m "feat(blackwall): Sessions tab and l jump to agent alerts"
```

Artifact `.../sessions-tab.md` (with captures), committed.

---

### Task 11: Documentation and end-to-end acceptance

**Files:**
- Modify: `tools/blackwall/README.md`, `tools/blackwall/docs/ARCHITECTURE.md`, `tests/README.md` (if anything is still missing), `blackwall-eye/STATUS.md`

- [ ] **Step 1: TUI README**

In `tools/blackwall/README.md`:
- "Configure" example: add the `indexer:` block (as in `internal/config/template.yaml`) and a sentence: optional; without it `:alerts` and Sessions show a hint.
- Keys table: add `l` (agent card → its alerts).
- New section `## Alerts` with the key table from the spec §5 (`↑↓/jk`, `↵`, `+/-`, `s`, `a`, `/`, `p`, `ctrl+r`), the default filters (`min_level: 5`, `exclude_groups: [sca, blackwall_sessions]`) and the 24 h / 200 / 500 limits.
- Agent detail table: add `| Sessions | e | Who is logged in now and recent logins (from the agent's `last`, rule 100100); ⚠ marks logins that coincided with a level ≥ 7 alert |`.
- Test environment section: the `lab` context now includes `indexer:` on `127.0.0.1:9200`.

- [ ] **Step 2: Architecture**

In `tools/blackwall/docs/ARCHITECTURE.md`, layers list: add
`internal/indexer` (Wazuh indexer/OpenSearch alert search), `internal/sessions` (`last` parser),
`internal/httpclient` (shared TLS client), `internal/ui/scroll` (scroll lists),
`internal/views/alerts` (`:alerts`). Add a paragraph "Keys and ticks": views implementing
`views.KeyCapturer` receive every key while capturing; views needing a private timer use their own
tick message with a generation counter (as `:alerts` does) instead of `tickcmd.TickMsg`.

- [ ] **Step 3: End-to-end acceptance on a fresh lab**

From the repo root:

```bash
tests/bin/bw-lab reset
tests/bin/bw-check all            # Expect: 0 failed
```

Then, with tmux as in Tasks 8 and 10:
1. `blackwall --context lab`, `3` → alerts ≥ 5, no `CIS Ubuntu` rows; `tests/triggers/bw-trigger user` → new row on top within ~10 s; `Enter` → card; `esc`.
2. `tests/bin/bw-lab ssh black-host-01 'sleep 120' &`, wait 70 s, agent card of `black-host-01` → `e` → session `still logged in`; after the ssh exits and ~70 s, reopen (`esc`, `Enter`, `e`) → the session moved to history with a duration.
3. `/` in `:alerts`, type `q`, `1`, `p` → they appear in the prompt; the TUI neither quits nor switches tabs.

Paste the captures into the artifact.

- [ ] **Step 4: STATUS and commit**

`blackwall-eye/STATUS.md`: in the 2026-10-14 `Done` entry add a line about the TUI `:alerts` and Sessions consuming rule 100100; set `next_task` to the previous next task (port `hosts/example.com` nftables into the role).

`git branch --show-current` → `claude-dev`:

```bash
git add tools/blackwall/README.md tools/blackwall/docs/ARCHITECTURE.md tests/README.md blackwall-eye/STATUS.md
git commit -m "docs(blackwall): alerts feed, sessions tab, indexer config"
```

Artifact `.../acceptance.md` (captures from Step 3), committed.
