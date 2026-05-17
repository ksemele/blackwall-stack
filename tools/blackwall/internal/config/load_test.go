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
