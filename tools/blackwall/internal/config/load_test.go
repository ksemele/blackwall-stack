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
