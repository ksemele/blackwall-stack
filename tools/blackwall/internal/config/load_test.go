package config

import (
	"path/filepath"
	"strings"
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

func TestSetCurrent_UpdatesField(t *testing.T) {
	content := `current: old-ctx
contexts:
  old-ctx:
    description: old
  new-ctx:
    description: new
`
	path := writeTempFile(t, "config.yaml", content)

	if err := SetCurrent(path, "new-ctx"); err != nil {
		t.Fatalf("SetCurrent: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load after SetCurrent: %v", err)
	}
	if cfg.Current != "new-ctx" {
		t.Errorf("Current = %q, want new-ctx", cfg.Current)
	}
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
	raw, err := Load(writeTempFile(t, "config.yaml", indexerYAML))
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
	raw, err := Load(writeTempFile(t, "config.yaml", body))
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
	raw, _ := Load(writeTempFile(t, "config.yaml", strings.Replace(indexerYAML, "      user: admin\n", "", 1)))
	cfg, _ := ExpandEnv(ApplyDefaults(raw))
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "contexts.lab.indexer") {
		t.Errorf("Validate err = %v, want contexts.lab.indexer error", err)
	}
}

func TestIndexer_ValidateLevelRange(t *testing.T) {
	t.Setenv("BW_TEST_IDX_PW", "x")
	raw, _ := Load(writeTempFile(t, "config.yaml", indexerYAML+"      alerts:\n        min_level: 16\n"))
	cfg, _ := ExpandEnv(ApplyDefaults(raw))
	if err := Validate(cfg); err == nil {
		t.Error("min_level 16 must be rejected (Wazuh levels are 0–15)")
	}
}
