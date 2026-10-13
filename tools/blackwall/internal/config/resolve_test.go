package config

import (
	"testing"
	"time"
)

func TestResolve_CLIBeatsConfig(t *testing.T) {
	cfg := Config{
		Current:  "in-file",
		Contexts: map[string]Context{"in-file": {}, "from-cli": {}},
	}
	got, err := Resolve(cfg, Overrides{Context: "from-cli", ContextWasSet: true}, EnvValues{})
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
		Current:  "in-file",
		Contexts: map[string]Context{"in-file": {}, "from-env": {}},
		UI:       UI{RefreshInterval: 5 * time.Second},
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
	cfg := Config{
		Current:  "in-file",
		Contexts: map[string]Context{"in-file": {}, "from-cli": {}, "from-env": {}},
	}
	got, err := Resolve(cfg, Overrides{Context: "from-cli", ContextWasSet: true}, ReadEnv())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Current != "from-cli" {
		t.Errorf("Current = %q, want from-cli", got.Current)
	}
}

func TestResolve_ExplicitContextDisablesPicker(t *testing.T) {
	cfg := Config{
		Current:              "x",
		SelectContextOnStart: true,
		Contexts:             map[string]Context{"x": {}},
	}
	got, err := Resolve(cfg, Overrides{Context: "x", ContextWasSet: true}, EnvValues{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.SelectContextOnStart {
		t.Errorf("explicit --context should disable picker")
	}
}

func TestResolve_NoSelectContextFlag(t *testing.T) {
	cfg := Config{
		Current:              "x",
		SelectContextOnStart: true,
		Contexts:             map[string]Context{"x": {}},
	}
	no := false
	got, err := Resolve(cfg, Overrides{SelectContextOnStart: &no}, EnvValues{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.SelectContextOnStart {
		t.Errorf("--no-select-context should disable picker")
	}
}
